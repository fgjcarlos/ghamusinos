package handlers

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/fgjcarlos/ghamusinos/internal/blocks"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

// MountBlocks registers training block routes on the authenticated v1 router.
func MountBlocks(r chi.Router, s *blocks.Store) {
	r.Method(http.MethodGet, "/blocks", blockHandler(s))
	r.Method(http.MethodPost, "/blocks", blockHandler(s))
	r.Method(http.MethodGet, "/blocks/{id}", blockHandler(s))
	r.Method(http.MethodPut, "/blocks/{id}", blockHandler(s))
	r.Method(http.MethodDelete, "/blocks/{id}", blockHandler(s))
}

func blockHandler(s *blocks.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user, ok := workoutAuth(w, r)
		if !ok {
			return
		}
		ctx := r.Context()
		if chi.URLParam(r, "id") == "" {
			if r.Method == http.MethodGet {
				listBlocks(w, r, s, user)
				return
			}
			var draft blocks.Draft
			if err := decodeWorkoutJSON(r, &draft); err != nil {
				writeWorkoutDecodeError(w, r, err)
				return
			}
			v, err := s.Create(ctx, user, draft)
			writeBlockResponse(w, r, http.StatusCreated, v, err)
			return
		}
		id, err := uuid.Parse(chi.URLParam(r, "id"))
		if err != nil {
			writeBlockResponse(w, r, 0, nil, blocks.ErrNotFound)
			return
		}
		switch r.Method {
		case http.MethodGet:
			v, err := s.Get(ctx, user, id)
			writeBlockResponse(w, r, http.StatusOK, v, err)
		case http.MethodPut:
			var draft blocks.Draft
			if err := decodeWorkoutJSON(r, &draft); err != nil {
				writeWorkoutDecodeError(w, r, err)
				return
			}
			v, err := s.Update(ctx, user, id, draft)
			writeBlockResponse(w, r, http.StatusOK, v, err)
		case http.MethodDelete:
			writeBlockResponse(w, r, http.StatusNoContent, nil, s.Delete(ctx, user, id))
		}
	}
}

func listBlocks(w http.ResponseWriter, r *http.Request, s *blocks.Store, user uuid.UUID) {
	limit, offset, ok := parseLimitOffset(w, r)
	if !ok {
		return
	}
	data, total, err := s.List(r.Context(), user, int32(limit), int32(offset))
	writeBlockResponse(w, r, http.StatusOK, listEnvelope(data, total, limit, offset), err)
}

// parseLimitOffset reads the standard list pagination: limit between 1 and 100
// (20 by default) and a non-negative offset (0 by default).
func parseLimitOffset(w http.ResponseWriter, r *http.Request) (limit, offset int64, ok bool) {
	q := r.URL.Query()
	limit, offset = 20, 0
	var err error
	if raw := q.Get("limit"); raw != "" {
		limit, err = strconv.ParseInt(raw, 10, 32)
		if err != nil || limit < 1 || limit > 100 {
			workoutProblem(w, r, http.StatusBadRequest, "limit must be between 1 and 100")
			return 0, 0, false
		}
	}
	if raw := q.Get("offset"); raw != "" {
		offset, err = strconv.ParseInt(raw, 10, 32)
		if err != nil || offset < 0 {
			workoutProblem(w, r, http.StatusBadRequest, "offset must be a non-negative integer")
			return 0, 0, false
		}
	}
	return limit, offset, true
}

// listEnvelope is the standard paginated list response.
func listEnvelope(data any, total int64, limit, offset int64) map[string]any {
	return map[string]any{"data": data, "total": total, "limit": limit, "offset": offset, "has_next": offset+int64(len(data.([]blocks.Block))) < total}
}

func writeBlockResponse(w http.ResponseWriter, r *http.Request, status int, v any, err error) {
	switch {
	case errors.Is(err, blocks.ErrNotFound):
		workoutProblem(w, r, http.StatusNotFound, err.Error())
	case errors.Is(err, blocks.ErrInvalid):
		workoutProblem(w, r, http.StatusUnprocessableEntity, "name, start_date, end_date or notes is invalid")
	case errors.Is(err, blocks.ErrOverlap):
		workoutConflict(w, r, err.Error())
	case err != nil:
		workoutProblem(w, r, http.StatusInternalServerError, "failed to process training block")
	case status == http.StatusNoContent:
		w.WriteHeader(status)
	default:
		writeWorkoutJSON(w, status, v)
	}
}
