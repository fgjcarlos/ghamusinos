package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5/middleware"

	"github.com/fgjcarlos/ghamusinos/internal/auth"
	"github.com/fgjcarlos/ghamusinos/internal/db/sqlc"
)

// Me returns the authenticated user's profile.
// Returns 200 with user JSON on success, 401 if no user in context.
func Me(q sqlc.Querier) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Get user from context (set by auth middleware)
		user := auth.AuthUser(r.Context())
		if user == nil {
			requestID := middleware.GetReqID(r.Context())
			problem := NewUnauthorized("not authenticated", requestID)
			WriteProblem(w, problem)
			return
		}

		// Return user as JSON
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		//nolint:errcheck

		//nolint:errcheck
		json.NewEncoder(w).Encode(map[string]interface{}{
			"id":            user.ID,
			"clerk_user_id": user.ClerkUserID,
			"email":         user.Email,
			"display_name":  user.DisplayName,
			"invite_status": user.InviteStatus,
		})
	})
}
