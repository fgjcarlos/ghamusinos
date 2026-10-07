package jobs

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/fgjcarlos/ghamusinos/internal/db/sqlc"
	"github.com/fgjcarlos/ghamusinos/internal/metrics"
)

// mockTrainingLoadStore implementa TrainingLoadStore sin sqlc. Acepta hooks
// para inyectar errores y captura upserts para asserts.
type mockTrainingLoadStore struct {
	MetaRow         sqlc.DashboardMetadatum
	MetaErr         error
	UpsertCount     int
	UpsertErr       error
	UpsertRows      []sqlc.UpsertTrainingLoadDailyParams
	MetaCalls       int
	MetaUpsertCalls int
	// PerUpsertErr simulates an upsert that succeeds for the first N
	// invocations and then fails. Used by the partial-failure test.
	PerUpsertErr error
	PerUpsertAt  int // 0 = never fail; positive = call index where it fails (1-based)
	UpsertArgs   []sqlc.UpsertDashboardMetadataParams
}

func (m *mockTrainingLoadStore) GetDashboardMetadata(_ context.Context, _ pgtype.UUID) (sqlc.DashboardMetadatum, error) {
	m.MetaCalls++
	return m.MetaRow, m.MetaErr
}

func (m *mockTrainingLoadStore) UpsertDashboardMetadata(_ context.Context, arg sqlc.UpsertDashboardMetadataParams) (sqlc.DashboardMetadatum, error) {
	m.MetaUpsertCalls++
	m.UpsertArgs = append(m.UpsertArgs, arg)
	return sqlc.DashboardMetadatum{}, m.UpsertErr
}

func (m *mockTrainingLoadStore) UpsertTrainingLoadDaily(_ context.Context, arg sqlc.UpsertTrainingLoadDailyParams) (sqlc.TrainingLoadDaily, error) {
	m.UpsertCount++
	if m.PerUpsertAt > 0 && m.UpsertCount == m.PerUpsertAt {
		return sqlc.TrainingLoadDaily{}, m.PerUpsertErr
	}
	if m.UpsertErr != nil {
		return sqlc.TrainingLoadDaily{}, m.UpsertErr
	}
	m.UpsertRows = append(m.UpsertRows, arg)
	return sqlc.TrainingLoadDaily{}, nil
}

func (m *mockTrainingLoadStore) ListTrainingLoadRange(_ context.Context, _ sqlc.ListTrainingLoadRangeParams) ([]sqlc.TrainingLoadDaily, error) {
	return nil, nil
}

// mockActivityLoader implementa ActivityRangeAdapter.
type mockActivityLoader struct {
	Activities []metrics.ActivitySummary
	FirstTime  pgtype.Timestamptz
	FirstErr   error
	RangeErr   error
	RangeCalls int
	FirstCalls int
}

func (m *mockActivityLoader) ListActivitiesInRange(_ context.Context, _ pgtype.UUID, _, _ time.Time) ([]metrics.ActivitySummary, error) {
	m.RangeCalls++
	return m.Activities, m.RangeErr
}

func (m *mockActivityLoader) FirstActivityForUser(_ context.Context, _ pgtype.UUID) (pgtype.Timestamptz, error) {
	m.FirstCalls++
	return m.FirstTime, m.FirstErr
}

// mockUserPhysiologyLoader implementa userPhysiologyLoaderAdapter.
type mockUserPhysiologyLoader struct {
	Phys metrics.UserPhysiology
	Err  error
}

func (m *mockUserPhysiologyLoader) LoadUserPhysiology(_ context.Context, _ pgtype.UUID) (metrics.UserPhysiology, error) {
	return m.Phys, m.Err
}

func newUserID(t *testing.T, s string) pgtype.UUID {
	t.Helper()
	var id pgtype.UUID
	if err := id.Scan(s); err != nil {
		t.Fatalf("bad uuid: %v", err)
	}
	return id
}

func TestRecalcTrainingLoad_FirstRunUsesFirstActivity(t *testing.T) {
	userID := newUserID(t, "11111111-1111-1111-1111-111111111111")
	day := time.Date(2026, 10, 1, 8, 0, 0, 0, time.UTC)

	store := &mockTrainingLoadStore{
		MetaRow: sqlc.DashboardMetadatum{UserID: userID},
	}
	actLoader := &mockActivityLoader{
		FirstTime: pgtype.Timestamptz{Time: day, Valid: true},
		Activities: []metrics.ActivitySummary{
			{
				StartedAt:      day,
				SportType:      "Ride",
				ElapsedSeconds: 3600,
				MovingSeconds:  3600,
				AvgPower:       pgtype.Int2{Int16: 200, Valid: true},
			},
		},
	}
	usr := &mockUserPhysiologyLoader{
		Phys: metrics.UserPhysiology{UserID: userID, FTP: 200},
	}

	now := day.Add(24 * time.Hour)
	w := NewRecalcTrainingLoadWorker(store, actLoader, usr, nil, func() time.Time { return now }, WindowFromDashboardMetadata)

	if err := w.recomputeForUser(context.Background(), userID, RecalcTrainingLoadArgs{UserID: userID.String()}); err != nil {
		t.Fatalf("Work: %v", err)
	}
	if store.UpsertCount != 1 {
		t.Fatalf("upsert count = %d, want 1", store.UpsertCount)
	}
	if actLoader.RangeCalls != 1 {
		t.Fatalf("ListActivitiesInRange calls = %d, want 1", actLoader.RangeCalls)
	}
	if actLoader.FirstCalls != 1 {
		t.Fatalf("FirstActivityForUser calls = %d, want 1", actLoader.FirstCalls)
	}
}

func TestRecalcTrainingLoad_UsesDashboardLastRecalcAt(t *testing.T) {
	userID := newUserID(t, "11111111-1111-1111-1111-111111111111")
	last := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	now := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)

	store := &mockTrainingLoadStore{
		MetaRow: sqlc.DashboardMetadatum{
			UserID:       userID,
			LastRecalcAt: pgtype.Timestamptz{Time: last, Valid: true},
		},
	}
	actLoader := &mockActivityLoader{}
	usr := &mockUserPhysiologyLoader{Phys: metrics.UserPhysiology{UserID: userID, FTP: 200}}

	w := NewRecalcTrainingLoadWorker(store, actLoader, usr, nil, func() time.Time { return now }, WindowFromDashboardMetadata)
	if err := w.recomputeForUser(context.Background(), userID, RecalcTrainingLoadArgs{UserID: userID.String()}); err != nil {
		t.Fatalf("Work: %v", err)
	}
	if actLoader.FirstCalls != 0 {
		t.Errorf("FirstActivityForUser must not be called when last_recalc_at is set; got %d", actLoader.FirstCalls)
	}
	if actLoader.RangeCalls != 1 {
		t.Errorf("ListActivitiesInRange calls = %d, want 1", actLoader.RangeCalls)
	}
}

func TestRecalcTrainingLoad_ExplicitArgsOverrideStrategy(t *testing.T) {
	userID := newUserID(t, "11111111-1111-1111-1111-111111111111")
	from := "2026-08-01T00:00:00Z"
	to := "2026-10-01T00:00:00Z"
	now := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)

	store := &mockTrainingLoadStore{}
	actLoader := &mockActivityLoader{}
	usr := &mockUserPhysiologyLoader{Phys: metrics.UserPhysiology{UserID: userID, FTP: 200}}

	w := NewRecalcTrainingLoadWorker(store, actLoader, usr, nil, func() time.Time { return now }, WindowFromDashboardMetadata)
	err := w.recomputeForUser(context.Background(), userID, RecalcTrainingLoadArgs{
		UserID: userID.String(),
		From:   &from,
		To:     &to,
	})
	if err != nil {
		t.Fatalf("Work: %v", err)
	}
	if actLoader.RangeCalls != 1 {
		t.Errorf("ListActivitiesInRange calls = %d, want 1", actLoader.RangeCalls)
	}
}

func TestRecalcTrainingLoad_NoActivitiesIsOk(t *testing.T) {
	userID := newUserID(t, "11111111-1111-1111-1111-111111111111")
	day := time.Date(2026, 10, 1, 8, 0, 0, 0, time.UTC)

	store := &mockTrainingLoadStore{}
	actLoader := &mockActivityLoader{
		FirstTime:  pgtype.Timestamptz{Valid: false}, // no activities
		Activities: nil,
	}
	usr := &mockUserPhysiologyLoader{Phys: metrics.UserPhysiology{UserID: userID, FTP: 200}}

	w := NewRecalcTrainingLoadWorker(store, actLoader, usr, nil, func() time.Time { return day }, WindowFromDashboardMetadata)
	if err := w.recomputeForUser(context.Background(), userID, RecalcTrainingLoadArgs{UserID: userID.String()}); err != nil {
		t.Fatalf("Work: %v", err)
	}
	if store.UpsertCount != 0 {
		t.Errorf("upsert count = %d, want 0 for no activities", store.UpsertCount)
	}
}

func TestRecalcTrainingLoad_UpsertErrorPropagated(t *testing.T) {
	userID := newUserID(t, "11111111-1111-1111-1111-111111111111")
	day := time.Date(2026, 10, 1, 8, 0, 0, 0, time.UTC)
	now := day.Add(24 * time.Hour)

	store := &mockTrainingLoadStore{
		UpsertErr: errors.New("disk full"),
	}
	actLoader := &mockActivityLoader{
		FirstTime: pgtype.Timestamptz{Time: day, Valid: true},
		Activities: []metrics.ActivitySummary{
			{StartedAt: day, SportType: "Ride", ElapsedSeconds: 3600, MovingSeconds: 3600, AvgPower: pgtype.Int2{Int16: 200, Valid: true}},
		},
	}
	usr := &mockUserPhysiologyLoader{Phys: metrics.UserPhysiology{UserID: userID, FTP: 200}}

	w := NewRecalcTrainingLoadWorker(store, actLoader, usr, nil, func() time.Time { return now }, WindowFromDashboardMetadata)
	err := w.recomputeForUser(context.Background(), userID, RecalcTrainingLoadArgs{UserID: userID.String()})
	if err == nil {
		t.Fatalf("Work: want error, got nil")
	}
}

func TestRecalcTrainingLoad_LoaderErrorPropagated(t *testing.T) {
	userID := newUserID(t, "11111111-1111-1111-1111-111111111111")
	day := time.Date(2026, 10, 1, 8, 0, 0, 0, time.UTC)
	now := day.Add(24 * time.Hour)

	store := &mockTrainingLoadStore{}
	actLoader := &mockActivityLoader{
		FirstTime: pgtype.Timestamptz{Time: day, Valid: true},
		Activities: []metrics.ActivitySummary{
			{StartedAt: day, SportType: "Ride", ElapsedSeconds: 3600, MovingSeconds: 3600, AvgPower: pgtype.Int2{Int16: 200, Valid: true}},
		},
	}
	usr := &mockUserPhysiologyLoader{
		Phys: metrics.UserPhysiology{UserID: userID, FTP: 200},
		Err:  errors.New("db unreachable"),
	}

	w := NewRecalcTrainingLoadWorker(store, actLoader, usr, nil, func() time.Time { return now }, WindowFromDashboardMetadata)
	err := w.recomputeForUser(context.Background(), userID, RecalcTrainingLoadArgs{UserID: userID.String()})
	if err == nil {
		t.Fatalf("Work: want error from loader, got nil")
	}
}

func TestRecalcTrainingLoad_IdempotentAcrossTwoRuns(t *testing.T) {
	userID := newUserID(t, "11111111-1111-1111-1111-111111111111")
	day := time.Date(2026, 10, 1, 8, 0, 0, 0, time.UTC)
	now := day.Add(24 * time.Hour)

	store := &mockTrainingLoadStore{
		MetaRow: sqlc.DashboardMetadatum{UserID: userID},
	}
	actLoader := &mockActivityLoader{
		FirstTime: pgtype.Timestamptz{Time: day, Valid: true},
		Activities: []metrics.ActivitySummary{
			{StartedAt: day, SportType: "Ride", ElapsedSeconds: 3600, MovingSeconds: 3600, AvgPower: pgtype.Int2{Int16: 200, Valid: true}},
		},
	}
	usr := &mockUserPhysiologyLoader{Phys: metrics.UserPhysiology{UserID: userID, FTP: 200}}

	w := NewRecalcTrainingLoadWorker(store, actLoader, usr, nil, func() time.Time { return now }, WindowFromDashboardMetadata)

	if err := w.recomputeForUser(context.Background(), userID, RecalcTrainingLoadArgs{UserID: userID.String()}); err != nil {
		t.Fatalf("first run: %v", err)
	}
	// Simulate second run after metadata updated: no MetronicError because
	// the mock store does not update last_recalc_at, so the strategy still
	// resolves via first activity — and upserts same row again. The upsert
	// count is the key: must be 2 (one per run).
	if err := w.recomputeForUser(context.Background(), userID, RecalcTrainingLoadArgs{UserID: userID.String()}); err != nil {
		t.Fatalf("second run: %v", err)
	}
	if store.UpsertCount != 2 {
		t.Errorf("upsert count = %d, want 2 (idempotent)", store.UpsertCount)
	}
}

// TestRecalcTrainingLoad_PartialFailurePreservesWindow is the regression
// test for the PR2 review's blocker finding #1: when an upsert fails
// mid-loop, the worker must NOT advance last_recalc_at, so River's retry
// processes the exact same window (and the days that already committed
// re-upsert idempotently via ON CONFLICT).
//
// Layout:
//   - MetaRow has a previous successful last_recalc_at = prev.
//   - 3 activities on 3 consecutive days.
//   - Upsert 3 fails (PerUpsertAt=3, PerUpsertErr="disk full").
//   - First run: 2 upserts succeed, the third fails. recordError is
//     called with the upsert error. last_recalc_at must equal prev
//     (not now()).
//   - Second run: fix the upstream error (PerUpsertAt=0, no per-call
//     error) and re-run. recordOK writes MIN(now, prev) for
//     last_recalc_at, training_load_rows = 3.
func TestRecalcTrainingLoad_PartialFailurePreservesWindow(t *testing.T) {
	userID := newUserID(t, "11111111-1111-1111-1111-111111111111")
	day1 := time.Date(2026, 10, 1, 8, 0, 0, 0, time.UTC)
	day2 := day1.Add(24 * time.Hour)
	day3 := day1.Add(48 * time.Hour)
	now := day3.Add(24 * time.Hour)
	prevLastRecalc := time.Date(2026, 9, 30, 12, 0, 0, 0, time.UTC)

	store := &mockTrainingLoadStore{
		MetaRow: sqlc.DashboardMetadatum{
			UserID:       userID,
			LastRecalcAt: pgtype.Timestamptz{Time: prevLastRecalc, Valid: true},
		},
	}
	actLoader := &mockActivityLoader{
		FirstTime: pgtype.Timestamptz{Time: day1, Valid: true},
		Activities: []metrics.ActivitySummary{
			{StartedAt: day1, SportType: "Ride", ElapsedSeconds: 3600, MovingSeconds: 3600, AvgPower: pgtype.Int2{Int16: 200, Valid: true}},
			{StartedAt: day2, SportType: "Ride", ElapsedSeconds: 3600, MovingSeconds: 3600, AvgPower: pgtype.Int2{Int16: 200, Valid: true}},
			{StartedAt: day3, SportType: "Ride", ElapsedSeconds: 3600, MovingSeconds: 3600, AvgPower: pgtype.Int2{Int16: 200, Valid: true}},
		},
	}
	usr := &mockUserPhysiologyLoader{Phys: metrics.UserPhysiology{UserID: userID, FTP: 200}}

	// First run: day3 upsert fails.
	store.PerUpsertAt = 3
	store.PerUpsertErr = errors.New("disk full")
	w := NewRecalcTrainingLoadWorker(store, actLoader, usr, nil, func() time.Time { return now }, WindowFromDashboardMetadata)
	if err := w.recomputeForUser(context.Background(), userID, RecalcTrainingLoadArgs{UserID: userID.String()}); err == nil {
		t.Fatalf("first run: want error, got nil")
	}
	if store.UpsertCount != 3 {
		t.Errorf("first run: UpsertCount = %d, want 3 (two committed, one failed)", store.UpsertCount)
	}
	// recordError wrote metadata: last_recalc_at must be preserved (equal
	// to prevLastRecalc, not advanced to now()).
	if len(store.UpsertArgs) == 0 {
		t.Fatalf("recordError did not call UpsertDashboardMetadata")
	}
	lastMetaWrite := store.UpsertArgs[len(store.UpsertArgs)-1]
	if !lastMetaWrite.LastRecalcAt.Valid {
		t.Errorf("recordError nulled last_recalc_at; want preserved value %v",
			prevLastRecalc)
	} else if !lastMetaWrite.LastRecalcAt.Time.Equal(prevLastRecalc) {
		t.Errorf("recordError advanced last_recalc_at to %v; want preserved value %v",
			lastMetaWrite.LastRecalcAt.Time, prevLastRecalc)
	}
	if lastMetaWrite.LastRecalcStatus.String != "error" {
		t.Errorf("recordError status = %q, want \"error\"", lastMetaWrite.LastRecalcStatus.String)
	}

	// Second run: clear the per-upsert error, simulate River retry. The
	// MetaRow still reflects the preserved last_recalc_at = prev (the
	// previous successful recalc anchor). Mock returns the same prev
	// because we never write to MetaRow directly.
	store.PerUpsertAt = 0
	upsertsBefore := store.UpsertCount
	if err := w.recomputeForUser(context.Background(), userID, RecalcTrainingLoadArgs{UserID: userID.String()}); err != nil {
		t.Fatalf("second run: %v", err)
	}
	if store.UpsertCount-upsertsBefore != 3 {
		t.Errorf("second run: %d new upserts, want 3 (the original window re-processed)",
			store.UpsertCount-upsertsBefore)
	}
	// Find the recordOK call (the last UpsertDashboardMetadata call).
	lastMetaWrite = store.UpsertArgs[len(store.UpsertArgs)-1]
	if lastMetaWrite.LastRecalcStatus.String != "ok" {
		t.Errorf("recordOK status = %q, want \"ok\"", lastMetaWrite.LastRecalcStatus.String)
	}
	if !lastMetaWrite.LastRecalcAt.Valid {
		t.Errorf("recordOK wrote NULL last_recalc_at; want now() since prev was older")
	} else if !lastMetaWrite.LastRecalcAt.Time.Equal(now) {
		t.Errorf("recordOK last_recalc_at = %v, want %v", lastMetaWrite.LastRecalcAt.Time, now)
	}
	if lastMetaWrite.TrainingLoadRows != 3 {
		t.Errorf("recordOK training_load_rows = %d, want 3", lastMetaWrite.TrainingLoadRows)
	}
}

// TestRecalcTrainingLoad_RecordOKUsesMinWhenPrevIsNewer covers the second
// half of the partner fix: when prevLastRecalcAt is the same as `now` (no
// clock drift, but a retry happened immediately), recordOK still writes
// the right value. The MIN(now, prev) safety net becomes a no-op when
// prev <= now, but we still want to verify the chosen effective timestamp.
//
// We seed prev exactly equal to now so the worker still gets a valid
// window (from = now, to = now). The window may emit zero rows because
// ListActivitiesInRange with started_at >= now AND started_at <= now
// only catches activities at exactly `now`. The test focuses on the
// metadata write, not on the row count.
func TestRecalcTrainingLoad_RecordOKUsesMinWhenPrevIsNewer(t *testing.T) {
	userID := newUserID(t, "11111111-1111-1111-1111-111111111111")
	now := time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC)

	store := &mockTrainingLoadStore{
		MetaRow: sqlc.DashboardMetadatum{
			UserID:       userID,
			LastRecalcAt: pgtype.Timestamptz{Time: now, Valid: true},
		},
	}
	actLoader := &mockActivityLoader{
		// No activities yet: empty list keeps the window valid.
		Activities: nil,
	}
	usr := &mockUserPhysiologyLoader{Phys: metrics.UserPhysiology{UserID: userID, FTP: 200}}

	w := NewRecalcTrainingLoadWorker(store, actLoader, usr, nil, func() time.Time { return now }, WindowFromDashboardMetadata)
	if err := w.recomputeForUser(context.Background(), userID, RecalcTrainingLoadArgs{UserID: userID.String()}); err != nil {
		t.Fatalf("Work: %v", err)
	}
	lastMetaWrite := store.UpsertArgs[len(store.UpsertArgs)-1]
	if !lastMetaWrite.LastRecalcAt.Valid || !lastMetaWrite.LastRecalcAt.Time.Equal(now) {
		t.Errorf("recordOK last_recalc_at = %v, want %v (equal: MIN(now, prev) = now)",
			lastMetaWrite.LastRecalcAt.Time, now)
	}
}
