package metrics

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
)

// mockActivityRangeQuerier implementa ActivityRangeQuerier con respuestas
// preconfiguradas. Tests pueden sobreescribir Activities o Err para
// escenarios negativos.
type mockActivityRangeQuerier struct {
	Activities []ActivitySummary
	Err        error
	Calls      int
}

func (m *mockActivityRangeQuerier) ListActivitiesInRange(_ context.Context, _ pgtype.UUID, _, _ time.Time) ([]ActivitySummary, error) {
	m.Calls++
	return m.Activities, m.Err
}

// mockUserLoader implementa UserLoader.
type mockUserLoader struct {
	Phys  UserPhysiology
	Err   error
	Calls int
}

func (m *mockUserLoader) LoadUserPhysiology(_ context.Context, _ pgtype.UUID) (UserPhysiology, error) {
	m.Calls++
	return m.Phys, m.Err
}

func newValidUserID() pgtype.UUID {
	var id pgtype.UUID
	_ = id.Scan("00000000-0000-0000-0000-000000000001")
	return id
}

func TestComputeDailyLoad_CyclingAggregatesByDay(t *testing.T) {
	userID := newValidUserID()
	day := time.Date(2026, 10, 1, 8, 0, 0, 0, time.UTC)

	actQ := &mockActivityRangeQuerier{
		Activities: []ActivitySummary{
			// 1h @ 200W con FTP 200 → IF=1.0 → TSS=100.
			{ID: mustUUID(2), StartedAt: day, SportType: "Ride", ElapsedSeconds: 3600, MovingSeconds: 3600, DistanceMeters: 30000, AvgPower: pgInt2(200)},
			// 30min @ 100W con FTP 200 → IF=0.5 → TSS=12.5.
			{ID: mustUUID(3), StartedAt: day.Add(2 * time.Hour), SportType: "Ride", ElapsedSeconds: 1800, MovingSeconds: 1800, DistanceMeters: 12000, AvgPower: pgInt2(100)},
		},
	}
	usr := &mockUserLoader{Phys: UserPhysiology{UserID: userID, FTP: 200, RunningThresholdSecPerKm: 240}}

	rows, err := ComputeDailyLoad(context.Background(), ComputeDailyLoadDeps{
		ActivityRangeQuerier: actQ,
		UserLoader:           usr,
	}, userID, day.Add(-24*time.Hour), day.Add(24*time.Hour))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("want 1 day row, got %d", len(rows))
	}
	wantTSS := 100.0 + 12.5
	if !floatNear(rows[0].TSS, wantTSS, 0.01) {
		t.Errorf("TSS = %v, want %v", rows[0].TSS, wantTSS)
	}
	if rows[0].ActivityCount != 2 {
		t.Errorf("ActivityCount = %d, want 2", rows[0].ActivityCount)
	}
	if !floatNear(rows[0].DistanceM, 42000, 0.001) {
		t.Errorf("DistanceM = %v, want 42000", rows[0].DistanceM)
	}
}

func TestComputeDailyLoad_RunningWithoutFTPIsZero(t *testing.T) {
	userID := newValidUserID()
	day := time.Date(2026, 10, 1, 8, 0, 0, 0, time.UTC)

	actQ := &mockActivityRangeQuerier{
		Activities: []ActivitySummary{
			{ID: mustUUID(2), StartedAt: day, SportType: "Run", ElapsedSeconds: 3600, MovingSeconds: 3600, DistanceMeters: 10000},
		},
	}
	usr := &mockUserLoader{Phys: UserPhysiology{UserID: userID, FTP: 0, RunningThresholdSecPerKm: 240}}

	rows, err := ComputeDailyLoad(context.Background(), ComputeDailyLoadDeps{
		ActivityRangeQuerier: actQ,
		UserLoader:           usr,
	}, userID, day.Add(-24*time.Hour), day.Add(24*time.Hour))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("want 1 day row, got %d", len(rows))
	}
	// TSSRunning con threshold=240 (presente) y pace correcto → > 0.
	if rows[0].TSS <= 0 {
		t.Errorf("TSS = %v, want > 0 (running threshold is set)", rows[0].TSS)
	}

	// Now case: FTP=0 + threshold=0 → ambos sentinels → TSS=0 sin panic.
	usr2 := &mockUserLoader{Phys: UserPhysiology{UserID: userID, FTP: 0, RunningThresholdSecPerKm: 0}}
	rows2, err := ComputeDailyLoad(context.Background(), ComputeDailyLoadDeps{
		ActivityRangeQuerier: actQ,
		UserLoader:           usr2,
	}, userID, day.Add(-24*time.Hour), day.Add(24*time.Hour))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rows2[0].TSS != 0 {
		t.Errorf("TSS = %v, want 0 when no FTP and no running threshold", rows2[0].TSS)
	}
}

func TestComputeDailyLoad_GroupsAcrossMultipleDays(t *testing.T) {
	userID := newValidUserID()
	day1 := time.Date(2026, 10, 1, 8, 0, 0, 0, time.UTC)
	day2 := time.Date(2026, 10, 2, 8, 0, 0, 0, time.UTC)
	day3 := time.Date(2026, 10, 3, 8, 0, 0, 0, time.UTC)

	actQ := &mockActivityRangeQuerier{
		Activities: []ActivitySummary{
			{ID: mustUUID(2), StartedAt: day1, SportType: "Ride", ElapsedSeconds: 3600, MovingSeconds: 3600, AvgPower: pgInt2(200)},
			{ID: mustUUID(3), StartedAt: day2, SportType: "Ride", ElapsedSeconds: 1800, MovingSeconds: 1800, AvgPower: pgInt2(150)},
			{ID: mustUUID(4), StartedAt: day2, SportType: "Ride", ElapsedSeconds: 1800, MovingSeconds: 1800, AvgPower: pgInt2(150)},
			{ID: mustUUID(5), StartedAt: day3, SportType: "Run", ElapsedSeconds: 3600, MovingSeconds: 3600, DistanceMeters: 10000},
		},
	}
	usr := &mockUserLoader{Phys: UserPhysiology{UserID: userID, FTP: 200, RunningThresholdSecPerKm: 240}}

	rows, err := ComputeDailyLoad(context.Background(), ComputeDailyLoadDeps{
		ActivityRangeQuerier: actQ,
		UserLoader:           usr,
	}, userID, day1.Add(-24*time.Hour), day3.Add(24*time.Hour))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(rows) != 3 {
		t.Fatalf("want 3 day rows, got %d (%v)", len(rows), rows)
	}
	// Order must be ascending. utcDay() truncates to UTC-day so 08:00 -> 00:00.
	wantDays := []time.Time{
		time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC),
		time.Date(2026, 10, 2, 0, 0, 0, 0, time.UTC),
		time.Date(2026, 10, 3, 0, 0, 0, 0, time.UTC),
	}
	for i, want := range wantDays {
		if !rows[i].Day.Equal(want) {
			t.Errorf("row[%d].Day = %v, want %v", i, rows[i].Day, want)
		}
	}
	// Day2: two activities, TSS aggregated.
	if rows[1].ActivityCount != 2 {
		t.Errorf("day2 ActivityCount = %d, want 2", rows[1].ActivityCount)
	}
}

func TestComputeDailyLoad_SkipsOtherSports(t *testing.T) {
	userID := newValidUserID()
	day := time.Date(2026, 10, 1, 8, 0, 0, 0, time.UTC)

	actQ := &mockActivityRangeQuerier{
		Activities: []ActivitySummary{
			{ID: mustUUID(2), StartedAt: day, SportType: "Walk", ElapsedSeconds: 3600, MovingSeconds: 3600, DistanceMeters: 5000},
			{ID: mustUUID(3), StartedAt: day, SportType: "Hike", ElapsedSeconds: 7200, MovingSeconds: 7200, DistanceMeters: 8000, ElevationGainM: 600},
		},
	}
	usr := &mockUserLoader{Phys: UserPhysiology{UserID: userID, FTP: 200, RunningThresholdSecPerKm: 240}}

	rows, err := ComputeDailyLoad(context.Background(), ComputeDailyLoadDeps{
		ActivityRangeQuerier: actQ,
		UserLoader:           usr,
	}, userID, day.Add(-24*time.Hour), day.Add(24*time.Hour))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("want 1 day row, got %d", len(rows))
	}
	if rows[0].TSS != 0 {
		t.Errorf("TSS = %v, want 0 for non-cycling/non-running", rows[0].TSS)
	}
	if rows[0].ActivityCount != 2 {
		t.Errorf("ActivityCount = %d, want 2 (KPI still counted)", rows[0].ActivityCount)
	}
}

func TestComputeDailyLoad_RangeErrors(t *testing.T) {
	userID := newValidUserID()
	from := time.Date(2026, 10, 2, 0, 0, 0, 0, time.UTC)
	to := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)

	_, err := ComputeDailyLoad(context.Background(), ComputeDailyLoadDeps{
		ActivityRangeQuerier: &mockActivityRangeQuerier{},
		UserLoader:           &mockUserLoader{},
	}, userID, from, to)
	if err == nil {
		t.Fatalf("want error when to < from, got nil")
	}
}

func TestComputeDailyLoad_LoaderErrorIsSurfaced(t *testing.T) {
	userID := newValidUserID()
	day := time.Date(2026, 10, 1, 8, 0, 0, 0, time.UTC)

	actQ := &mockActivityRangeQuerier{}
	usr := &mockUserLoader{Err: errors.New("db down")}

	_, err := ComputeDailyLoad(context.Background(), ComputeDailyLoadDeps{
		ActivityRangeQuerier: actQ,
		UserLoader:           usr,
	}, userID, day.Add(-24*time.Hour), day.Add(24*time.Hour))
	if err == nil {
		t.Fatalf("want loader error propagated, got nil")
	}
}

func TestComputeDailyLoad_InvalidUserID(t *testing.T) {
	_, err := ComputeDailyLoad(context.Background(), ComputeDailyLoadDeps{
		ActivityRangeQuerier: &mockActivityRangeQuerier{},
		UserLoader:           &mockUserLoader{},
	}, pgtype.UUID{}, time.Now(), time.Now())
	if err == nil {
		t.Fatalf("want error for invalid userID, got nil")
	}
}

func TestComputeDailyLoad_NilDeps(t *testing.T) {
	userID := newValidUserID()
	_, err := ComputeDailyLoad(context.Background(), ComputeDailyLoadDeps{}, userID, time.Now(), time.Now())
	if err == nil {
		t.Fatalf("want error for nil ActivityRangeQuerier, got nil")
	}
}

// Helpers
func mustUUID(v int) pgtype.UUID {
	var id pgtype.UUID
	_ = id.Scan(uuidString(v))
	return id
}

func uuidString(v int) string {
	const hex = "0123456789abcdef"
	out := make([]byte, 36)
	out[8] = '-'
	out[13] = '-'
	out[18] = '-'
	out[23] = '-'
	// simple deterministic: 0..0..0..0..N (16 hex digits spread across groups)
	for i := 0; i < 8; i++ {
		out[i] = hex[0]
	}
	for i := 9; i < 13; i++ {
		out[i] = hex[0]
	}
	for i := 14; i < 18; i++ {
		out[i] = hex[0]
	}
	for i := 19; i < 23; i++ {
		out[i] = hex[0]
	}
	// last group encodes the int v in hex
	tmp := v
	var vstr []byte
	if tmp == 0 {
		vstr = []byte("0")
	} else {
		buf := make([]byte, 0, 8)
		for tmp > 0 {
			buf = append([]byte{hex[tmp%16]}, buf...)
			tmp /= 16
		}
		vstr = buf
	}
	for i := 0; i < len(vstr) && 24+i < 36; i++ {
		out[24+i] = vstr[i]
	}
	return string(out)
}

func pgInt2(v int16) pgtype.Int2 {
	return pgtype.Int2{Int16: v, Valid: true}
}

func floatNear(a, b, eps float64) bool {
	d := a - b
	if d < 0 {
		d = -d
	}
	return d < eps
}
