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
	"fmt"
	"net/http"
	"sort"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/require"

	"github.com/fgjcarlos/ghamusinos/internal/config"
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
	h := buildRouter(cfg, nil, nil, nil)

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

// TestBuildRouter_StravaOptionalRoutesWhenConfigured verifies the conditional
// wiring: when cfg.Strava is fully populated, the Strava OAuth routes appear;
// when it is nil, they do not. The two preceding tests cover the "Strava nil"
// side, this test covers the populated side.
//
// Note: /strava/webhook is intentionally NOT in this list. buildRouter does
// not call WithWebhooks, so the webhook routes are unreachable from production
// even when Strava is configured — see audit finding C3 and issue #165.
func TestBuildRouter_StravaOptionalRoutesWhenConfigured(t *testing.T) {
	cfg := &config.Config{
		ClerkJWKSURL: "https://clerk.example.com/jwks",
		Strava: &config.StravaConfig{
			ClientID:     "12345",
			ClientSecret: "secret",
			RedirectURL:  "http://localhost/cb",
			Scopes:       "read",
			CipherKey:    bytes32(t),
		},
	}
	h := buildRouter(cfg, nil, nil, nil)

	got := collectRoutes(t, h)

	stravaSubset := []string{
		// /api/v1/strava/connect stays under /api because the frontend calls
		// it with fetch() and carries the Authorization header. AUD-02 (#163)
		// moved it out of an r.Route("/strava") group but kept it inside /api.
		"GET /api/v1/strava/connect",
		// /strava/callback moved OUT of /api (AUD-02, finding C1): the
		// browser-top-level redirect from Strava does not carry an
		// Authorization header, so the handler must be reachable without
		// going through AuthMiddleware. The user_id rides inside the
		// signed state.
		"GET /strava/callback",
	}
	for _, want := range stravaSubset {
		require.Contains(t, got, want, "expected Strava route %q to be mounted when cfg.Strava is configured", want)
	}

	// /api/v1/strava/callback used to be in this list (the old design put
	// the callback under /api too); assert it is NOT mounted there now.
	require.NotContains(t, got, "GET /api/v1/strava/callback", "callback must live outside /api (AUD-02)")

	// Webhook routes must NOT be present: buildRouter does not call WithWebhooks.
	// AUD-03 (issue #165) is the issue that wires them. When it lands, this
	// assertion will fail and the list will need to be updated alongside the fix.
	for _, mustNot := range []string{"GET /strava/webhook", "POST /strava/webhook"} {
		require.NotContains(t, got, mustNot, "unexpected webhook route %q mounted; this PR should not wire WithWebhooks (that is AUD-03)", mustNot)
	}
}
