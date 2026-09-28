// Tests for DELETE /api/v1/strava/connection (closes-159-strava-disconnect, PR2).
//
// Reuses preferencesMockQuerier (defined in me_preferences_test.go) to
// drive DeleteStravaTokensByUserID with a configurable hook. All
// other Querier methods stay no-ops because the handler only touches
// the delete path.

package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/fgjcarlos/ghamusinos/internal/auth"
)

const testUserID = "00000000-0000-0000-0000-000000000001"

func newDeleteReq(userID string) *http.Request {
	return httptest.NewRequestWithContext(
		auth.WithAuthUser(context.Background(), &auth.User{ID: userID}),
		http.MethodDelete, "/api/v1/strava/connection", nil)
}

func TestDeleteStravaConnection_Success(t *testing.T) {
	var (
		calledWith pgtype.UUID
		called     bool
	)
	mock := &preferencesMockQuerier{
		t: t,
		deleteStravaTokens: func(userID pgtype.UUID) error {
			called = true
			calledWith = userID
			return nil
		},
	}
	h := DeleteStravaConnection(mock)
	req := newDeleteReq(testUserID)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusNoContent {
		t.Errorf("expected 204 No Content, got %d (body=%s)", w.Code, w.Body.String())
	}
	if w.Body.Len() != 0 {
		t.Errorf("expected empty body on 204, got %q", w.Body.String())
	}
	if !called {
		t.Fatal("expected DeleteStravaTokensByUserID to be called once")
	}
	if !calledWith.Valid {
		t.Errorf("expected delete called with a valid pgtype.UUID, got invalid")
	}
	wantUUID, err := uuid.Parse(testUserID)
	if err != nil {
		t.Fatalf("test setup: invalid uuid: %v", err)
	}
	if calledWith.Bytes != wantUUID {
		t.Errorf("expected delete called with %v, got %v", wantUUID, calledWith.Bytes)
	}
}

func TestDeleteStravaConnection_Unauthorized(t *testing.T) {
	called := false
	mock := &preferencesMockQuerier{
		t: t,
		deleteStravaTokens: func(userID pgtype.UUID) error {
			called = true
			return nil
		},
	}
	h := DeleteStravaConnection(mock)
	req := httptest.NewRequestWithContext(context.Background(), http.MethodDelete, "/api/v1/strava/connection", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", w.Code)
	}
	if called {
		t.Errorf("delete must not run without an authenticated user")
	}
	var problem ProblemDetail
	if err := json.Unmarshal(w.Body.Bytes(), &problem); err != nil {
		t.Fatalf("expected ProblemDetail JSON, got %q", w.Body.String())
	}
	if problem.Status != http.StatusUnauthorized {
		t.Errorf("expected problem.Status=401, got %d", problem.Status)
	}
}

func TestDeleteStravaConnection_InvalidUserID(t *testing.T) {
	called := false
	mock := &preferencesMockQuerier{
		t: t,
		deleteStravaTokens: func(userID pgtype.UUID) error {
			called = true
			return nil
		},
	}
	h := DeleteStravaConnection(mock)
	req := httptest.NewRequestWithContext(
		auth.WithAuthUser(context.Background(), &auth.User{ID: "not-a-uuid"}),
		http.MethodDelete, "/api/v1/strava/connection", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected 500 for malformed user id, got %d", w.Code)
	}
	if called {
		t.Errorf("delete must not run with a malformed user id")
	}
	var problem ProblemDetail
	if err := json.Unmarshal(w.Body.Bytes(), &problem); err != nil {
		t.Fatalf("expected ProblemDetail JSON, got %q", w.Body.String())
	}
	if !strings.Contains(problem.Detail, "invalid user id") {
		t.Errorf("expected detail to mention invalid user id, got %q", problem.Detail)
	}
}

func TestDeleteStravaConnection_DBError(t *testing.T) {
	mock := &preferencesMockQuerier{
		t: t,
		deleteStravaTokens: func(userID pgtype.UUID) error {
			return errors.New("synthetic boom")
		},
	}
	h := DeleteStravaConnection(mock)
	req := newDeleteReq(testUserID)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected 500 on DB error, got %d", w.Code)
	}
	var problem ProblemDetail
	if err := json.Unmarshal(w.Body.Bytes(), &problem); err != nil {
		t.Fatalf("expected ProblemDetail JSON, got %q", w.Body.String())
	}
	if !strings.Contains(problem.Detail, "disconnect strava") {
		t.Errorf("expected detail to mention disconnect strava, got %q", problem.Detail)
	}
}

// Idempotent at the user level: a second DELETE with the same token
// still returns 204 (sqlc's :exec delete treats zero-rows-affected
// as nil). This matches the user-facing semantics: clicking the
// disconnect button twice does not flip success to failure.
func TestDeleteStravaConnection_Idempotent(t *testing.T) {
	calls := 0
	mock := &preferencesMockQuerier{
		t: t,
		deleteStravaTokens: func(userID pgtype.UUID) error {
			calls++
			return nil
		},
	}
	h := DeleteStravaConnection(mock)
	for i := 0; i < 2; i++ {
		req := newDeleteReq(testUserID)
		w := httptest.NewRecorder()
		h.ServeHTTP(w, req)
		if w.Code != http.StatusNoContent {
			t.Errorf("call %d: expected 204, got %d", i, w.Code)
		}
	}
	if calls != 2 {
		t.Errorf("expected 2 delete calls, got %d", calls)
	}
}