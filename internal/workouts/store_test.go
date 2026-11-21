package workouts

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/fgjcarlos/ghamusinos/internal/db/sqlc"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
)

// mockQuerier implementa los metodos que el store necesita.
type mockQuerier struct {
	createdParams sqlc.CreatePlannedSessionParams
	getParams     sqlc.GetPlannedSessionParams
	updateParams  sqlc.UpdatePlannedSessionParams
	deleteParams  sqlc.DeletePlannedSessionParams
	listParams    sqlc.ListPlannedSessionsByRangeParams
	linkParams    sqlc.LinkPlannedSessionActivityParams
	unlinkParams  sqlc.UnlinkPlannedSessionActivityParams
	prefsID       pgtype.UUID
	prefs         sqlc.GetUserPreferencesByIDRow
	prefsErr      error
	activityArgs  sqlc.ListActivitiesInRangeParams
	activityRows  []sqlc.ListActivitiesInRangeRow
	activityErr   error
	calendarArgs  sqlc.ListCalendarActivitiesParams
	calendarRows  []sqlc.ListCalendarActivitiesRow
	calendarErr   error
	calendarCalls int
	deletedRows   int64
	getErr        error
	linkErr       error
	unlinkErr     error
	row           sqlc.PlannedSession
	listRows      []sqlc.ListPlannedSessionsByRangeRow
	fallbackRows  []sqlc.ListPlannedSessionsByRangeRow
	fallbackErr   error
	listCalls     int
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
	m.listCalls++
	if m.listCalls > 1 {
		return m.fallbackRows, m.fallbackErr
	}
	return m.listRows, nil
}

func TestStoreListRangeRecoversTotalPastLastOffset(t *testing.T) {
	day := time.Date(2026, 11, 20, 0, 0, 0, 0, time.UTC)
	userID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	mock := &mockQuerier{fallbackRows: []sqlc.ListPlannedSessionsByRangeRow{{
		ID:          pgtype.UUID{Bytes: uuid.MustParse("33333333-3333-3333-3333-333333333333"), Valid: true},
		UserID:      pgtype.UUID{Bytes: userID, Valid: true},
		ScheduledOn: pgtype.Date{Time: day, Valid: true},
		WorkoutType: string(WorkoutTypeRodaje),
		TotalCount:  3,
	}}}
	rows, total, err := NewStore(mock).ListRange(context.Background(), userID, day, day, 2, 8)
	if err != nil {
		t.Fatalf("ListRange() error = %v", err)
	}
	if len(rows) != 0 || total != 3 {
		t.Fatalf("ListRange() = (%d rows, total %d), want (0, 3)", len(rows), total)
	}
	if mock.listCalls != 2 || mock.listParams.Offset != 0 || mock.listParams.Limit != 1 {
		t.Fatalf("fallback params/calls = %+v / %d, want offset 0 limit 1 and 2 calls", mock.listParams, mock.listCalls)
	}
}

func TestStoreListRangeReturnsFallbackError(t *testing.T) {
	day := time.Date(2026, 11, 20, 0, 0, 0, 0, time.UTC)
	userID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	fallbackErr := errors.New("fallback failure")
	mock := &mockQuerier{fallbackErr: fallbackErr}
	// The first call returns an empty page; the fallback call fails.
	_, _, err := NewStore(mock).ListRange(context.Background(), userID, day, day, 2, 8)
	if !errors.Is(err, fallbackErr) {
		t.Fatalf("ListRange() error = %v, want wrapped fallback error", err)
	}
	if mock.listCalls != 2 || mock.listParams.Offset != 0 || mock.listParams.Limit != 1 {
		t.Fatalf("fallback params/calls = %+v / %d, want offset 0 limit 1 and 2 calls", mock.listParams, mock.listCalls)
	}
}

func (m *mockQuerier) UpdatePlannedSession(_ context.Context, arg sqlc.UpdatePlannedSessionParams) (sqlc.PlannedSession, error) {
	m.updateParams = arg
	return m.row, nil
}

func (m *mockQuerier) DeletePlannedSession(_ context.Context, arg sqlc.DeletePlannedSessionParams) (int64, error) {
	m.deleteParams = arg
	return m.deletedRows, nil
}

func (m *mockQuerier) LinkPlannedSessionActivity(_ context.Context, arg sqlc.LinkPlannedSessionActivityParams) (sqlc.PlannedSession, error) {
	m.linkParams = arg
	if m.linkErr != nil {
		return sqlc.PlannedSession{}, m.linkErr
	}
	return m.row, nil
}

func (m *mockQuerier) UnlinkPlannedSessionActivity(_ context.Context, arg sqlc.UnlinkPlannedSessionActivityParams) (sqlc.PlannedSession, error) {
	m.unlinkParams = arg
	if m.unlinkErr != nil {
		return sqlc.PlannedSession{}, m.unlinkErr
	}
	return m.row, nil
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

// TestStoreDefaultsEmptyStatusToPlanned proves el estado vacio (sesion
// recien construida por el handler) persiste como 'planned', el DEFAULT
// de la migracion 00017.
func TestStoreDefaultsEmptyStatusToPlanned(t *testing.T) {
	userID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	sessionID := uuid.MustParse("33333333-3333-3333-3333-333333333333")
	day := time.Date(2026, 11, 20, 0, 0, 0, 0, time.UTC)
	mock := &mockQuerier{}
	store := NewStore(mock)
	ctx := context.Background()

	if _, err := store.Create(ctx, userID, Session{ScheduledOn: day, Title: "Rodaje", WorkoutType: WorkoutTypeRodaje}); err != nil {
		t.Fatalf("Create() unexpected error: %v (el estado vacio debe validarse como planned, no rechazarse)", err)
	}
	// El INSERT no lleva status: aplica el DEFAULT 'planned' de la tabla,
	// que el test de integracion (migracion 00017) verifica de punta a punta.

	if _, err := store.Update(ctx, userID, sessionID, Session{ScheduledOn: day, Title: "Rodaje", WorkoutType: WorkoutTypeRodaje}); err != nil {
		t.Fatalf("Update() unexpected error: %v", err)
	}
	if got := Status(mock.updateParams.Status); got != StatusPlanned {
		t.Errorf("Update() status = %q, want %q", got, StatusPlanned)
	}
}

// TestStoreLinkActivityScopesAndMapsErrors proves la vinculacion lleva el
// user_id del llamador a los dos filtros (sesion y actividad) y traduce
// pgx.ErrNoRows y la violacion de UNIQUE a los errores de dominio.
func TestStoreLinkActivityScopesAndMapsErrors(t *testing.T) {
	userID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	sessionID := uuid.MustParse("33333333-3333-3333-3333-333333333333")
	activityID := uuid.MustParse("44444444-4444-4444-4444-444444444444")
	row := sqlc.PlannedSession{
		ID:          pgtype.UUID{Bytes: sessionID, Valid: true},
		UserID:      pgtype.UUID{Bytes: userID, Valid: true},
		ActivityID:  pgtype.UUID{Bytes: activityID, Valid: true},
		Status:      string(StatusCompleted),
		ScheduledOn: pgtype.Date{Time: time.Date(2026, 11, 20, 0, 0, 0, 0, time.UTC), Valid: true},
		Title:       "Rodaje",
		WorkoutType: string(WorkoutTypeRodaje),
	}
	ctx := context.Background()

	linked, err := NewStore(&mockQuerier{row: row}).LinkActivity(ctx, userID, sessionID, activityID)
	if err != nil {
		t.Fatalf("LinkActivity() unexpected error: %v", err)
	}
	if linked.Status != StatusCompleted || linked.ActivityID == nil || *linked.ActivityID != activityID {
		t.Errorf("LinkActivity() = %+v, want status completed and activity %s", linked, activityID)
	}

	mock := &mockQuerier{linkErr: pgx.ErrNoRows}
	if _, err := NewStore(mock).LinkActivity(ctx, userID, sessionID, activityID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("LinkActivity() error = %v, want ErrNotFound", err)
	}
	if mock.linkParams.UserID.Bytes != userID || mock.linkParams.ID.Bytes != sessionID || mock.linkParams.ActivityID.Bytes != activityID {
		t.Errorf("LinkActivity() params = %+v, want caller-scoped session and activity", mock.linkParams)
	}

	duplicate := &pgconn.PgError{Code: "23505"}
	if _, err := NewStore(&mockQuerier{linkErr: duplicate}).LinkActivity(ctx, userID, sessionID, activityID); !errors.Is(err, ErrActivityInUse) {
		t.Fatalf("LinkActivity() error = %v, want ErrActivityInUse", err)
	}

	boom := errors.New("connection refused")
	if _, err := NewStore(&mockQuerier{linkErr: boom}).LinkActivity(ctx, userID, sessionID, activityID); !errors.Is(err, boom) || errors.Is(err, ErrNotFound) {
		t.Fatalf("LinkActivity() error = %v, want the wrapped failure and no sentinel", err)
	}
}

// TestStoreUnlinkActivityScopesAndMapsErrors proves la desvinculacion
// limpia el enlace sin tocar el estado y mantiene el aislamiento.
func TestStoreUnlinkActivityScopesAndMapsErrors(t *testing.T) {
	userID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	sessionID := uuid.MustParse("33333333-3333-3333-3333-333333333333")
	day := time.Date(2026, 11, 20, 0, 0, 0, 0, time.UTC)
	row := sqlc.PlannedSession{
		ID:          pgtype.UUID{Bytes: sessionID, Valid: true},
		UserID:      pgtype.UUID{Bytes: userID, Valid: true},
		Status:      string(StatusSkipped),
		ScheduledOn: pgtype.Date{Time: day, Valid: true},
		Title:       "Rodaje",
		WorkoutType: string(WorkoutTypeRodaje),
	}
	ctx := context.Background()

	unlinked, err := NewStore(&mockQuerier{row: row}).UnlinkActivity(ctx, userID, sessionID)
	if err != nil {
		t.Fatalf("UnlinkActivity() unexpected error: %v", err)
	}
	if unlinked.Status != StatusSkipped || unlinked.ActivityID != nil {
		t.Errorf("UnlinkActivity() = %+v, want status kept and no activity", unlinked)
	}

	mock := &mockQuerier{unlinkErr: pgx.ErrNoRows}
	if _, err := NewStore(mock).UnlinkActivity(ctx, userID, sessionID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("UnlinkActivity() error = %v, want ErrNotFound", err)
	}
	if mock.unlinkParams.UserID.Bytes != userID || mock.unlinkParams.ID.Bytes != sessionID {
		t.Errorf("UnlinkActivity() params = %+v, want caller-scoped session", mock.unlinkParams)
	}
}
