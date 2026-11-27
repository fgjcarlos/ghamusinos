package jobs

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/riverqueue/river"
	"github.com/stretchr/testify/require"

	"github.com/fgjcarlos/ghamusinos/internal/config"
	"github.com/fgjcarlos/ghamusinos/internal/db/sqlc"
	"github.com/fgjcarlos/ghamusinos/internal/reports"
)

// TestReportPeriodicJobs covers S4: the weekly cron is registered only when
// REPORTS_SCHEDULE_ENABLED is on.
func TestReportPeriodicJobs(t *testing.T) {
	enabled := ReportPeriodicJobs(&config.Config{Reports: config.ReportsConfig{
		ScheduleEnabled: true, ScheduleInterval: time.Hour}})
	require.Len(t, enabled, 1)

	disabled := ReportPeriodicJobs(&config.Config{Reports: config.ReportsConfig{
		ScheduleEnabled: false, ScheduleInterval: time.Hour}})
	require.Empty(t, disabled)
}

// fakeReportGenerator records the periods the per-user job asks for.
type fakeReportGenerator struct {
	weeks    []time.Time
	months   []time.Time
	blocks   []uuid.UUID
	blockErr error
}

func (f *fakeReportGenerator) GenerateWeek(_ context.Context, _ uuid.UUID, week time.Time) (reports.Snapshot, error) {
	f.weeks = append(f.weeks, week)
	return reports.Snapshot{}, nil
}

func (f *fakeReportGenerator) GenerateMonth(_ context.Context, _ uuid.UUID, month time.Time) (reports.Snapshot, error) {
	f.months = append(f.months, month)
	return reports.Snapshot{}, nil
}

func (f *fakeReportGenerator) GenerateBlock(_ context.Context, _ uuid.UUID, block uuid.UUID, _ time.Time) (reports.Snapshot, error) {
	f.blocks = append(f.blocks, block)
	return reports.Snapshot{}, f.blockErr
}

type fakePreferences struct{ timezone string }

func (f fakePreferences) GetUserPreferencesByID(context.Context, pgtype.UUID) (sqlc.GetUserPreferencesByIDRow, error) {
	return sqlc.GetUserPreferencesByIDRow{Timezone: f.timezone}, nil
}

// TestGenerateReportWorker_ClosedPeriods covers S4: the per-user job generates
// the last Monday–Sunday week that ended before the user's local today, plus
// the previous month on the 1st, computed in the user's zone (Europe/Madrid is
// one or two hours ahead of UTC, so every case below has a different UTC date).
func TestGenerateReportWorker_ClosedPeriods(t *testing.T) {
	madrid, err := time.LoadLocation("Europe/Madrid")
	require.NoError(t, err)

	for _, tc := range []struct {
		name       string
		now        time.Time
		wantWeek   [2]string
		wantMonth  [2]string
		monthCalls int
	}{
		{
			// 00:30 on Monday in Madrid is still Sunday in UTC.
			name:       "monday at 00:30 local",
			now:        time.Date(2026, 3, 1, 23, 30, 0, 0, time.UTC),
			wantWeek:   [2]string{"2026-02-23", "2026-03-01"},
			monthCalls: 0,
		},
		{
			name:       "mid-week",
			now:        time.Date(2026, 3, 4, 8, 0, 0, 0, time.UTC),
			wantWeek:   [2]string{"2026-02-23", "2026-03-01"},
			monthCalls: 0,
		},
		{
			// 00:30 on the 1st in Madrid is still the last day of the
			// previous month in UTC: the month only closes in the user's zone.
			name:       "first of the month at 00:30 local",
			now:        time.Date(2026, 3, 31, 22, 30, 0, 0, time.UTC),
			wantWeek:   [2]string{"2026-03-23", "2026-03-29"},
			wantMonth:  [2]string{"2026-03-01", "2026-03-31"},
			monthCalls: 1,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			gen := &fakeReportGenerator{}
			worker := NewGenerateReportWorker(gen, fakePreferences{timezone: "Europe/Madrid"},
				func() time.Time { return tc.now })
			user := uuid.New()

			err := worker.Work(context.Background(), &river.Job[GenerateReportArgs]{
				Args: GenerateReportArgs{UserID: user.String()}})
			require.NoError(t, err)

			require.Len(t, gen.weeks, 1)
			start, end := reports.WeekPeriod(gen.weeks[0], madrid)
			require.Equal(t, tc.wantWeek[0], start.Format(time.DateOnly))
			require.Equal(t, tc.wantWeek[1], end.Format(time.DateOnly))

			require.Len(t, gen.months, tc.monthCalls)
			if tc.monthCalls == 1 {
				start, end := reports.MonthPeriod(gen.months[0], madrid)
				require.Equal(t, tc.wantMonth[0], start.Format(time.DateOnly))
				require.Equal(t, tc.wantMonth[1], end.Format(time.DateOnly))
			}
		})
	}
}

// TestGenerateReportWorker_ExplicitRequest covers S5 (#320): a job enqueued by
// the on-demand endpoint generates exactly the period it names, not the closed
// schedule, in the user's zone.
func TestGenerateReportWorker_ExplicitRequest(t *testing.T) {
	madrid, err := time.LoadLocation("Europe/Madrid")
	require.NoError(t, err)
	gen := &fakeReportGenerator{}
	// The clock is far from the requested periods: a job that falls back to the
	// schedule would ask for other dates.
	worker := NewGenerateReportWorker(gen, fakePreferences{timezone: "Europe/Madrid"},
		func() time.Time { return time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC) })
	user := uuid.New()
	run := func(args GenerateReportArgs) error {
		t.Helper()
		args.UserID = user.String()
		return worker.Work(context.Background(), &river.Job[GenerateReportArgs]{Args: args})
	}

	// A past week: any day inside it names it.
	require.NoError(t, run(GenerateReportArgs{ReportKind: "week", Date: "2026-02-05"}))
	require.Len(t, gen.weeks, 1)
	start, end := reports.WeekPeriod(gen.weeks[0], madrid)
	require.Equal(t, "2026-02-02", start.Format(time.DateOnly))
	require.Equal(t, "2026-02-08", end.Format(time.DateOnly))
	require.Empty(t, gen.months)
	require.Empty(t, gen.blocks)

	// A month.
	require.NoError(t, run(GenerateReportArgs{ReportKind: "month", Date: "2026-02-17"}))
	require.Len(t, gen.months, 1)
	start, end = reports.MonthPeriod(gen.months[0], madrid)
	require.Equal(t, "2026-02-01", start.Format(time.DateOnly))
	require.Equal(t, "2026-02-28", end.Format(time.DateOnly))
	require.Len(t, gen.weeks, 1)

	// A block: exactly that block, through the store's block report.
	block := uuid.New()
	require.NoError(t, run(GenerateReportArgs{ReportKind: "block", BlockID: block.String()}))
	require.Equal(t, []uuid.UUID{block}, gen.blocks)
	require.Len(t, gen.weeks, 1)

	// The date is a civil date in the user's zone: a Monday stays the Monday of
	// its own week.
	require.NoError(t, run(GenerateReportArgs{ReportKind: "week", Date: "2026-02-02"}))
	start, _ = reports.WeekPeriod(gen.weeks[1], madrid)
	require.Equal(t, "2026-02-02", start.Format(time.DateOnly))

	// A malformed request is an error, not a fallback to the schedule.
	require.Error(t, run(GenerateReportArgs{ReportKind: "year", Date: "2026-02-05"}))
	require.Error(t, run(GenerateReportArgs{ReportKind: "week", Date: "2026-2-5"}))
	require.Error(t, run(GenerateReportArgs{ReportKind: "block", BlockID: "not-a-uuid"}))
	require.Len(t, gen.weeks, 2)
	require.Len(t, gen.months, 1)
	require.Len(t, gen.blocks, 1)

	// Without a request the scheduled behaviour is unchanged: the closed week.
	open := &fakeReportGenerator{}
	scheduled := NewGenerateReportWorker(open, fakePreferences{timezone: "Europe/Madrid"},
		func() time.Time { return time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC) })
	require.NoError(t, scheduled.Work(context.Background(), &river.Job[GenerateReportArgs]{
		Args: GenerateReportArgs{UserID: user.String()}}))
	require.Len(t, open.weeks, 1)
	start, end = reports.WeekPeriod(open.weeks[0], madrid)
	require.Equal(t, "2026-05-25", start.Format(time.DateOnly))
	require.Equal(t, "2026-05-31", end.Format(time.DateOnly))
	require.Empty(t, open.blocks)
}

// TestGenerateReportWorker_DeletedBlockIsNoOp covers the on-demand block request
// whose block was deleted between the enqueue and the work: there is nothing
// left to report on, so the job succeeds instead of retrying forever. Any other
// failure and any malformed request still fail.
func TestGenerateReportWorker_DeletedBlockIsNoOp(t *testing.T) {
	block := uuid.New()
	gone := &fakeReportGenerator{blockErr: reports.ErrNotFound}
	worker := NewGenerateReportWorker(gone, fakePreferences{timezone: "UTC"}, nil)
	require.NoError(t, worker.Work(context.Background(), &river.Job[GenerateReportArgs]{
		Args: GenerateReportArgs{UserID: uuid.New().String(), ReportKind: "block", BlockID: block.String()}}))
	require.Equal(t, []uuid.UUID{block}, gone.blocks, "the request still reached the generator")

	broken := &fakeReportGenerator{blockErr: errors.New("boom")}
	worker = NewGenerateReportWorker(broken, fakePreferences{timezone: "UTC"}, nil)
	err := worker.Work(context.Background(), &river.Job[GenerateReportArgs]{
		Args: GenerateReportArgs{UserID: uuid.New().String(), ReportKind: "block", BlockID: block.String()}})
	require.ErrorContains(t, err, "boom")

	require.Error(t, worker.Work(context.Background(), &river.Job[GenerateReportArgs]{
		Args: GenerateReportArgs{UserID: uuid.New().String(), ReportKind: "block", BlockID: "not-a-uuid"}}))
	require.Empty(t, broken.blocks[1:])
}

// fakeReportUsers lists the users of the fan-out.
type fakeReportUsers struct{ ids []pgtype.UUID }

func (f fakeReportUsers) ListUserIDsForTrainingLoadRecalc(context.Context) ([]pgtype.UUID, error) {
	return f.ids, nil
}

// TestReportFanoutWorker_EnqueuesOneJobPerUser covers S4: one generate_report
// per user, deduplicated for the fan-out interval, and none without users.
func TestReportFanoutWorker_EnqueuesOneJobPerUser(t *testing.T) {
	interval := 6 * time.Hour
	first, second := uuid.New(), uuid.New()
	capt := &enqueueCapturingInserter{}
	worker := NewReportFanoutWorker(fakeReportUsers{ids: []pgtype.UUID{
		{Bytes: first, Valid: true},
		{Bytes: second, Valid: true},
	}}, capt, interval)

	require.NoError(t, worker.Work(context.Background(), &river.Job[ReportFanoutArgs]{}))
	require.Len(t, capt.Args, 2)
	for i, want := range []uuid.UUID{first, second} {
		args, ok := capt.Args[i].(*GenerateReportArgs)
		require.True(t, ok, "job %d has type %T", i, capt.Args[i])
		require.Equal(t, want.String(), args.UserID)
		require.NotNil(t, capt.Opts[i])
		require.True(t, capt.Opts[i].UniqueOpts.ByArgs)
		require.Equal(t, interval, capt.Opts[i].UniqueOpts.ByPeriod)
	}

	empty := &enqueueCapturingInserter{}
	require.NoError(t, NewReportFanoutWorker(fakeReportUsers{}, empty, interval).
		Work(context.Background(), &river.Job[ReportFanoutArgs]{}))
	require.Empty(t, empty.Args)
}

// S4: sin cliente de River el fan-out falla en voz alta en vez de saltear usuarios.
func TestReportFanoutWorker_MissingClientFails(t *testing.T) {
	w := NewReportFanoutWorker(fakeReportUsers{ids: []pgtype.UUID{{Bytes: uuid.New(), Valid: true}}}, nil, time.Hour)
	err := w.Work(context.Background(), &river.Job[ReportFanoutArgs]{})
	require.ErrorContains(t, err, "river client")
}
