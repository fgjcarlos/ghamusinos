// Package handlers — dashboard endpoints for Fase 1.4 (issue #16, PR3).
//
// Four read/write endpoints under /api/v1/dashboard/* plus a fourth
// companion handler for POST /dashboard/recalc (kept in dashboard_recalc.go
// to avoid a single file > 350 LoC):
//
//   - GET  /summary      weekly KPIs + 7d/30d trends (DA-001).
//   - GET  /load         CTL/ATL/TSB daily series (DA-002).
//   - GET  /hr-zones     minutes per HR zone with degraded fallback (DA-003).
//   - POST /recalc       enqueue full-history recalc job (DA-004).
//
// Each factory follows the existing pattern in this package:
// takes a sqlc.Querier (and one or two auxiliary dependencies),
// returns http.Handler, runs auth.AuthUser gating, and writes JSON
// or RFC 9457 problem details via WriteProblem.
package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5/middleware"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/fgjcarlos/ghamusinos/internal/auth"
	"github.com/fgjcarlos/ghamusinos/internal/db/sqlc"
	"github.com/fgjcarlos/ghamusinos/internal/metrics"
)

// pgUUIDFromUserID converts an auth user ID (string UUID) into pgtype.UUID.
// Mirrors the helper in activities.go but kept local so this file compiles
// standalone and tests can import only what they need.
func pgUUIDFromUserID(userID string) (pgtype.UUID, error) {
	parsed := pgtype.UUID{}
	err := parsed.Scan(userID)
	return parsed, err
}

// DashboardRangeQuery is the minimal sqlc subset needed by /load and
// /summary. Both endpoints call ListTrainingLoadRange with different
// windows, so this interface lets the handler depend only on what it
// actually uses (testable without a real DB pool).
type DashboardRangeQuery interface {
	ListTrainingLoadRange(ctx context.Context, arg sqlc.ListTrainingLoadRangeParams) ([]sqlc.TrainingLoadDaily, error)
}

// GetDashboardSummary returns weekly KPIs and 7d/30d trends.
//
// Shape: { weekly_volume_m, weekly_elevation_m, weekly_activities_count,
// trend_7d_pct, trend_30d_pct }. Trends are computed against the
// previous window of equal length; when the previous window has zero
// rows the trend is `null` (avoids NaN / divide-by-zero).
//
// 401 if no auth user; 500 on internal error.
func GetDashboardSummary(q DashboardRangeQuery) http.Handler {
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

		now := time.Now().UTC()
		// ISO-week starts Monday 00:00 UTC. Go's time.Date with weekday
		// math handles this; we let `weekday` round-trip through Time.
		weekStart := isoWeekStart(now)
		day7Start := now.AddDate(0, 0, -7)
		day30Start := now.AddDate(0, 0, -30)

		// Activities count needs its own count path (training_load_daily
		// stores activity_count per day; summing it gives the same total
		// without a separate activities query).
		weeklyRows, err := q.ListTrainingLoadRange(r.Context(), sqlc.ListTrainingLoadRangeParams{
			UserID: userUUID,
			Day:    pgtype.Date{Time: weekStart, Valid: true},
			Day_2:  pgtype.Date{Time: now, Valid: true},
		})
		if err != nil {
			requestID := middleware.GetReqID(r.Context())
			WriteProblem(w, NewInternalError("failed to load weekly training load", requestID))
			return
		}

		// KPIs from current week. DistanceM / ElevationGainM come back as
		// plain float64 from sqlc (column type DOUBLE PRECISION); the
		// nullable case is signalled by NaN on read in pgtype, but the
		// column is NOT NULL with DEFAULT 0, so we can sum directly.
		var weeklyVolume, weeklyElevation float64
		var weeklyCount int
		for _, row := range weeklyRows {
			weeklyVolume += row.DistanceM
			weeklyElevation += row.ElevationGainM
			weeklyCount += int(row.ActivityCount)
		}

		// Trends: present window vs immediately preceding equal-length window.
		trend7d := computeTSSRatio(r.Context(), q, userUUID, day7Start, now)
		trend30d := computeTSSRatio(r.Context(), q, userUUID, day30Start, now)

		resp := map[string]interface{}{
			"weekly_volume_m":         round2(weeklyVolume),
			"weekly_elevation_m":      round2(weeklyElevation),
			"weekly_activities_count": weeklyCount,
			"trend_7d_pct":            trend7d,
			"trend_30d_pct":           trend30d,
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(resp)
	})
}

// GetTrainingLoad returns the filled daily CTL/ATL/TSB series.
//
// Reads `from`/`to` query params (ISO dates). Defaults: to=today,
// from=to-30d. Returns { from, to, series: [{day, ctl, atl, tsb}, ...] }
// with exactly one entry per calendar day in [from, to] inclusive
// (delegating to metrics.FillMissingDays).
//
// 401 if no auth user; 400 on invalid date params; 500 on internal.
func GetTrainingLoad(q DashboardRangeQuery) http.Handler {
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

		rows, err := q.ListTrainingLoadRange(r.Context(), sqlc.ListTrainingLoadRangeParams{
			UserID: userUUID,
			Day:    pgtype.Date{Time: from, Valid: true},
			Day_2:  pgtype.Date{Time: to, Valid: true},
		})
		if err != nil {
			requestID := middleware.GetReqID(r.Context())
			WriteProblem(w, NewInternalError("failed to load training load", requestID))
			return
		}

		// Convert raw rows into metrics.DailyLoad, fill gaps with zero,
		// then walk the filled series accumulating CTL/ATL/TSB.
		raw := make([]metrics.DailyLoad, 0, len(rows))
		for _, row := range rows {
			raw = append(raw, metrics.DailyLoad{Day: row.Day.Time, TSS: row.Tss})
		}
		filled := metrics.FillMissingDays(from, to, raw)

		// Cumulative EMA across the window. The series starts at the
		// window's first day; pre-window data (before `from`) is irrelevant
		// here because the user only sees what they asked for. We still
		// document this in the spec scenario for /load.
		ctlFinal := metrics.CTL(filled)
		atlFinal := metrics.ATL(filled)
		tsbFinal := metrics.TSB(ctlFinal, atlFinal)

		type seriesEntry struct {
			Day string  `json:"day"`
			CTL float64 `json:"ctl"`
			ATL float64 `json:"atl"`
			TSB float64 `json:"tsb"`
			TSS float64 `json:"tss"`
		}
		// The series exposes per-day CTL/ATL/TSB using a *running* EMA
		// across the window so the chart shows the curve, not just the
		// final value. Cheap (O(n)) and matches what most dashboards do.
		series := make([]seriesEntry, 0, len(filled))
		var ctlRun, atlRun float64
		alphaCTL := 1 - math.Exp(-1/float64(metrics.TauCTL))
		alphaATL := 1 - math.Exp(-1/float64(metrics.TauATL))
		for i, d := range filled {
			if i == 0 {
				ctlRun = d.TSS
				atlRun = d.TSS
			} else {
				ctlRun += (d.TSS - ctlRun) * alphaCTL
				atlRun += (d.TSS - atlRun) * alphaATL
			}
			series = append(series, seriesEntry{
				Day: d.Day.Format("2006-01-02"),
				CTL: round2(ctlRun),
				ATL: round2(atlRun),
				TSB: round2(ctlRun - atlRun),
				TSS: round2(d.TSS),
			})
		}

		resp := map[string]interface{}{
			"from":      from.Format("2006-01-02"),
			"to":        to.Format("2006-01-02"),
			"final_ctl": round2(ctlFinal),
			"final_atl": round2(atlFinal),
			"final_tsb": round2(tsbFinal),
			"series":    series,
		}

		w.Header().Set("Cache-Control", "private, max-age=60")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(resp)
	})
}

// HRZonesQuerier is the sqlc subset required by GetHRZones. Splitting
// it from the broader Querier interface keeps tests minimal.
type HRZonesQuerier interface {
	SumHRZonesInRange(ctx context.Context, arg sqlc.SumHRZonesInRangeParams) (sqlc.SumHRZonesInRangeRow, error)
	SumActivitiesDurationInRange(ctx context.Context, arg sqlc.SumActivitiesDurationInRangeParams) (sqlc.SumActivitiesDurationInRangeRow, error)
}

// GetHRZones returns minutes per HR zone for the window.
//
// When `hr_zones` has rows for activities in the range, the handler
// returns the exact sums with degraded=false. When `hr_zones` is empty
// for the range (e.g. the user never ingested HR streams), it falls
// back to `activities.avg_hr × elapsed_seconds / 60` partitioned across
// the 5 zones (uniform split per DA-003 spec) and returns degraded=true
// so the UI can label the chart "aproximación (sin streams HR)".
//
// 401 if no auth user; 400 on invalid date params; 500 on internal.
func GetHRZones(q HRZonesQuerier) http.Handler {
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

		hazones, err := q.SumHRZonesInRange(r.Context(), sqlc.SumHRZonesInRangeParams{
			UserID:      userUUID,
			StartedAt:   pgtype.Timestamptz{Time: from, Valid: true},
			StartedAt_2: pgtype.Timestamptz{Time: to, Valid: true},
		})
		if err != nil {
			requestID := middleware.GetReqID(r.Context())
			WriteProblem(w, NewInternalError("failed to load hr zones", requestID))
			return
		}

		var degraded bool
		var minutes [5]float64
		if hazones.ActivitiesWithZones > 0 {
			// Exact: sum zN_seconds → minutes (rounded to 2 dp at output).
			minutes = [5]float64{
				float64(hazones.Z1Seconds) / 60.0,
				float64(hazones.Z2Seconds) / 60.0,
				float64(hazones.Z3Seconds) / 60.0,
				float64(hazones.Z4Seconds) / 60.0,
				float64(hazones.Z5Seconds) / 60.0,
			}
		} else {
			// Fallback: get totals with avg_hr; partition by avg_hr × elapsed / 60
			// uniformly across the 5 zones. The spec requires `degraded:true`
			// when streams are absent; we honour that even though we used
			// hr_zones (decisión de usuario: hr_zones es la fuente primaria).
			totals, qErr := q.SumActivitiesDurationInRange(r.Context(), sqlc.SumActivitiesDurationInRangeParams{
				UserID:      userUUID,
				StartedAt:   pgtype.Timestamptz{Time: from, Valid: true},
				StartedAt_2: pgtype.Timestamptz{Time: to, Valid: true},
			})
			if qErr != nil {
				requestID := middleware.GetReqID(r.Context())
				WriteProblem(w, NewInternalError("failed to load activity totals", requestID))
				return
			}
			// Total minutes with avg_hr available: even split across z1..z5.
			totalMinutes := float64(totals.ElapsedSecondsWithAvgHr) / 60.0
			perZone := totalMinutes / 5.0
			for i := range minutes {
				minutes[i] = perZone
			}
			degraded = true
		}

		zones := make([]map[string]interface{}, 0, 5)
		zoneNames := []string{"z1", "z2", "z3", "z4", "z5"}
		for i, name := range zoneNames {
			zones = append(zones, map[string]interface{}{
				"zone":    name,
				"minutes": round2(minutes[i]),
			})
		}

		resp := map[string]interface{}{
			"from":     from.Format("2006-01-02"),
			"to":       to.Format("2006-01-02"),
			"degraded": degraded,
			"zones":    zones,
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(resp)
	})
}

// ----- Helpers -----

// parseDashboardRange reads `from`/`to` query params as ISO dates
// (YYYY-MM-DD). Defaults: to=today UTC, from=to-30d. Returns 400-style
// error text on invalid format or inverted range.
func parseDashboardRange(r *http.Request) (time.Time, time.Time, error) {
	now := time.Now().UTC()
	to := now
	from := to.AddDate(0, 0, -30)

	q := r.URL.Query()
	if v := q.Get("to"); v != "" {
		parsed, err := time.ParseInLocation("2006-01-02", v, time.UTC)
		if err != nil {
			return time.Time{}, time.Time{}, fmt.Errorf("invalid 'to' date: expected YYYY-MM-DD")
		}
		to = parsed
	}
	if v := q.Get("from"); v != "" {
		parsed, err := time.ParseInLocation("2006-01-02", v, time.UTC)
		if err != nil {
			return time.Time{}, time.Time{}, fmt.Errorf("invalid 'from' date: expected YYYY-MM-DD")
		}
		from = parsed
	}
	if from.After(to) {
		return time.Time{}, time.Time{}, errors.New("'from' must be on or before 'to'")
	}
	// Normalize to start of UTC day so callers don't need to.
	from = time.Date(from.Year(), from.Month(), from.Day(), 0, 0, 0, 0, time.UTC)
	to = time.Date(to.Year(), to.Month(), to.Day(), 0, 0, 0, 0, time.UTC)
	return from, to, nil
}

// isoWeekStart returns the Monday 00:00 UTC of the ISO week containing t.
func isoWeekStart(t time.Time) time.Time {
	t = t.UTC()
	dayOffset := int(t.Weekday())
	if dayOffset == 0 { // Sunday
		dayOffset = 7
	}
	monday := t.AddDate(0, 0, -(dayOffset - 1))
	return time.Date(monday.Year(), monday.Month(), monday.Day(), 0, 0, 0, 0, time.UTC)
}

// computeTSSRatio returns the percentage change of average daily TSS
// between two equal-length windows. Returns nil when the previous
// window has zero total TSS (avoids NaN / divide-by-zero).
//
// The current implementation reads `currentRows` for the present window
// and re-queries the same Querier for the previous window of equal
// length immediately preceding `currentStart`. When the previous window
// has no rows, the trend is `null` (per DA-001 scenario).
func computeTSSRatio(ctx context.Context, q DashboardRangeQuery, userID pgtype.UUID, currentStart, currentEnd time.Time) interface{} {
	prevEnd := currentStart.AddDate(0, 0, -1)
	windowLen := int(currentEnd.Sub(currentStart).Hours() / 24)
	if windowLen <= 0 {
		return nil
	}
	prevStart := prevEnd.AddDate(0, 0, -(windowLen - 1))

	currRows, err := q.ListTrainingLoadRange(ctx, sqlc.ListTrainingLoadRangeParams{
		UserID: userID,
		Day:    pgtype.Date{Time: currentStart, Valid: true},
		Day_2:  pgtype.Date{Time: currentEnd, Valid: true},
	})
	if err != nil {
		return nil
	}
	prevRows, err := q.ListTrainingLoadRange(ctx, sqlc.ListTrainingLoadRangeParams{
		UserID: userID,
		Day:    pgtype.Date{Time: prevStart, Valid: true},
		Day_2:  pgtype.Date{Time: prevEnd, Valid: true},
	})
	if err != nil {
		return nil
	}

	var currSum, prevSum float64
	for _, r := range currRows {
		currSum += r.Tss
	}
	for _, r := range prevRows {
		prevSum += r.Tss
	}
	if prevSum == 0 {
		return nil
	}
	pct := (currSum - prevSum) / prevSum * 100.0
	return round2(pct)
}

// round2 rounds a float to 2 decimal places; used everywhere the
// handler exposes numeric values.
func round2(f float64) float64 {
	return math.Round(f*100) / 100
}
