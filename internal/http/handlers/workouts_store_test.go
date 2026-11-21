package handlers

import (
	"context"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/fgjcarlos/ghamusinos/internal/db/sqlc"
	"github.com/fgjcarlos/ghamusinos/internal/workouts"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stretchr/testify/require"
)

type workoutQuerierStub struct {
	row         sqlc.PlannedSession
	getParams   sqlc.GetPlannedSessionParams
	updateCalls int
	listCalls   int
	listError   error
}

func (q *workoutQuerierStub) CreatePlannedSession(_ context.Context, arg sqlc.CreatePlannedSessionParams) (sqlc.PlannedSession, error) {
	return q.row, nil
}
func (q *workoutQuerierStub) GetPlannedSession(_ context.Context, arg sqlc.GetPlannedSessionParams) (sqlc.PlannedSession, error) {
	q.getParams = arg
	if arg.UserID != q.row.UserID || arg.ID != q.row.ID {
		return sqlc.PlannedSession{}, pgx.ErrNoRows
	}
	return q.row, nil
}
func (q *workoutQuerierStub) ListPlannedSessionsByRange(_ context.Context, arg sqlc.ListPlannedSessionsByRangeParams) ([]sqlc.ListPlannedSessionsByRangeRow, error) {
	q.listCalls++
	if q.listError != nil && q.listCalls > 1 {
		return nil, q.listError
	}
	if q.listCalls == 1 {
		return nil, nil
	}
	return []sqlc.ListPlannedSessionsByRangeRow{{TotalCount: 1}}, nil
}
func (q *workoutQuerierStub) UpdatePlannedSession(_ context.Context, _ sqlc.UpdatePlannedSessionParams) (sqlc.PlannedSession, error) {
	q.updateCalls++
	return q.row, nil
}
func (q *workoutQuerierStub) DeletePlannedSession(context.Context, sqlc.DeletePlannedSessionParams) (int64, error) {
	return 0, nil
}
func (q *workoutQuerierStub) LinkPlannedSessionActivity(_ context.Context, _ sqlc.LinkPlannedSessionActivityParams) (sqlc.PlannedSession, error) {
	return q.row, nil
}
func (q *workoutQuerierStub) UnlinkPlannedSessionActivity(_ context.Context, _ sqlc.UnlinkPlannedSessionActivityParams) (sqlc.PlannedSession, error) {
	return q.row, nil
}

func TestWorkoutHandlerUsesStoreForIsolationAtomicityAndPaging(t *testing.T) {
	userID := uuid.MustParse(workoutTestUser)
	id := uuid.MustParse(workoutTestID)
	day := time.Date(2026, 11, 20, 0, 0, 0, 0, time.UTC)
	row := sqlc.PlannedSession{ID: pgtype.UUID{Bytes: id, Valid: true}, UserID: pgtype.UUID{Bytes: userID, Valid: true}, ScheduledOn: pgtype.Date{Time: day, Valid: true}, Title: "Easy", WorkoutType: "rodaje", TargetDurationSec: pgtype.Int4{Int32: 3600, Valid: true}}
	q := &workoutQuerierStub{row: row}
	store := workouts.NewStore(q)

	t.Run("other owner is hidden through store query scope", func(t *testing.T) {
		rec := httptest.NewRecorder()
		GetWorkout(store).ServeHTTP(rec, workoutRequest("GET", "/api/v1/workouts/"+workoutTestID, "", otherWorkoutUser))
		require.Equal(t, 404, rec.Code)
		require.Equal(t, uuid.MustParse(otherWorkoutUser), uuid.UUID(q.getParams.UserID.Bytes))
	})

	t.Run("invalid candidate never reaches store update", func(t *testing.T) {
		before := q.row
		rec := httptest.NewRecorder()
		PatchWorkout(store).ServeHTTP(rec, workoutRequest("PATCH", "/api/v1/workouts/"+workoutTestID, `{"title":"Changed","target_duration_sec":-1}`, workoutTestUser))
		require.Equal(t, 422, rec.Code)
		require.Zero(t, q.updateCalls)
		require.Equal(t, before, q.row)
	})

	t.Run("empty page beyond end recovers total through store", func(t *testing.T) {
		rec := httptest.NewRecorder()
		ListWorkouts(store).ServeHTTP(rec, workoutRequest("GET", "/api/v1/workouts?from=2026-11-01&to=2026-11-30&offset=10", "", workoutTestUser))
		require.Equal(t, 200, rec.Code)
		require.JSONEq(t, `{"data":[],"total":1,"limit":20,"offset":10,"has_next":false}`, rec.Body.String())
		require.Equal(t, 2, q.listCalls)
	})
}
