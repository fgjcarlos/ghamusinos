// Package handlers: Strava connection management handlers.
//
// closes-159-strava-disconnect (PR2) introduces the destructive side
// of the Strava lifecycle: disconnecting the OAuth link by deleting
// the encrypted tokens row. The query
// (DeleteStravaTokensByUserID) already exists in
// internal/db/sqlc/strava_tokens.sql.go; this handler is the thin
// HTTP wrapper around it.
//
// Design notes:
//
//   - Mounted under /api/v1/strava/connection (DELETE) so it sits
//     next to the OAuth connect handler (GET /api/v1/strava/connect).
//   - The handler returns 204 No Content on success — there is no
//     resource body to ship back. Imported Strava activities are
//     intentionally NOT deleted; the user keeps their history.
//   - The handler is independent of the Strava OAuth client; the
//     tokens are stored encrypted on our side, so "disconnect" is a
//     pure local operation. The Strava-side grant revocation
//     (DELETE /oauth/deauthorize) is a separate concern tracked
//     outside this slice.
//   - PR3 will add GET /api/v1/strava/connection to expose the
//     current connection state to the SPA.

package handlers

import (
	"net/http"

	"github.com/go-chi/chi/v5/middleware"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/fgjcarlos/ghamusinos/internal/auth"
	"github.com/fgjcarlos/ghamusinos/internal/db/sqlc"
)

// DeleteStravaConnection deletes the encrypted Strava tokens row
// for the authenticated user, severing the OAuth link.
//
// Returns:
//
//   - 204 No Content on success.
//   - 401 Unauthorized if the request has no authenticated user.
//   - 500 Internal Server Error if the database call fails or the
//     user id format is malformed.
//
// Idempotent at the user level: a second DELETE with the same
// token still returns 204 (the row is already gone; sqlc's
// :exec DELETE returns nil rows-affected but no error).
func DeleteStravaConnection(q sqlc.Querier) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user := auth.AuthUser(r.Context())
		if user == nil {
			requestID := middleware.GetReqID(r.Context())
			problem := NewUnauthorized("not authenticated", requestID)
			WriteProblem(w, problem)
			return
		}

		userID, err := uuid.Parse(user.ID)
		if err != nil {
			requestID := middleware.GetReqID(r.Context())
			problem := NewInternalError("invalid user id format", requestID)
			WriteProblem(w, problem)
			return
		}

		if err := q.DeleteStravaTokensByUserID(r.Context(), pgtype.UUID{Bytes: userID, Valid: true}); err != nil {
			requestID := middleware.GetReqID(r.Context())
			problem := NewInternalError("failed to disconnect strava", requestID)
			WriteProblem(w, problem)
			return
		}

		w.WriteHeader(http.StatusNoContent)
	})
}
