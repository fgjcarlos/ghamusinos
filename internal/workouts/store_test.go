package workouts

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/fgjcarlos/ghamusinos/internal/db/sqlc"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

// mockQuerier implementa los cinco metodos que el store necesita.
type mockQuerier struct {
	createdParams sqlc.CreatePlannedSessionParams
	getParams     sqlc.GetPlannedSessionParams
	updateParams  sqlc.UpdatePlannedSessionParams
	deleteParams  sqlc.DeletePlannedSessionParams
	listParams    sqlc.ListPlannedSessionsByRangeParams
	deletedRows   int64
	getErr        error
	row           sqlc.PlannedSession
}

func (m *mockQuerier) CreatePlannedSession(_ context.Context, arg sqlc.CreatePlannedSessionParams) (sqlc.PlannedSession, error) {
	m.createdParams = arg
	return m.row, nil
}

func (m *mockQuerier) GetPlannedSession(_ context.Context, arg sqlc.GetPlannedSessionParams) (sqlc.PlannedSession, error) {
	m.getParams = arg
	if m.getErr != nil {
		return sqlc.PlannedSession{}, m.getErr
	}
	return m.row, nil
}

func (m *mockQuerier) ListPlannedSessionsByRange(_ context.Context, arg sqlc.ListPlannedSessionsByRangeParams) ([]sqlc.ListPlannedSessionsByRangeRow, error) {
	m.listParams = arg
	return nil, nil
}

func (m *mockQuerier) UpdatePlannedSession(_ context.Context, arg sqlc.UpdatePlannedSessionParams) (sqlc.PlannedSession, error) {
	m.updateParams = arg
	return m.row, nil
}

func (m *mockQuerier) DeletePlannedSession(_ context.Context, arg sqlc.DeletePlannedSessionParams) (int64, error) {
	m.deleteParams = arg
	return m.deletedRows, nil
}

// TestStoreAlwaysScopesByUser proves the store passes the caller's user_id
// to every query: el aislamiento se aplica en el SQL, no en el Go.
func TestStoreAlwaysScopesByUser(t *testing.T) {
	userID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	sessionID := uuid.MustParse("33333333-3333-3333-3333-333333333333")
	day := time.Date(2026, 11, 20, 0, 0, 0, 0, time.UTC)

	mock := &mockQuerier{
		deletedRows: 1,
		row: sqlc.PlannedSession{
			ID:          pgtype.UUID{Bytes: sessionID, Valid: true},
			UserID:      pgtype.UUID{Bytes: userID, Valid: true},
			ScheduledOn: pgtype.Date{Time: day, Valid: true},
			Title:       "Rodaje",
			WorkoutType: string(WorkoutTypeRodaje),
		},
	}
	store := NewStore(mock)
	ctx := context.Background()

	if _, err := store.Create(ctx, userID, Session{
		ScheduledOn: day,
		Title:       "Rodaje",
		WorkoutType: WorkoutTypeRodaje,
	}); err != nil {
		t.Fatalf("Create() unexpected error: %v", err)
	}
	if mock.createdParams.UserID.Bytes != userID {
		t.Errorf("Create() user_id = %s, want %s", mock.createdParams.UserID.Bytes, userID)
	}

	if _, err := store.Get(ctx, userID, sessionID); err != nil {
		t.Fatalf("Get() unexpected error: %v", err)
	}
	if mock.getParams.UserID.Bytes != userID || mock.getParams.ID.Bytes != sessionID {
		t.Errorf("Get() params = %+v, want user %s and session %s", mock.getParams, userID, sessionID)
	}

	if _, _, err := store.ListRange(ctx, userID, day, day, 10, 0); err != nil {
		t.Fatalf("ListRange() unexpected error: %v", err)
	}
	if mock.listParams.UserID.Bytes != userID {
		t.Errorf("ListRange() user_id = %s, want %s", mock.listParams.UserID.Bytes, userID)
	}

	if _, err := store.Update(ctx, userID, sessionID, Session{
		ScheduledOn: day,
		Title:       "Rodaje largo",
		WorkoutType: WorkoutTypeRodaje,
	}); err != nil {
		t.Fatalf("Update() unexpected error: %v", err)
	}
	if mock.updateParams.UserID.Bytes != userID || mock.updateParams.ID.Bytes != sessionID {
		t.Errorf("Update() params = %+v, want user %s and session %s", mock.updateParams, userID, sessionID)
	}

	if err := store.Delete(ctx, userID, sessionID); err != nil {
		t.Fatalf("Delete() unexpected error: %v", err)
	}
	if mock.deleteParams.UserID.Bytes != userID || mock.deleteParams.ID.Bytes != sessionID {
		t.Errorf("Delete() params = %+v, want user %s and session %s", mock.deleteParams, userID, sessionID)
	}
}

// TestStoreGetMissingSession proves a missing row is ErrNotFound, not a
// wrapped pgx error.
func TestStoreGetMissingSession(t *testing.T) {
	userID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	mock := &mockQuerier{getErr: pgx.ErrNoRows}
	store := NewStore(mock)

	_, err := store.Get(context.Background(), userID, uuid.New())
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("Get() error = %v, want ErrNotFound", err)
	}
}

// TestStoreDeleteMissingSession proves deleting a row that does not exist
// (or belongs to another user) reports ErrNotFound.
func TestStoreDeleteMissingSession(t *testing.T) {
	userID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	mock := &mockQuerier{deletedRows: 0}
	store := NewStore(mock)

	err := store.Delete(context.Background(), userID, uuid.New())
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("Delete() error = %v, want ErrNotFound", err)
	}
}

// TestStoreRejectsInvalidSession proves the store validates before it
// touches the database.
func TestStoreRejectsInvalidSession(t *testing.T) {
	userID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	mock := &mockQuerier{}
	store := NewStore(mock)

	_, err := store.Create(context.Background(), userID, Session{
		ScheduledOn: time.Now(),
		Title:       "",
		WorkoutType: WorkoutTypeRodaje,
	})
	if !errors.Is(err, ErrValidation) {
		t.Fatalf("Create() error = %v, want ErrValidation", err)
	}
	if mock.createdParams.Title != "" {
		t.Error("Create() reached the database with an invalid session")
	}
}

// TestStoreKeepsUnplannedTargetsUnset proves that an objective which was
// never planned stays absent (nil) instead of becoming a zero: the two
// are different facts and the calendar has to be able to tell them apart.
func TestStoreKeepsUnplannedTargetsUnset(t *testing.T) {
	userID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	sessionID := uuid.MustParse("33333333-3333-3333-3333-333333333333")
	day := time.Date(2026, 11, 20, 0, 0, 0, 0, time.UTC)

	mock := &mockQuerier{row: sqlc.PlannedSession{
		ID:          pgtype.UUID{Bytes: sessionID, Valid: true},
		UserID:      pgtype.UUID{Bytes: userID, Valid: true},
		ScheduledOn: pgtype.Date{Time: day, Valid: true},
		Title:       "Rodaje",
		WorkoutType: string(WorkoutTypeRodaje),
	}}
	store := NewStore(mock)

	session, err := store.Create(context.Background(), userID, Session{
		ScheduledOn: day,
		Title:       "Rodaje",
		WorkoutType: WorkoutTypeRodaje,
	})
	if err != nil {
		t.Fatalf("Create() unexpected error: %v", err)
	}
	if session.TargetDistanceM != nil {
		t.Errorf("target distance = %d, want nil (unplanned is not zero)", *session.TargetDistanceM)
	}
	if session.TargetIntensity != nil {
		t.Errorf("target intensity = %q, want nil", *session.TargetIntensity)
	}
	if mock.createdParams.TargetDistanceM.Valid {
		t.Error("the query received a valid target distance for an unplanned objective")
	}
}
