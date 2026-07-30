package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5/middleware"
	"github.com/google/uuid"

	"github.com/fgjcarlos/ghamusinos/internal/auth"
	"github.com/fgjcarlos/ghamusinos/internal/db/sqlc"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

// ListActivities returns a paginated list of the authenticated user's activities,
// sorted by started_at DESC.
// Returns 200 with {data, page, total, has_next} on success, 401 if not authenticated.
func ListActivities(q sqlc.Querier) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Get user from context
		user := auth.AuthUser(r.Context())
		if user == nil {
			requestID := middleware.GetReqID(r.Context())
			problem := NewUnauthorized("not authenticated", requestID)
			WriteProblem(w, problem)
			return
		}

		// Parse pagination parameters
		limit, offset := parsePagination(r)

		// Query activities
		activities, err := q.ListActivitiesByUser(r.Context(), sqlc.ListActivitiesByUserParams{
			UserID: parseUserID(user.ID),
			Limit:  int32(limit),
			Offset: int32(offset),
		})
		if err != nil {
			requestID := middleware.GetReqID(r.Context())
			problem := NewInternalError("failed to fetch activities", requestID)
			WriteProblem(w, problem)
			return
		}

		// Build response
		page := offset/limit + 1
		hasNext := len(activities) == limit
		total := offset + len(activities)
		if hasNext {
			total += 1 // Indicate there's at least one more
		}

		resp := map[string]interface{}{
			"data":     activities,
			"page":     page,
			"total":    total,
			"has_next": hasNext,
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		//nolint:errcheck
		json.NewEncoder(w).Encode(resp)
	})
}

// GetActivity returns a single activity by external ID if owned by the authenticated user.
// Returns 200 with activity JSON, 401 if not authenticated, 404 if not found or not owned.
func GetActivity(q sqlc.Querier) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Get user from context
		user := auth.AuthUser(r.Context())
		if user == nil {
			requestID := middleware.GetReqID(r.Context())
			problem := NewUnauthorized("not authenticated", requestID)
			WriteProblem(w, problem)
			return
		}

		// Parse external ID from path parameter (chi style or direct URL parsing)
		var externalIDStr string
		// Try chi URLParam first (for chi router)
		if val := r.Context().Value("id"); val != nil {
			if v, ok := val.(string); ok {
				externalIDStr = v
			}
		}
		// Fallback: try extracting from URL path directly for tests
		if externalIDStr == "" {
			// Extract from /.../{id}
			parts := strings.Split(strings.TrimSuffix(r.URL.Path, "/"), "/")
			if len(parts) > 0 {
				externalIDStr = parts[len(parts)-1]
			}
		}

		externalID, err := strconv.ParseInt(externalIDStr, 10, 64)
		if err != nil {
			requestID := middleware.GetReqID(r.Context())
			problem := NewBadRequest("invalid activity id", requestID)
			WriteProblem(w, problem)
			return
		}

		// Query activity
		userUUID := parseUserID(user.ID)
		activity, err := q.GetActivityByExternalID(r.Context(), sqlc.GetActivityByExternalIDParams{
			UserID:         userUUID,
			ExternalSource: "strava",
			ExternalID:     externalID,
		})
		if err != nil {
			if err == pgx.ErrNoRows {
				requestID := middleware.GetReqID(r.Context())
				problem := NewNotFound("activity not found", requestID)
				WriteProblem(w, problem)
				return
			}
			requestID := middleware.GetReqID(r.Context())
			problem := NewInternalError("failed to fetch activity", requestID)
			WriteProblem(w, problem)
			return
		}

		// Defensive check: verify returned activity belongs to authenticated user
		// (should never happen since SQL query filters by user_id, but good defensive practice)
		if activity.UserID != userUUID {
			requestID := middleware.GetReqID(r.Context())
			problem := NewNotFound("activity not found", requestID)
			WriteProblem(w, problem)
			return
		}

		// Return activity
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		//nolint:errcheck
		json.NewEncoder(w).Encode(activity)
	})
}

// SyncStatus returns the most recent sync_session for the authenticated user.
// Returns 200 with sync session JSON, 401 if not authenticated, 404 if no session exists.
func SyncStatus(q sqlc.Querier) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Get user from context
		user := auth.AuthUser(r.Context())
		if user == nil {
			requestID := middleware.GetReqID(r.Context())
			problem := NewUnauthorized("not authenticated", requestID)
			WriteProblem(w, problem)
			return
		}

		// Query latest sync session
		session, err := q.GetLatestSyncSession(r.Context(), parseUserID(user.ID))
		if err != nil {
			if err == pgx.ErrNoRows {
				requestID := middleware.GetReqID(r.Context())
				problem := NewNotFound("no sync session found", requestID)
				WriteProblem(w, problem)
				return
			}
			requestID := middleware.GetReqID(r.Context())
			problem := NewInternalError("failed to fetch sync status", requestID)
			WriteProblem(w, problem)
			return
		}

		// Return session
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		//nolint:errcheck
		json.NewEncoder(w).Encode(session)
	})
}

// parsePagination extracts limit and offset from query parameters.
// Defaults: limit=20, page=1 (offset=0). Max limit=100.
func parsePagination(r *http.Request) (limit, offset int) {
	limit = 20
	page := 1

	if l := r.URL.Query().Get("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 {
			limit = parsed
			if limit > 100 {
				limit = 100
			}
		}
	}

	if p := r.URL.Query().Get("page"); p != "" {
		if parsed, err := strconv.Atoi(p); err == nil && parsed > 0 {
			page = parsed
		}
	}

	offset = (page - 1) * limit
	return
}

// parseUserID converts a string user ID to pgtype.UUID for database queries.
func parseUserID(userID string) pgtype.UUID {
	parsed, err := uuid.Parse(userID)
	if err != nil {
		// Return invalid UUID on error (should not happen in normal operation)
		return pgtype.UUID{Valid: false}
	}
	return pgtype.UUID{Bytes: parsed, Valid: true}
}
