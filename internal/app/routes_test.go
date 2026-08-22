// TestBuildRouter_MountsExpectedRoutes is the AUD-01 regression guard.
//
// The August 2026 audit found three slices that were written, tested against
// mocks, and never wired into buildRouter:
//
//   - WithWebhooks / Strava webhook store (Server.WithWebhooks, router.go:66)
//   - jobs.ConfigureTokenRefresher (workers.go:31)
//   - ListActivities / GetActivity / SyncStatus (handlers exist, router mounts none)
//
// Each slice passed its own unit tests because the tests bypassed buildRouter
// entirely. This test closes the class: it walks the actual router that
// production serves and asserts the exact list of mounted routes.
//
// The expected list is hand-written and ordered. Comment out any r.Get / r.Post
// in router.go and this test fails with a diff. Add a route without updating
// the list and this test fails. That is the point.
//
// Refs #162 (AUD-01, C3/C5/C6 closure).
package app

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sort"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stretchr/testify/require"

	"github.com/fgjcarlos/ghamusinos/internal/config"
	"github.com/fgjcarlos/ghamusinos/internal/db/sqlc"
)

// collectRoutes walks the chi router and returns a sorted slice of
// "METHOD /path" entries. chi.Walk also reports the catch-all /* used by the
// SPA handler; we keep it in the list — anyone removing it will see the diff.
func collectRoutes(t *testing.T, h http.Handler) []string {
	t.Helper()
	routes, ok := h.(chi.Routes)
	require.True(t, ok, "handler does not implement chi.Routes; pass the chi.Mux, not a wrapped http.Handler")
	var collected []string
	err := chi.Walk(routes, func(method, route string, _ http.Handler, _ ...func(http.Handler) http.Handler) error {
		collected = append(collected, fmt.Sprintf("%s %s", method, route))
		return nil
	})
	require.NoError(t, err)
	sort.Strings(collected)
	return collected
}

// TestBuildRouter_DefaultRoutesAreWired walks the router with cfg.Strava == nil
// and asserts the hand-maintained list of routes. If a slice is dropped from
// router.go without updating this list, the test fails.
func TestBuildRouter_DefaultRoutesAreWired(t *testing.T) {
	cfg := &config.Config{
		ClerkJWKSURL: "https://clerk.example.com/jwks",
	}
	h := buildRouter(cfg, nil, nil, nil, nil)

	got := collectRoutes(t, h)

	// Order matches sort.Strings of "METHOD /path". Do not hand-reorder: if a
	// future change moves a route into a different position, chi.Walk reports
	// it in lexicographic order and require.Equal will catch the drift.
	want := []string{
		"CONNECT /*",
		"DELETE /*",
		"DELETE /api/v1/gpx/{id}",
		"GET /*",
		"GET /api/v1/gpx/",
		"GET /api/v1/gpx/{id}",
		"GET /api/v1/me",
		"GET /healthz",
		"GET /readyz",
		"HEAD /*",
		"OPTIONS /*",
		"PATCH /*",
		"POST /*",
		"POST /api/v1/gpx/compare",
		"POST /api/v1/gpx/upload",
		"PUT /*",
		"QUERY /*",
		"TRACE /*",
	}

	require.Equal(t, want, got, "route list drifted; check router.go for new/changed/deleted routes and update the list in this test")
}

// TestBuildRouterMountsEveryRoute pins the complete production route tree with
// every optional Strava dependency enabled. The expected list is deliberately
// hand-maintained: adding or removing any mounted route requires an explicit
// review of this contract.
func TestBuildRouterMountsEveryRoute(t *testing.T) {
	cfg := &config.Config{
		ClerkJWKSURL: "https://clerk.example.com/jwks",
		Strava: &config.StravaConfig{
			ClientID:           "12345",
			ClientSecret:       "secret",
			RedirectURL:        "http://localhost/cb",
			Scopes:             "read",
			CipherKey:          bytes32(t),
			WebhookVerifyToken: "route-test-token",
		},
	}
	h := buildRouter(cfg, nil, nil, nil, &routeWebhookStore{})

	got := collectRoutes(t, h)

	want := []string{
		"CONNECT /*",
		"DELETE /*",
		"DELETE /api/v1/gpx/{id}",
		"GET /*",
		"GET /api/v1/gpx/",
		"GET /api/v1/gpx/{id}",
		"GET /api/v1/me",
		"GET /api/v1/strava/connect",
		"GET /healthz",
		"GET /readyz",
		"GET /strava/callback",
		"GET /strava/webhook",
		"HEAD /*",
		"OPTIONS /*",
		"PATCH /*",
		"POST /*",
		"POST /api/v1/gpx/compare",
		"POST /api/v1/gpx/upload",
		"POST /strava/webhook",
		"PUT /*",
		"QUERY /*",
		"TRACE /*",
	}
	require.Equal(t, want, got, "complete production route list drifted")

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet,
		"/strava/webhook?hub.mode=subscribe&hub.challenge=route-proof&hub.verify_token=route-test-token", nil)
	recorder := httptest.NewRecorder()
	h.ServeHTTP(recorder, req)
	require.Equal(t, http.StatusOK, recorder.Code)
	var challenge map[string]string
	require.NoError(t, json.NewDecoder(recorder.Body).Decode(&challenge))
	require.Equal(t, "route-proof", challenge["hub.challenge"])
}

type routeWebhookStore struct{}

func (*routeWebhookStore) GetUserIDByAthleteID(context.Context, int64) (pgtype.UUID, error) {
	return pgtype.UUID{}, nil
}

func (*routeWebhookStore) EnqueueActivityEvent(context.Context, sqlc.EnqueueActivityEventParams) (sqlc.ActivityEvent, error) {
	return sqlc.ActivityEvent{}, nil
}

func (*routeWebhookStore) EnqueueActivityEventJob(context.Context, string) error {
	return nil
}
