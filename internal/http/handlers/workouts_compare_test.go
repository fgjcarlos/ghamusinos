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
)

// compareCall captura lo que el handler pasa al store.
type compareCall struct {
	user  uuid.UUID
	from  time.Time
	to    time.Time
	group workouts.CompareGroup
}

func (s *workoutsMemoryStore) Compare(_ context.Context, uid uuid.UUID, from, to time.Time, group workouts.CompareGroup) ([]workouts.ComparePeriod, error) {
	s.compareCalls = append(s.compareCalls, compareCall{user: uid, from: from, to: to, group: group})
	if s.compareErr != nil {
		return nil, s.compareErr
	}
	return s.compareOut, nil
}

func workoutCompareRequest(query string) *http.Request {
	r := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/v1/workouts/compare?"+query, nil)
	ctx := chi.NewRouteContext()
	r = r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, ctx))
	return r.WithContext(auth.WithAuthUser(r.Context(), &auth.User{ID: workoutTestUser}))
}

// TestCompareWorkoutsValidation: el contrato del rango es el mismo del
// listado (fechas locales estrictas, from <= to) y group es obligatorio
// day|week; el store nunca se llama con input invalido.
func TestCompareWorkoutsValidation(t *testing.T) {
	cases := []struct{ name, query string }{
		{"missing group", "from=2026-11-16&to=2026-11-22"},
		{"empty group", "from=2026-11-16&to=2026-11-22&group="},
		{"unknown group", "from=2026-11-16&to=2026-11-22&group=month"},
		{"uppercase group", "from=2026-11-16&to=2026-11-22&group=WEEK"},
		{"invalid from", "from=2026-02-30&to=2026-11-22&group=day"},
		{"invalid to", "from=2026-11-16&to=nonsense&group=day"},
		{"inverted range", "from=2026-11-23&to=2026-11-16&group=week"},
	}
	for _, tc := range cases {
		s := &workoutsMemoryStore{rows: map[uuid.UUID]workouts.Session{}}
		rec := httptest.NewRecorder()
		CompareWorkouts(s).ServeHTTP(rec, workoutCompareRequest(tc.query))
		if rec.Code != http.StatusBadRequest {
			t.Errorf("%s: status = %d, want 400 (%s)", tc.name, rec.Code, rec.Body.String())
			continue
		}
		if !strings.Contains(rec.Header().Get("Content-Type"), "application/problem+json") {
			t.Errorf("%s: content type = %q, want problem+json", tc.name, rec.Header().Get("Content-Type"))
		}
		if len(s.compareCalls) != 0 {
			t.Errorf("%s: store reached with invalid input", tc.name)
		}
	}
}

// TestCompareWorkoutsRequiresAuthentication.
func TestCompareWorkoutsRequiresAuthentication(t *testing.T) {
	r := workoutCompareRequest("from=2026-11-16&to=2026-11-22&group=week")
	r = r.WithContext(auth.WithAuthUser(r.Context(), nil))
	rec := httptest.NewRecorder()
	CompareWorkouts(&workoutsMemoryStore{}).ServeHTTP(rec, r)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rec.Code)
	}
}

// TestCompareWorkoutsGoldenJSON fija el contrato explicito del wire:
// periods[].{period_start,planned,actual,compliance} y sus unidades.
func TestCompareWorkoutsGoldenJSON(t *testing.T) {
	s := &workoutsMemoryStore{
		rows: map[uuid.UUID]workouts.Session{},
		compareOut: []workouts.ComparePeriod{
			{
				PeriodStart: "2026-11-16",
				Planned:     workouts.CompareTotals{DurationSeconds: 3600, DistanceM: 10000, ElevationGainM: 300},
				Actual:      workouts.CompareTotals{DurationSeconds: 3600, DistanceM: 15000.5, ElevationGainM: 250.5},
				Compliance:  workouts.CompareCompliance{Total: 2, Completed: 1, Skipped: 1},
			},
			{
				PeriodStart: "2026-11-23",
				Actual:      workouts.CompareTotals{DurationSeconds: 1800, DistanceM: 5000, ElevationGainM: 100},
				Compliance:  workouts.CompareCompliance{Total: 3, Modified: 1},
			},
		},
	}
	rec := httptest.NewRecorder()
	CompareWorkouts(s).ServeHTTP(rec, workoutCompareRequest("from=2026-11-16&to=2026-11-29&group=week"))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (%s)", rec.Code, rec.Body.String())
	}
	want := `{"periods":[` +
		`{"period_start":"2026-11-16","planned":{"duration_seconds":3600,"distance_m":10000,"elevation_gain_m":300},"actual":{"duration_seconds":3600,"distance_m":15000.5,"elevation_gain_m":250.5},"compliance":{"total":2,"completed":1,"skipped":1,"modified":0}},` +
		`{"period_start":"2026-11-23","planned":{"duration_seconds":0,"distance_m":0,"elevation_gain_m":0},"actual":{"duration_seconds":1800,"distance_m":5000,"elevation_gain_m":100},"compliance":{"total":3,"completed":0,"skipped":0,"modified":1}}` +
		`]}`
	if want != strings.TrimSpace(rec.Body.String()) {
		t.Fatalf("body = %s, want %s", rec.Body.String(), want)
	}
	if len(s.compareCalls) != 1 {
		t.Fatalf("store calls = %d, want 1", len(s.compareCalls))
	}
	call := s.compareCalls[0]
	if call.user != uuid.MustParse(workoutTestUser) || call.group != workouts.CompareGroupWeek {
		t.Errorf("store call = %+v, want user %s and group week", call, workoutTestUser)
	}
	if call.from.Format("2006-01-02") != "2026-11-16" || call.to.Format("2006-01-02") != "2026-11-29" {
		t.Errorf("store range = [%s, %s], want [2026-11-16, 2026-11-29]", call.from.Format("2006-01-02"), call.to.Format("2006-01-02"))
	}
}

// TestCompareWorkoutsEmptyRange: sin registros el chart va vacio, con un
// array (no null).
func TestCompareWorkoutsEmptyRange(t *testing.T) {
	s := &workoutsMemoryStore{rows: map[uuid.UUID]workouts.Session{}}
	rec := httptest.NewRecorder()
	CompareWorkouts(s).ServeHTTP(rec, workoutCompareRequest("from=2026-11-16&to=2026-11-22&group=week"))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if got := strings.TrimSpace(rec.Body.String()); got != `{"periods":[]}` {
		t.Fatalf("body = %s, want {\"periods\":[]}", got)
	}
}

// TestCompareWorkoutsStoreFailureMapsTo500.
func TestCompareWorkoutsStoreFailureMapsTo500(t *testing.T) {
	s := &workoutsMemoryStore{rows: map[uuid.UUID]workouts.Session{}, compareErr: errors.New("db down")}
	rec := httptest.NewRecorder()
	CompareWorkouts(s).ServeHTTP(rec, workoutCompareRequest("from=2026-11-16&to=2026-11-22&group=day"))
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", rec.Code)
	}
	var problem map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &problem); err != nil {
		t.Fatalf("body is not a problem: %v", err)
	}
	if !strings.Contains(rec.Header().Get("Content-Type"), "application/problem+json") {
		t.Errorf("content type = %q, want problem+json", rec.Header().Get("Content-Type"))
	}
}
