package workouts

import (
	"context"
	"errors"
	"math"
	"testing"
	"time"

	"github.com/fgjcarlos/ghamusinos/internal/db/sqlc"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

// GetUserPreferencesByID y ListActivitiesInRange extienden el mockQuerier
// de store_test.go para el endpoint de comparacion (#312): zona horaria
// del usuario y actividades reales del rango.
func (m *mockQuerier) GetUserPreferencesByID(_ context.Context, id pgtype.UUID) (sqlc.GetUserPreferencesByIDRow, error) {
	m.prefsID = id
	if m.prefsErr != nil {
		return sqlc.GetUserPreferencesByIDRow{}, m.prefsErr
	}
	return m.prefs, nil
}

func (m *mockQuerier) ListActivitiesInRange(_ context.Context, arg sqlc.ListActivitiesInRangeParams) ([]sqlc.ListActivitiesInRangeRow, error) {
	m.activityArgs = arg
	if m.activityErr != nil {
		return nil, m.activityErr
	}
	return m.activityRows, nil
}

// compareSessionRow construye una fila del listado con objetivos
// opcionales (nil = no planificado).
func compareSessionRow(id uuid.UUID, userID uuid.UUID, day time.Time, status Status, durationSec, distanceM, gainM *int32) sqlc.ListPlannedSessionsByRangeRow {
	i4 := func(v *int32) pgtype.Int4 {
		if v == nil {
			return pgtype.Int4{}
		}
		return pgtype.Int4{Int32: *v, Valid: true}
	}
	return sqlc.ListPlannedSessionsByRangeRow{
		ID:                   pgtype.UUID{Bytes: id, Valid: true},
		UserID:               pgtype.UUID{Bytes: userID, Valid: true},
		ScheduledOn:          pgtype.Date{Time: day, Valid: true},
		Title:                "Sesion",
		WorkoutType:          string(WorkoutTypeRodaje),
		TargetDurationSec:    i4(durationSec),
		TargetDistanceM:      i4(distanceM),
		TargetElevationGainM: i4(gainM),
		Status:               string(status),
		TotalCount:           1,
	}
}

// mustNumeric construye un pgtype.Numeric desde su texto decimal.
func mustNumeric(t *testing.T, s string) pgtype.Numeric {
	t.Helper()
	var n pgtype.Numeric
	if err := n.Scan(s); err != nil {
		t.Fatalf("Numeric.Scan(%q): %v", s, err)
	}
	return n
}

// compareActivityRow construye una actividad del rango con started_at en
// UTC (como viene de timestamptz) y metricas Numeric.
func compareActivityRow(t *testing.T, id, userID uuid.UUID, startedAt time.Time, movingSec int32, distanceM, gainM string) sqlc.ListActivitiesInRangeRow {
	t.Helper()
	return sqlc.ListActivitiesInRangeRow{
		ID:             pgtype.UUID{Bytes: id, Valid: true},
		UserID:         pgtype.UUID{Bytes: userID, Valid: true},
		SportType:      "Run",
		StartedAt:      pgtype.Timestamptz{Time: startedAt, Valid: true},
		MovingSeconds:  movingSec,
		DistanceMeters: mustNumeric(t, distanceM),
		ElevationGainM: mustNumeric(t, gainM),
	}
}

var (
	compareUser = uuid.MustParse("11111111-1111-1111-1111-111111111111")
	i32p        = func(v int32) *int32 { return &v }
)

func day(t time.Time) time.Time { return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC) }

// TestCompareWeekBucketsUseUserZoneOverDST cubre el caso limite del spec
// S3: los limites de semana en una zona distinta de UTC, atravesando el
// cambio de horario de Madrid (CEST->CET el 2026-10-25). Una actividad a
// las 22:30Z del domingo sigue en la semana anterior; a las 23:30Z ya es
// lunes 00:30 local y cae en la semana nueva. La actividad vinculada
// (A2) cuenta igual que la libre (A1).
func TestCompareWeekBucketsUseUserZoneOverDST(t *testing.T) {
	madrid := "Europe/Madrid"
	from := day(time.Date(2026, 10, 19, 0, 0, 0, 0, time.UTC)) // lunes
	to := day(time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC))    // domingo
	activityID := uuid.MustParse("44444444-4444-4444-4444-444444444444")
	mock := &mockQuerier{
		prefs: sqlc.GetUserPreferencesByIDRow{Timezone: madrid},
		listRows: []sqlc.ListPlannedSessionsByRangeRow{
			// Lunes 26: completada y vinculada a A2.
			compareSessionRow(uuid.New(), compareUser, day(time.Date(2026, 10, 26, 0, 0, 0, 0, time.UTC)), StatusCompleted, i32p(3600), i32p(10000), i32p(300)),
			// Domingo 1 de noviembre: skipped; sus objetivos no suman.
			compareSessionRow(uuid.New(), compareUser, day(time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)), StatusSkipped, i32p(5400), i32p(20000), i32p(500)),
		},
		activityRows: []sqlc.ListActivitiesInRangeRow{
			compareActivityRow(t, uuid.New(), compareUser, time.Date(2026, 10, 25, 22, 30, 0, 0, time.UTC), 1800, "5000", "100"),
			compareActivityRow(t, activityID, compareUser, time.Date(2026, 10, 25, 23, 30, 0, 0, time.UTC), 3600, "15000.5", "250.5"),
		},
	}
	linkA2 := mock.listRows[0]
	linkA2.ActivityID = pgtype.UUID{Bytes: activityID, Valid: true}
	mock.listRows[0] = linkA2

	periods, err := NewStore(mock).Compare(context.Background(), compareUser, from, to, CompareGroupWeek)
	if err != nil {
		t.Fatalf("Compare() error = %v", err)
	}
	want := []ComparePeriod{
		{
			PeriodStart: "2026-10-19",
			Actual:      CompareTotals{DurationSeconds: 1800, DistanceM: 5000, ElevationGainM: 100},
		},
		{
			PeriodStart: "2026-10-26",
			Planned:     CompareTotals{DurationSeconds: 3600, DistanceM: 10000, ElevationGainM: 300},
			Actual:      CompareTotals{DurationSeconds: 3600, DistanceM: 15000.5, ElevationGainM: 250.5},
			Compliance:  CompareCompliance{Total: 2, Completed: 1, Skipped: 1},
		},
	}
	if len(periods) != len(want) {
		t.Fatalf("Compare() returned %d periods, want %d: %+v", len(periods), len(want), periods)
	}
	for i := range want {
		if periods[i] != want[i] {
			t.Errorf("period[%d] = %+v, want %+v", i, periods[i], want[i])
		}
	}
}

func TestCompareIncludesDayWhenLocalMidnightDoesNotExist(t *testing.T) {
	date := civilUTC(t, "2018-11-04 00:00")
	mock := &mockQuerier{
		prefs: sqlc.GetUserPreferencesByIDRow{Timezone: "America/Sao_Paulo"},
		activityRows: []sqlc.ListActivitiesInRangeRow{
			compareActivityRow(t, uuid.New(), compareUser, civilUTC(t, "2018-11-04 03:00"), 60, "10", "2"),
			compareActivityRow(t, uuid.New(), compareUser, civilUTC(t, "2018-11-05 02:01"), 120, "20", "4"),
		},
	}
	buckets, err := NewStore(mock).Compare(context.Background(), compareUser, date, date, CompareGroupDay)
	if err != nil || len(buckets) != 1 || buckets[0].PeriodStart != "2018-11-04" || buckets[0].Actual.DurationSeconds != 60 {
		t.Fatalf("Compare() = %+v, %v; want 60 seconds on 2018-11-04", buckets, err)
	}
}

// TestCompareDayBucketsSkipEmptyDays: los dias sin plan y sin actividad
// no generan bucket; un dia solo-skipped aparece con cumplimiento y sin
// objetivos.
func TestCompareDayBucketsSkipEmptyDays(t *testing.T) {
	from := day(time.Date(2026, 11, 16, 0, 0, 0, 0, time.UTC))
	to := day(time.Date(2026, 11, 20, 0, 0, 0, 0, time.UTC))
	mock := &mockQuerier{
		listRows: []sqlc.ListPlannedSessionsByRangeRow{
			compareSessionRow(uuid.New(), compareUser, day(time.Date(2026, 11, 16, 0, 0, 0, 0, time.UTC)), StatusPlanned, i32p(1800), i32p(5000), i32p(100)),
			compareSessionRow(uuid.New(), compareUser, day(time.Date(2026, 11, 18, 0, 0, 0, 0, time.UTC)), StatusSkipped, i32p(3600), i32p(10000), i32p(0)),
		},
		activityRows: []sqlc.ListActivitiesInRangeRow{
			compareActivityRow(t, uuid.New(), compareUser, time.Date(2026, 11, 17, 10, 0, 0, 0, time.UTC), 600, "2000.25", "50.25"),
		},
	}
	periods, err := NewStore(mock).Compare(context.Background(), compareUser, from, to, CompareGroupDay)
	if err != nil {
		t.Fatalf("Compare() error = %v", err)
	}
	want := []ComparePeriod{
		{
			PeriodStart: "2026-11-16",
			Planned:     CompareTotals{DurationSeconds: 1800, DistanceM: 5000, ElevationGainM: 100},
			Compliance:  CompareCompliance{Total: 1},
		},
		{
			PeriodStart: "2026-11-17",
			Actual:      CompareTotals{DurationSeconds: 600, DistanceM: 2000.25, ElevationGainM: 50.25},
		},
		{
			PeriodStart: "2026-11-18",
			Compliance:  CompareCompliance{Total: 1, Skipped: 1},
		},
	}
	if len(periods) != len(want) {
		t.Fatalf("Compare() returned %d periods, want %d: %+v", len(periods), len(want), periods)
	}
	for i := range want {
		if periods[i] != want[i] {
			t.Errorf("period[%d] = %+v, want %+v", i, periods[i], want[i])
		}
	}
}

// TestCompareEmptyRangeReturnsEmptySlice: sin registros el chart va
// vacio, con un array (no null).
func TestCompareEmptyRangeReturnsEmptySlice(t *testing.T) {
	mock := &mockQuerier{}
	periods, err := NewStore(mock).Compare(context.Background(), compareUser, day(time.Date(2026, 11, 16, 0, 0, 0, 0, time.UTC)), day(time.Date(2026, 11, 22, 0, 0, 0, 0, time.UTC)), CompareGroupWeek)
	if err != nil {
		t.Fatalf("Compare() error = %v", err)
	}
	if periods == nil || len(periods) != 0 {
		t.Fatalf("Compare() = %#v, want empty non-nil slice", periods)
	}
}

// TestCompareTimezoneFallback: una zona guardada invalida (o una fila de
// preferencias ausente) cae a UTC, nunca a un error 500.
func TestCompareTimezoneFallback(t *testing.T) {
	from := day(time.Date(2026, 11, 16, 0, 0, 0, 0, time.UTC))
	// 23:30Z del lunes: en UTC es 16/11 (dentro); en Madrid seria 17/11
	// (fuera del rango de un solo dia). Si aparece, se uso UTC.
	rows := []sqlc.ListActivitiesInRangeRow{
		compareActivityRow(t, uuid.New(), compareUser, time.Date(2026, 11, 16, 23, 30, 0, 0, time.UTC), 600, "1000", "10"),
	}
	for name, tc := range map[string]struct {
		prefs sqlc.GetUserPreferencesByIDRow
		err   error
	}{
		"invalid zone": {prefs: sqlc.GetUserPreferencesByIDRow{Timezone: "Mars/Olympus"}},
		"missing row":  {err: pgx.ErrNoRows},
	} {
		mock := &mockQuerier{prefs: tc.prefs, prefsErr: tc.err, activityRows: rows}
		periods, err := NewStore(mock).Compare(context.Background(), compareUser, from, from, CompareGroupDay)
		if err != nil {
			t.Fatalf("%s: Compare() error = %v", name, err)
		}
		if len(periods) != 1 || periods[0].PeriodStart != "2026-11-16" || periods[0].Actual.DurationSeconds != 600 {
			t.Errorf("%s: Compare() = %+v, want the activity bucketed on 2026-11-16 (UTC fallback)", name, periods)
		}
	}
}

// TestCompareScopesQueriesAndMapsErrors: el aislamiento por usuario vive
// en las queries; las ventanas llegan en la zona del usuario; los fallos
// reales se envuelven.
func TestCompareScopesQueriesAndMapsErrors(t *testing.T) {
	from := day(time.Date(2026, 10, 19, 0, 0, 0, 0, time.UTC))
	to := day(time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC))
	mock := &mockQuerier{prefs: sqlc.GetUserPreferencesByIDRow{Timezone: "Europe/Madrid"}}
	store := NewStore(mock)
	ctx := context.Background()

	if _, err := store.Compare(ctx, compareUser, from, to, CompareGroupWeek); err != nil {
		t.Fatalf("Compare() unexpected error: %v", err)
	}
	if mock.prefsID.Bytes != compareUser {
		t.Errorf("GetUserPreferencesByID user = %s, want %s", mock.prefsID.Bytes, compareUser)
	}
	if mock.listParams.UserID.Bytes != compareUser || mock.listParams.Limit != math.MaxInt32 || mock.listParams.Offset != 0 {
		t.Errorf("ListPlannedSessionsByRange params = %+v, want caller-scoped full window", mock.listParams)
	}
	// 19/10 00:00 CEST = 18/10 22:00Z; 02/11 00:00 CET = 01/11 23:00Z.
	wantStart := from.AddDate(0, 0, -1)
	wantEnd := to.AddDate(0, 0, 2)
	if !mock.activityArgs.StartedAt.Time.Equal(wantStart) || !mock.activityArgs.StartedAt_2.Time.Equal(wantEnd) {
		t.Errorf("ListActivitiesInRange window = [%v, %v], want [%v, %v] (padded UTC window)", mock.activityArgs.StartedAt.Time, mock.activityArgs.StartedAt_2.Time, wantStart, wantEnd)
	}
	if mock.activityArgs.UserID.Bytes != compareUser {
		t.Errorf("ListActivitiesInRange user = %s, want %s", mock.activityArgs.UserID.Bytes, compareUser)
	}

	boom := errors.New("connection refused")
	mock.prefsErr = boom
	if _, err := store.Compare(ctx, compareUser, from, to, CompareGroupWeek); !errors.Is(err, boom) {
		t.Fatalf("Compare() prefs error = %v, want wrapped %v", err, boom)
	}
	mock.prefsErr = nil
	mock.activityErr = boom
	if _, err := store.Compare(ctx, compareUser, from, to, CompareGroupWeek); !errors.Is(err, boom) {
		t.Fatalf("Compare() activities error = %v, want wrapped %v", err, boom)
	}
}
