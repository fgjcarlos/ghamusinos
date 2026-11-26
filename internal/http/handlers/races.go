package handlers

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/fgjcarlos/ghamusinos/internal/races"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

// MountRaces registers race routes on the authenticated v1 router.
func MountRaces(r chi.Router, s *races.Store) {
	r.Method(http.MethodGet, "/races", raceHandler(s))
	r.Method(http.MethodPost, "/races", raceHandler(s))
	r.Method(http.MethodGet, "/races/{id}", raceHandler(s))
	r.Method(http.MethodPut, "/races/{id}", raceHandler(s))
	r.Method(http.MethodDelete, "/races/{id}", raceHandler(s))
	r.Method(http.MethodPut, "/races/{id}/tracks/{trackId}", raceTrackHandler(s))
	r.Method(http.MethodDelete, "/races/{id}/tracks/{trackId}", raceTrackHandler(s))
}

func raceHandler(s *races.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user, ok := workoutAuth(w, r)
		if !ok {
			return
		}
		ctx := r.Context()
		if chi.URLParam(r, "id") == "" {
			if r.Method == http.MethodGet {
				listRaces(w, r, s, user)
				return
			}
			var draft races.Draft
			if err := decodeWorkoutJSON(r, &draft); err != nil {
				writeWorkoutDecodeError(w, r, err)
				return
			}
			v, err := s.Create(ctx, user, draft)
			writeRaceResponse(w, r, http.StatusCreated, v, err)
			return
		}
		id, err := uuid.Parse(chi.URLParam(r, "id"))
		if err != nil {
			writeRaceResponse(w, r, 0, nil, races.ErrNotFound)
			return
		}
		switch r.Method {
		case http.MethodGet:
			v, err := s.Get(ctx, user, id)
			writeRaceResponse(w, r, http.StatusOK, v, err)
		case http.MethodPut:
			var draft races.Draft
			if err := decodeWorkoutJSON(r, &draft); err != nil {
				writeWorkoutDecodeError(w, r, err)
				return
			}
			v, err := s.Update(ctx, user, id, draft)
			writeRaceResponse(w, r, http.StatusOK, v, err)
		case http.MethodDelete:
			writeRaceResponse(w, r, http.StatusNoContent, nil, s.Delete(ctx, user, id))
		}
	}
}

func listRaces(w http.ResponseWriter, r *http.Request, s *races.Store, user uuid.UUID) {
	q := r.URL.Query()
	limit, offset := int64(20), int64(0)
	var err error
	if raw := q.Get("limit"); raw != "" {
		limit, err = strconv.ParseInt(raw, 10, 32)
		if err != nil || limit < 1 || limit > 100 {
			workoutProblem(w, r, http.StatusBadRequest, "limit must be between 1 and 100")
			return
		}
	}
	if raw := q.Get("offset"); raw != "" {
		offset, err = strconv.ParseInt(raw, 10, 32)
		if err != nil || offset < 0 {
			workoutProblem(w, r, http.StatusBadRequest, "offset must be a non-negative integer")
			return
		}
	}
	data, total, err := s.List(r.Context(), user, int32(limit), int32(offset))
	writeRaceResponse(w, r, http.StatusOK, map[string]any{
		"data": data, "total": total, "limit": limit, "offset": offset, "has_next": offset+int64(len(data)) < total,
	}, err)
}

func raceTrackHandler(s *races.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user, ok := workoutAuth(w, r)
		if !ok {
			return
		}
		id, idErr := uuid.Parse(chi.URLParam(r, "id"))
		track, trackErr := uuid.Parse(chi.URLParam(r, "trackId"))
		if idErr != nil || trackErr != nil {
			writeRaceResponse(w, r, 0, nil, races.ErrNotFound)
			return
		}
		writeRaceResponse(w, r, http.StatusNoContent, nil, s.SetTrack(r.Context(), user, id, track, r.Method == http.MethodPut))
	}
}

func writeRaceResponse(w http.ResponseWriter, r *http.Request, status int, v any, err error) {
	switch {
	case errors.Is(err, races.ErrNotFound):
		workoutProblem(w, r, http.StatusNotFound, err.Error())
	case errors.Is(err, races.ErrInvalid):
		workoutProblem(w, r, http.StatusUnprocessableEntity, "name, race_date, distance_m, elevation_gain_m, priority or notes is invalid")
	case err != nil:
		workoutProblem(w, r, http.StatusInternalServerError, "failed to process race")
	case status == http.StatusNoContent:
		w.WriteHeader(status)
	default:
		writeWorkoutJSON(w, status, v)
	}
}
