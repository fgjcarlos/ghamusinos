package handlers

import (
	"context"
	"errors"
	"net/http"

	"github.com/fgjcarlos/ghamusinos/internal/auth"
	"github.com/fgjcarlos/ghamusinos/internal/gpx"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

type GPXDetailStore interface {
	GetDetail(context.Context, pgtype.UUID, pgtype.UUID) (*gpx.StoredTrackDetail, error)
}

func GetGPX(store GPXDetailStore) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestID := middleware.GetReqID(r.Context())
		user := auth.AuthUser(r.Context())
		if user == nil {
			WriteProblem(w, NewUnauthorized("not authenticated", requestID))
			return
		}
		parsedID, err := uuid.Parse(chi.URLParam(r, "id"))
		if err != nil {
			WriteProblem(w, NewNotFound("GPX track not found", requestID))
			return
		}
		trackID := pgtype.UUID{Bytes: parsedID, Valid: true}
		detail, err := store.GetDetail(r.Context(), parseUserID(user.ID), trackID)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				WriteProblem(w, NewNotFound("GPX track not found", requestID))
				return
			}
			WriteProblem(w, NewInternalError("failed to fetch GPX track", requestID))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		escribirJSON(w, detail)
	})
}
