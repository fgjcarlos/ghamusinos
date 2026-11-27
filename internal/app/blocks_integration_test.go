//go:build integration

package app

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/fgjcarlos/ghamusinos/internal/auth"
	"github.com/fgjcarlos/ghamusinos/internal/blocks"
	"github.com/fgjcarlos/ghamusinos/internal/db/sqlc"
	"github.com/fgjcarlos/ghamusinos/internal/db/status"
	"github.com/fgjcarlos/ghamusinos/internal/http/handlers"
	"github.com/fgjcarlos/ghamusinos/internal/races"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
)

func TestIntegration_TrainingBlocksCRUDAndOwnership(t *testing.T) {
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
	newRace := func(owner string) string {
		row, err := q.CreateRace(ctx, sqlc.CreateRaceParams{UserID: pgtype.UUID{Bytes: uuid.MustParse(owner), Valid: true}, Name: "Summit",
			RaceDate: pgtype.Date{Time: time.Date(2027, 6, 10, 0, 0, 0, 0, time.UTC), Valid: true}, DistanceM: 42000, Priority: "A"})
		require.NoError(t, err)
		return uuid.UUID(row.ID.Bytes).String()
	}
	owner, other := newUser(), newUser()
	mux := chi.NewRouter()
	mux.Route("/api/v1", func(r chi.Router) {
		handlers.MountBlocks(r, blocks.NewStore(pool))
		handlers.MountRaces(r, races.NewStore(q))
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
	decode := func(raw string, v any) {
		t.Helper()
		require.NoError(t, json.Unmarshal([]byte(raw), v))
	}
	// keys lists the JSON member names of a response body.
	keys := func(raw string) []string {
		t.Helper()
		var m map[string]json.RawMessage
		decode(raw, &m)
		out := make([]string, 0, len(m))
		for k := range m {
			out = append(out, k)
		}
		return out
	}
	type page struct {
		Data    []blocks.Block `json:"data"`
		Total   int            `json:"total"`
		Limit   int            `json:"limit"`
		Offset  int            `json:"offset"`
		HasNext bool           `json:"has_next"`
	}
	list := func(user, query string) page {
		t.Helper()
		var p page
		decode(request(user, "GET", "/blocks"+query, "", 200).Body.String(), &p)
		return p
	}
	drafts := func(in []blocks.Block) []blocks.Draft {
		out := make([]blocks.Draft, 0, len(in))
		for _, b := range in {
			out = append(out, b.Draft)
		}
		return out
	}

	const block = `{"name":"Base","start_date":"2027-01-04","end_date":"2027-01-31"}`
	const invalid = `{"type":"about:blank","title":"Unprocessable Entity","status":422,"detail":"name, start_date, end_date or notes is invalid"}`
	const conflict = `{"type":"about:blank","title":"Conflict","status":409,"detail":"training block overlaps an existing block"}`
	const missing = `{"type":"about:blank","title":"Not Found","status":404,"detail":"training block or race not found"}`
	const blockKeys = "id,name,start_date,end_date,notes,created_at,updated_at"

	// An invalid range or field answers 422 and stores nothing.
	for _, draft := range []string{
		strings.Replace(block, "2027-01-31", "2027-01-03", 1), // ends before it starts
		strings.Replace(block, "2027-01-04", "2027-02-30", 1), // date that does not exist
		strings.Replace(block, "2027-01-31", "20270131", 1),   // not YYYY-MM-DD
		strings.Replace(block, `"Base"`, `"  "`, 1),           // blank name
		strings.Replace(block, `"end_date":"2027-01-31"`, `"end_date":"2027-01-31","notes":"`+strings.Repeat("n", 2001)+`"`, 1),
	} {
		require.JSONEq(t, invalid, request(owner, "POST", "/blocks", draft, 422).Body.String())
	}
	require.JSONEq(t, `{"type":"about:blank","title":"Bad Request","status":400,"detail":"invalid json body"}`,
		request(owner, "POST", "/blocks", `{"name":"Base","unexpected":1}`, 400).Body.String())
	empty := list(owner, "")
	require.Empty(t, empty.Data)
	require.Equal(t, 20, empty.Limit)

	created := request(owner, "POST", "/blocks", block, 201)
	var base blocks.Block
	decode(created.Body.String(), &base)
	require.NotEqual(t, uuid.Nil, base.ID)
	require.Equal(t, blocks.Draft{Name: "Base", StartDate: "2027-01-04", EndDate: "2027-01-31"}, base.Draft)
	require.False(t, base.CreatedAt.IsZero())
	require.ElementsMatch(t, strings.Split(blockKeys, ","), keys(created.Body.String()))

	// Any overlap answers 409, including sharing a single day or being identical.
	for _, draft := range []string{
		strings.Replace(block, "2027-01-31", "2027-02-28", 1), // same start, longer
		strings.Replace(block, "2027-01-04", "2027-01-05", 1), // inside
		strings.Replace(block, "2027-01-04", "2026-12-01", 1), // around
		strings.Replace(block, "2027-01-31", "2027-01-04", 1), // only its first day
		strings.Replace(block, "2027-01-04", "2027-01-31", 1), // only its last day
		block,
	} {
		require.JSONEq(t, conflict, request(owner, "POST", "/blocks", draft, 409).Body.String())
	}
	require.Equal(t, []blocks.Draft{base.Draft}, drafts(list(owner, "?limit=100").Data))

	// Blocks that touch without sharing a day are accepted.
	var intro, peak blocks.Block
	decode(request(owner, "POST", "/blocks", `{"name":"Intro","start_date":"2026-12-27","end_date":"2027-01-03"}`, 201).Body.String(), &intro)
	decode(request(owner, "POST", "/blocks", `{"name":"Peak","start_date":"2027-02-01","end_date":"2027-02-28","notes":"taper"}`, 201).Body.String(), &peak)
	require.Equal(t, "taper", peak.Notes)

	// Ordered by start_date with the standard pagination.
	first := list(owner, "?limit=2")
	require.Equal(t, []uuid.UUID{intro.ID, base.ID}, []uuid.UUID{first.Data[0].ID, first.Data[1].ID})
	require.Equal(t, page{Total: 3, Limit: 2, Offset: 0, HasNext: true}, page{Total: first.Total, Limit: first.Limit, Offset: first.Offset, HasNext: first.HasNext})
	last := list(owner, "?limit=2&offset=2")
	require.Equal(t, []blocks.Draft{peak.Draft}, drafts(last.Data))
	require.Equal(t, page{Limit: 2, Offset: 2, Total: 3}, page{Limit: last.Limit, Offset: last.Offset, Total: last.Total})
	require.False(t, last.HasNext)
	require.JSONEq(t, `{"data":[], "total":0, "limit":100, "offset":0, "has_next":false}`,
		request(other, "GET", "/blocks?limit=100", "", 200).Body.String())
	require.JSONEq(t, `{"type":"about:blank","title":"Bad Request","status":400,"detail":"limit must be between 1 and 100"}`,
		request(owner, "GET", "/blocks?limit=101", "", 400).Body.String())
	require.JSONEq(t, `{"type":"about:blank","title":"Bad Request","status":400,"detail":"offset must be a non-negative integer"}`,
		request(owner, "GET", "/blocks?offset=-1", "", 400).Body.String())

	// The target race must belong to the user; deleting it leaves the block without a race.
	own, foreign := newRace(owner), newRace(other)
	const taper = `{"name":"Race week","start_date":"2027-03-01","end_date":"2027-03-07","race_id":"%s","notes":"taper"}`
	require.JSONEq(t, missing, request(owner, "POST", "/blocks", strings.Replace(taper, "%s", foreign, 1), 404).Body.String())
	require.Len(t, list(owner, "?limit=100").Data, 3)
	raced := request(owner, "POST", "/blocks", strings.Replace(taper, "%s", own, 1), 201)
	var raceWeek blocks.Block
	decode(raced.Body.String(), &raceWeek)
	require.Equal(t, own, raceWeek.RaceID.String())
	require.ElementsMatch(t, strings.Split(blockKeys+",race_id", ","), keys(raced.Body.String()))
	request(owner, "DELETE", "/races/"+own, "", 204)
	require.Nil(t, list(owner, "?limit=100").Data[3].RaceID)

	// Another user's block, a missing block and a malformed id all answer 404.
	path, missingPath := "/blocks/"+base.ID.String(), "/blocks/"+uuid.NewString()
	for _, target := range []string{path, missingPath, "/blocks/not-a-uuid"} {
		request(other, "GET", target, "", 404)
		request(other, "PUT", target, block, 404)
		request(other, "DELETE", target, "", 404)
	}
	require.JSONEq(t, missing, request(owner, "GET", missingPath, "", 404).Body.String())
	require.JSONEq(t, missing, request(owner, "GET", "/blocks/not-a-uuid", "", 404).Body.String())

	const moved = `{"name":"Base moved","start_date":"2027-01-10","end_date":"2027-01-20","notes":"edited"}`
	movedDraft := blocks.Draft{Name: "Base moved", StartDate: "2027-01-10", EndDate: "2027-01-20", Notes: "edited"}
	storedBlock := func() blocks.Block {
		t.Helper()
		var current blocks.Block
		decode(request(owner, "GET", path, "", 200).Body.String(), &current)
		require.Equal(t, base.ID, current.ID)
		return current
	}
	var updated blocks.Block
	decode(request(owner, "PUT", path, moved, 200).Body.String(), &updated)
	require.Equal(t, movedDraft, updated.Draft)
	require.Equal(t, movedDraft, storedBlock().Draft)
	// Its own range is not an overlap.
	request(owner, "PUT", path, moved, 200)

	// A rejected update leaves the stored block unchanged.
	for _, draft := range []string{
		strings.Replace(moved, "2027-01-20", "2027-01-01", 1), // range
		strings.Replace(moved, `"Base moved"`, `""`, 1),       // name
	} {
		require.JSONEq(t, invalid, request(owner, "PUT", path, draft, 422).Body.String())
		require.Equal(t, movedDraft, storedBlock().Draft)
	}
	// Intro ends on 2027-01-03, so starting there overlaps.
	require.JSONEq(t, conflict, request(owner, "PUT", path, strings.Replace(moved, "2027-01-10", "2027-01-03", 1), 409).Body.String())
	require.Equal(t, movedDraft, storedBlock().Draft)
	require.JSONEq(t, missing, request(owner, "PUT", path, strings.Replace(moved, `"edited"`, `"edited","race_id":"`+foreign+`"`, 1), 404).Body.String())
	require.Equal(t, movedDraft, storedBlock().Draft)

	// A PUT with race_id links the block; a PUT without it clears the link.
	again := newRace(owner)
	var linked blocks.Block
	decode(request(owner, "PUT", path, strings.Replace(moved, `"edited"`, `"edited","race_id":"`+again+`"`, 1), 200).Body.String(), &linked)
	require.Equal(t, again, linked.RaceID.String())
	require.Equal(t, again, storedBlock().RaceID.String())
	request(owner, "PUT", path, moved, 200)
	require.Nil(t, storedBlock().RaceID)

	request(owner, "DELETE", path, "", 204)
	request(owner, "GET", path, "", 404)
	remaining := list(owner, "?limit=100")
	require.Len(t, remaining.Data, 3)
	require.Equal(t, 3, remaining.Total)
	for _, b := range remaining.Data {
		require.NotEqual(t, base.ID, b.ID)
	}

	// Concurrent overlapping creates: the per-user lock lets exactly one through.
	const concurrent = `{"name":"Concurrent","start_date":"2028-03-01","end_date":"2028-03-31"}`
	codes := make(chan int, 8)
	var wg sync.WaitGroup
	for range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			r := httptest.NewRequestWithContext(ctx, http.MethodPost, "/api/v1/blocks", strings.NewReader(concurrent))
			r = r.WithContext(auth.WithAuthUser(r.Context(), &auth.User{ID: owner}))
			rec := httptest.NewRecorder()
			mux.ServeHTTP(rec, r)
			codes <- rec.Code
		}()
	}
	wg.Wait()
	close(codes)
	var created201, conflicts int
	for code := range codes {
		if code == http.StatusCreated {
			created201++
		}
		if code == http.StatusConflict {
			conflicts++
		}
	}
	require.Equal(t, 1, created201, "exactly one concurrent create must succeed")
	require.Equal(t, 7, conflicts)
}
