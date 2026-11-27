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
	"github.com/fgjcarlos/ghamusinos/internal/db/sqlc"
	"github.com/fgjcarlos/ghamusinos/internal/db/status"
	"github.com/fgjcarlos/ghamusinos/internal/http/handlers"
	"github.com/fgjcarlos/ghamusinos/internal/jobs"
	"github.com/fgjcarlos/ghamusinos/internal/reports"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/rivertype"
	"github.com/stretchr/testify/require"
)

// reportsHarness is the shared fixture of the reports API tests: a migrated
// database, a mux with the reports routes mounted and helpers to create users
// and to talk to the API as one of them.
type reportsHarness struct {
	ctx    context.Context
	pool   *pgxpool.Pool
	q      *sqlc.Queries
	mux    *chi.Mux
	queued *countingReportInserter
}

func newReportsHarness(t *testing.T) *reportsHarness {
	t.Helper()
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
	h := &reportsHarness{ctx: ctx, pool: pool, q: sqlc.New(pool), queued: &countingReportInserter{}}
	h.mux = chi.NewRouter()
	h.mux.Route("/api/v1", func(r chi.Router) {
		handlers.MountReports(r, reports.NewStore(h.q), h.queued)
	})
	return h
}

// user creates an active user that the cleanup deletes, so its reports,
// activities and blocks go with it.
func (h *reportsHarness) user(t *testing.T) string {
	t.Helper()
	key := uuid.NewString()
	u, err := h.q.CreateUser(h.ctx, sqlc.CreateUserParams{ClerkUserID: key, Email: key + "@example.test",
		InviteStatus: status.InviteStatusActive})
	require.NoError(t, err)
	t.Cleanup(func() {
		_, err := h.pool.Exec(context.Background(), "DELETE FROM users WHERE id = $1", u.ID)
		require.NoError(t, err)
	})
	return uuid.UUID(u.ID.Bytes).String()
}

func (h *reportsHarness) setTimezone(t *testing.T, user, timezone string) {
	t.Helper()
	_, err := h.q.UpdateUserPreferences(h.ctx, sqlc.UpdateUserPreferencesParams{
		ID: pgtype.UUID{Bytes: uuid.MustParse(user), Valid: true}, Timezone: timezone})
	require.NoError(t, err)
}

func (h *reportsHarness) block(t *testing.T, owner, start, end string) string {
	t.Helper()
	row, err := h.q.CreateTrainingBlock(h.ctx, sqlc.CreateTrainingBlockParams{
		UserID: pgtype.UUID{Bytes: uuid.MustParse(owner), Valid: true}, Name: "Base",
		StartDate: reportDate(t, start), EndDate: reportDate(t, end)})
	require.NoError(t, err)
	return uuid.UUID(row.ID.Bytes).String()
}

// snapshot stores a report row with a payload built from the version 1 model,
// so the API is tested against the same shape the generator writes.
func (h *reportsHarness) snapshot(t *testing.T, owner, kind, start, end string, blockID string, value reports.Snapshot) string {
	t.Helper()
	value.SchemaVersion, value.Kind = reports.SchemaVersion, reports.Kind(kind)
	value.PeriodStart, value.PeriodEnd = start, end
	payload, err := json.Marshal(value)
	require.NoError(t, err)
	params := sqlc.UpsertReportSnapshotParams{UserID: pgtype.UUID{Bytes: uuid.MustParse(owner), Valid: true},
		Kind: kind, PeriodStart: reportDate(t, start), PeriodEnd: reportDate(t, end), Payload: payload}
	if blockID != "" {
		params.BlockID = pgtype.UUID{Bytes: uuid.MustParse(blockID), Valid: true}
	}
	row, err := h.q.UpsertReportSnapshot(h.ctx, params)
	require.NoError(t, err)
	return uuid.UUID(row.ID.Bytes).String()
}

func (h *reportsHarness) count(t *testing.T, owner string) int64 {
	t.Helper()
	var total int64
	require.NoError(t, h.pool.QueryRow(h.ctx, "SELECT count(*) FROM report_snapshots WHERE user_id = $1",
		pgtype.UUID{Bytes: uuid.MustParse(owner), Valid: true}).Scan(&total))
	return total
}

func (h *reportsHarness) request(t *testing.T, user, method, path, body string, code int) *httptest.ResponseRecorder {
	t.Helper()
	r := httptest.NewRequestWithContext(h.ctx, method, "/api/v1"+path, strings.NewReader(body))
	r = r.WithContext(auth.WithAuthUser(r.Context(), &auth.User{ID: user}))
	rec := httptest.NewRecorder()
	h.mux.ServeHTTP(rec, r)
	require.Equal(t, code, rec.Code, rec.Body.String())
	return rec
}

func reportDate(t *testing.T, raw string) pgtype.Date {
	t.Helper()
	parsed, err := time.Parse(time.DateOnly, raw)
	require.NoError(t, err)
	return pgtype.Date{Time: parsed, Valid: true}
}

// countingReportInserter records the jobs the generate endpoint enqueues.
type countingReportInserter struct {
	mu   sync.Mutex
	args []jobs.GenerateReportArgs
	code int
}

func (c *countingReportInserter) Insert(_ context.Context, args river.JobArgs, _ *river.InsertOpts) (*rivertype.JobInsertResult, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.code != 0 {
		return nil, pgx.ErrTxClosed
	}
	generate, ok := args.(*jobs.GenerateReportArgs)
	if ok {
		c.args = append(c.args, *generate)
	}
	return &rivertype.JobInsertResult{Job: &rivertype.JobRow{ID: int64(len(c.args))}}, nil
}

func (c *countingReportInserter) count() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.args)
}

func (c *countingReportInserter) last() jobs.GenerateReportArgs {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.args[len(c.args)-1]
}

func (c *countingReportInserter) users() []string {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := make([]string, 0, len(c.args))
	for _, a := range c.args {
		out = append(out, a.UserID)
	}
	return out
}

type reportItem struct {
	ID          uuid.UUID      `json:"id"`
	Kind        string         `json:"kind"`
	PeriodStart string         `json:"period_start"`
	PeriodEnd   string         `json:"period_end"`
	BlockID     *uuid.UUID     `json:"block_id"`
	GeneratedAt time.Time      `json:"generated_at"`
	Summary     map[string]any `json:"summary"`
}

type reportPage struct {
	Data    []reportItem `json:"data"`
	Total   int          `json:"total"`
	Limit   int          `json:"limit"`
	Offset  int          `json:"offset"`
	HasNext bool         `json:"has_next"`
}

func TestIntegration_ReportsAPIListAndDetail(t *testing.T) {
	h := newReportsHarness(t)
	owner, other, stranger := h.user(t), h.user(t), h.user(t)
	// Two weeks, a month that contains them and a block before them, each with
	// its own numbers so the list summary is traceable to its stored payload.
	week1 := h.snapshot(t, owner, "week", "2026-11-02", "2026-11-08", "", reports.Snapshot{
		Load:        reports.Load{TSS: 210.5},
		Volume:      reports.Volume{Activities: 5, DistanceM: 42000, ElevationGainM: 800},
		Performance: reports.Performance{AvgEF: ptrFloat(1.25), AvgGAPSecPerKm: ptrFloat(305)},
		Plan:        reports.Plan{Planned: 4, Completed: 3, Adherence: ptrFloat(0.75)},
	})
	week2 := h.snapshot(t, owner, "week", "2026-11-09", "2026-11-15", "", reports.Snapshot{
		Load:        reports.Load{TSS: 320},
		Volume:      reports.Volume{Activities: 6, DistanceM: 51000, ElevationGainM: 1200},
		Performance: reports.Performance{EFSamples: 0, GAPSamples: 0},
		Plan:        reports.Plan{Planned: 5, Skipped: 5},
	})
	month := h.snapshot(t, owner, "month", "2026-11-01", "2026-11-30", "", reports.Snapshot{
		Load:   reports.Load{TSS: 940.25},
		Volume: reports.Volume{Activities: 21, DistanceM: 180000, ElevationGainM: 4100},
		Plan:   reports.Plan{Planned: 20, Completed: 10, Modified: 5, Skipped: 5, Adherence: ptrFloat(0.75)},
	})
	blockID := h.block(t, owner, "2026-10-01", "2026-10-31")
	block := h.snapshot(t, owner, "block", "2026-10-01", "2026-10-31", blockID, reports.Snapshot{
		Load:   reports.Load{TSS: 610},
		Volume: reports.Volume{Activities: 16, DistanceM: 140000, ElevationGainM: 2200},
	})
	foreign := h.snapshot(t, other, "week", "2026-11-02", "2026-11-08", "", reports.Snapshot{Load: reports.Load{TSS: 1}})

	list := func(user, query string) reportPage {
		t.Helper()
		var page reportPage
		require.NoError(t, json.Unmarshal(h.request(t, user, "GET", "/reports"+query, "", 200).Body.Bytes(), &page))
		return page
	}
	ids := func(page reportPage) []uuid.UUID {
		out := make([]uuid.UUID, 0, len(page.Data))
		for _, item := range page.Data {
			out = append(out, item.ID)
		}
		return out
	}

	// Newest period first, then id; the other user's report never shows up.
	all := list(owner, "")
	require.Equal(t, []uuid.UUID{uuid.MustParse(week2), uuid.MustParse(week1), uuid.MustParse(month), uuid.MustParse(block)}, ids(all))
	require.Equal(t, reportPage{Total: 4, Limit: 20, Offset: 0, HasNext: false},
		reportPage{Total: all.Total, Limit: all.Limit, Offset: all.Offset, HasNext: all.HasNext})
	require.NotContains(t, ids(all), uuid.MustParse(foreign))

	// The summary of every item is the stored payload, not the payload itself.
	require.JSONEq(t, summaryJSON(t, reports.Snapshot{
		Load:        reports.Load{TSS: 210.5},
		Volume:      reports.Volume{Activities: 5, DistanceM: 42000, ElevationGainM: 800},
		Performance: reports.Performance{AvgEF: ptrFloat(1.25), AvgGAPSecPerKm: ptrFloat(305)},
		Plan:        reports.Plan{Planned: 4, Completed: 3, Adherence: ptrFloat(0.75)},
	}), jsonOf(t, all.Data[1].Summary))
	// A payload without samples keeps the averages null instead of zero.
	require.JSONEq(t, `{"tss":320,"activities":6,"distance_m":51000,"elevation_gain_m":1200,"avg_ef":null,"avg_gap_sec_per_km":null,"adherence":null}`,
		jsonOf(t, all.Data[0].Summary))
	require.Equal(t, "week", all.Data[1].Kind)
	require.Equal(t, "2026-11-02", all.Data[1].PeriodStart)
	require.Equal(t, "2026-11-08", all.Data[1].PeriodEnd)
	require.Nil(t, all.Data[1].BlockID)
	require.False(t, all.Data[1].GeneratedAt.IsZero())
	require.Equal(t, blockID, all.Data[3].BlockID.String())

	// kind filters one type; an unknown kind is rejected.
	require.Equal(t, []uuid.UUID{uuid.MustParse(week2), uuid.MustParse(week1)}, ids(list(owner, "?kind=week")))
	require.Equal(t, []uuid.UUID{uuid.MustParse(block)}, ids(list(owner, "?kind=block")))
	require.Equal(t, []uuid.UUID{uuid.MustParse(month)}, ids(list(owner, "?kind=month")))

	// from/to filter on overlap with [period_start, period_end]: a report that
	// merely touches the window, or contains it, is included.
	require.Equal(t, []uuid.UUID{uuid.MustParse(week2), uuid.MustParse(month)}, ids(list(owner, "?from=2026-11-09")))
	require.Equal(t, []uuid.UUID{uuid.MustParse(week1), uuid.MustParse(month), uuid.MustParse(block)},
		ids(list(owner, "?to=2026-11-02")))
	require.Equal(t, []uuid.UUID{uuid.MustParse(week1), uuid.MustParse(month)}, ids(list(owner, "?from=2026-11-02&to=2026-11-02")))
	require.Empty(t, list(owner, "?from=2027-01-01").Data)

	// Pagination: the envelope reports the page and whether another follows.
	first := list(owner, "?limit=2")
	require.Equal(t, []uuid.UUID{uuid.MustParse(week2), uuid.MustParse(week1)}, ids(first))
	require.Equal(t, reportPage{Total: 4, Limit: 2, Offset: 0, HasNext: true},
		reportPage{Total: first.Total, Limit: first.Limit, Offset: first.Offset, HasNext: first.HasNext})
	last := list(owner, "?limit=2&offset=2")
	require.Equal(t, []uuid.UUID{uuid.MustParse(month), uuid.MustParse(block)}, ids(last))
	require.Equal(t, reportPage{Total: 4, Limit: 2, Offset: 2}, reportPage{Total: last.Total, Limit: last.Limit, Offset: last.Offset})
	require.False(t, last.HasNext)
	require.Empty(t, list(owner, "?limit=100&offset=4").Data)

	const badLimit = `{"type":"about:blank","title":"Bad Request","status":400,"detail":"limit must be between 1 and 100"}`
	const badOffset = `{"type":"about:blank","title":"Bad Request","status":400,"detail":"offset must be a non-negative integer"}`
	require.JSONEq(t, badLimit, h.request(t, owner, "GET", "/reports?limit=0", "", 400).Body.String())
	require.JSONEq(t, badLimit, h.request(t, owner, "GET", "/reports?limit=101", "", 400).Body.String())
	require.JSONEq(t, badLimit, h.request(t, owner, "GET", "/reports?limit=many", "", 400).Body.String())
	require.JSONEq(t, badOffset, h.request(t, owner, "GET", "/reports?offset=-1", "", 400).Body.String())
	for _, query := range []string{
		"?kind=year", "?kind=WEEK", "?from=2026-13-01", "?from=20261101", "?to=not-a-date",
		"?from=2026-11-30&to=2026-11-01", "?from=2026-11-02&to=2026-11-01",
	} {
		rec := h.request(t, owner, "GET", "/reports"+query, "", 400)
		require.Equal(t, "application/problem+json", rec.Header().Get("Content-Type"), query)
	}

	// Another user sees nothing at all.
	empty := list(stranger, "")
	require.Empty(t, empty.Data)
	require.Equal(t, 0, empty.Total)
	require.Equal(t, 20, empty.Limit)

	// Detail returns the full stored payload plus id and generated_at.
	detail := h.request(t, owner, "GET", "/reports/"+week1, "", 200)
	var body map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(detail.Body.Bytes(), &body))
	require.ElementsMatch(t, []string{"id", "generated_at", "payload"}, mapKeys(body))
	require.JSONEq(t, `"`+week1+`"`, string(body["id"]))
	var payload map[string]any
	require.NoError(t, json.Unmarshal(body["payload"], &payload))
	require.Equal(t, "week", payload["kind"])
	require.Equal(t, "2026-11-02", payload["period_start"])
	require.Equal(t, "2026-11-08", payload["period_end"])
	require.Equal(t, 210.5, payload["load"].(map[string]any)["tss"])
	require.Equal(t, 5.0, payload["volume"].(map[string]any)["activities"])

	// Missing, malformed and foreign ids all answer the same 404.
	const missing = `{"type":"about:blank","title":"Not Found","status":404,"detail":"report, training block or race not found"}`
	for _, id := range []string{uuid.NewString(), "not-a-uuid", foreign} {
		require.JSONEq(t, missing, h.request(t, owner, "GET", "/reports/"+id, "", 404).Body.String())
	}
	require.JSONEq(t, missing, h.request(t, other, "GET", "/reports/"+week1, "", 404).Body.String())
	require.Equal(t, int64(4), h.count(t, owner))
}

func TestIntegration_ReportsAPIGenerate(t *testing.T) {
	h := newReportsHarness(t)
	owner, other := h.user(t), h.user(t)
	ownBlock := h.block(t, owner, "2027-01-04", "2027-01-31")
	foreignBlock := h.block(t, other, "2027-02-01", "2027-02-28")
	generate := func(user, body string, code int) *httptest.ResponseRecorder {
		t.Helper()
		return h.request(t, user, "POST", "/reports/generate", body, code)
	}

	// The period of a week and a month is the one the generator computes, and
	// the request answers before generating anything.
	require.JSONEq(t, `{"status":"queued","kind":"week","period_start":"2026-11-02","period_end":"2026-11-08"}`,
		generate(owner, `{"kind":"week","date":"2026-11-05"}`, http.StatusAccepted).Body.String())
	require.JSONEq(t, `{"status":"queued","kind":"month","period_start":"2026-02-01","period_end":"2026-02-28"}`,
		generate(owner, `{"kind":"month","date":"2026-02-17"}`, http.StatusAccepted).Body.String())
	require.JSONEq(t, `{"status":"queued","kind":"block","period_start":"2027-01-04","period_end":"2027-01-31"}`,
		generate(owner, `{"kind":"block","block_id":"`+ownBlock+`"}`, http.StatusAccepted).Body.String())
	// Every call enqueues a job for its user, even when it repeats the period.
	generate(owner, `{"kind":"week","date":"2026-11-05"}`, http.StatusAccepted)
	require.Equal(t, 4, h.queued.count())
	require.Equal(t, []string{owner, owner, owner, owner}, h.queued.users())
	require.Equal(t, int64(0), h.count(t, owner), "generate must not write a snapshot synchronously")

	// The period is computed in the user's time zone: a Monday is the Monday of
	// its own week even west of UTC.
	h.setTimezone(t, owner, "America/New_York")
	require.JSONEq(t, `{"status":"queued","kind":"week","period_start":"2026-11-02","period_end":"2026-11-08"}`,
		generate(owner, `{"kind":"week","date":"2026-11-02"}`, http.StatusAccepted).Body.String())
	require.JSONEq(t, `{"status":"queued","kind":"week","period_start":"2026-02-23","period_end":"2026-03-01"}`,
		generate(owner, `{"kind":"week","date":"2026-03-01"}`, http.StatusAccepted).Body.String())
	require.JSONEq(t, `{"status":"queued","kind":"month","period_start":"2026-01-01","period_end":"2026-01-31"}`,
		generate(owner, `{"kind":"month","date":"2026-01-01"}`, http.StatusAccepted).Body.String())

	// Unknown kind or a missing/invalid field answers 422 and enqueues nothing.
	const invalid = `{"type":"about:blank","title":"Unprocessable Entity","status":422,"detail":"kind, date or block_id is invalid"}`
	for _, body := range []string{
		`{"kind":"year","date":"2026-11-05"}`,
		`{"kind":"","date":"2026-11-05"}`,
		`{"date":"2026-11-05"}`,
		`{"kind":"week"}`,
		`{"kind":"week","date":""}`,
		`{"kind":"week","date":"2026-11-5"}`,
		`{"kind":"week","date":"2026-02-30"}`,
		`{"kind":"week","date":"05/11/2026"}`,
		`{"kind":"month","date":"2026-13-01"}`,
		`{"kind":"block"}`,
		`{"kind":"block","date":"2027-01-04"}`,
		`{"kind":"block","block_id":"not-a-uuid"}`,
	} {
		require.JSONEq(t, invalid, generate(owner, body, 422).Body.String(), body)
	}
	require.Equal(t, 7, h.queued.count())
	require.JSONEq(t, `{"type":"about:blank","title":"Bad Request","status":400,"detail":"invalid json body"}`,
		generate(owner, `{"kind":"week","date":"2026-11-05","extra":1}`, 400).Body.String())

	// A missing or foreign block answers 404 without telling them apart.
	const missing = `{"type":"about:blank","title":"Not Found","status":404,"detail":"report, training block or race not found"}`
	for _, body := range []string{
		`{"kind":"block","block_id":"` + uuid.NewString() + `"}`,
		`{"kind":"block","block_id":"` + foreignBlock + `"}`,
	} {
		require.JSONEq(t, missing, generate(owner, body, 404).Body.String(), body)
	}
	require.Equal(t, 7, h.queued.count())

	// Without a River queue the endpoint answers 503 instead of blocking.
	offline := &reportsHarness{ctx: h.ctx, pool: h.pool, q: h.q, queued: h.queued}
	offline.mux = chi.NewRouter()
	offline.mux.Route("/api/v1", func(r chi.Router) {
		handlers.MountReports(r, reports.NewStore(h.q), nil)
	})
	rec := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(h.ctx, http.MethodPost, "/api/v1/reports/generate",
		strings.NewReader(`{"kind":"week","date":"2026-11-05"}`))
	req = req.WithContext(auth.WithAuthUser(req.Context(), &auth.User{ID: owner}))
	offline.mux.ServeHTTP(rec, req)
	require.Equal(t, http.StatusServiceUnavailable, rec.Code, rec.Body.String())
	require.Equal(t, "application/problem+json", rec.Header().Get("Content-Type"))
	require.JSONEq(t, `{"type":"about:blank","title":"Service Unavailable","status":503,"detail":"report generation unavailable: river queue not configured"}`,
		rec.Body.String())

	// A failing enqueue also answers 503.
	h.queued.mu.Lock()
	h.queued.code = 1
	h.queued.mu.Unlock()
	rec = generate(owner, `{"kind":"week","date":"2026-11-05"}`, http.StatusServiceUnavailable)
	require.Equal(t, "application/problem+json", rec.Header().Get("Content-Type"))
}

// TestIntegration_ReportsAPIGenerateWritesRequestedPeriod covers S5 (#320): the
// on-demand request must end up writing exactly the period it named, so the
// screen can generate a past week, a month or a block and then read it back.
func TestIntegration_ReportsAPIGenerateWritesRequestedPeriod(t *testing.T) {
	h := newReportsHarness(t)
	owner := h.user(t)
	block := h.block(t, owner, "2027-01-04", "2027-01-31")
	// The job the endpoint enqueues is the one the worker consumes; running it
	// here is what a live River queue would do.
	run := func() {
		t.Helper()
		args := h.queued.last()
		worker := jobs.NewGenerateReportWorker(reports.NewStore(h.q), h.q, nil)
		require.NoError(t, worker.Work(h.ctx, &river.Job[jobs.GenerateReportArgs]{Args: args}))
	}
	periods := func(kind string) []string {
		t.Helper()
		var page reportPage
		require.NoError(t, json.Unmarshal(h.request(t, owner, "GET", "/reports?kind="+kind, "", 200).Body.Bytes(), &page))
		out := make([]string, 0, len(page.Data))
		for _, item := range page.Data {
			out = append(out, item.PeriodStart+"/"+item.PeriodEnd)
		}
		return out
	}

	// A past week: the request writes that week, not the last closed one.
	h.request(t, owner, "POST", "/reports/generate", `{"kind":"week","date":"2026-11-05"}`, http.StatusAccepted)
	run()
	require.Equal(t, []string{"2026-11-02/2026-11-08"}, periods("week"))

	// A month.
	h.request(t, owner, "POST", "/reports/generate", `{"kind":"month","date":"2026-02-17"}`, http.StatusAccepted)
	run()
	require.Equal(t, []string{"2026-02-01/2026-02-28"}, periods("month"))

	// A block, with the block link in the row and in the payload.
	h.request(t, owner, "POST", "/reports/generate", `{"kind":"block","block_id":"`+block+`"}`, http.StatusAccepted)
	run()
	require.Equal(t, []string{"2027-01-04/2027-01-31"}, periods("block"))

	// The written reports are readable through the list and the detail.
	var page reportPage
	require.NoError(t, json.Unmarshal(h.request(t, owner, "GET", "/reports?limit=100", "", 200).Body.Bytes(), &page))
	require.Len(t, page.Data, 3)
	require.Equal(t, 3, page.Total)
	byKind := make(map[string]reportItem, len(page.Data))
	for _, item := range page.Data {
		byKind[item.Kind] = item
	}
	require.Equal(t, "2027-01-31", byKind["block"].PeriodEnd)
	require.Equal(t, block, byKind["block"].BlockID.String())

	for _, tc := range []struct{ kind, start, end string }{
		{"week", "2026-11-02", "2026-11-08"},
		{"month", "2026-02-01", "2026-02-28"},
		{"block", "2027-01-04", "2027-01-31"},
	} {
		item := byKind[tc.kind]
		detail := h.request(t, owner, "GET", "/reports/"+item.ID.String(), "", 200)
		var body struct {
			Payload reports.Snapshot `json:"payload"`
		}
		require.NoError(t, json.Unmarshal(detail.Body.Bytes(), &body))
		require.Equal(t, tc.kind, string(body.Payload.Kind))
		require.Equal(t, tc.start, body.Payload.PeriodStart)
		require.Equal(t, tc.end, body.Payload.PeriodEnd)
		require.Equal(t, item.Summary["tss"], body.Payload.Load.TSS)
	}

	// Regenerating the same period replaces the snapshot instead of duplicating it.
	h.request(t, owner, "POST", "/reports/generate", `{"kind":"week","date":"2026-11-02"}`, http.StatusAccepted)
	run()
	require.Equal(t, []string{"2026-11-02/2026-11-08"}, periods("week"))
	require.Equal(t, int64(3), h.count(t, owner))
}

func ptrFloat(v float64) *float64 { return &v }

func jsonOf(t *testing.T, v any) string {
	t.Helper()
	raw, err := json.Marshal(v)
	require.NoError(t, err)
	return string(raw)
}

// summaryJSON is the list summary the API must derive from a stored payload.
func summaryJSON(t *testing.T, s reports.Snapshot) string {
	t.Helper()
	return jsonOf(t, map[string]any{"tss": s.Load.TSS, "activities": s.Volume.Activities,
		"distance_m": s.Volume.DistanceM, "elevation_gain_m": s.Volume.ElevationGainM,
		"avg_ef": s.Performance.AvgEF, "avg_gap_sec_per_km": s.Performance.AvgGAPSecPerKm,
		"adherence": s.Plan.Adherence})
}

func mapKeys(m map[string]json.RawMessage) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
