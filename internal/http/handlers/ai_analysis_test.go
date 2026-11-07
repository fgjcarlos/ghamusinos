// Tests for the optional AI analysis API (phase 1.5, T7 — S3/S4/S5/S6/S7).
//
// The API is the only door the web app uses to ask for an analysis. Every
// rule of the contract is observable here:
//
//   - the status endpoint reports the global state and the consent of the
//     CURRENT USER without leaking the key or the model,
//   - a request without consent is refused and creates nothing,
//   - a request with the AI globally off or unavailable is a 503,
//   - an activity of another user is a 404 and creates nothing,
//   - an invalid kind, a malformed window_start or a window beyond the
//     maximum is a 422 and creates nothing,
//   - a valid request creates the pending row with the documented scope and
//     enqueues the job with the documented maximum attempts,
//   - the list endpoint returns only the analyses of the authenticated user
//     and honours the limit,
//   - a missing single analysis is a 404,
//   - a failed analysis exposes only its safe code and message.
//
// No database and no network: the store and the River inserter are fakes.

package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/fgjcarlos/ghamusinos/internal/ai"
	"github.com/fgjcarlos/ghamusinos/internal/auth"
	"github.com/fgjcarlos/ghamusinos/internal/config"
	"github.com/fgjcarlos/ghamusinos/internal/db/sqlc"
	"github.com/fgjcarlos/ghamusinos/internal/jobs"
)

// Fixtures of the AI handler tests.
const (
	aiTestUserID     = "11111111-1111-1111-1111-111111111111"
	aiTestOtherUser  = "22222222-2222-2222-2222-222222222222"
	aiTestActivityID = "33333333-3333-3333-3333-333333333333"
)

// aiReadyConfig is the global configuration of an AI that can be used.
func aiReadyConfig() config.AIConfig {
	return config.AIConfig{
		Enabled:    true,
		Available:  true,
		Provider:   "openai",
		Model:      "gpt-4o-mini",
		APIKey:     "sk-secret-do-not-leak",
		MaxRetries: 3,
		Timeout:    30 * time.Second,
	}
}

// aiRequest builds a request with the authenticated user in the context.
func aiRequest(method, path, body string) *http.Request {
	ctx := auth.WithAuthUser(context.Background(), &auth.User{ID: aiTestUserID})
	var reader *strings.Reader
	if body == "" {
		reader = strings.NewReader("")
	} else {
		reader = strings.NewReader(body)
	}
	req := httptest.NewRequestWithContext(ctx, method, path, reader)
	req.Header.Set("Content-Type", "application/json")
	return req
}

// aiUUID parses a fixture UUID into the pgtype representation.
func aiUUID(t *testing.T, value string) pgtype.UUID {
	t.Helper()
	parsed, err := uuid.Parse(value)
	if err != nil {
		t.Fatalf("parse uuid %q: %v", value, err)
	}
	return pgtype.UUID{Bytes: parsed, Valid: true}
}

// decodeAIResponse decodes a JSON body into a generic map.
func decodeAIResponse(t *testing.T, body []byte) map[string]any {
	t.Helper()
	var out map[string]any
	if err := json.Unmarshal(body, &out); err != nil {
		t.Fatalf("expected JSON body, got %q (err=%v)", string(body), err)
	}
	return out
}

// ---------------------------------------------------------------------------
// GET /api/v1/ai/status
// ---------------------------------------------------------------------------

// TestGetAIStatus_ReportsGlobalStateAndConsentWithoutLeakingSecrets proves
// that the status endpoint tells the app whether the feature can be used
// right now (global state + provider) and whether the CURRENT USER has
// consented, without ever exposing the API key or the model.
func TestGetAIStatus_ReportsGlobalStateAndConsentWithoutLeakingSecrets(t *testing.T) {
	cfg := aiReadyConfig()
	store := &fakeAIStore{
		prefs: sqlc.GetUserPreferencesByIDRow{AiEnabled: true, Timezone: "UTC"},
	}

	h := GetAIStatus(store, cfg)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, aiRequest(http.MethodGet, "/api/v1/ai/status", ""))

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d (body=%s)", w.Code, w.Body.String())
	}
	body := decodeAIResponse(t, w.Body.Bytes())
	if body["enabled"] != true {
		t.Errorf("expected enabled=true, got %v", body["enabled"])
	}
	if body["available"] != true {
		t.Errorf("expected available=true, got %v", body["available"])
	}
	if body["status"] != config.AIStatusReady {
		t.Errorf("expected status=%q, got %v", config.AIStatusReady, body["status"])
	}
	if body["provider"] != "openai" {
		t.Errorf("expected provider=openai, got %v", body["provider"])
	}
	if body["consented"] != true {
		t.Errorf("expected consented=true, got %v", body["consented"])
	}
	if strings.Contains(w.Body.String(), cfg.APIKey) {
		t.Errorf("the status response leaked the API key: %s", w.Body.String())
	}
	if strings.Contains(w.Body.String(), cfg.Model) {
		t.Errorf("the status response leaked the model: %s", w.Body.String())
	}
}

// TestGetAIStatus_WorksWhenTheAIIsOff proves that the endpoint answers
// normally when the global flag is off: the app must be able to render the
// "AI not available" state without an error (S7).
func TestGetAIStatus_WorksWhenTheAIIsOff(t *testing.T) {
	store := &fakeAIStore{
		prefs: sqlc.GetUserPreferencesByIDRow{AiEnabled: false, Timezone: "UTC"},
	}

	h := GetAIStatus(store, config.AIConfig{})
	w := httptest.NewRecorder()
	h.ServeHTTP(w, aiRequest(http.MethodGet, "/api/v1/ai/status", ""))

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d (body=%s)", w.Code, w.Body.String())
	}
	body := decodeAIResponse(t, w.Body.Bytes())
	if body["enabled"] != false {
		t.Errorf("expected enabled=false, got %v", body["enabled"])
	}
	if body["available"] != false {
		t.Errorf("expected available=false, got %v", body["available"])
	}
	if body["status"] != config.AIStatusDisabled {
		t.Errorf("expected status=%q, got %v", config.AIStatusDisabled, body["status"])
	}
	if body["provider"] != nil {
		t.Errorf("expected no provider when the AI is off, got %v", body["provider"])
	}
	if body["consented"] != false {
		t.Errorf("expected consented=false, got %v", body["consented"])
	}
}

// TestGetAIStatus_RequiresAuthentication proves that the status endpoint is
// part of the authenticated group.
func TestGetAIStatus_RequiresAuthentication(t *testing.T) {
	h := GetAIStatus(&fakeAIStore{}, aiReadyConfig())
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/v1/ai/status", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
}

// ---------------------------------------------------------------------------
// POST /api/v1/ai/analyses
// ---------------------------------------------------------------------------

// TestPostAIAnalysis_WithoutConsentIsRefused proves that the consent is
// re-checked at enqueue time: without it the request is a 403 and no row
// and no job are created (S3).
func TestPostAIAnalysis_WithoutConsentIsRefused(t *testing.T) {
	store := &fakeAIStore{
		prefs: sqlc.GetUserPreferencesByIDRow{AiEnabled: false, Timezone: "UTC"},
		activity: map[string]sqlc.GetActivityForUserRow{
			aiTestActivityID: {ID: aiUUID(t, aiTestActivityID)},
		},
	}
	inserter := &fakeAIInserter{}

	h := PostAIAnalysis(store, inserter, aiReadyConfig())
	w := httptest.NewRecorder()
	h.ServeHTTP(w, aiRequest(http.MethodPost, "/api/v1/ai/analyses", `{"kind":"activity","activity_id":"`+aiTestActivityID+`"}`))

	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d (body=%s)", w.Code, w.Body.String())
	}
	if len(store.created) != 0 {
		t.Errorf("expected no row to be created, got %d", len(store.created))
	}
	if len(inserter.calls) != 0 {
		t.Errorf("expected no job to be enqueued, got %d", len(inserter.calls))
	}
}

// TestPostAIAnalysis_GloballyDisabledOrUnavailableIsRefused proves that the
// global flag is checked before anything else: with the AI off or not
// available the request is a 503 and no row is created (S7).
func TestPostAIAnalysis_GloballyDisabledOrUnavailableIsRefused(t *testing.T) {
	cases := []struct {
		name string
		cfg  config.AIConfig
	}{
		{"globally disabled", config.AIConfig{Enabled: false, Available: false}},
		{"enabled but unavailable", config.AIConfig{Enabled: true, Available: false, Provider: "openai"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			store := &fakeAIStore{
				prefs: sqlc.GetUserPreferencesByIDRow{AiEnabled: true, Timezone: "UTC"},
				activity: map[string]sqlc.GetActivityForUserRow{
					aiTestActivityID: {ID: aiUUID(t, aiTestActivityID)},
				},
			}
			inserter := &fakeAIInserter{}

			h := PostAIAnalysis(store, inserter, c.cfg)
			w := httptest.NewRecorder()
			h.ServeHTTP(w, aiRequest(http.MethodPost, "/api/v1/ai/analyses", `{"kind":"activity","activity_id":"`+aiTestActivityID+`"}`))

			if w.Code != http.StatusServiceUnavailable {
				t.Fatalf("expected 503, got %d (body=%s)", w.Code, w.Body.String())
			}
			if len(store.created) != 0 {
				t.Errorf("expected no row to be created, got %d", len(store.created))
			}
			if len(inserter.calls) != 0 {
				t.Errorf("expected no job to be enqueued, got %d", len(inserter.calls))
			}
		})
	}
}

// TestPostAIAnalysis_WithoutJobQueueIsRefused proves that a missing job
// queue is a 503 and does not create a row: the row would stay pending
// forever.
func TestPostAIAnalysis_WithoutJobQueueIsRefused(t *testing.T) {
	store := &fakeAIStore{
		prefs: sqlc.GetUserPreferencesByIDRow{AiEnabled: true, Timezone: "UTC"},
		activity: map[string]sqlc.GetActivityForUserRow{
			aiTestActivityID: {ID: aiUUID(t, aiTestActivityID)},
		},
	}

	h := PostAIAnalysis(store, nil, aiReadyConfig())
	w := httptest.NewRecorder()
	h.ServeHTTP(w, aiRequest(http.MethodPost, "/api/v1/ai/analyses", `{"kind":"activity","activity_id":"`+aiTestActivityID+`"}`))

	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d (body=%s)", w.Code, w.Body.String())
	}
	if len(store.created) != 0 {
		t.Errorf("expected no row to be created, got %d", len(store.created))
	}
}

// TestPostAIAnalysis_ForeignActivityIsNotFound proves that an activity of
// another user (or an unknown one) is a 404 and is never analysed: no row
// is created and no job is enqueued.
func TestPostAIAnalysis_ForeignActivityIsNotFound(t *testing.T) {
	// The fake store only knows the activities of aiTestOtherUser.
	store := &fakeAIStore{
		prefs: sqlc.GetUserPreferencesByIDRow{AiEnabled: true, Timezone: "UTC"},
		activity: map[string]sqlc.GetActivityForUserRow{
			aiTestActivityID: {ID: aiUUID(t, aiTestActivityID), UserID: aiUUID(t, aiTestOtherUser)},
		},
	}
	inserter := &fakeAIInserter{}

	h := PostAIAnalysis(store, inserter, aiReadyConfig())
	w := httptest.NewRecorder()
	h.ServeHTTP(w, aiRequest(http.MethodPost, "/api/v1/ai/analyses", `{"kind":"activity","activity_id":"`+aiTestActivityID+`"}`))

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d (body=%s)", w.Code, w.Body.String())
	}
	if len(store.created) != 0 {
		t.Errorf("expected no row to be created, got %d", len(store.created))
	}
	if len(inserter.calls) != 0 {
		t.Errorf("expected no job to be enqueued, got %d", len(inserter.calls))
	}
}

// TestPostAIAnalysis_InvalidRequestIsUnprocessable proves that an invalid
// kind, a malformed window_start and a window beyond the maximum are a 422
// and create nothing.
func TestPostAIAnalysis_InvalidRequestIsUnprocessable(t *testing.T) {
	cases := []struct {
		name string
		body string
	}{
		{"unknown kind", `{"kind":"banana"}`},
		{"missing kind", `{}`},
		{"activity without activity_id", `{"kind":"activity"}`},
		{"activity with a malformed activity_id", `{"kind":"activity","activity_id":"not-a-uuid"}`},
		{"malformed window_start", `{"kind":"weekly","window_start":"2026-13-01"}`},
		{"window beyond the maximum", `{"kind":"load","window_days":` + strconv.Itoa(ai.MaxWindowDays+1) + `}`},
		{"negative window_days", `{"kind":"load","window_days":-1}`},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			store := &fakeAIStore{
				prefs: sqlc.GetUserPreferencesByIDRow{AiEnabled: true, Timezone: "UTC"},
				activity: map[string]sqlc.GetActivityForUserRow{
					aiTestActivityID: {ID: aiUUID(t, aiTestActivityID)},
				},
			}
			inserter := &fakeAIInserter{}

			h := PostAIAnalysis(store, inserter, aiReadyConfig())
			w := httptest.NewRecorder()
			h.ServeHTTP(w, aiRequest(http.MethodPost, "/api/v1/ai/analyses", c.body))

			if w.Code != http.StatusUnprocessableEntity {
				t.Fatalf("expected 422, got %d (body=%s)", w.Code, w.Body.String())
			}
			if len(store.created) != 0 {
				t.Errorf("expected no row to be created, got %d", len(store.created))
			}
			if len(inserter.calls) != 0 {
				t.Errorf("expected no job to be enqueued, got %d", len(inserter.calls))
			}
		})
	}
}

// TestPostAIAnalysis_CreatesPendingRowAndEnqueuesTheJob proves the happy
// path: the pending row is created with the documented scope, the job is
// enqueued with the documented maximum attempts and the answer is a 202
// with the created analysis.
func TestPostAIAnalysis_CreatesPendingRowAndEnqueuesTheJob(t *testing.T) {
	store := &fakeAIStore{
		prefs: sqlc.GetUserPreferencesByIDRow{AiEnabled: true, Timezone: "UTC"},
		activity: map[string]sqlc.GetActivityForUserRow{
			aiTestActivityID: {ID: aiUUID(t, aiTestActivityID)},
		},
	}
	inserter := &fakeAIInserter{}
	cfg := aiReadyConfig()

	h := PostAIAnalysis(store, inserter, cfg)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, aiRequest(http.MethodPost, "/api/v1/ai/analyses", `{"kind":"activity","activity_id":"`+aiTestActivityID+`"}`))

	if w.Code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d (body=%s)", w.Code, w.Body.String())
	}

	if len(store.created) != 1 {
		t.Fatalf("expected exactly one row created, got %d", len(store.created))
	}
	created := store.created[0]
	if created.Kind != string(ai.KindActivity) {
		t.Errorf("expected kind=%q, got %q", ai.KindActivity, created.Kind)
	}
	expectedScope := ai.ActivityScope(aiTestActivityID)
	if created.Scope != expectedScope {
		t.Errorf("expected scope=%q, got %q", expectedScope, created.Scope)
	}
	if created.UserID != aiUUID(t, aiTestUserID) {
		t.Errorf("expected the row to belong to the authenticated user, got %v", created.UserID)
	}

	if len(inserter.calls) != 1 {
		t.Fatalf("expected exactly one job enqueued, got %d", len(inserter.calls))
	}
	call := inserter.calls[0]
	args, ok := call.args.(jobs.AIAnalysisArgs)
	if !ok {
		t.Fatalf("expected jobs.AIAnalysisArgs, got %T", call.args)
	}
	if args.UserID != aiTestUserID {
		t.Errorf("expected user_id=%q, got %q", aiTestUserID, args.UserID)
	}
	if args.AnalysisKind != string(ai.KindActivity) {
		t.Errorf("expected analysis_kind=%q, got %q", ai.KindActivity, args.AnalysisKind)
	}
	if args.Scope != expectedScope {
		t.Errorf("expected scope=%q, got %q", expectedScope, args.Scope)
	}
	if args.ActivityID != aiTestActivityID {
		t.Errorf("expected activity_id=%q, got %q", aiTestActivityID, args.ActivityID)
	}
	if call.opts == nil {
		t.Fatal("expected insert options with the maximum attempts")
	}
	if call.opts.MaxAttempts != jobs.AIAnalysisMaxAttempts(cfg) {
		t.Errorf("expected MaxAttempts=%d, got %d", jobs.AIAnalysisMaxAttempts(cfg), call.opts.MaxAttempts)
	}

	body := decodeAIResponse(t, w.Body.Bytes())
	if body["kind"] != string(ai.KindActivity) {
		t.Errorf("expected kind=%q in the response, got %v", ai.KindActivity, body["kind"])
	}
	if body["scope"] != expectedScope {
		t.Errorf("expected scope=%q in the response, got %v", expectedScope, body["scope"])
	}
	if body["status"] != "pending" {
		t.Errorf("expected status=pending in the response, got %v", body["status"])
	}
	if body["id"] == "" || body["id"] == nil {
		t.Errorf("expected the id of the created analysis in the response, got %v", body["id"])
	}
}

// TestPostAIAnalysis_DerivesTheWindowForWeeklyAndLoad proves the documented
// defaults: the weekly window is 7 days and the load window is 28 days,
// both starting today (UTC), and the scope is built with the shared helpers
// so the row and the job target the same objective.
func TestPostAIAnalysis_DerivesTheWindowForWeeklyAndLoad(t *testing.T) {
	today := time.Now().UTC().Format(ai.ScopeDateLayout)

	cases := []struct {
		name         string
		body         string
		expectedKind string
		expectedDays int
	}{
		{"weekly defaults", `{"kind":"weekly"}`, string(ai.KindWeekly), 7},
		{"load defaults", `{"kind":"load"}`, string(ai.KindLoad), 28},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			store := &fakeAIStore{
				prefs: sqlc.GetUserPreferencesByIDRow{AiEnabled: true, Timezone: "UTC"},
			}
			inserter := &fakeAIInserter{}

			h := PostAIAnalysis(store, inserter, aiReadyConfig())
			w := httptest.NewRecorder()
			h.ServeHTTP(w, aiRequest(http.MethodPost, "/api/v1/ai/analyses", c.body))

			if w.Code != http.StatusAccepted {
				t.Fatalf("expected 202, got %d (body=%s)", w.Code, w.Body.String())
			}
			if len(store.created) != 1 {
				t.Fatalf("expected exactly one row created, got %d", len(store.created))
			}
			created := store.created[0]
			if created.Kind != c.expectedKind {
				t.Errorf("expected kind=%q, got %q", c.expectedKind, created.Kind)
			}
			expectedScope := ai.WeeklyScope(mustParseDate(t, today))
			if c.expectedKind == string(ai.KindLoad) {
				expectedScope = ai.LoadScope(mustParseDate(t, today), c.expectedDays)
			}
			if created.Scope != expectedScope {
				t.Errorf("expected scope=%q, got %q", expectedScope, created.Scope)
			}
			if len(inserter.calls) != 1 {
				t.Fatalf("expected exactly one job enqueued, got %d", len(inserter.calls))
			}
			args := inserter.calls[0].args.(jobs.AIAnalysisArgs)
			if args.WindowStart != today {
				t.Errorf("expected window_start=%q, got %q", today, args.WindowStart)
			}
			if args.WindowDays != c.expectedDays {
				t.Errorf("expected window_days=%d, got %d", c.expectedDays, args.WindowDays)
			}
		})
	}
}

// TestPostAIAnalysis_ExplicitWindowIsUsed proves that an explicit window is
// honoured and encoded in the scope key.
func TestPostAIAnalysis_ExplicitWindowIsUsed(t *testing.T) {
	store := &fakeAIStore{
		prefs: sqlc.GetUserPreferencesByIDRow{AiEnabled: true, Timezone: "UTC"},
	}
	inserter := &fakeAIInserter{}
	start := mustParseDate(t, "2026-01-05")

	h := PostAIAnalysis(store, inserter, aiReadyConfig())
	w := httptest.NewRecorder()
	h.ServeHTTP(w, aiRequest(http.MethodPost, "/api/v1/ai/analyses", `{"kind":"load","window_start":"2026-01-05","window_days":14}`))

	if w.Code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d (body=%s)", w.Code, w.Body.String())
	}
	expectedScope := ai.LoadScope(start, 14)
	if store.created[0].Scope != expectedScope {
		t.Errorf("expected scope=%q, got %q", expectedScope, store.created[0].Scope)
	}
}

// ---------------------------------------------------------------------------
// GET /api/v1/ai/analyses
// ---------------------------------------------------------------------------

// TestListAIAnalyses_ReturnsOnlyTheAuthenticatedUsersAnalyses proves that
// the listing is scoped to the authenticated user and honours the limit.
func TestListAIAnalyses_ReturnsOnlyTheAuthenticatedUsersAnalyses(t *testing.T) {
	store := &fakeAIStore{
		prefs: sqlc.GetUserPreferencesByIDRow{AiEnabled: true, Timezone: "UTC"},
		listed: []sqlc.AiAnalysis{
			{ID: aiUUID(t, aiTestActivityID), Kind: string(ai.KindActivity), Scope: ai.ActivityScope(aiTestActivityID), Status: "pending"},
		},
	}

	h := ListAIAnalyses(store)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, aiRequest(http.MethodGet, "/api/v1/ai/analyses", ""))

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d (body=%s)", w.Code, w.Body.String())
	}
	if len(store.listParams) != 1 {
		t.Fatalf("expected one listing query, got %d", len(store.listParams))
	}
	if store.listParams[0].UserID != aiUUID(t, aiTestUserID) {
		t.Errorf("expected the query to be scoped to the authenticated user, got %v", store.listParams[0].UserID)
	}
	if store.listParams[0].Limit != aiDefaultListLimit {
		t.Errorf("expected the default limit %d, got %d", aiDefaultListLimit, store.listParams[0].Limit)
	}
	body := decodeAIResponse(t, w.Body.Bytes())
	data, ok := body["data"].([]any)
	if !ok {
		t.Fatalf("expected a data array, got %v", body["data"])
	}
	if len(data) != 1 {
		t.Fatalf("expected 1 analysis, got %d", len(data))
	}
	first, ok := data[0].(map[string]any)
	if !ok {
		t.Fatalf("expected an analysis object, got %v", data[0])
	}
	if first["scope"] != ai.ActivityScope(aiTestActivityID) {
		t.Errorf("expected the scope of the analysis, got %v", first["scope"])
	}
}

// TestListAIAnalyses_HonoursTheLimit proves that the limit is bounded: a
// small limit is forwarded to the query and a huge one is capped.
func TestListAIAnalyses_HonoursTheLimit(t *testing.T) {
	cases := []struct {
		query string
		want  int32
	}{
		{"", aiDefaultListLimit},
		{"?limit=5", 5},
		{"?limit=500", aiMaxListLimit},
		{"?limit=0", aiDefaultListLimit},
	}
	for _, c := range cases {
		t.Run("limit "+c.query, func(t *testing.T) {
			store := &fakeAIStore{listed: []sqlc.AiAnalysis{}}

			h := ListAIAnalyses(store)
			w := httptest.NewRecorder()
			h.ServeHTTP(w, aiRequest(http.MethodGet, "/api/v1/ai/analyses"+c.query, ""))

			if w.Code != http.StatusOK {
				t.Fatalf("expected 200, got %d (body=%s)", w.Code, w.Body.String())
			}
			if len(store.listParams) != 1 {
				t.Fatalf("expected one listing query, got %d", len(store.listParams))
			}
			if store.listParams[0].Limit != c.want {
				t.Errorf("expected limit %d, got %d", c.want, store.listParams[0].Limit)
			}
		})
	}
}

// TestListAIAnalyses_MissingSingleAnalysisIsNotFound proves that asking for
// an analysis that does not exist is a 404.
func TestListAIAnalyses_MissingSingleAnalysisIsNotFound(t *testing.T) {
	store := &fakeAIStore{gotErr: pgx.ErrNoRows}

	h := ListAIAnalyses(store)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, aiRequest(http.MethodGet, "/api/v1/ai/analyses?kind=activity&scope="+ai.ActivityScope(aiTestActivityID), ""))

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d (body=%s)", w.Code, w.Body.String())
	}
}

// TestListAIAnalyses_ReturnsTheRequestedSingleAnalysis proves that the
// single analysis is read with the scope of the authenticated user.
func TestListAIAnalyses_ReturnsTheRequestedSingleAnalysis(t *testing.T) {
	scope := ai.ActivityScope(aiTestActivityID)
	store := &fakeAIStore{
		got: sqlc.AiAnalysis{
			ID:     aiUUID(t, aiTestActivityID),
			Kind:   string(ai.KindActivity),
			Scope:  scope,
			Status: "completed",
		},
	}

	h := ListAIAnalyses(store)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, aiRequest(http.MethodGet, "/api/v1/ai/analyses?kind=activity&scope="+scope, ""))

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d (body=%s)", w.Code, w.Body.String())
	}
	if len(store.getParams) != 1 {
		t.Fatalf("expected one single-analysis query, got %d", len(store.getParams))
	}
	if store.getParams[0].UserID != aiUUID(t, aiTestUserID) {
		t.Errorf("expected the query to be scoped to the authenticated user, got %v", store.getParams[0].UserID)
	}
	if store.getParams[0].Scope != scope {
		t.Errorf("expected scope=%q, got %q", scope, store.getParams[0].Scope)
	}
	body := decodeAIResponse(t, w.Body.Bytes())
	if body["scope"] != scope {
		t.Errorf("expected scope=%q in the response, got %v", scope, body["scope"])
	}
}

// TestListAIAnalyses_FailedAnalysisExposesOnlyItsSafeCodeAndMessage proves
// that a failed analysis exposes the safe error code and message and never
// the stored payload.
func TestListAIAnalyses_FailedAnalysisExposesOnlyItsSafeCodeAndMessage(t *testing.T) {
	scope := ai.ActivityScope(aiTestActivityID)
	store := &fakeAIStore{
		got: sqlc.AiAnalysis{
			ID:           aiUUID(t, aiTestActivityID),
			Kind:         string(ai.KindActivity),
			Scope:        scope,
			Status:       "failed",
			Payload:      []byte(`{"distance_meters":12345,"avg_heart_rate":150}`),
			ErrorCode:    pgtype.Text{String: "provider_timeout", Valid: true},
			ErrorMessage: pgtype.Text{String: "El proveedor de IA no respondió a tiempo.", Valid: true},
		},
	}

	h := ListAIAnalyses(store)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, aiRequest(http.MethodGet, "/api/v1/ai/analyses?kind=activity&scope="+scope, ""))

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d (body=%s)", w.Code, w.Body.String())
	}
	body := decodeAIResponse(t, w.Body.Bytes())
	if body["error_code"] != "provider_timeout" {
		t.Errorf("expected error_code=provider_timeout, got %v", body["error_code"])
	}
	if body["error_message"] != "El proveedor de IA no respondió a tiempo." {
		t.Errorf("expected the safe error message, got %v", body["error_message"])
	}
	if body["output"] != nil {
		t.Errorf("expected no output for a failed analysis, got %v", body["output"])
	}
	if strings.Contains(w.Body.String(), "distance_meters") {
		t.Errorf("the response leaked the stored payload: %s", w.Body.String())
	}
}

// mustParseDate parses a YYYY-MM-DD date for the scope assertions.
func mustParseDate(t *testing.T, value string) time.Time {
	t.Helper()
	parsed, err := time.Parse(ai.ScopeDateLayout, value)
	if err != nil {
		t.Fatalf("parse date %q: %v", value, err)
	}
	return parsed
}
