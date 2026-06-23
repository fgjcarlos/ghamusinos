package auth

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/fgjcarlos/ghamusinos/internal/db/sqlc"
)

// AuthMiddleware validates JWT tokens from the Authorization header.
// Extracts the Bearer token, validates it, and injects Claims into the context.
// Returns 401 on validation failure.
func AuthMiddleware(validator JWTValidator) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Extract Authorization header
			auth := r.Header.Get("Authorization")
			if auth == "" {
				jsonError(w, "unauthorized", http.StatusUnauthorized)
				return
			}

			// Extract Bearer token
			const bearerPrefix = "Bearer "
			if !strings.HasPrefix(auth, bearerPrefix) {
				jsonError(w, "unauthorized", http.StatusUnauthorized)
				return
			}

			token := auth[len(bearerPrefix):]
			if token == "" {
				jsonError(w, "unauthorized", http.StatusUnauthorized)
				return
			}

			// Validate token
			claims, err := validator.Validate(r.Context(), token)
			if err != nil {
				jsonError(w, "unauthorized", http.StatusUnauthorized)
				return
			}

			// Inject claims into context and proceed
			ctx := WithAuthClaims(r.Context(), claims)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// ResolveMiddleware resolves Claims to a User record.
// Expects Claims to already be in the context (typically added by AuthMiddleware).
// Injects the resolved User into the context for downstream handlers.
// Returns 500 on resolution error.
func ResolveMiddleware(resolver UserResolver) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Get claims from context (should be set by AuthMiddleware)
			claims := AuthClaims(r.Context())
			if claims == nil {
				jsonError(w, "unauthorized", http.StatusUnauthorized)
				return
			}

			// Resolve user
			sqlcUser, err := resolver.Resolve(r.Context(), claims)
			if err != nil {
				jsonError(w, "internal error", http.StatusInternalServerError)
				return
			}

			// Convert sqlc.User to internal User type
			user := &User{
				ID:           sqlcUser.ID.String(),
				ClerkUserID:  sqlcUser.ClerkUserID,
				Email:        sqlcUser.Email,
				DisplayName:  sqlcUser.DisplayName.String,
				InviteStatus: string(sqlcUser.InviteStatus),
			}

			// Inject user into context and proceed
			ctx := WithAuthUser(r.Context(), user)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// InviteGateMiddleware enforces invite-based access control.
// Expects User to already be in the context (typically added by ResolveMiddleware).
// Rules:
//   - invite_status='active': allow
//   - invite_status='pending': check for valid pending invite; if found, mark accepted and promote user
//   - invite_status='blocked': deny with 403
//
// Returns 403 if access is denied.
func InviteGateMiddleware(q sqlc.Querier) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Get user from context (should be set by ResolveMiddleware)
			user := AuthUser(r.Context())
			if user == nil {
				jsonError(w, "unauthorized", http.StatusUnauthorized)
				return
			}

			// Check invite status
			switch user.InviteStatus {
			case "active":
				// Allow
				next.ServeHTTP(w, r)
			case "pending":
				// Check for valid pending invite
				inviteRow, err := q.GetActiveInviteByEmail(r.Context(), user.Email)
				if err != nil {
					// No valid invite found
					jsonError(w, "forbidden", http.StatusForbidden)
					return
				}

				// Mark invite as accepted
				if err := q.MarkInviteAccepted(r.Context(), inviteRow.ID); err != nil {
					jsonError(w, "internal error", http.StatusInternalServerError)
					return
				}

				// TODO: Update user invite_status to 'active' in a separate query
				// (This would require an UpdateUserInviteStatus query in sqlc)
				// For now, promote in memory for this request
				user.InviteStatus = "active"
				ctx := WithAuthUser(r.Context(), user)
				next.ServeHTTP(w, r.WithContext(ctx))

			default:
				// blocked or unknown
				jsonError(w, "forbidden", http.StatusForbidden)
			}
		})
	}
}

// jsonError writes a JSON error response.
func jsonError(w http.ResponseWriter, message string, code int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": message})
}
