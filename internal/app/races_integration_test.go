//go:build integration

package app

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/fgjcarlos/ghamusinos/internal/auth"
	"github.com/fgjcarlos/ghamusinos/internal/db/sqlc"
	"github.com/fgjcarlos/ghamusinos/internal/db/status"
	"github.com/fgjcarlos/ghamusinos/internal/gpx"
	"github.com/fgjcarlos/ghamusinos/internal/http/handlers"
	"github.com/fgjcarlos/ghamusinos/internal/races"
	"github.com/fgjcarlos/ghamusinos/internal/workouts"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
)

func TestIntegration_RaceGoalsCRUDAndOwnership(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		dsn = os.Getenv("DATABASE_URL")
	}
	if dsn == "" {
		t.Skip("integration requires a migrated test database")
	}
	ctx := t.Context()
	pool, err := pgxpool.New(ctx, dsn)
	require.NoError(t, err)
	t.Cleanup(pool.Close)
	q := sqlc.New(pool)
	newUser := func() string {
		key := uuid.NewString()
		u, err := q.CreateUser(ctx, sqlc.CreateUserParams{ClerkUserID: key, Email: key + "@example.test", InviteStatus: status.InviteStatusActive})
		require.NoError(t, err)
		t.Cleanup(func() {
			_, err := pool.Exec(context.Background(), "DELETE FROM users WHERE id = $1", u.ID)
			require.NoError(t, err)
		})
		return uuid.UUID(u.ID.Bytes).String()
	}
	newTrack := func(owner string) string {
		var id pgtype.UUID
		err := pool.QueryRow(ctx, `INSERT INTO gpx_tracks
			(user_id, name, file_hash, file_size_bytes, coordinates, distance_m, moving_time_s, d_plus_m, d_minus_m,
			 difficulty_score, difficulty_label, track_type)
			VALUES ($1, 'Summit', $2, 20, '[]', 42000, 5000, 1000, 1000, 20, 'beginner', 'circular') RETURNING id`, owner, uuid.NewString()).Scan(&id)
		require.NoError(t, err)
		return uuid.UUID(id.Bytes).String()
	}
	owner, other := newUser(), newUser()
	track, foreignTrack := newTrack(owner), newTrack(other)
	mux := chi.NewRouter()
	mux.Route("/api/v1", func(r chi.Router) {
		handlers.MountRaces(r, races.NewStore(q))
		w := workouts.NewStore(q)
		r.Post("/workouts", handlers.CreateWorkout(w).ServeHTTP)
		r.Get("/workouts/{id}", handlers.GetWorkout(w).ServeHTTP)
		r.Patch("/workouts/{id}", handlers.PatchWorkout(w).ServeHTTP)
		r.Get("/gpx/{id}", handlers.GetGPX(gpx.NewSQLCStore(q)).ServeHTTP)
	})
	request := func(user, method, path, body string, code int) *httptest.ResponseRecorder {
		t.Helper()
		r := httptest.NewRequestWithContext(ctx, method, "/api/v1"+path, strings.NewReader(body))
		r = r.WithContext(auth.WithAuthUser(r.Context(), &auth.User{ID: user}))
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, r)
		require.Equal(t, code, rec.Code, rec.Body.String())
		if code == http.StatusNoContent {
			require.Empty(t, rec.Body.String())
		}
		return rec
	}
	const raceDraft = `{"name":"Summit","race_date":"2027-06-10","distance_m":42000,"elevation_gain_m":1000,"priority":"A"}`
	badPriority := request(owner, "POST", "/races", strings.Replace(raceDraft, `"A"`, `"D"`, 1), 422)
	require.JSONEq(t, `{"type":"about:blank","title":"Unprocessable Entity","status":422,"detail":"name, race_date, distance_m, elevation_gain_m, priority or notes is invalid"}`, badPriority.Body.String())
	require.JSONEq(t, `{"data":[], "total":0, "limit":20, "offset":0, "has_next":false}`, request(owner, "GET", "/races", "", 200).Body.String())

	var race, second, foreign races.Race
	require.NoError(t, json.Unmarshal(request(owner, "POST", "/races", raceDraft, 201).Body.Bytes(), &race))
	require.NoError(t, json.Unmarshal(request(owner, "POST", "/races", strings.Replace(raceDraft, "2027-06-10", "2027-05-01", 1), 201).Body.Bytes(), &second))
	require.NoError(t, json.Unmarshal(request(other, "POST", "/races", raceDraft, 201).Body.Bytes(), &foreign))
	var page struct {
		Data    []races.Race `json:"data"`
		Total   int          `json:"total"`
		HasNext bool         `json:"has_next"`
	}
	require.NoError(t, json.Unmarshal(request(owner, "GET", "/races?limit=1", "", 200).Body.Bytes(), &page))
	require.Equal(t, 2, page.Total)
	require.True(t, page.HasNext)
	require.Len(t, page.Data, 1)
	require.Equal(t, second.ID, page.Data[0].ID)
	require.JSONEq(t, `{"data":[], "total":2, "limit":1, "offset":2, "has_next":false}`, request(owner, "GET", "/races?limit=1&offset=2", "", 200).Body.String())

	racePath, otherPath := "/races/"+race.ID.String(), "/races/"+foreign.ID.String()
	patch := strings.Replace(raceDraft, `"A"`, `"B"`, 1)
	var updated races.Race
	require.NoError(t, json.Unmarshal(request(owner, "PUT", racePath, patch, 200).Body.Bytes(), &updated))
	require.Equal(t, "B", updated.Priority)
	request(other, "GET", racePath, "", 404)
	request(other, "GET", racePath+"/progress", "", 404)
	request(owner, "GET", racePath+"/progress", "", 200)
	request(other, "PUT", racePath, raceDraft, 404)
	request(other, "DELETE", racePath, "", 404)

	link := racePath + "/tracks/" + track
	for _, method := range []string{"PUT", "DELETE"} {
		request(owner, method, racePath+"/tracks/"+foreignTrack, "", 404)
		request(owner, method, otherPath+"/tracks/"+track, "", 404)
	}
	request(owner, "DELETE", link, "", 204)
	request(owner, "PUT", link, "", 204)
	request(owner, "PUT", link, "", 204)
	request(owner, "DELETE", link, "", 204)
	request(owner, "PUT", link, "", 204)

	workout := `{"scheduled_on":"2027-05-01","title":"Hill repeats","workout_type":"cuestas","race_id":"%s"}`
	request(owner, "POST", "/workouts", strings.Replace(workout, "%s", foreign.ID.String(), 1), 404)
	var session struct {
		ID uuid.UUID `json:"id"`
	}
	require.NoError(t, json.Unmarshal(request(owner, "POST", "/workouts", strings.Replace(workout, "%s", race.ID.String(), 1), 201).Body.Bytes(), &session))
	workoutPath := "/workouts/" + session.ID.String()
	request(owner, "PATCH", workoutPath, `{"title":"Must not change","race_id":"`+foreign.ID.String()+`"}`, 404)
	request(other, "PATCH", workoutPath, `{"race_id":null}`, 404)
	var detail races.Detail
	require.NoError(t, json.Unmarshal(request(owner, "GET", racePath, "", 200).Body.Bytes(), &detail))
	require.Equal(t, "B", detail.Priority)
	require.Equal(t, []races.Track{{ID: uuid.MustParse(track), Name: "Summit"}}, detail.Tracks)
	require.Equal(t, []races.Session{{ID: session.ID, Title: "Hill repeats", ScheduledOn: "2027-05-01", Status: "planned"}}, detail.Sessions)

	request(owner, "DELETE", racePath, "", 204)
	request(owner, "GET", racePath, "", 404)
	request(owner, "GET", "/gpx/"+track, "", 200)
	var surviving struct {
		ID     uuid.UUID  `json:"id"`
		Title  string     `json:"title"`
		RaceID *uuid.UUID `json:"race_id"`
	}
	require.NoError(t, json.Unmarshal(request(owner, "GET", workoutPath, "", 200).Body.Bytes(), &surviving))
	require.Equal(t, session.ID, surviving.ID)
	require.Equal(t, "Hill repeats", surviving.Title)
	require.Nil(t, surviving.RaceID)
}
