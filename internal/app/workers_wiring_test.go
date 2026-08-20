// TestRiverWorkers_AppWiringCallsConfigureTokenRefresher is the AUD-01
// regression guard for the worker wiring closure (finding C5).
//
// The audit found that ConfigureTokenRefresher (internal/jobs/workers.go:31)
// is never called from production wiring. NewRiverWorkers() reads package
// globals set by ConfigureTokenRefresher; with the call missing, every River
// worker registers with nil dependencies and the first job at runtime blows
// up silently.
//
// This file ships as a placeholder. AUD-01 says explicitly:
//
//	"La forma concreta depende de AUD-04 — coordinad las dos, pero este
//	test se escribe primero y se ve fallar."
//
// The concrete assertion needs a stable seam: either an InitWorkers(pool, cfg,
// client, cipherKey) helper that both app.Run and the test can call, or an
// exported function that returns the workers built with their dependencies.
// AUD-04 (issue #164) introduces that seam; this test activates in the same
// PR.
//
// Why a Skip and not a runtime check on River internals: River does not
// expose the registered workers' dependency fields, and reflecting into
// unexported fields is exactly the "vigilar maquinaria" the audit warns
// against. Better to wait for the seam AUD-04 introduces.
//
// Refs #162 (AUD-01, C5 closure). Companion C3 (webhooks not wired via
// WithWebhooks) is covered by TestBuildRouter_StravaOptionalRoutesWhenConfigured
// in routes_test.go.
package app

import "testing"

// TestRiverWorkers_AppWiringCallsConfigureTokenRefresher is intentionally a
// skip. Activate in the same PR as the AUD-04 helper:
//
//   - Replace the t.Skip below with the agreed assertion:
//     assert that the helper wired into app.Run produces a workers set
//     whose every dependency field is non-nil.
//   - Remove this file's long preamble once the assertion is in place; it
//     documents the audit finding, not the test mechanics.
func TestRiverWorkers_AppWiringCallsConfigureTokenRefresher(t *testing.T) {
	t.Skip("AUD-01 placeholder: activates in the AUD-04 (issue #164) PR, " +
		"when NewRiverWorkers / InitWorkers gains an exported seam that this " +
		"test can call with real dependencies. See file comment for the audit " +
		"context.")
}
