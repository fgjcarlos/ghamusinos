// Package handlers contiene los handlers HTTP del servidor.
package handlers

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"reflect"
	"time"

	"github.com/go-chi/chi/v5/middleware"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/fgjcarlos/ghamusinos/internal/auth"
	"github.com/fgjcarlos/ghamusinos/internal/db/sqlc"
)

// InternalHealthHeader is the gating header for the extended /healthz
// response. When present (value "1") the handler returns the full
// internal section set; without it the public shape `{status:"ok"}` is
// kept verbatim — never a breaking change. Decision G3 (issue #16).
const InternalHealthHeader = "X-Internal-Health"

// HealthQuerier is the sqlc subset needed by the extended /healthz
// handler: dashboard_metadata for last_recalc_at + training_load_rows,
// and the latest sync session for strava.last_sync_at. Auth is not
// required; the same user-id-from-context lookup does not happen.
type HealthQuerier interface {
	GetDashboardMetadata(ctx context.Context, userID pgtype.UUID) (sqlc.DashboardMetadatum, error)
	GetLatestSyncSession(ctx context.Context, userID pgtype.UUID) (sqlc.SyncSession, error)
}

// HealthDBPinger is the pinger subset used by /healthz. We redeclare
// the interface locally so tests don't need the handlers package's
// DBPinger (which has identical semantics but is meant for /readyz).
type HealthDBPinger interface {
	Ping(ctx context.Context) error
}

// Health handles GET /healthz. The behaviour is:
//
//   - No X-Internal-Health header:
//       - DB OK  → 200 { status: "ok" }     (public shape, unchanged)
//       - DB down → 503 { status: "degraded", db: { ok: false } }
//   - With X-Internal-Health: 1:
//       - DB OK  → 200 with extended sections (db, strava,
//                   last_recalc_at, training_load_rows, ai)
//       - DB down → 503 { status: "degraded", db: { ok: false } }
//                   DB-dependent sections (strava, last_recalc_at,
//                   training_load_rows, ai) are omitted because they
//                   cannot be computed without a working DB.
//
// pinger may be nil (dev mode); the handler treats nil as DB-down.
// q may be nil when no querier is wired (router skips /api routes).
func Health(pinger HealthDBPinger, q HealthQuerier) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		// We resolve the auth user from context to scope dashboard
		// metadata. When the public request has no auth, we fall back
		// to a zero UUID which the SQL queries treat as "no row";
		// public /healthz is unaffected because the gate is only on
		// the *internal* branch.
		var userID pgtype.UUID
		if user := auth.AuthUser(r.Context()); user != nil {
			if u, err := pgUUIDFromUserID(user.ID); err == nil {
				userID = u
			}
		}

		internal := r.Header.Get(InternalHealthHeader) == "1"

		// ----- DB ping -----
		// A typed-nil pinger (e.g. *pgxpool.Pool passed as nil) must
		// be caught before calling Ping() to avoid a nil-deref panic.
		// Both nil-interface and typed-nil cases fall to the degraded
		// branch (DA-005). This matches the dev-stack path where the
		// router is built without a pool (unit tests, dev binaries).
		dbOK := false
		var dbLatencyMS int64
		pingable := isPingable(pinger)
		if pingable {
			ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
			defer cancel()
			start := time.Now()
			if err := pinger.Ping(ctx); err == nil {
				dbOK = true
				dbLatencyMS = time.Since(start).Milliseconds()
			} else {
				slog.Warn("healthz: ping a la base de datos fallido", "err", err)
			}
		} else {
			slog.Warn("healthz: pinger es nil, tratando como DB no cableada")
		}

		if !dbOK {
			w.WriteHeader(http.StatusServiceUnavailable)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"status": "degraded",
				"db":     map[string]bool{"ok": false},
			})
			return
		}

		if !internal {
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
			return
		}

		// ----- Extended (internal) -----
		// Build the response with whatever sections we can fetch. We
		// gracefully degrade: a missing querier / dashboard_metadata
		// row leaves the corresponding keys nil so the UI can render
		// "no data yet" without us hiding the key.
		resp := map[string]interface{}{
			"status": "ok",
			"db": map[string]interface{}{
				"ok":         true,
				"latency_ms": dbLatencyMS,
			},
		}

		if q != nil {
			if md, err := q.GetDashboardMetadata(r.Context(), userID); err == nil && md.LastRecalcAt.Valid {
				resp["last_recalc_at"] = md.LastRecalcAt.Time.UTC().Format(time.RFC3339)
				resp["training_load_rows"] = md.TrainingLoadRows
			}
			if ss, err := q.GetLatestSyncSession(r.Context(), userID); err == nil && ss.FinishedAt.Valid {
				resp["strava"] = map[string]interface{}{
					"ok":           true,
					"last_sync_at": ss.FinishedAt.Time.UTC().Format(time.RFC3339),
				}
			} else {
				resp["strava"] = map[string]interface{}{"ok": false, "last_sync_at": nil}
			}
		} else {
			resp["strava"] = map[string]interface{}{"ok": false, "last_sync_at": nil}
		}

		// AI: placeholder per spec ("ai.enabled"). Until Phase 1.5 lands,
		// we expose `false` so the UI can already show the section
		// without surprises.
		resp["ai"] = map[string]bool{"enabled": false}

		requestID := middleware.GetReqID(r.Context())
		_ = requestID // reserved for future error-mapping

		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(resp)
	})
}

// HealthPublic is a thin wrapper kept for tests and the public-only
// path. Returns the simple {status:"ok"} response with no querier
// dependency. Used by TestHealth in health_test.go.
func HealthPublic(w http.ResponseWriter, _ *http.Request) {
	Health(nil, nil).ServeHTTP(w, &http.Request{})
}

// isPingable uses reflect to safely check whether the underlying value
// is a non-nil pointer. Without this check, calling Ping() on a
// typed-nil interface (e.g. *pgxpool.Pool(nil) passed into
// HealthDBPinger) would panic with a nil-deref. Returning false here
// means the handler degrades to 503 instead.
func isPingable(p HealthDBPinger) bool {
	if p == nil {
		return false
	}
	v := reflect.ValueOf(p)
	switch v.Kind() {
	case reflect.Ptr, reflect.Interface, reflect.Map, reflect.Slice, reflect.Chan, reflect.Func:
		return !v.IsNil()
	default:
		return true
	}
}