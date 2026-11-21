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

// CalendarActivities (GET /api/v1/workouts/calendar-activities, #313
// T3): la bandeja del calendario. El store se extiende igual que Compare.

type calendarActivitiesCall struct {
	user uuid.UUID
	from time.Time
	to   time.Time
}

func (s *workoutsMemoryStore) CalendarActivities(_ context.Context, uid uuid.UUID, from, to time.Time) ([]workouts.CalendarActivity, error) {
	s.calendarActivitiesCalls = append(s.calendarActivitiesCalls, calendarActivitiesCall{user: uid, from: from, to: to})
	if s.calendarActivitiesErr != nil {
		return nil, s.calendarActivitiesErr
	}
	return s.calendarActivitiesOut, nil
}

func calendarActivitiesRequest(query string) *http.Request {
	r := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/v1/workouts/calendar-activities?"+query, nil)
	ctx := chi.NewRouteContext()
	r = r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, ctx))
	return r.WithContext(auth.WithAuthUser(r.Context(), &auth.User{ID: workoutTestUser}))
}

// TestCalendarActivitiesValidation: mismas fechas civiles estrictas que
// el listado; el store nunca se alcanza con input invalido.
func TestCalendarActivitiesValidation(t *testing.T) {
	for _, tc := range []struct{ name, query string }{
		{"invalid from", "from=2026-02-30&to=2026-11-30"},
		{"non strict from", "from=2026-11-1&to=2026-11-30"},
		{"invalid to", "from=2026-11-01&to=nonsense"},
		{"inverted range", "from=2026-11-30&to=2026-11-01"},
	} {
		s := &workoutsMemoryStore{}
		rec := httptest.NewRecorder()
		CalendarActivities(s).ServeHTTP(rec, calendarActivitiesRequest(tc.query))
		require.Equal(t, http.StatusBadRequest, rec.Code, tc.name)
		require.Contains(t, rec.Header().Get("Content-Type"), "application/problem+json", tc.name)
		require.Empty(t, s.calendarActivitiesCalls, tc.name)
	}
}

// TestCalendarActivitiesRequiresAuthentication.
func TestCalendarActivitiesRequiresAuthentication(t *testing.T) {
	r := calendarActivitiesRequest("from=2026-11-16&to=2026-11-22")
	r = r.WithContext(auth.WithAuthUser(r.Context(), nil))
	rec := httptest.NewRecorder()
	CalendarActivities(&workoutsMemoryStore{}).ServeHTTP(rec, r)
	require.Equal(t, http.StatusUnauthorized, rec.Code)
}

// TestCalendarActivitiesGoldenJSON fija el contrato del wire:
// activities[].{id,name,scheduled_on,linked}, fecha civil ISO local.
func TestCalendarActivitiesGoldenJSON(t *testing.T) {
	s := &workoutsMemoryStore{
		calendarActivitiesOut: []workouts.CalendarActivity{
			{ID: uuid.MustParse("00000000-0000-0000-0000-00000000000a"), Name: "Rodaje matinal", ScheduledOn: "2026-11-26", Linked: true},
			{ID: uuid.MustParse("00000000-0000-0000-0000-00000000000b"), Name: "Series", ScheduledOn: "2026-11-24", Linked: false},
		},
	}
	rec := httptest.NewRecorder()
	CalendarActivities(s).ServeHTTP(rec, calendarActivitiesRequest("from=2026-11-24&to=2026-11-30"))
	require.Equal(t, http.StatusOK, rec.Code)
	want := `{"activities":[` +
		`{"id":"00000000-0000-0000-0000-00000000000a","name":"Rodaje matinal","scheduled_on":"2026-11-26","linked":true},` +
		`{"id":"00000000-0000-0000-0000-00000000000b","name":"Series","scheduled_on":"2026-11-24","linked":false}` +
		`]}`
	require.Equal(t, want, strings.TrimSpace(rec.Body.String()))
	require.Len(t, s.calendarActivitiesCalls, 1)
	call := s.calendarActivitiesCalls[0]
	require.Equal(t, uuid.MustParse(workoutTestUser), call.user)
	require.True(t, call.from.Equal(time.Date(2026, 11, 24, 0, 0, 0, 0, time.UTC)))
	require.True(t, call.to.Equal(time.Date(2026, 11, 30, 0, 0, 0, 0, time.UTC)))
}

// TestCalendarActivitiesEmptyAndFailure: rango sin actividades devuelve
// un array vacio (no null) y un fallo del store es 500 problem+json.
func TestCalendarActivitiesEmptyAndFailure(t *testing.T) {
	s := &workoutsMemoryStore{}
	rec := httptest.NewRecorder()
	CalendarActivities(s).ServeHTTP(rec, calendarActivitiesRequest("from=2026-11-16&to=2026-11-22"))
	require.Equal(t, http.StatusOK, rec.Code)
	var body struct {
		Activities []json.RawMessage `json:"activities"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	require.Empty(t, body.Activities, "sin actividades el array es [], no null")

	broken := &workoutsMemoryStore{calendarActivitiesErr: errors.New("db")}
	rec = httptest.NewRecorder()
	CalendarActivities(broken).ServeHTTP(rec, calendarActivitiesRequest("from=2026-11-16&to=2026-11-22"))
	require.Equal(t, http.StatusInternalServerError, rec.Code)
	require.Contains(t, rec.Header().Get("Content-Type"), "application/problem+json")
}
