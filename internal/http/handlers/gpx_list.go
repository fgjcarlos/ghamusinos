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

// GPXListStore es el subset del GPXStore que usa ListGPX. Definido
// localmente para que el handler sea testeable con un stub sin
// importar pgxpool ni sqlc.
type GPXListStore interface {
	List(ctx context.Context, userID pgtype.UUID, params gpx.ListParams) (*gpx.PaginatedTracks, error)
}

const (
	defaultGPXListLimit = 20
	maxGPXListLimit     = 100
)

// ListGPX devuelve los tracks del usuario autenticado, paginados.
// Query params soportados:
//   - limit: 1..maxGPXListLimit (default 20)
//   - offset: >=0 (default 0)
//   - sort: por ahora sólo se acepta el valor por defecto (created_at DESC).
//     Valores distintos devuelven 400 Bad Request — el SQL está optimizado
//     para ese ORDER BY y cambiarlo requiere tocar la query SQLC.
//     # ponytail: sort real por difficulty/distance/duration cuando alguna
//     feature concreta lo pida (issue #124 lo nombraba pero no hay UX hoy
//     que lo consuma).
func ListGPX(store GPXListStore) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestID := middleware.GetReqID(r.Context())
		user := auth.AuthUser(r.Context())
		if user == nil {
			WriteProblem(w, NewUnauthorized("not authenticated", requestID))
			return
		}

		limit, err := parseLimit(r.URL.Query().Get("limit"))
		if err != nil {
			WriteProblem(w, NewBadRequest(err.Error(), requestID))
			return
		}
		offset, err := parseOffset(r.URL.Query().Get("offset"))
		if err != nil {
			WriteProblem(w, NewBadRequest(err.Error(), requestID))
			return
		}
		if sort := r.URL.Query().Get("sort"); sort != "" && sort != "created_at DESC" {
			WriteProblem(w, NewBadRequest(
				"sort must be 'created_at DESC' (only supported order for now)", requestID))
			return
		}

		result, err := store.List(r.Context(), parseUserID(user.ID), gpx.ListParams{
			Limit:  limit,
			Offset: offset,
		})
		if err != nil {
			WriteProblem(w, NewInternalError("failed to list GPX tracks", requestID))
			return
		}

		// El store puede haber clamp-eado el limit (p.ej. al default);
		// propagamos los valores resueltos al response para que el
		// cliente vea lo que realmente se aplicó.
		result.Limit = limit
		result.Offset = offset

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		escribirJSON(w, result)
	})
}

// DeleteGPX elimina un track del usuario autenticado. El cascade
// (gpx_climbs, gpx_risk_zones) lo gestiona el FK ON DELETE CASCADE
// de la migración 00006 — el handler no necesita transacción.
func DeleteGPX(store GPXListStore) http.Handler {
	// Reutilizamos la interfaz porque Delete está en GPXStore, pero
	// GPXListStore es lo que tenemos en este handler. Refactor
	// redundante para no inflar interfaces: tipamos por la operación.
	type deleter interface {
		Delete(ctx context.Context, userID pgtype.UUID, trackID pgtype.UUID) error
	}
	d, ok := store.(deleter)
	if !ok {
		// En producción GPXStore implementa ambos; este panic sólo se
		// dispara si alguien pasa un store fake sin Delete en un test.
		panic("DeleteGPX: store does not implement Delete")
	}
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
		if err := d.Delete(r.Context(), parseUserID(user.ID), pgtype.UUID{Bytes: parsedID, Valid: true}); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				WriteProblem(w, NewNotFound("GPX track not found", requestID))
				return
			}
			WriteProblem(w, NewInternalError("failed to delete GPX track", requestID))
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})
}

// parseLimit convierte el query param "limit" a int32 validado.
// Default y tope definidos por defaultGPXListLimit / maxGPXListLimit.
func parseLimit(raw string) (int32, error) {
	if raw == "" {
		return defaultGPXListLimit, nil
	}
	n, err := strconv.Atoi(raw)
	if err != nil {
		return 0, errors.New("limit must be a non-negative integer")
	}
	if n < 1 {
		return 0, errors.New("limit must be >= 1")
	}
	if n > maxGPXListLimit {
		return 0, errors.New("limit must be <= 100")
	}
	return int32(n), nil
}

// parseOffset convierte el query param "offset" a int32 validado.
func parseOffset(raw string) (int32, error) {
	if raw == "" {
		return 0, nil
	}
	n, err := strconv.Atoi(raw)
	if err != nil {
		return 0, errors.New("offset must be a non-negative integer")
	}
	if n < 0 {
		return 0, errors.New("offset must be >= 0")
	}
	return int32(n), nil
}
