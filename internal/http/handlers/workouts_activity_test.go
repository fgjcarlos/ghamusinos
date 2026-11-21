package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/fgjcarlos/ghamusinos/internal/auth"
	"github.com/fgjcarlos/ghamusinos/internal/workouts"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

// Fase 2.2 (issue #311): estado de la sesion y vinculacion de actividad.
//
// Los mocks replican la semantica de las queries atomicas: la sesion y la
// actividad tienen que ser del llamador, y una actividad enlazada a otra
// sesion choca con el UNIQUE(activity_id).

const workoutActivityID = "00000000-0000-0000-0000-000000000004"
const otherUserActivityID = "00000000-0000-0000-0000-000000000005"

// LinkActivity aplica las reglas del UPDATE ... FROM activities: sesion y
// actividad del llamador, estado a completed salvo modified, y el
// UNIQUE(activity_id) como frontera de duplicado.
func (s *workoutsMemoryStore) LinkActivity(_ context.Context, uid, sessionID, activityID uuid.UUID) (workouts.Session, error) {
	if s.linkErr != nil {
		return workouts.Session{}, s.linkErr
	}
	v, ok := s.rows[sessionID]
	if !ok || v.UserID != uid {
		return workouts.Session{}, workouts.ErrNotFound
	}
	owner, ok := s.activities[activityID]
	if !ok || owner != uid {
		return workouts.Session{}, workouts.ErrNotFound
	}
	for _, other := range s.rows {
		if other.ActivityID != nil && *other.ActivityID == activityID && other.ID != sessionID {
			return workouts.Session{}, workouts.ErrActivityInUse
		}
	}
	id := activityID
	v.ActivityID = &id
	if v.Status != workouts.StatusModified {
		v.Status = workouts.StatusCompleted
	}
	s.rows[sessionID] = v
	return v, nil
}

// UnlinkActivity limpia el enlace sin tocar el estado ni la actividad.
func (s *workoutsMemoryStore) UnlinkActivity(_ context.Context, uid, sessionID uuid.UUID) (workouts.Session, error) {
	if s.unlinkErr != nil {
		return workouts.Session{}, s.unlinkErr
	}
	v, ok := s.rows[sessionID]
	if !ok || v.UserID != uid {
		return workouts.Session{}, workouts.ErrNotFound
	}
	v.ActivityID = nil
	s.rows[sessionID] = v
	return v, nil
}

// workoutActivityRequest construye una peticion a /workouts/{id}/activity
// con el {id} de ruta resuelto a mano, igual que workoutRequest.
func workoutActivityRequest(method, body, user string) *http.Request {
	r := httptest.NewRequestWithContext(context.Background(), method, "/api/v1/workouts/"+workoutTestID+"/activity", strings.NewReader(body))
	ctx := chi.NewRouteContext()
	ctx.URLParams.Add("id", workoutTestID)
	r = r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, ctx))
	if user != "" {
		r = r.WithContext(auth.WithAuthUser(r.Context(), &auth.User{ID: user}))
	}
	return r
}

func linkedSessionStore(status workouts.Status) *workoutsMemoryStore {
	id := uuid.MustParse(workoutTestID)
	activity := uuid.MustParse(workoutActivityID)
	session := workouts.Session{
		ID:          id,
		UserID:      uuid.MustParse(workoutTestUser),
		ScheduledOn: time.Date(2026, 11, 20, 0, 0, 0, 0, time.UTC),
		Title:       "Easy",
		WorkoutType: workouts.WorkoutTypeRodaje,
		Status:      status,
	}
	return &workoutsMemoryStore{
		rows: map[uuid.UUID]workouts.Session{id: session},
		activities: map[uuid.UUID]uuid.UUID{
			activity:                            uuid.MustParse(workoutTestUser),
			uuid.MustParse(otherUserActivityID): uuid.MustParse(otherWorkoutUser),
		},
	}
}

func TestPatchWorkoutStatus(t *testing.T) {
	id := uuid.MustParse(workoutTestID)

	t.Run("valid status updates the session", func(t *testing.T) {
		s := linkedSessionStore(workouts.StatusPlanned)
		rec := httptest.NewRecorder()
		PatchWorkout(s).ServeHTTP(rec, workoutRequest(http.MethodPatch, "/api/v1/workouts/"+workoutTestID, `{"status":"completed"}`, workoutTestUser))
		require.Equal(t, http.StatusOK, rec.Code)
		require.Equal(t, workouts.StatusCompleted, s.rows[id].Status)
		var b map[string]any
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &b))
		require.Equal(t, "completed", b["status"])
	})

	t.Run("unknown status is rejected without touching the stored session", func(t *testing.T) {
		s := linkedSessionStore(workouts.StatusPlanned)
		before := s.rows[id]
		rec := httptest.NewRecorder()
		PatchWorkout(s).ServeHTTP(rec, workoutRequest(http.MethodPatch, "/api/v1/workouts/"+workoutTestID, `{"status":"teleported"}`, workoutTestUser))
		require.Equal(t, http.StatusUnprocessableEntity, rec.Code)
		require.Equal(t, before, s.rows[id], "invalid status changed the stored session")
		require.False(t, s.updated)
	})

	t.Run("null status is a bad request and leaves the session untouched", func(t *testing.T) {
		s := linkedSessionStore(workouts.StatusPlanned)
		before := s.rows[id]
		rec := httptest.NewRecorder()
		PatchWorkout(s).ServeHTTP(rec, workoutRequest(http.MethodPatch, "/api/v1/workouts/"+workoutTestID, `{"status":null}`, workoutTestUser))
		require.Equal(t, http.StatusBadRequest, rec.Code)
		require.Equal(t, before, s.rows[id])
	})

	t.Run("an ordinary patch preserves the status", func(t *testing.T) {
		s := linkedSessionStore(workouts.StatusSkipped)
		rec := httptest.NewRecorder()
		PatchWorkout(s).ServeHTTP(rec, workoutRequest(http.MethodPatch, "/api/v1/workouts/"+workoutTestID, `{"notes":"moved to evening"}`, workoutTestUser))
		require.Equal(t, http.StatusOK, rec.Code)
		require.Equal(t, workouts.StatusSkipped, s.rows[id].Status)
	})
}

func TestLinkWorkoutActivity(t *testing.T) {
	id := uuid.MustParse(workoutTestID)
	activity := uuid.MustParse(workoutActivityID)

	t.Run("linking completes the session and returns it", func(t *testing.T) {
		s := linkedSessionStore(workouts.StatusPlanned)
		rec := httptest.NewRecorder()
		LinkWorkoutActivity(s).ServeHTTP(rec, workoutActivityRequest(http.MethodPut, `{"activity_id":"`+workoutActivityID+`"}`, workoutTestUser))
		require.Equal(t, http.StatusOK, rec.Code)
		require.NotNil(t, s.rows[id].ActivityID)
		require.Equal(t, activity, *s.rows[id].ActivityID)
		require.Equal(t, workouts.StatusCompleted, s.rows[id].Status)
		var b map[string]any
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &b))
		require.Equal(t, workoutActivityID, b["activity_id"])
		require.Equal(t, "completed", b["status"])
	})

	t.Run("linking keeps a session already marked modified", func(t *testing.T) {
		s := linkedSessionStore(workouts.StatusModified)
		rec := httptest.NewRecorder()
		LinkWorkoutActivity(s).ServeHTTP(rec, workoutActivityRequest(http.MethodPut, `{"activity_id":"`+workoutActivityID+`"}`, workoutTestUser))
		require.Equal(t, http.StatusOK, rec.Code)
		require.Equal(t, workouts.StatusModified, s.rows[id].Status)
	})

	t.Run("other user's session and other user's activity are 404", func(t *testing.T) {
		s := linkedSessionStore(workouts.StatusPlanned)
		rec := httptest.NewRecorder()
		LinkWorkoutActivity(s).ServeHTTP(rec, workoutActivityRequest(http.MethodPut, `{"activity_id":"`+workoutActivityID+`"}`, otherWorkoutUser))
		require.Equal(t, http.StatusNotFound, rec.Code)
		require.Nil(t, s.rows[id].ActivityID)

		rec = httptest.NewRecorder()
		LinkWorkoutActivity(s).ServeHTTP(rec, workoutActivityRequest(http.MethodPut, `{"activity_id":"`+otherUserActivityID+`"}`, workoutTestUser))
		require.Equal(t, http.StatusNotFound, rec.Code)
		require.Nil(t, s.rows[id].ActivityID, "a foreign activity must not be linked")
	})

	t.Run("missing activity is 404", func(t *testing.T) {
		s := linkedSessionStore(workouts.StatusPlanned)
		rec := httptest.NewRecorder()
		LinkWorkoutActivity(s).ServeHTTP(rec, workoutActivityRequest(http.MethodPut, `{"activity_id":"`+uuid.New().String()+`"}`, workoutTestUser))
		require.Equal(t, http.StatusNotFound, rec.Code)
	})

	t.Run("activity already linked to another session is 409", func(t *testing.T) {
		s := linkedSessionStore(workouts.StatusPlanned)
		other := workouts.Session{ID: uuid.New(), UserID: uuid.MustParse(workoutTestUser), ScheduledOn: time.Date(2026, 11, 20, 0, 0, 0, 0, time.UTC), Title: "Other", WorkoutType: workouts.WorkoutTypeRodaje, ActivityID: &activity}
		s.rows[other.ID] = other
		rec := httptest.NewRecorder()
		LinkWorkoutActivity(s).ServeHTTP(rec, workoutActivityRequest(http.MethodPut, `{"activity_id":"`+workoutActivityID+`"}`, workoutTestUser))
		require.Equal(t, http.StatusConflict, rec.Code)
		require.Contains(t, rec.Header().Get("Content-Type"), "application/problem+json")
		require.Nil(t, s.rows[id].ActivityID, "a duplicate link must not stick")
	})

	t.Run("invalid bodies are 400 and leave the store untouched", func(t *testing.T) {
		for _, body := range []string{`{}`, `{"activity_id":null}`, `{"activity_id":"not-a-uuid"}`, `{"activity_id":"` + workoutActivityID + `","extra":1}`} {
			s := linkedSessionStore(workouts.StatusPlanned)
			rec := httptest.NewRecorder()
			LinkWorkoutActivity(s).ServeHTTP(rec, workoutActivityRequest(http.MethodPut, body, workoutTestUser))
			require.Equal(t, http.StatusBadRequest, rec.Code, body)
			require.Nil(t, s.rows[id].ActivityID, body)
		}
	})

	t.Run("store failures map to 500 and unauthenticated to 401", func(t *testing.T) {
		s := linkedSessionStore(workouts.StatusPlanned)
		s.linkErr = errors.New("db")
		rec := httptest.NewRecorder()
		LinkWorkoutActivity(s).ServeHTTP(rec, workoutActivityRequest(http.MethodPut, `{"activity_id":"`+workoutActivityID+`"}`, workoutTestUser))
		require.Equal(t, http.StatusInternalServerError, rec.Code)

		s2 := linkedSessionStore(workouts.StatusPlanned)
		rec = httptest.NewRecorder()
		LinkWorkoutActivity(s2).ServeHTTP(rec, workoutActivityRequest(http.MethodPut, `{"activity_id":"`+workoutActivityID+`"}`, ""))
		require.Equal(t, http.StatusUnauthorized, rec.Code)
	})
}

func TestUnlinkWorkoutActivity(t *testing.T) {
	id := uuid.MustParse(workoutTestID)
	activity := uuid.MustParse(workoutActivityID)

	newLinkedStore := func() *workoutsMemoryStore {
		s := linkedSessionStore(workouts.StatusPlanned)
		session := s.rows[id]
		session.ActivityID = &activity
		session.Status = workouts.StatusCompleted
		s.rows[id] = session
		return s
	}

	t.Run("unlink clears the link, keeps the status and keeps the activity", func(t *testing.T) {
		s := newLinkedStore()
		rec := httptest.NewRecorder()
		UnlinkWorkoutActivity(s).ServeHTTP(rec, workoutActivityRequest(http.MethodDelete, "", workoutTestUser))
		require.Equal(t, http.StatusOK, rec.Code)
		require.Nil(t, s.rows[id].ActivityID)
		require.Equal(t, workouts.StatusCompleted, s.rows[id].Status, "unlink must not change the status")
		_, stillThere := s.activities[activity]
		require.True(t, stillThere, "unlink deleted the activity")
		var b map[string]any
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &b))
		require.Nil(t, b["activity_id"])
		require.Equal(t, "completed", b["status"])
	})

	t.Run("unlinking a session without a link is idempotent", func(t *testing.T) {
		s := linkedSessionStore(workouts.StatusPlanned)
		rec := httptest.NewRecorder()
		UnlinkWorkoutActivity(s).ServeHTTP(rec, workoutActivityRequest(http.MethodDelete, "", workoutTestUser))
		require.Equal(t, http.StatusOK, rec.Code)
		require.Nil(t, s.rows[id].ActivityID)
	})

	t.Run("other user's session is 404 and store failure is 500", func(t *testing.T) {
		s := newLinkedStore()
		rec := httptest.NewRecorder()
		UnlinkWorkoutActivity(s).ServeHTTP(rec, workoutActivityRequest(http.MethodDelete, "", otherWorkoutUser))
		require.Equal(t, http.StatusNotFound, rec.Code)

		s2 := newLinkedStore()
		s2.unlinkErr = errors.New("db")
		rec = httptest.NewRecorder()
		UnlinkWorkoutActivity(s2).ServeHTTP(rec, workoutActivityRequest(http.MethodDelete, "", workoutTestUser))
		require.Equal(t, http.StatusInternalServerError, rec.Code)
	})

	t.Run("unauthenticated is 401", func(t *testing.T) {
		s := newLinkedStore()
		rec := httptest.NewRecorder()
		UnlinkWorkoutActivity(s).ServeHTTP(rec, workoutActivityRequest(http.MethodDelete, "", ""))
		require.Equal(t, http.StatusUnauthorized, rec.Code)
	})
}
