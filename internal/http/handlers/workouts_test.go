package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/fgjcarlos/ghamusinos/internal/auth"
	"github.com/fgjcarlos/ghamusinos/internal/workouts"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

const workoutTestUser = "00000000-0000-0000-0000-000000000001"
const otherWorkoutUser = "00000000-0000-0000-0000-000000000002"
const workoutTestID = "00000000-0000-0000-0000-000000000003"

type workoutsMemoryStore struct {
	rows      map[uuid.UUID]workouts.Session
	updated   bool
	creates   int
	createdID uuid.UUID
	createErr error
	getErr    error
	listErr   error
	updateErr error
	deleteErr error
}

func (s *workoutsMemoryStore) Create(_ context.Context, uid uuid.UUID, v workouts.Session) (workouts.Session, error) {
	s.creates++
	if s.createErr != nil {
		return workouts.Session{}, s.createErr
	}
	if s.createdID != uuid.Nil {
		v.ID = s.createdID
	} else {
		v.ID = uuid.New()
	}
	v.UserID = uid
	s.rows[v.ID] = v
	return v, nil
}
func (s *workoutsMemoryStore) Get(_ context.Context, uid, id uuid.UUID) (workouts.Session, error) {
	if s.getErr != nil {
		return workouts.Session{}, s.getErr
	}
	v, ok := s.rows[id]
	if !ok || v.UserID != uid {
		return workouts.Session{}, workouts.ErrNotFound
	}
	return v, nil
}
func (s *workoutsMemoryStore) ListRange(_ context.Context, uid uuid.UUID, from, to time.Time, limit, offset int32) ([]workouts.Session, int64, error) {
	if s.listErr != nil {
		return nil, 0, s.listErr
	}
	var out []workouts.Session
	for _, v := range s.rows {
		if v.UserID == uid && !v.ScheduledOn.Before(from) && !v.ScheduledOn.After(to) {
			out = append(out, v)
		}
	}
	return out, int64(len(out)), nil
}
func (s *workoutsMemoryStore) Update(_ context.Context, uid, id uuid.UUID, v workouts.Session) (workouts.Session, error) {
	if s.updateErr != nil {
		return workouts.Session{}, s.updateErr
	}
	old, ok := s.rows[id]
	if !ok || old.UserID != uid {
		return workouts.Session{}, workouts.ErrNotFound
	}
	v.ID = id
	v.UserID = uid
	s.rows[id] = v
	s.updated = true
	return v, nil
}
func (s *workoutsMemoryStore) Delete(_ context.Context, uid, id uuid.UUID) error {
	if s.deleteErr != nil {
		return s.deleteErr
	}
	v, ok := s.rows[id]
	if !ok || v.UserID != uid {
		return workouts.ErrNotFound
	}
	delete(s.rows, id)
	return nil
}

func workoutRequest(method, path, body, user string) *http.Request {
	r := httptest.NewRequestWithContext(context.Background(), method, path, strings.NewReader(body))
	ctx := chi.NewRouteContext()
	ctx.URLParams.Add("id", strings.TrimPrefix(path, "/api/v1/workouts/"))
	r = r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, ctx))
	if user != "" {
		r = r.WithContext(auth.WithAuthUser(r.Context(), &auth.User{ID: user}))
	}
	return r
}
func intPointer(value int) *int { return &value }

func TestWorkoutsHandlers(t *testing.T) {
	id := uuid.MustParse(workoutTestID)
	base := workouts.Session{ID: id, UserID: uuid.MustParse(workoutTestUser), ScheduledOn: time.Date(2026, 11, 20, 0, 0, 0, 0, time.UTC), Title: "Easy", WorkoutType: workouts.WorkoutTypeRodaje}
	t.Run("create golden JSON numbers", func(t *testing.T) {
		s := &workoutsMemoryStore{rows: map[uuid.UUID]workouts.Session{}, createdID: id}
		rec := httptest.NewRecorder()
		CreateWorkout(s).ServeHTTP(rec, workoutRequest("POST", "/api/v1/workouts", `{"scheduled_on":"2026-11-20","title":"Easy","workout_type":"rodaje","target_duration_sec":3600}`, workoutTestUser))
		require.Equal(t, 201, rec.Code)
		require.JSONEq(t, `{"id":"00000000-0000-0000-0000-000000000003","scheduled_on":"2026-11-20","title":"Easy","notes":"","workout_type":"rodaje","target_duration_sec":3600,"target_distance_m":null,"target_elevation_gain_m":null,"target_intensity":null}`, rec.Body.String())
	})
	t.Run("create validation and malformed trailing JSON", func(t *testing.T) {
		for _, tc := range []struct {
			body   string
			status int
		}{{`{"scheduled_on":"2026-11-20","title":"","workout_type":"rodaje"}`, 422}, {`{"scheduled_on":"2026-11-20","title":"x","workout_type":"rodaje"} {}`, 400}} {
			s := &workoutsMemoryStore{rows: map[uuid.UUID]workouts.Session{}}
			rec := httptest.NewRecorder()
			CreateWorkout(s).ServeHTTP(rec, workoutRequest("POST", "/api/v1/workouts", tc.body, workoutTestUser))
			require.Equal(t, tc.status, rec.Code)
		}
	})
	t.Run("list pagination validates each query independently", func(t *testing.T) {
		s := &workoutsMemoryStore{rows: map[uuid.UUID]workouts.Session{id: base}}
		rec := httptest.NewRecorder()
		ListWorkouts(s).ServeHTTP(rec, workoutRequest("GET", "/api/v1/workouts?from=2026-11-01&to=2026-11-30&limit=10&offset=0", "", workoutTestUser))
		require.Equal(t, 200, rec.Code)
		var b map[string]any
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &b))
		require.Equal(t, float64(10), b["limit"])
		require.JSONEq(t, `{"data":[{"id":"00000000-0000-0000-0000-000000000003","scheduled_on":"2026-11-20","title":"Easy","notes":"","workout_type":"rodaje","target_duration_sec":null,"target_distance_m":null,"target_elevation_gain_m":null,"target_intensity":null}],"total":1,"limit":10,"offset":0,"has_next":false}`, rec.Body.String())
		for _, query := range []string{"from=2026-11-01&to=2026-11-30&limit=nope", "from=2026-11-01&to=2026-11-30&offset=-1", "from=2026-11-01&to=2026-11-30&offset=2147483648", "from=2026-11-30&to=2026-11-01", "from=2026-11-01&to=2026-02-30"} {
			rec = httptest.NewRecorder()
			ListWorkouts(s).ServeHTTP(rec, workoutRequest("GET", "/api/v1/workouts?"+query, "", workoutTestUser))
			require.Equal(t, http.StatusBadRequest, rec.Code, query)
		}
	})
	t.Run("cross user get patch delete are 404", func(t *testing.T) {
		for _, tc := range []struct {
			m, p, b string
			h       func(WorkoutStore) http.Handler
		}{{"GET", "/api/v1/workouts/" + workoutTestID, "", GetWorkout}, {"PATCH", "/api/v1/workouts/" + workoutTestID, `{"title":"Changed"}`, PatchWorkout}, {"DELETE", "/api/v1/workouts/" + workoutTestID, "", DeleteWorkout}} {
			s := &workoutsMemoryStore{rows: map[uuid.UUID]workouts.Session{id: base}}
			rec := httptest.NewRecorder()
			tc.h(s).ServeHTTP(rec, workoutRequest(tc.m, tc.p, tc.b, otherWorkoutUser))
			require.Equal(t, 404, rec.Code)
		}
	})
	t.Run("invalid patch never updates", func(t *testing.T) {
		before := base
		before.TargetDurationSec = intPointer(3600)
		s := &workoutsMemoryStore{rows: map[uuid.UUID]workouts.Session{id: before}}
		rec := httptest.NewRecorder()
		PatchWorkout(s).ServeHTTP(rec, workoutRequest("PATCH", "/api/v1/workouts/"+workoutTestID, `{"title":"Changed","target_duration_sec":-1}`, workoutTestUser))
		require.Equal(t, http.StatusUnprocessableEntity, rec.Code)
		require.False(t, s.updated)
		require.True(t, reflect.DeepEqual(before, s.rows[id]), "invalid patch changed stored session: got %+v", s.rows[id])
	})
	t.Run("optional patch null clears and omission preserves", func(t *testing.T) {
		intensity := workouts.IntensityZ2
		before := base
		before.Notes = "keep"
		before.TargetDurationSec = intPointer(3600)
		before.TargetIntensity = &intensity
		s := &workoutsMemoryStore{rows: map[uuid.UUID]workouts.Session{id: before}}
		rec := httptest.NewRecorder()
		PatchWorkout(s).ServeHTTP(rec, workoutRequest("PATCH", "/api/v1/workouts/"+workoutTestID, `{"target_duration_sec":null}`, workoutTestUser))
		require.Equal(t, http.StatusOK, rec.Code)
		after := s.rows[id]
		require.Nil(t, after.TargetDurationSec)
		require.Equal(t, before.Title, after.Title)
		require.Equal(t, before.Notes, after.Notes)
		require.Equal(t, before.TargetIntensity, after.TargetIntensity)
	})
	t.Run("missing resource returns 404 for get patch delete", func(t *testing.T) {
		for _, tc := range []struct {
			method  string
			handler func(WorkoutStore) http.Handler
			body    string
		}{{http.MethodGet, GetWorkout, ""}, {http.MethodPatch, PatchWorkout, `{"title":"changed"}`}, {http.MethodDelete, DeleteWorkout, ""}} {
			rec := httptest.NewRecorder()
			tc.handler(&workoutsMemoryStore{rows: map[uuid.UUID]workouts.Session{}}).ServeHTTP(rec, workoutRequest(tc.method, "/api/v1/workouts/"+workoutTestID, tc.body, workoutTestUser))
			require.Equal(t, http.StatusNotFound, rec.Code)
			require.Contains(t, rec.Header().Get("Content-Type"), "application/problem+json")
		}
	})
	t.Run("create rejects oversized target values before store", func(t *testing.T) {
		for _, field := range []string{"target_duration_sec", "target_distance_m", "target_elevation_gain_m"} {
			s := &workoutsMemoryStore{rows: map[uuid.UUID]workouts.Session{}}
			body := `{"scheduled_on":"2026-11-20","title":"Easy","workout_type":"rodaje","` + field + `":2147483648}`
			rec := httptest.NewRecorder()
			CreateWorkout(s).ServeHTTP(rec, workoutRequest(http.MethodPost, "/api/v1/workouts", body, workoutTestUser))
			require.Equal(t, http.StatusUnprocessableEntity, rec.Code, field)
			require.Zero(t, s.creates, field)
		}
	})
	t.Run("body limit errors return 413", func(t *testing.T) {
		s := &workoutsMemoryStore{rows: map[uuid.UUID]workouts.Session{}}
		rec := httptest.NewRecorder()
		req := workoutRequest(http.MethodPost, "/api/v1/workouts", `{"scheduled_on":"2026-11-20","title":"Easy","workout_type":"rodaje"}`, workoutTestUser)
		req.Body = http.MaxBytesReader(rec, req.Body, 8)
		CreateWorkout(s).ServeHTTP(rec, req)
		require.Equal(t, http.StatusRequestEntityTooLarge, rec.Code)
		require.Zero(t, s.creates)
		rec = httptest.NewRecorder()
		req = workoutRequest(http.MethodPost, "/api/v1/workouts", `{}          `, workoutTestUser)
		req.Body = http.MaxBytesReader(rec, req.Body, 2)
		CreateWorkout(s).ServeHTTP(rec, req)
		require.Equal(t, http.StatusRequestEntityTooLarge, rec.Code, "limit hit while checking trailing JSON")
	})
	t.Run("all operations reject unauthenticated requests", func(t *testing.T) {
		s := &workoutsMemoryStore{rows: map[uuid.UUID]workouts.Session{}}
		for _, tc := range []struct {
			method string
			path   string
			body   string
			h      func(WorkoutStore) http.Handler
		}{{http.MethodPost, "/api/v1/workouts", `{}`, CreateWorkout}, {http.MethodGet, "/api/v1/workouts?from=2026-11-01&to=2026-11-30", "", ListWorkouts}, {http.MethodGet, "/api/v1/workouts/" + workoutTestID, "", GetWorkout}, {http.MethodPatch, "/api/v1/workouts/" + workoutTestID, `{}`, PatchWorkout}, {http.MethodDelete, "/api/v1/workouts/" + workoutTestID, "", DeleteWorkout}} {
			rec := httptest.NewRecorder()
			tc.h(s).ServeHTTP(rec, workoutRequest(tc.method, tc.path, tc.body, ""))
			require.Equal(t, http.StatusUnauthorized, rec.Code, tc.method+" "+tc.path)
		}
	})
	t.Run("store failures map to internal problem responses", func(t *testing.T) {
		broken := &workoutsMemoryStore{rows: map[uuid.UUID]workouts.Session{}, createErr: errors.New("db"), listErr: errors.New("db"), getErr: errors.New("db"), deleteErr: errors.New("db")}
		requests := []struct {
			method, path, body string
			handler            func(WorkoutStore) http.Handler
		}{
			{http.MethodPost, "/api/v1/workouts", `{"scheduled_on":"2026-11-20","title":"Easy","workout_type":"rodaje"}`, CreateWorkout},
			{http.MethodGet, "/api/v1/workouts?from=2026-11-01&to=2026-11-30", "", ListWorkouts},
			{http.MethodGet, "/api/v1/workouts/" + workoutTestID, "", GetWorkout},
			{http.MethodDelete, "/api/v1/workouts/" + workoutTestID, "", DeleteWorkout},
		}
		for _, tc := range requests {
			rec := httptest.NewRecorder()
			tc.handler(broken).ServeHTTP(rec, workoutRequest(tc.method, tc.path, tc.body, workoutTestUser))
			require.Equal(t, http.StatusInternalServerError, rec.Code, tc.method+" "+tc.path)
			require.Contains(t, rec.Header().Get("Content-Type"), "application/problem+json")
		}
		broken.getErr = nil
		broken.rows = map[uuid.UUID]workouts.Session{id: base}
		broken.updateErr = errors.New("db")
		rec := httptest.NewRecorder()
		PatchWorkout(broken).ServeHTTP(rec, workoutRequest(http.MethodPatch, "/api/v1/workouts/"+workoutTestID, `{"title":"updated"}`, workoutTestUser))
		require.Equal(t, http.StatusInternalServerError, rec.Code)
	})
	t.Run("get patch and delete happy paths", func(t *testing.T) {
		s := &workoutsMemoryStore{rows: map[uuid.UUID]workouts.Session{id: base}}
		rec := httptest.NewRecorder()
		GetWorkout(s).ServeHTTP(rec, workoutRequest("GET", "/api/v1/workouts/"+workoutTestID, "", workoutTestUser))
		require.Equal(t, 200, rec.Code)
		require.JSONEq(t, `{"id":"00000000-0000-0000-0000-000000000003","scheduled_on":"2026-11-20","title":"Easy","notes":"","workout_type":"rodaje","target_duration_sec":null,"target_distance_m":null,"target_elevation_gain_m":null,"target_intensity":null}`, rec.Body.String())
		rec = httptest.NewRecorder()
		PatchWorkout(s).ServeHTTP(rec, workoutRequest("PATCH", "/api/v1/workouts/"+workoutTestID, `{"title":"Tempo"}`, workoutTestUser))
		require.Equal(t, 200, rec.Code)
		require.Equal(t, "Tempo", s.rows[id].Title)
		require.JSONEq(t, `{"id":"00000000-0000-0000-0000-000000000003","scheduled_on":"2026-11-20","title":"Tempo","notes":"","workout_type":"rodaje","target_duration_sec":null,"target_distance_m":null,"target_elevation_gain_m":null,"target_intensity":null}`, rec.Body.String())
		rec = httptest.NewRecorder()
		DeleteWorkout(s).ServeHTTP(rec, workoutRequest("DELETE", "/api/v1/workouts/"+workoutTestID, "", workoutTestUser))
		require.Equal(t, 204, rec.Code)
	})
}
