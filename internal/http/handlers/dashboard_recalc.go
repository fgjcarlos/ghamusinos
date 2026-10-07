// POST /api/v1/dashboard/recalc — DA-004 (issue #16, PR3).
//
// Enqueues a full-history RecalcTrainingLoad River job for the
// authenticated user and returns 202 with { job_id, queued_at,
// will_recompute_from }. The River inserter is the same one used by
// the Strava webhook adapter (jobs.RiverJobInserter); this handler
// does NOT own the worker — `internal/jobs/recalc_training_load.go`
// already does. `will_recompute_from` is the user's first-activity
// date when activities exist, else null.
//
// 401 if no auth user; 503 if the River inserter is nil (dev stack
// without River); 500 on internal.
package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5/middleware"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/rivertype"

	"github.com/fgjcarlos/ghamusinos/internal/auth"
	"github.com/fgjcarlos/ghamusinos/internal/jobs"
)

// FirstActivityQuerier is the sqlc subset needed to compute
// will_recompute_from. Mirrors FirstActivityForUser exactly.
type FirstActivityQuerier interface {
	FirstActivityForUser(ctx context.Context, userID pgtype.UUID) (pgtype.Timestamptz, error)
}

// recalcInsertResult carries enough of River's JobInsertResult to
// satisfy the 202 response without leaking the full struct.
type recalcInsertResult interface {
	JobID() int64
}

// riverJobInsertResultAdapter wraps rivertype.JobInsertResult to
// satisfy recalcInsertResult when the real River client returns it.
// Tests can substitute their own fake returning a fixed JobID.
type riverJobInsertResultAdapter struct {
	r *rivertype.JobInsertResult
}

func (a riverJobInsertResultAdapter) JobID() int64 { return a.r.Job.ID }

// PostDashboardRecalc enqueues a full-history recalc job for the auth user.
//
// Returns 202 Accepted on success, 401 if unauthenticated, 503 if
// the inserter is nil, 500 on internal error.
func PostDashboardRecalc(q FirstActivityQuerier, inserter jobs.RiverJobInserter) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user := auth.AuthUser(r.Context())
		if user == nil {
			requestID := middleware.GetReqID(r.Context())
			WriteProblem(w, NewUnauthorized("not authenticated", requestID))
			return
		}
		if inserter == nil {
			requestID := middleware.GetReqID(r.Context())
			WriteProblem(w, NewProblem503("recalc unavailable: river queue not configured", requestID))
			return
		}
		userUUID, err := pgUUIDFromUserID(user.ID)
		if err != nil {
			requestID := middleware.GetReqID(r.Context())
			WriteProblem(w, NewInternalError("invalid user id", requestID))
			return
		}

		// Look up the user's first activity for `will_recompute_from`.
		// A missing first activity (Timestamptz Valid=false) → null in JSON.
		var willRecomputeFrom *string
		if first, err := q.FirstActivityForUser(r.Context(), userUUID); err == nil && first.Valid {
			s := first.Time.UTC().Format("2006-01-02")
			willRecomputeFrom = &s
		}

		// Enqueue with a 1-minute unique window so a burst of webhook
		// inserts (Strava reconnect + backfill reruns) doesn't pile up
		// duplicate recalcs (E7 from PR2 — same UniqueOpts pattern).
		args := jobs.RecalcTrainingLoadArgs{
			UserID: user.ID,
			From:   nil,
			To:     nil,
		}
		opts := &river.InsertOpts{
			UniqueOpts: river.UniqueOpts{
				ByPeriod: 1 * time.Minute,
			},
		}
		result, err := inserter.Insert(r.Context(), args, opts)
		if err != nil {
			requestID := middleware.GetReqID(r.Context())
			WriteProblem(w, NewInternalError("failed to enqueue recalc", requestID))
			return
		}

		jobID := int64(0)
		if result != nil {
			jobID = result.Job.ID
		}
		queuedAt := time.Now().UTC().Format(time.RFC3339)

		resp := map[string]interface{}{
			"job_id":               formatJobID(jobID),
			"queued_at":            queuedAt,
			"will_recompute_from":  willRecomputeFrom,
		}
		_ = FirstActivityQuerier(q) // type assertion silence: the variable is genuinely used.

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusAccepted)
		_ = json.NewEncoder(w).Encode(resp)
	})
}

// formatJobID converts an int64 job id into its decimal string form.
// River stores ids as int64; we keep them as strings in JSON to avoid
// JavaScript precision surprises (2^53).
func formatJobID(id int64) string {
	// Avoid importing strconv just for this; fmt.Sprintf keeps the
	// dependency surface flat and matches what the rest of the
	// handlers use.
	return formatInt(id)
}

func formatInt(n int64) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}

// NewProblem503 returns a 503 ProblemDetail. We don't have a helper in
// errors.go for it yet, so this file ships one locally to keep the
// handler self-contained.
func NewProblem503(detail, instance string) ProblemDetail {
	return ProblemDetail{
		Type:     "about:blank",
		Title:    "Service Unavailable",
		Status:   503,
		Detail:   detail,
		Instance: instance,
	}
}