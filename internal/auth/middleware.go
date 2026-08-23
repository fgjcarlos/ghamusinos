package auth

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/jackc/pgx/v5/pgtype"

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
//
// Status codes:
//   - 401 if claims are missing from context (should never happen if wired correctly)
//   - 409 if the email is already bound to a different Clerk account (issue #168)
//   - 500 on any other resolution error
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
				if errors.Is(err, ErrEmailTaken) {
					jsonError(w, "an account already exists with this email", http.StatusConflict)
					return
				}
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
//   - invite_status='pending': check for valid pending invite; if found, ask the
//     InvitePromoter to flip it to active atomically (issue #168). If promotion
//     succeeds, the request proceeds with the promoted user; if it fails, 500.
//   - invite_status='blocked': deny with 403
//
// The promoter owns the transaction so that MarkInviteAccepted and
// UpdateUserInviteStatus either both commit or both roll back.
func InviteGateMiddleware(q sqlc.Querier, promoter InvitePromoter) func(http.Handler) http.Handler {
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

				// Promote pending → active atomically. If the second write
				// fails, MarkInviteAccepted is rolled back too (issue #168,
				// AC "si el segundo falla, la invitación no debe quedar
				// marcada como aceptada").
				userID, err := pgtypeUUIDFromString(user.ID)
				if err != nil {
					jsonError(w, "internal error", http.StatusInternalServerError)
					return
				}
				updated, err := promoter.PromotePendingInvite(r.Context(), inviteRow.ID, userID)
				if err != nil {
					jsonError(w, "internal error", http.StatusInternalServerError)
					return
				}

				// Build the promoted user straight from the row so subsequent
				// requests no longer take the pending branch.
				promoted := &User{
					ID:           updated.ID.String(),
					ClerkUserID:  updated.ClerkUserID,
					Email:        updated.Email,
					DisplayName:  updated.DisplayName.String,
					InviteStatus: string(updated.InviteStatus),
				}
				ctx := WithAuthUser(r.Context(), promoted)
				next.ServeHTTP(w, r.WithContext(ctx))

			default:
				// blocked or unknown
				jsonError(w, "forbidden", http.StatusForbidden)
			}
		})
	}
}

// pgtypeUUIDFromString converts an internal user ID (string form) to the
// pgtype.UUID the SQLC queries expect. Internal auth IDs are already UUID
// strings because ResolveMiddleware uses sqlcUser.ID.String().
func pgtypeUUIDFromString(s string) (pgtype.UUID, error) {
	var id pgtype.UUID
	if err := id.Scan(s); err != nil {
		return pgtype.UUID{}, err
	}
	return id, nil
}

// jsonError writes a JSON error response.
// ponytail: encoding errors here are dropped because the response writer is
// already past WriteHeader; we cannot recover them in any useful way. One
// //nolint covers the single Encode call.
func jsonError(w http.ResponseWriter, message string, code int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	//nolint:errcheck
	json.NewEncoder(w).Encode(map[string]string{"error": message})
}
