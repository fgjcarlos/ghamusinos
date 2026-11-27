package jobs

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/riverqueue/river"

	"github.com/fgjcarlos/ghamusinos/internal/config"
	"github.com/fgjcarlos/ghamusinos/internal/db/sqlc"
	"github.com/fgjcarlos/ghamusinos/internal/reports"
	"github.com/fgjcarlos/ghamusinos/internal/workouts"
)

// ReportFanoutArgs triggers the report fan-out over every user with activities.
type ReportFanoutArgs struct{}

// Kind is the River job-kind identifier.
func (ReportFanoutArgs) Kind() string { return string(KindReportFanout) }

// GenerateReportArgs generates the reports of one user: the closed periods on a
// schedule tick (bare UserID), or exactly the period an on-demand request names.
type GenerateReportArgs struct {
	UserID string `json:"user_id"`
	// ReportKind, Date and BlockID carry an explicit on-demand request (#320).
	// An empty ReportKind keeps the scheduled behaviour: the last closed week
	// and, on the user's local 1st, the closed previous month. The field is not
	// named Kind because Kind() is already the River job kind.
	ReportKind string `json:"kind,omitempty"`
	Date       string `json:"date,omitempty"`
	BlockID    string `json:"block_id,omitempty"`
}

// Kind is the River job-kind identifier.
func (GenerateReportArgs) Kind() string { return string(KindGenerateReport) }

// ReportUserLister lists the users the fan-out iterates; *sqlc.Queries
// satisfies it through ListUserIDsForTrainingLoadRecalc.
type ReportUserLister interface {
	ListUserIDsForTrainingLoadRecalc(ctx context.Context) ([]pgtype.UUID, error)
}

// ReportGenerator is the generator subset the per-user job needs;
// *reports.Store satisfies it.
type ReportGenerator interface {
	GenerateWeek(ctx context.Context, user uuid.UUID, week time.Time) (reports.Snapshot, error)
	GenerateMonth(ctx context.Context, user uuid.UUID, month time.Time) (reports.Snapshot, error)
	GenerateBlock(ctx context.Context, user, blockID uuid.UUID, now time.Time) (reports.Snapshot, error)
}

// UserTimezoneQuerier resolves users.timezone; *sqlc.Queries satisfies it.
type UserTimezoneQuerier interface {
	GetUserPreferencesByID(ctx context.Context, id pgtype.UUID) (sqlc.GetUserPreferencesByIDRow, error)
}

// ReportPeriodicJobs returns the periodic jobs of the reports feature (fase
// 2.4, #319): one fan-out that enqueues a generate_report per user. River
// v0.51 has no cron expression, so the schedule is the fixed
// REPORTS_SCHEDULE_INTERVAL. With REPORTS_SCHEDULE_ENABLED off nothing is
// registered.
func ReportPeriodicJobs(cfg *config.Config) []*river.PeriodicJob {
	if cfg == nil || !cfg.Reports.ScheduleEnabled {
		return nil
	}
	interval := cfg.Reports.ScheduleInterval
	if interval <= 0 {
		interval = config.DefaultReportScheduleInterval
	}
	return []*river.PeriodicJob{
		river.NewPeriodicJob(
			river.PeriodicInterval(interval),
			func() (river.JobArgs, *river.InsertOpts) { return &ReportFanoutArgs{}, nil },
			&river.PeriodicJobOpts{ID: "report_fanout"},
		),
	}
}

// ReportFanoutWorker enqueues one generate_report job per user with
// activities. ByArgs+ByPeriod collapse a repeated tick into one pending job
// per user.
type ReportFanoutWorker struct {
	river.WorkerDefaults[ReportFanoutArgs]

	users    ReportUserLister
	inserter RiverJobInserter
	period   time.Duration
}

// NewReportFanoutWorker builds a fan-out worker. inserter may be nil: the
// worker is registered before the River client exists, so it then takes the
// live client from the job context. period is the uniqueness window.
func NewReportFanoutWorker(users ReportUserLister, inserter RiverJobInserter, period time.Duration) *ReportFanoutWorker {
	return &ReportFanoutWorker{users: users, inserter: inserter, period: period}
}

// Work enqueues one generate_report per user.
func (w *ReportFanoutWorker) Work(ctx context.Context, _ *river.Job[ReportFanoutArgs]) error {
	inserter := w.inserter
	if inserter == nil {
		client, err := river.ClientFromContextSafely[pgx.Tx](ctx)
		if err != nil {
			return fmt.Errorf("report_fanout: river client: %w", err)
		}
		inserter = client
	}
	users, err := w.users.ListUserIDsForTrainingLoadRecalc(ctx)
	if err != nil {
		return fmt.Errorf("report_fanout: list users: %w", err)
	}
	for _, id := range users {
		args := &GenerateReportArgs{UserID: uuid.UUID(id.Bytes).String()}
		if _, err := inserter.Insert(ctx, args, &river.InsertOpts{
			UniqueOpts: river.UniqueOpts{ByArgs: true, ByPeriod: w.period}}); err != nil {
			return fmt.Errorf("report_fanout: enqueue user %s: %w", args.UserID, err)
		}
	}
	return nil
}

// GenerateReportWorker generates the closed reports of one user: the last
// Monday–Sunday week that ended before the user's local today and, on the
// local 1st, the previous month. Both are idempotent through the snapshot
// upsert, so a repeated tick never duplicates rows.
type GenerateReportWorker struct {
	river.WorkerDefaults[GenerateReportArgs]

	store ReportGenerator
	prefs UserTimezoneQuerier
	now   func() time.Time
}

// NewGenerateReportWorker builds a per-user worker. now can be nil (time.Now).
func NewGenerateReportWorker(store ReportGenerator, prefs UserTimezoneQuerier, now func() time.Time) *GenerateReportWorker {
	if now == nil {
		now = time.Now
	}
	return &GenerateReportWorker{store: store, prefs: prefs, now: now}
}

// Work generates the period an on-demand request names or, without one, the
// closed reports of the user: the last Monday–Sunday week that ended before the
// user's local today and, on the local 1st, the previous month. Both are
// idempotent through the snapshot upsert, so a repeated tick never duplicates
// rows.
func (w *GenerateReportWorker) Work(ctx context.Context, job *river.Job[GenerateReportArgs]) error {
	user, err := uuid.Parse(job.Args.UserID)
	if err != nil {
		return fmt.Errorf("generate_report: parse user_id %q: %w", job.Args.UserID, err)
	}
	loc, err := w.location(ctx, user)
	if err != nil {
		return err
	}
	if job.Args.ReportKind != "" {
		return w.generateRequested(ctx, user, loc, job.Args)
	}
	now := w.now()
	// The last closed week ends the day before the current week starts, so
	// any day of it names it: on a Monday, yesterday's Sunday.
	weekStart, _ := reports.WeekPeriod(now, loc)
	if _, err := w.store.GenerateWeek(ctx, user, weekStart.AddDate(0, 0, -1)); err != nil {
		return fmt.Errorf("generate_report: week: %w", err)
	}
	// The month only closes once the user's local date enters the next one.
	if now.In(loc).Day() == 1 {
		if _, err := w.store.GenerateMonth(ctx, user, now.In(loc).AddDate(0, 0, -1)); err != nil {
			return fmt.Errorf("generate_report: month: %w", err)
		}
	}
	return nil
}

// generateRequested writes the snapshot of exactly the period an on-demand
// request names (#320), so a past week, a month or a block exists without
// waiting for the schedule. The date is a civil date in the user's zone, which
// keeps a Monday its own week's Monday on both sides of UTC.
func (w *GenerateReportWorker) generateRequested(ctx context.Context, user uuid.UUID, loc *time.Location, args GenerateReportArgs) error {
	switch reports.Kind(args.ReportKind) {
	case reports.KindWeek, reports.KindMonth:
		date, err := time.ParseInLocation(time.DateOnly, args.Date, loc)
		if err != nil || date.Format(time.DateOnly) != args.Date {
			return fmt.Errorf("generate_report: date %q must use YYYY-MM-DD", args.Date)
		}
		if reports.Kind(args.ReportKind) == reports.KindWeek {
			if _, err := w.store.GenerateWeek(ctx, user, date); err != nil {
				return fmt.Errorf("generate_report: week: %w", err)
			}
			return nil
		}
		if _, err := w.store.GenerateMonth(ctx, user, date); err != nil {
			return fmt.Errorf("generate_report: month: %w", err)
		}
		return nil
	case reports.KindBlock:
		block, err := uuid.Parse(args.BlockID)
		if err != nil {
			return fmt.Errorf("generate_report: parse block_id %q: %w", args.BlockID, err)
		}
		if _, err := w.store.GenerateBlock(ctx, user, block, w.now()); err != nil {
			// The block was deleted (or handed to another user) between the
			// enqueue and the work: there is nothing left to report on, so this
			// is not a retryable failure.
			if errors.Is(err, reports.ErrNotFound) {
				return nil
			}
			return fmt.Errorf("generate_report: block: %w", err)
		}
		return nil
	default:
		return fmt.Errorf("generate_report: unknown kind %q", args.ReportKind)
	}
}

// location resolves the user's saved time zone, falling back to UTC when the
// user has no preferences row.
func (w *GenerateReportWorker) location(ctx context.Context, user uuid.UUID) (*time.Location, error) {
	prefs, err := w.prefs.GetUserPreferencesByID(ctx, pgtype.UUID{Bytes: user, Valid: true})
	if errors.Is(err, pgx.ErrNoRows) {
		return time.UTC, nil
	}
	if err != nil {
		return nil, fmt.Errorf("generate_report: user preferences: %w", err)
	}
	return workouts.UserLocation(prefs.Timezone), nil
}
