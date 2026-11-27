package handlers

import (
	"errors"
	"net/http"
	"time"

	"github.com/fgjcarlos/ghamusinos/internal/jobs"
	"github.com/fgjcarlos/ghamusinos/internal/reports"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/google/uuid"
)

// MountReports registers the reports API (fase 2.4, #320) on the authenticated
// v1 router. inserter may be nil: POST /reports/generate then answers 503
// instead of blocking, like /dashboard/recalc.
func MountReports(r chi.Router, s *reports.Store, inserter jobs.RiverJobInserter) {
	r.Get("/reports", listReports(s))
	r.Get("/reports/{id}", getReport(s))
	r.Post("/reports/generate", generateReport(s, inserter))
}

// listReports handles GET /api/v1/reports: the user's snapshots, newest period
// first, with the compact summary of each payload.
func listReports(s *reports.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user, ok := workoutAuth(w, r)
		if !ok {
			return
		}
		filter, ok := parseReportFilter(w, r)
		if !ok {
			return
		}
		limit, offset, ok := parseLimitOffset(w, r)
		if !ok {
			return
		}
		data, total, err := s.List(r.Context(), user, filter, int32(limit), int32(offset))
		if err != nil {
			writeReportResponse(w, r, http.StatusOK, nil, err)
			return
		}
		writeReportResponse(w, r, http.StatusOK, map[string]any{"data": data, "total": total, "limit": limit,
			"offset": offset, "has_next": offset+int64(len(data)) < total}, nil)
	}
}

// getReport handles GET /api/v1/reports/{id}: one report with its full payload.
func getReport(s *reports.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user, ok := workoutAuth(w, r)
		if !ok {
			return
		}
		id, err := uuid.Parse(chi.URLParam(r, "id"))
		if err != nil {
			writeReportResponse(w, r, http.StatusOK, nil, reports.ErrNotFound)
			return
		}
		record, err := s.Get(r.Context(), user, id)
		writeReportResponse(w, r, http.StatusOK, map[string]any{"id": record.ID,
			"generated_at": record.GeneratedAt, "payload": record.Payload}, err)
	}
}

// generateReport handles POST /api/v1/reports/generate: it validates and
// answers the period the request names, then enqueues the generate_report job
// of the user instead of generating inline.
func generateReport(s *reports.Store, inserter jobs.RiverJobInserter) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user, ok := workoutAuth(w, r)
		if !ok {
			return
		}
		var in reports.GenerateInput
		if err := decodeWorkoutJSON(r, &in); err != nil {
			writeWorkoutDecodeError(w, r, err)
			return
		}
		period, err := s.Period(r.Context(), user, in)
		if err != nil {
			writeReportResponse(w, r, 0, nil, err)
			return
		}
		if inserter == nil {
			WriteProblem(w, NewProblem503("report generation unavailable: river queue not configured", middleware.GetReqID(r.Context())))
			return
		}
		// The explicit kind/date/block makes the worker generate this period
		// instead of the closed ones the schedule covers.
		args := &jobs.GenerateReportArgs{UserID: user.String(), ReportKind: string(in.Kind),
			Date: in.Date, BlockID: in.BlockID}
		if _, err := inserter.Insert(r.Context(), args, nil); err != nil {
			WriteProblem(w, NewProblem503("failed to enqueue report generation", middleware.GetReqID(r.Context())))
			return
		}
		writeReportResponse(w, r, http.StatusAccepted, map[string]any{"status": "queued", "kind": period.Kind,
			"period_start": period.PeriodStart, "period_end": period.PeriodEnd}, nil)
	}
}

// parseReportFilter reads the optional kind and from/to of the list. from and to
// are civil dates and filter on overlap with the report period, so a report that
// merely touches the window is included.
func parseReportFilter(w http.ResponseWriter, r *http.Request) (reports.Filter, bool) {
	q := r.URL.Query()
	filter := reports.Filter{Kind: reports.Kind(q.Get("kind"))}
	switch filter.Kind {
	case "", reports.KindWeek, reports.KindMonth, reports.KindBlock:
	default:
		workoutProblem(w, r, http.StatusBadRequest, "kind must be week, month or block")
		return reports.Filter{}, false
	}
	for _, side := range []struct {
		name string
		dst  *time.Time
	}{{"from", &filter.From}, {"to", &filter.To}} {
		raw := q.Get(side.name)
		if raw == "" {
			continue
		}
		date, err := parseWorkoutDate(raw)
		if err != nil {
			workoutProblem(w, r, http.StatusBadRequest, side.name+" must use YYYY-MM-DD")
			return reports.Filter{}, false
		}
		*side.dst = date
	}
	if !filter.From.IsZero() && !filter.To.IsZero() && filter.To.Before(filter.From) {
		workoutProblem(w, r, http.StatusBadRequest, "from must not be after to")
		return reports.Filter{}, false
	}
	return filter, true
}

// writeReportResponse maps the report errors to the same problem bodies the rest
// of the API uses. status is ignored for the error branches.
func writeReportResponse(w http.ResponseWriter, r *http.Request, status int, v any, err error) {
	switch {
	case errors.Is(err, reports.ErrNotFound):
		workoutProblem(w, r, http.StatusNotFound, err.Error())
	case errors.Is(err, reports.ErrInvalid):
		workoutProblem(w, r, http.StatusUnprocessableEntity, "kind, date or block_id is invalid")
	case err != nil:
		workoutProblem(w, r, http.StatusInternalServerError, "failed to process report")
	default:
		writeWorkoutJSON(w, status, v)
	}
}
