package handlers

import (
	"context"
	"errors"
	"net/http"
	"strconv"

	"github.com/fgjcarlos/ghamusinos/internal/auth"
	"github.com/fgjcarlos/ghamusinos/internal/gpx"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

type GPXDetailStore interface {
	GetDetail(context.Context, pgtype.UUID, pgtype.UUID, int) (*gpx.StoredTrackDetail, error)
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
		// ?resolution=N: el cliente puede pedir más o menos puntos para
		// la pantalla de detalle. 0 o ausente → DefaultResolution (2000,
		// en gpx.DefaultResolution). issue #170, M9.
		resolution := parseResolution(r.URL.Query().Get("resolution"))
		detail, err := store.GetDetail(r.Context(), parseUserID(user.ID), trackID, resolution)
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

// parseResolution traduce el query param a un entero positivo. Vacío o
// no numérico → 0 (la store aplica DefaultResolution). Negativos
// también caen a 0, mismo motivo.
func parseResolution(raw string) int {
	if raw == "" {
		return 0
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n <= 0 {
		return 0
	}
	return n
}
