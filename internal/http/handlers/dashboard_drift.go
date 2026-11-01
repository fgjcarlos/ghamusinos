// Package handlers — endpoint de cardiac drift del dashboard.
//
// Implementa DA-007 (issue #16, PR-B): GET /api/v1/dashboard/cardiac-drift.
//
//   - GET /cardiac-drift  drift por actividad en el rango (DA-007).
//
// El handler sigue el patrón del resto del paquete: factory que recibe el
// subconjunto de sqlc.Querier que necesita, gating con auth.AuthUser y
// escritura de JSON o de problem details RFC 9457 vía WriteProblem.
//
// La fuente de datos es `activity_streams` con `stream_type = 'heartrate'`
// (columna `data` JSONB). El JSONB se parsea admitiendo las dos formas
// que conviven en el repo: array plano de números (modelo actual,
// internal/jobs/streams.go:hrSamplesFromFrame) y pares `[time, value]`
// (forma real de la API de Strava).
//
// El cálculo del drift NO se reimplementa aquí: se delega en
// internal/metrics.CardiacDriftSeries (media del primer cuartil vs media
// del tercer cuartil de las muestras de FC, en porcentaje).
package handlers

import (
	"context"
	"encoding/json"
	"math"
	"net/http"

	"github.com/go-chi/chi/v5/middleware"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/fgjcarlos/ghamusinos/internal/auth"
	"github.com/fgjcarlos/ghamusinos/internal/db/sqlc"
	"github.com/fgjcarlos/ghamusinos/internal/metrics"
)

// CardiacDriftQuerier es el subconjunto de sqlc.Querier que necesita
// GetCardiacDrift. Separarlo del Querier completo mantiene los tests
// mínimos (sin pool de base de datos) y documenta la dependencia real
// del handler.
type CardiacDriftQuerier interface {
	ListHeartRateStreamsInRange(ctx context.Context, arg sqlc.ListHeartRateStreamsInRangeParams) ([]sqlc.ListHeartRateStreamsInRangeRow, error)
}

// cardiacDriftActivity es una entrada del array `activities` de DA-007.
type cardiacDriftActivity struct {
	ActivityID  string  `json:"activity_id"`
	DriftPct    float64 `json:"drift_pct"`
	SampleCount int     `json:"sample_count"`
}

// cardiacDriftResponse es el body 200 del endpoint de drift.
type cardiacDriftResponse struct {
	From       string                 `json:"from"`
	To         string                 `json:"to"`
	Available  bool                   `json:"available"`
	Activities []cardiacDriftActivity `json:"activities"`
}

// GetCardiacDrift devuelve el drift cardíaco por actividad del usuario en
// el rango `from`/`to` (query params ISO, mismos defaults que el resto de
// endpoints del dashboard: to=hoy, from=to-30d).
//
// Body 200: { from, to, available, activities: [{ activity_id, drift_pct,
// sample_count }] }. `available` es true cuando hay al menos una actividad
// con drift calculable (2 o más muestras de FC utilizables).
//
// 401 si no hay usuario autenticado; 400 con rango inválido o invertido;
// 500 en error interno (fallo de la query).
func GetCardiacDrift(q CardiacDriftQuerier) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user := auth.AuthUser(r.Context())
		if user == nil {
			requestID := middleware.GetReqID(r.Context())
			WriteProblem(w, NewUnauthorized("not authenticated", requestID))
			return
		}
		userUUID, err := pgUUIDFromUserID(user.ID)
		if err != nil {
			requestID := middleware.GetReqID(r.Context())
			WriteProblem(w, NewInternalError("invalid user id", requestID))
			return
		}

		from, to, err := parseDashboardRange(r)
		if err != nil {
			requestID := middleware.GetReqID(r.Context())
			WriteProblem(w, NewBadRequest(err.Error(), requestID))
			return
		}

		// El rango cubre días completos: el límite superior es EXCLUSIVO
		// (00:00 UTC del día siguiente a `to`) para que la actividad del
		// último día entre completa.
		rows, err := q.ListHeartRateStreamsInRange(r.Context(), sqlc.ListHeartRateStreamsInRangeParams{
			UserID:      userUUID,
			StartedAt:   pgtype.Timestamptz{Time: from, Valid: true},
			StartedAt_2: pgtype.Timestamptz{Time: to.AddDate(0, 0, 1), Valid: true},
		})
		if err != nil {
			requestID := middleware.GetReqID(r.Context())
			WriteProblem(w, NewInternalError("failed to load heart rate streams", requestID))
			return
		}

		activities := make([]cardiacDriftActivity, 0, len(rows))
		for _, row := range rows {
			samples := parseHeartRateSamples(row.Data)
			// Con menos de 2 muestras no hay drift calculable: la
			// actividad se omite del array (DA-007).
			if len(samples) < 2 {
				continue
			}
			activities = append(activities, cardiacDriftActivity{
				ActivityID:  row.ActivityID.String(),
				DriftPct:    round2(metrics.CardiacDriftSeries(samples)),
				SampleCount: len(samples),
			})
		}

		resp := cardiacDriftResponse{
			From:       from.Format("2006-01-02"),
			To:         to.Format("2006-01-02"),
			Available:  len(activities) > 0,
			Activities: activities,
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(resp)
	})
}

// parseHeartRateSamples extrae las muestras de frecuencia cardíaca de un
// stream JSONB (`activity_streams.data`).
//
// Admite las dos formas que conviven en el repo:
//   - array plano de números: `[120, 125, 130]` (modelo actual, el mismo
//     que produce internal/jobs/streams.go:hrSamplesFromFrame).
//   - pares `[time, value]`: `[[0, 120], [1, 125]]` (forma real de la API
//     de Strava); el valor de FC es el segundo elemento del par.
//
// Las entradas que no son numéricas se ignoran, y los valores 0 o
// negativos se descartan porque no son pulsaciones reales.
func parseHeartRateSamples(data []byte) []int {
	if len(data) == 0 {
		return nil
	}
	var raw []json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil
	}
	samples := make([]int, 0, len(raw))
	for _, item := range raw {
		value, ok := heartRateValue(item)
		if !ok || value <= 0 {
			continue
		}
		samples = append(samples, value)
	}
	return samples
}

// heartRateValue extrae el valor de FC de una entrada del stream: un número
// plano o el segundo elemento de un par `[time, value]`. Devuelve false
// cuando la entrada no tiene un valor numérico interpretable.
func heartRateValue(item json.RawMessage) (int, bool) {
	var number float64
	if err := json.Unmarshal(item, &number); err == nil {
		return int(math.Round(number)), true
	}
	var pair []float64
	if err := json.Unmarshal(item, &pair); err == nil && len(pair) >= 2 {
		return int(math.Round(pair[1])), true
	}
	return 0, false
}
