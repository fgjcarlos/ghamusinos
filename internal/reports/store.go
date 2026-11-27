package reports

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/fgjcarlos/ghamusinos/internal/db/sqlc"
	"github.com/fgjcarlos/ghamusinos/internal/workouts"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

var ErrNotFound = errors.New("report, training block or race not found")

// sessionLimit bounds the planned sessions a report reads; one year of daily
// sessions fits comfortably.
const sessionLimit = 10000

// Store generates and reads report snapshots. Generation is idempotent: the
// UNIQUE (user_id, kind, period_start) of the table replaces the row.
type Store struct{ q sqlc.Querier }

func NewStore(q sqlc.Querier) *Store { return &Store{q: q} }

// Record is a stored snapshot: its row and the payload it holds.
type Record struct {
	ID          uuid.UUID  `json:"id"`
	Kind        Kind       `json:"kind"`
	PeriodStart string     `json:"period_start"`
	PeriodEnd   string     `json:"period_end"`
	BlockID     *uuid.UUID `json:"block_id"`
	Payload     Snapshot   `json:"payload"`
	GeneratedAt time.Time  `json:"generated_at"`
}

// Summary is the compact view of a stored payload that the reports list
// carries, so the list does not need the whole payload per row.
type Summary struct {
	TSS            float64  `json:"tss"`
	Activities     int      `json:"activities"`
	DistanceM      float64  `json:"distance_m"`
	ElevationGainM float64  `json:"elevation_gain_m"`
	AvgEF          *float64 `json:"avg_ef"`
	AvgGAPSecPerKm *float64 `json:"avg_gap_sec_per_km"`
	Adherence      *float64 `json:"adherence"`
}

// Summary extracts the list view of this payload.
func (s Snapshot) Summary() Summary {
	return Summary{TSS: s.Load.TSS, Activities: s.Volume.Activities, DistanceM: s.Volume.DistanceM,
		ElevationGainM: s.Volume.ElevationGainM, AvgEF: s.Performance.AvgEF,
		AvgGAPSecPerKm: s.Performance.AvgGAPSecPerKm, Adherence: s.Plan.Adherence}
}

// Item is one row of the reports list.
type Item struct {
	ID          uuid.UUID  `json:"id"`
	Kind        Kind       `json:"kind"`
	PeriodStart string     `json:"period_start"`
	PeriodEnd   string     `json:"period_end"`
	BlockID     *uuid.UUID `json:"block_id"`
	GeneratedAt time.Time  `json:"generated_at"`
	Summary     Summary    `json:"summary"`
}

// Filter narrows the reports list: an optional kind and an optional window the
// report must overlap. A zero value leaves that side open.
type Filter struct {
	Kind Kind
	From time.Time
	To   time.Time
}

// GenerateInput names the period an on-demand generation asks for: any day
// inside it for week and month, or the block for block.
type GenerateInput struct {
	Kind    Kind   `json:"kind"`
	Date    string `json:"date"`
	BlockID string `json:"block_id"`
}

// Period is the period a generate request names, computed as the generator does.
type Period struct {
	Kind        Kind   `json:"kind"`
	PeriodStart string `json:"period_start"`
	PeriodEnd   string `json:"period_end"`
}

// GenerateWeek builds and stores the report of the week that contains week, in
// the user's time zone.
func (s *Store) GenerateWeek(ctx context.Context, user uuid.UUID, week time.Time) (Snapshot, error) {
	loc, err := s.location(ctx, user)
	if err != nil {
		return Snapshot{}, err
	}
	start, end := WeekPeriod(week, loc)
	return s.generate(ctx, user, loc, Input{Kind: KindWeek, Now: time.Now(), Location: loc, PeriodStart: start, PeriodEnd: end})
}

// GenerateMonth builds and stores the report of the month that contains month.
func (s *Store) GenerateMonth(ctx context.Context, user uuid.UUID, month time.Time) (Snapshot, error) {
	loc, err := s.location(ctx, user)
	if err != nil {
		return Snapshot{}, err
	}
	start, end := MonthPeriod(month, loc)
	return s.generate(ctx, user, loc, Input{Kind: KindMonth, Now: time.Now(), Location: loc, PeriodStart: start, PeriodEnd: end})
}

// GenerateBlock builds and stores the report of a block of the user, from its
// start date to its end date. A missing or foreign block answers ErrNotFound.
func (s *Store) GenerateBlock(ctx context.Context, user, blockID uuid.UUID, now time.Time) (Snapshot, error) {
	loc, err := s.location(ctx, user)
	if err != nil {
		return Snapshot{}, err
	}
	block, err := s.q.GetTrainingBlock(ctx, sqlc.GetTrainingBlockParams{UserID: dbID(user), ID: dbID(blockID)})
	if err != nil {
		return Snapshot{}, mapMissing(err)
	}
	in := Input{Kind: KindBlock, Now: now, Location: loc, Block: &block,
		PeriodStart: block.StartDate.Time, PeriodEnd: block.EndDate.Time}
	if block.RaceID.Valid {
		race, err := s.q.GetRace(ctx, sqlc.GetRaceParams{UserID: dbID(user), ID: block.RaceID})
		if err != nil {
			return Snapshot{}, mapMissing(err)
		}
		in.Race = &race
	}
	return s.generate(ctx, user, loc, in)
}

// List returns one page of the user's reports, newest period first, and the
// total matching the same filter.
func (s *Store) List(ctx context.Context, user uuid.UUID, f Filter, limit, offset int32) ([]Item, int64, error) {
	from, to := optDay(f.From), optDay(f.To)
	rows, err := s.q.ListReportSnapshots(ctx, sqlc.ListReportSnapshotsParams{UserID: dbID(user),
		Kind: string(f.Kind), FromDate: from, ToDate: to, Limit: limit, Offset: offset})
	if err != nil {
		return nil, 0, fmt.Errorf("reports: list snapshots: %w", err)
	}
	total, err := s.q.CountReportSnapshots(ctx, sqlc.CountReportSnapshotsParams{UserID: dbID(user),
		Kind: string(f.Kind), FromDate: from, ToDate: to})
	if err != nil {
		return nil, 0, fmt.Errorf("reports: count snapshots: %w", err)
	}
	data := make([]Item, 0, len(rows))
	for _, row := range rows {
		var payload Snapshot
		if err := json.Unmarshal(row.Payload, &payload); err != nil {
			return nil, 0, fmt.Errorf("reports: decode payload: %w", err)
		}
		data = append(data, Item{ID: uuidOf(row.ID), Kind: Kind(row.Kind),
			PeriodStart: row.PeriodStart.Time.Format(time.DateOnly), PeriodEnd: row.PeriodEnd.Time.Format(time.DateOnly),
			BlockID: fromOptID(row.BlockID), GeneratedAt: row.GeneratedAt.Time, Summary: payload.Summary()})
	}
	return data, total, nil
}

// Period resolves the period an on-demand generation names, computed exactly as
// the generator does. A kind or date that is not valid answers ErrInvalid; a
// missing or foreign block answers ErrNotFound.
func (s *Store) Period(ctx context.Context, user uuid.UUID, in GenerateInput) (Period, error) {
	loc, err := s.location(ctx, user)
	if err != nil {
		return Period{}, err
	}
	var start, end time.Time
	switch in.Kind {
	case KindWeek, KindMonth:
		// The day is a civil date in the user's zone, so its own week and month
		// stay the same on both sides of UTC.
		date, err := time.ParseInLocation(time.DateOnly, in.Date, loc)
		if err != nil || date.Format(time.DateOnly) != in.Date || date.Year() < 1 {
			return Period{}, ErrInvalid
		}
		if in.Kind == KindWeek {
			start, end = WeekPeriod(date, loc)
		} else {
			start, end = MonthPeriod(date, loc)
		}
	case KindBlock:
		id, err := uuid.Parse(in.BlockID)
		if err != nil {
			return Period{}, ErrInvalid
		}
		block, err := s.q.GetTrainingBlock(ctx, sqlc.GetTrainingBlockParams{UserID: dbID(user), ID: dbID(id)})
		if err != nil {
			return Period{}, mapMissing(err)
		}
		start, end = block.StartDate.Time, block.EndDate.Time
	default:
		return Period{}, ErrInvalid
	}
	return Period{Kind: in.Kind, PeriodStart: start.Format(time.DateOnly), PeriodEnd: end.Format(time.DateOnly)}, nil
}

// Get returns a snapshot of the user with its stored payload. A missing or
// foreign report answers ErrNotFound.
func (s *Store) Get(ctx context.Context, user, id uuid.UUID) (Record, error) {
	row, err := s.q.GetReportSnapshot(ctx, sqlc.GetReportSnapshotParams{UserID: dbID(user), ID: dbID(id)})
	if err != nil {
		return Record{}, mapMissing(err)
	}
	var payload Snapshot
	if err := json.Unmarshal(row.Payload, &payload); err != nil {
		return Record{}, fmt.Errorf("reports: decode payload: %w", err)
	}
	return Record{ID: uuidOf(row.ID), Kind: Kind(row.Kind), PeriodStart: row.PeriodStart.Time.Format(time.DateOnly),
		PeriodEnd: row.PeriodEnd.Time.Format(time.DateOnly), BlockID: fromOptID(row.BlockID), Payload: payload,
		GeneratedAt: row.GeneratedAt.Time}, nil
}

// generate loads the user's inputs scoped to that user, builds the report and
// upserts it, so regenerating a period replaces it instead of duplicating it.
func (s *Store) generate(ctx context.Context, user uuid.UUID, loc *time.Location, in Input) (Snapshot, error) {
	start, end := workouts.LocalDate(in.PeriodStart, loc), workouts.LocalDate(in.PeriodEnd, loc)
	prevStart, _ := PreviousPeriod(in.Kind, start, end)
	load, err := s.q.ListTrainingLoadFromFirstActivity(ctx, dbID(user))
	if err != nil {
		return Snapshot{}, fmt.Errorf("reports: load series: %w", err)
	}
	// The UTC window is padded by two days on each side; Build keeps only the
	// activities of the user's civil dates (same pattern as workouts.Compare).
	activities, err := s.q.ListActivitiesInRange(ctx, sqlc.ListActivitiesInRangeParams{UserID: dbID(user),
		StartedAt:   pgtype.Timestamptz{Time: prevStart.AddDate(0, 0, -2), Valid: true},
		StartedAt_2: pgtype.Timestamptz{Time: end.AddDate(0, 0, 2), Valid: true}})
	if err != nil {
		return Snapshot{}, fmt.Errorf("reports: activities: %w", err)
	}
	sessions, err := s.q.ListPlannedSessionsByRange(ctx, sqlc.ListPlannedSessionsByRangeParams{UserID: dbID(user),
		FromDate: day(prevStart), ToDate: day(end), Limit: sessionLimit})
	if err != nil {
		return Snapshot{}, fmt.Errorf("reports: planned sessions: %w", err)
	}
	in.Load, in.Activities, in.Sessions = load, activities, sessions
	snapshot, err := Build(in)
	if err != nil {
		return Snapshot{}, err
	}
	payload, err := json.Marshal(snapshot)
	if err != nil {
		return Snapshot{}, fmt.Errorf("reports: encode payload: %w", err)
	}
	if _, err := s.q.UpsertReportSnapshot(ctx, sqlc.UpsertReportSnapshotParams{UserID: dbID(user),
		Kind: string(in.Kind), PeriodStart: day(start), PeriodEnd: day(end), BlockID: blockRef(in.Block), Payload: payload}); err != nil {
		return Snapshot{}, fmt.Errorf("reports: store snapshot: %w", err)
	}
	return snapshot, nil
}

// location resolves the user's saved time zone, falling back to UTC when the
// user has no preferences row.
func (s *Store) location(ctx context.Context, user uuid.UUID) (*time.Location, error) {
	prefs, err := s.q.GetUserPreferencesByID(ctx, dbID(user))
	if errors.Is(err, pgx.ErrNoRows) {
		return time.UTC, nil
	}
	if err != nil {
		return nil, fmt.Errorf("reports: user preferences: %w", err)
	}
	return workouts.UserLocation(prefs.Timezone), nil
}

// blockRef maps the block of a block report to the row's nullable block_id.
func blockRef(block *sqlc.TrainingBlock) pgtype.UUID {
	if block == nil {
		return pgtype.UUID{}
	}
	return block.ID
}

func mapMissing(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	return err
}

func dbID(id uuid.UUID) pgtype.UUID { return pgtype.UUID{Bytes: id, Valid: true} }

func day(t time.Time) pgtype.Date { return pgtype.Date{Time: t, Valid: true} }

// optDay maps a zero time to a NULL date, which leaves that filter side open.
func optDay(t time.Time) pgtype.Date {
	if t.IsZero() {
		return pgtype.Date{}
	}
	return day(t)
}

func fromOptID(id pgtype.UUID) *uuid.UUID {
	if !id.Valid {
		return nil
	}
	value := uuidOf(id)
	return &value
}
