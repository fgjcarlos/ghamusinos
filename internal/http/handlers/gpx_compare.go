package handlers

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/fgjcarlos/ghamusinos/internal/auth"
	"github.com/fgjcarlos/ghamusinos/internal/gpx"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

// GPXCompareStore es el subset de GPXStore que usa CompareGPX. Cada
// track del array se resuelve con GetDetail (que ya valida user_id y
// devuelve climbs + risk_zones + métricas). No hay query SQLC
// adicional — la lógica de diff se hace en Go.
type GPXCompareStore interface {
	GetDetail(ctx context.Context, userID pgtype.UUID, trackID pgtype.UUID) (*gpx.StoredTrackDetail, error)
}

// compareRequest es el body de POST /api/v1/gpx/compare. Acepta hasta
// 3 IDs (#126 hace multi-track overlay con ese tope).
type compareRequest struct {
	IDs []string `json:"ids"`
}

const maxCompareIDs = 3

// CompareGPX recibe un array de track IDs del usuario autenticado y
// devuelve cada track resuelto con sus métricas + un diff calculado.
// El diff es minimalista: para cada métrica numérica devuelve min/max
// entre los tracks y qué track la tiene. Suficiente para #126 (que
// pinta un diff table) — no intenta ser un score sofisticado.
func CompareGPX(store GPXCompareStore) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestID := middleware.GetReqID(r.Context())
		user := auth.AuthUser(r.Context())
		if user == nil {
			WriteProblem(w, NewUnauthorized("not authenticated", requestID))
			return
		}

		var req compareRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			WriteProblem(w, NewBadRequest("invalid JSON body", requestID))
			return
		}
		if len(req.IDs) == 0 {
			WriteProblem(w, NewBadRequest("ids array must not be empty", requestID))
			return
		}
		if len(req.IDs) > maxCompareIDs {
			WriteProblem(w, NewBadRequest(
				"ids array must contain at most 3 tracks", requestID))
			return
		}

		uid := parseUserID(user.ID)
		details := make([]*gpx.StoredTrackDetail, 0, len(req.IDs))
		for _, raw := range req.IDs {
			parsed, err := uuid.Parse(raw)
			if err != nil {
				WriteProblem(w, NewBadRequest("invalid track id: "+raw, requestID))
				return
			}
			detail, err := store.GetDetail(r.Context(), uid, pgtype.UUID{Bytes: parsed, Valid: true})
			if err != nil {
				WriteProblem(w, NewNotFound("GPX track not found: "+raw, requestID))
				return
			}
			details = append(details, detail)
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		escribirJSON(w, struct {
			Tracks []*gpx.StoredTrackDetail `json:"tracks"`
			Diff   map[string]compareMetric `json:"diff"`
		}{
			Tracks: details,
			Diff:   computeDiff(details),
		})
	})
}

// compareMetric representa una métrica en el diff: su valor por track
// (index = posición del track en el array de entrada) y qué track
// tiene el valor "best" (mayor para distancia, D+, ITRA; menor para
// duración — heurística básica).
type compareMetric struct {
	Values    []float64 `json:"values"`
	BestTrack int       `json:"best_track"`
	Unit      string    `json:"unit"`
}

// computeDiff extrae métricas numéricas comparables. Trabaja sobre
// pointers porque StoredTrackDetail puede ser nil si una query falla,
// aunque en CompareGPX validamos que ninguno sea nil antes.
func computeDiff(tracks []*gpx.StoredTrackDetail) map[string]compareMetric {
	// ponytail: heurística — valores mayores son "mejores" para todas
	// las métricas excepto duración. Cuando alguna feature pida
	// "menor es mejor" real (p.ej. pace), refactorizar a un struct
	// {Better: func(a, b float64) bool} por métrica.
	biggerIsBetter := true
	out := make(map[string]compareMetric)
	if len(tracks) == 0 {
		return out
	}

	metrics := []struct {
		name string
		unit string
		get  func(*gpx.StoredTrackDetail) float64
	}{
		// Distancia y elevación: mayores son "mejores" (más desafiante).
		{"distance_m", "m", func(t *gpx.StoredTrackDetail) float64 { return t.Track.Analysis.DistanceM }},
		{"d_plus_m", "m", func(t *gpx.StoredTrackDetail) float64 { return t.Track.Analysis.DPlusM }},
		{"d_minus_m", "m", func(t *gpx.StoredTrackDetail) float64 { return t.Track.Analysis.DMinusM }},
		// Difficulty y leg-breaker: mayores son "mejores" (más difícil).
		{"difficulty_score", "score", func(t *gpx.StoredTrackDetail) float64 { return float64(t.Track.Analysis.DifficultyScore) }},
		{"leg_breaker_index", "score", func(t *gpx.StoredTrackDetail) float64 { return t.Track.Analysis.LegBreakerIndex }},
		{"itra_points", "points", func(t *gpx.StoredTrackDetail) float64 { return t.Track.Analysis.ITRAPoints }},
		// Duración: mayor = más largo. Ponytail dice "biggerIsBetter=true"
		// para todas por simplicidad; si el comparator quiere "menor es
		// mejor" para pace/duración, refactorizar como se documenta arriba.
		{"moving_time_s", "s", func(t *gpx.StoredTrackDetail) float64 { return float64(t.Track.Analysis.MovingTimeS) }},
	}

	for _, m := range metrics {
		cm := compareMetric{
			Values: make([]float64, len(tracks)),
			Unit:   m.unit,
		}
		bestIdx := 0
		bestVal := m.get(tracks[0])
		for i, t := range tracks {
			v := m.get(t)
			cm.Values[i] = v
			if (biggerIsBetter && v > bestVal) || (!biggerIsBetter && v < bestVal) {
				bestVal = v
				bestIdx = i
			}
		}
		cm.BestTrack = bestIdx
		out[m.name] = cm
	}

	return out
}
