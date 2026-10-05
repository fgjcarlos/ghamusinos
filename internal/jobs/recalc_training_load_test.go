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
	MetaRow     sqlc.DashboardMetadatum
	MetaErr     error
	UpsertCount int
	UpsertErr   error
	UpsertRows  []sqlc.UpsertTrainingLoadDailyParams
	MetaCalls   int
}

func (m *mockTrainingLoadStore) GetDashboardMetadata(_ context.Context, _ pgtype.UUID) (sqlc.DashboardMetadatum, error) {
	m.MetaCalls++
	return m.MetaRow, m.MetaErr
}

func (m *mockTrainingLoadStore) UpsertDashboardMetadata(_ context.Context, _ sqlc.UpsertDashboardMetadataParams) (sqlc.DashboardMetadatum, error) {
	return sqlc.DashboardMetadatum{}, m.UpsertErr
}

func (m *mockTrainingLoadStore) UpsertTrainingLoadDaily(_ context.Context, arg sqlc.UpsertTrainingLoadDailyParams) (sqlc.TrainingLoadDaily, error) {
	m.UpsertCount++
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
