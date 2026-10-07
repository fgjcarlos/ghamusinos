// Package metrics — capa de servicio que agrega TSS diario a partir de
// actividades crudas (fase 1.4, issue #16 PR2).
//
// ComputeDailyLoad no es una función pura (necesita SQL): el dominio se
// mantiene puro en metrics.TSSCycling/TSSRunning/PR1; aquí solo orquestamos
// filas → TSS → buckets por día UTC, aplicando FillMissingDays del paquete
// metrics. Los tests usan mocks para evitar un Postgres en `go test ./...`.
package metrics

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
)

// UserPhysiology recoge los parámetros fisiológicos del usuario que
// ComputeDailyLoad necesita. Si FTP=0, no se calcula TSS cycling
// (defensivo, mismo patrón que metrics.TSSCycling). Si
// RunningThresholdSecPerKm=0, no se calcula TSS running (idem).
// HR max / lthr se exponen para uso futuro (zonas FC).
type UserPhysiology struct {
	UserID                   pgtype.UUID
	FTP                      int
	HRMax                    int
	LTHR                     int
	RunningThresholdSecPerKm int
	Timezone                 string
}

// ActivitySummary contiene las columnas mínimas de `activities` necesarias
// para calcular TSS diario. Coincide con la proyección de activities_ext.sql
// (ListActivitiesInRange) para evitar acoplar ComputeDailyLoad al paquete
// sqlc directamente.
type ActivitySummary struct {
	ID             pgtype.UUID
	StartedAt      time.Time
	SportType      string
	ElapsedSeconds int32
	MovingSeconds  int32
	DistanceMeters float64
	ElevationGainM float64
	AvgHR          pgtype.Int2
	AvgPower       pgtype.Int2
}

// ActivityRangeQuerier es el contrato de las queries que ComputeDailyLoad
// necesita. Las implementaciones viven en internal/db/sqlc; los tests
// satisfacen la interfaz con un mock.
type ActivityRangeQuerier interface {
	ListActivitiesInRange(ctx context.Context, userID pgtype.UUID, from, to time.Time) ([]ActivitySummary, error)
}

// UserLoader carga la fisiología del usuario (FTP, threshold running, etc.).
type UserLoader interface {
	LoadUserPhysiology(ctx context.Context, userID pgtype.UUID) (UserPhysiology, error)
}

// ComputeDailyLoadDeps agrupa las dependencias del orquestador.
type ComputeDailyLoadDeps struct {
	ActivityRangeQuerier ActivityRangeQuerier
	UserLoader           UserLoader
}

// DailyLoadRow es el resultado agregado de un día: TSS sumado + KPI crudos
// para persistir en training_load_daily.
type DailyLoadRow struct {
	Day            time.Time
	TSS            float64
	ActivityCount  int
	DistanceM      float64
	ElevationGainM float64
}

// ErrNoPhysiology es un sentinel para distinguir "TSS=0 por falta de FTP/
// threshold" (no es error) de errores reales de carga de fisiología.
var ErrNoPhysiology = errors.New("compute_daily_load: user physiology unavailable")

// isRunningSport canonicaliza la detección de actividades de carrera.
// Strava envía tipos como "Run", "TrailRun", "VirtualRun", "Walk"... Solo
// las carreras puras (Run / TrailRun / VirtualRun) reciben TSS running;
// "Walk" e "Hike" cuentan para KPI pero sin TSS (decisión documentada
// en odd/tasks/phase-1-4-performance-dashboard.md).
func isRunningSport(sportType string) bool {
	switch sportType {
	case "Run", "TrailRun", "VirtualRun":
		return true
	default:
		return false
	}
}

// isCyclingSport detecta actividades de ciclismo. Strava: "Ride",
// "MountainBikeRide", "GravelRide", "VirtualRide", "EMountainBikeRide".
func isCyclingSport(sportType string) bool {
	switch sportType {
	case "Ride", "MountainBikeRide", "GravelRide", "VirtualRide", "EMountainBikeRide":
		return true
	default:
		return false
	}
}

// activityTSS calcula el TSS de UNA actividad. Devuelve (tss, contributes)
// donde contributes=false indica "actividad presente en KPI pero sin TSS"
// (otros deportes, o running sin threshold configurado). Errores de cálculo
// se reducen a 0 sin propagar excepción: la causa ya está documentada en
// el caller (defensive sentinel del paquete metrics).
func activityTSS(act ActivitySummary, phys UserPhysiology) float64 {
	if isCyclingSport(act.SportType) {
		np := npForCycling(act)
		return TSSCycling(phys.FTP, int(act.ElapsedSeconds), np)
	}
	if isRunningSport(act.SportType) {
		actualPace := paceSecPerKm(act)
		if actualPace <= 0 {
			return 0
		}
		return TSSRunning(phys.RunningThresholdSecPerKm, int(act.MovingSeconds), actualPace)
	}
	return 0
}

// npForCycling calcula Normalized Power aproximado. Si tenemos avg_power
// lo usamos; si no, derivamos NP ≈ avg_speed * coeff de modo que IF=1
// (sin streams ni power, no se puede calcular TSS cycling estricto).
// Para mantener consistencia con el sentinel del paquete metrics
// (FTP=0 → TSS=0), devolvemos 0 cuando no hay avg_power y el caller
// omitirá la actividad del TSS diario.
func npForCycling(act ActivitySummary) int {
	if act.AvgPower.Valid {
		return int(act.AvgPower.Int16)
	}
	return 0
}

// paceSecPerKm deriva el pace real en segundos por kilómetro. Necesita
// distancia + tiempo en movimiento. Sin distancia no se calcula TSS
// running (decisión PR2: sin pace no hay TSS running fiable).
func paceSecPerKm(act ActivitySummary) int {
	if act.DistanceMeters <= 0 || act.MovingSeconds <= 0 {
		return 0
	}
	distanceKm := act.DistanceMeters / 1000.0
	if distanceKm <= 0 {
		return 0
	}
	pace := float64(act.MovingSeconds) / distanceKm
	return int(pace + 0.5)
}

// ComputeDailyLoad agrega las actividades en el rango (user_id, from, to)
// y devuelve un DailyLoadRow por día UTC. La salida puede contener gaps si
// no todas las fechas tienen actividades; el caller debe aplicar
// metrics.FillMissingDays si quiere una serie continua para CTL/ATL.
//
// computeDailyLoad no llama a FillMissingDays: la decisión de si la serie
// final debe ser continua la tiene el job de recálculo, no ComputeDailyLoad
// (un test que solo carga 3 actividades distribuidas en 2 días espera
// exactamente 2 filas).
func ComputeDailyLoad(ctx context.Context, deps ComputeDailyLoadDeps, userID pgtype.UUID, from, to time.Time) ([]DailyLoadRow, error) {
	if deps.ActivityRangeQuerier == nil {
		return nil, errors.New("compute_daily_load: ActivityRangeQuerier is nil")
	}
	if deps.UserLoader == nil {
		return nil, errors.New("compute_daily_load: UserLoader is nil")
	}
	if !userID.Valid {
		return nil, errors.New("compute_daily_load: userID is invalid")
	}
	if to.Before(from) {
		return nil, errors.New("compute_daily_load: to before from")
	}

	phys, err := deps.UserLoader.LoadUserPhysiology(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("load user physiology: %w", err)
	}

	activities, err := deps.ActivityRangeQuerier.ListActivitiesInRange(ctx, userID, from, to)
	if err != nil {
		return nil, fmt.Errorf("list activities in range: %w", err)
	}

	buckets := make(map[time.Time]*DailyLoadRow, 8)
	for _, act := range activities {
		day := utcDay(act.StartedAt)
		row, exists := buckets[day]
		if !exists {
			row = &DailyLoadRow{Day: day}
			buckets[day] = row
		}
		row.TSS += activityTSS(act, phys)
		row.ActivityCount++
		row.DistanceM += act.DistanceMeters
		row.ElevationGainM += act.ElevationGainM
	}

	rows := make([]DailyLoadRow, 0, len(buckets))
	for _, row := range buckets {
		rows = append(rows, *row)
	}

	// Sort by day ascending for deterministic output. The map iteration order
	// in Go is random; tests assert exact ordered output.
	sortRowsByDay(rows)
	return rows, nil
}

// sortRowsByDay is an insertion sort on Day ASC. N es pequeño (nº días
// con actividad del usuario en el rango); no justifica un sort.Slice.
func sortRowsByDay(rows []DailyLoadRow) {
	for i := 1; i < len(rows); i++ {
		j := i
		for j > 0 && rows[j-1].Day.After(rows[j].Day) {
			rows[j-1], rows[j] = rows[j], rows[j-1]
			j--
		}
	}
}

// _ keeps strconv imported for future numeric conversions (e.g. distance
// unit normalization if needed). Removing strconv would currently leave the
// import unused in some Go versions.
var _ = strconv.Itoa
