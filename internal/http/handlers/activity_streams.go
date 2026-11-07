package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/riverqueue/river"

	"github.com/fgjcarlos/ghamusinos/internal/auth"
	"github.com/fgjcarlos/ghamusinos/internal/db/sqlc"
	"github.com/fgjcarlos/ghamusinos/internal/jobs"
)

// ActivityStreamsStore is the narrow querier surface the streams handlers
// need. Keeping it narrow avoids dragging the whole sqlc.Querier into the
// handler tests; *sqlc.Queries satisfies it in production.
type ActivityStreamsStore interface {
	GetActivityByExternalID(context.Context, sqlc.GetActivityByExternalIDParams) (sqlc.Activity, error)
	GetActivityStreams(context.Context, pgtype.UUID) ([]sqlc.ActivityStream, error)
}

// requiredActivityStreams are the streams Post-Activity needs to build
// plan-vs-real splits. Phase 1.6, issue #18. Without any of them the read
// endpoint returns 409 with the missing types in the detail.
var requiredActivityStreams = []string{"latlng", "time"}

type activityStreamsResponse struct {
	ActivityID string                     `json:"activity_id"`
	StartedAt  time.Time                  `json:"started_at"`
	Streams    map[string]json.RawMessage `json:"streams"`
}

// GetActivityStreams serves GET /api/v1/activities/{id}/streams.
//
// It returns every stored stream for the activity as raw JSON so the web
// client can read `time`, `latlng` and `altitude` without the backend
// duplicating their shapes. The activity is resolved by external id and
// user, exactly like GetActivity, so crossing users is impossible.
//
// 409 with a `streams unavailable` detail when `time` or `latlng` is
// missing: the client then offers the refresh action, which enqueues the
// import job for this activity.
func GetActivityStreams(store ActivityStreamsStore) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestID := middleware.GetReqID(r.Context())
		user := auth.AuthUser(r.Context())
		if user == nil {
			WriteProblem(w, NewUnauthorized("not authenticated", requestID))
			return
		}

		externalID, ok := parseActivityExternalID(w, r, requestID)
		if !ok {
			return
		}

		activity, err := store.GetActivityByExternalID(r.Context(), sqlc.GetActivityByExternalIDParams{
			UserID:         parseUserID(user.ID),
			ExternalSource: "strava",
			ExternalID:     externalID,
		})
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				WriteProblem(w, NewNotFound("activity not found", requestID))
				return
			}
			WriteProblem(w, NewInternalError("failed to fetch activity", requestID))
			return
		}

		rows, err := store.GetActivityStreams(r.Context(), activity.ID)
		if err != nil {
			WriteProblem(w, NewInternalError("failed to fetch activity streams", requestID))
			return
		}

		streams := make(map[string]json.RawMessage, len(rows))
		for _, row := range rows {
			streams[row.StreamType] = json.RawMessage(row.Data)
		}

		if missing := missingActivityStreams(streams); len(missing) > 0 {
			WriteProblem(w, newStreamsUnavailable(
				"activity streams unavailable: missing "+strings.Join(missing, ", "),
				requestID,
			))
			return
		}

		resp := activityStreamsResponse{
			ActivityID: uuid.UUID(activity.ID.Bytes).String(),
			StartedAt:  activity.StartedAt.Time.UTC(),
			Streams:    streams,
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		//nolint:errcheck
		json.NewEncoder(w).Encode(resp)
	})
}

// PostActivityStreamsRefresh serves POST /api/v1/activities/{id}/streams/refresh.
//
// It enqueues the Strava streams import for one activity the user owns and
// returns 202. This is the recovery path for activities imported before
// the `time` stream was requested: their row lacks `time`, the read
// endpoint returns 409, and the UI offers this button.
func PostActivityStreamsRefresh(store ActivityStreamsStore, inserter jobs.RiverJobInserter) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestID := middleware.GetReqID(r.Context())
		user := auth.AuthUser(r.Context())
		if user == nil {
			WriteProblem(w, NewUnauthorized("not authenticated", requestID))
			return
		}

		externalID, ok := parseActivityExternalID(w, r, requestID)
		if !ok {
			return
		}

		if _, err := store.GetActivityByExternalID(r.Context(), sqlc.GetActivityByExternalIDParams{
			UserID:         parseUserID(user.ID),
			ExternalSource: "strava",
			ExternalID:     externalID,
		}); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				WriteProblem(w, NewNotFound("activity not found", requestID))
				return
			}
			WriteProblem(w, NewInternalError("failed to fetch activity", requestID))
			return
		}

		if inserter == nil {
			WriteProblem(w, NewProblem503("activity streams unavailable: the job queue is not configured", requestID))
			return
		}

		args := jobs.ImportStravaStreamsArgs{
			UserID:             user.ID,
			ActivityExternalID: strconv.FormatInt(externalID, 10),
			StravaActivityID:   externalID,
		}
		// ByArgs collapses repeated clicks while a job for this activity is
		// still queued. Once it completes, a new click enqueues a fresh run.
		opts := &river.InsertOpts{UniqueOpts: river.UniqueOpts{ByArgs: true}}
		result, err := inserter.Insert(r.Context(), args, opts)
		if err != nil {
			WriteProblem(w, NewProblem503("activity streams unavailable: failed to enqueue the import job", requestID))
			return
		}

		jobID := int64(0)
		if result != nil {
			jobID = result.Job.ID
		}

		resp := map[string]interface{}{
			"status":    "queued",
			"job_id":    formatJobID(jobID),
			"queued_at": time.Now().UTC().Format(time.RFC3339),
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusAccepted)
		//nolint:errcheck
		json.NewEncoder(w).Encode(resp)
	})
}

// parseActivityExternalID reads the `{id}` chi parameter and writes a 400
// problem when it is not a valid Strava external id. The bool is false
// when the response was already written.
func parseActivityExternalID(w http.ResponseWriter, r *http.Request, requestID string) (int64, bool) {
	externalID, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		WriteProblem(w, NewBadRequest("invalid activity id", requestID))
		return 0, false
	}
	return externalID, true
}

// missingActivityStreams returns the required stream types that are absent
// or empty. A JSON `null` or `[]` is treated as missing: it gives the
// client nothing to plot.
func missingActivityStreams(streams map[string]json.RawMessage) []string {
	missing := make([]string, 0, len(requiredActivityStreams))
	for _, streamType := range requiredActivityStreams {
		raw, ok := streams[streamType]
		if !ok || streamIsEmpty(raw) {
			missing = append(missing, streamType)
		}
	}
	return missing
}

func streamIsEmpty(raw json.RawMessage) bool {
	trimmed := strings.TrimSpace(string(raw))
	return trimmed == "" || trimmed == "null" || trimmed == "[]"
}

// newStreamsUnavailable builds the 409 problem used when the activity
// exists but lacks the streams Post-Activity needs. The title is stable so
// the web client can map it without parsing the detail text.
func newStreamsUnavailable(detail, instance string) ProblemDetail {
	return ProblemDetail{
		Type:     "about:blank",
		Title:    "Streams Unavailable",
		Status:   http.StatusConflict,
		Detail:   detail,
		Instance: instance,
	}
}
