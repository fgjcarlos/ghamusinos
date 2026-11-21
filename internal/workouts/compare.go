package workouts

import (
	"context"
	"errors"
	"fmt"
	"math"
	"sort"
	"time"

	"github.com/fgjcarlos/ghamusinos/internal/db/sqlc"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

// CompareGroup es la agrupacion de periodos del endpoint de comparacion
// (issue #312): dias o semanas lunes-based.
type CompareGroup string

const (
	// CompareGroupDay agrupa por dia civil en la zona del usuario.
	CompareGroupDay CompareGroup = "day"
	// CompareGroupWeek agrupa por semana ISO (lunes), recortada al rango
	// pedido: period_start es siempre el lunes de esa semana.
	CompareGroupWeek CompareGroup = "week"
)

// CompareTotals son las cargas sumadas de un periodo. `planned` suma los
// objetivos (target_*) de las sesiones no skipped; `actual` suma las
// metricas de las actividades del periodo, vinculadas o no.
type CompareTotals struct {
	DurationSeconds int64   `json:"duration_seconds"`
	DistanceM       float64 `json:"distance_m"`
	ElevationGainM  float64 `json:"elevation_gain_m"`
}

// CompareCompliance es el cumplimiento del periodo: sesiones en cada
// estado frente al total de sesiones (planned = total - resto).
type CompareCompliance struct {
	Total     int `json:"total"`
	Completed int `json:"completed"`
	Skipped   int `json:"skipped"`
	Modified  int `json:"modified"`
}

// ComparePeriod es un bucket de la comparacion. Solo aparecen los
// periodos con al menos una sesion o una actividad: los dias vacios no
// ensucian el chart, y un dia solo-skipped aparece con cumplimiento y
// sin objetivos.
type ComparePeriod struct {
	PeriodStart string            `json:"period_start"`
	Planned     CompareTotals     `json:"planned"`
	Actual      CompareTotals     `json:"actual"`
	Compliance  CompareCompliance `json:"compliance"`
}

// Compare agrega la carga planificada frente a la realizada en la
// ventana civil inclusiva [from, to], agrupada por dia o semana en la
// zona horaria guardada del usuario. Reutiliza las queries existentes
// (ListPlannedSessionsByRange y ListActivitiesInRange): no hay SQL
// nuevo, solo agregacion en Go.
func (s *Store) Compare(ctx context.Context, userID uuid.UUID, from, to time.Time, group CompareGroup) ([]ComparePeriod, error) {
	loc, err := s.userZone(ctx, userID)
	if err != nil {
		return nil, err
	}
	rows, err := s.q.ListPlannedSessionsByRange(ctx, sqlc.ListPlannedSessionsByRangeParams{
		UserID:   toUUID(userID),
		FromDate: toDate(from),
		ToDate:   toDate(to),
		Limit:    math.MaxInt32,
	})
	if err != nil {
		return nil, fmt.Errorf("workouts: compare sessions: %w", err)
	}
	// Pad the UTC search window and filter by local date below. A local
	// midnight can be skipped or repeated; constructing it is ambiguous.
	activities, err := s.q.ListActivitiesInRange(ctx, sqlc.ListActivitiesInRangeParams{
		UserID:      toUUID(userID),
		StartedAt:   pgtype.Timestamptz{Time: from.AddDate(0, 0, -1), Valid: true},
		StartedAt_2: pgtype.Timestamptz{Time: to.AddDate(0, 0, 2), Valid: true},
	})
	if err != nil {
		return nil, fmt.Errorf("workouts: compare activities: %w", err)
	}

	fromKey := from.Format("2006-01-02")
	toKey := to.Format("2006-01-02")
	buckets := make(map[string]*ComparePeriod)
	bucket := func(date time.Time) *ComparePeriod {
		key := date.Format("2006-01-02")
		if group == CompareGroupWeek {
			key = WeekStart(date).Format("2006-01-02")
		}
		b, ok := buckets[key]
		if !ok {
			b = &ComparePeriod{PeriodStart: key}
			buckets[key] = b
		}
		return b
	}

	for _, row := range rows {
		session := fromListRow(row)
		b := bucket(session.ScheduledOn)
		b.Compliance.Total++
		switch session.Status {
		case StatusCompleted:
			b.Compliance.Completed++
		case StatusSkipped:
			b.Compliance.Skipped++
		case StatusModified:
			b.Compliance.Modified++
		}
		if session.Status == StatusSkipped {
			continue // omitida a proposito: sus objetivos no son carga planificada
		}
		if session.TargetDurationSec != nil {
			b.Planned.DurationSeconds += int64(*session.TargetDurationSec)
		}
		if session.TargetDistanceM != nil {
			b.Planned.DistanceM += float64(*session.TargetDistanceM)
		}
		if session.TargetElevationGainM != nil {
			b.Planned.ElevationGainM += float64(*session.TargetElevationGainM)
		}
	}
	for _, a := range activities {
		local := LocalDate(a.StartedAt.Time, loc)
		key := local.Format("2006-01-02")
		if key < fromKey || key > toKey {
			continue
		}
		b := bucket(local)
		b.Actual.DurationSeconds += int64(a.MovingSeconds)
		b.Actual.DistanceM += numericOrZero(a.DistanceMeters)
		b.Actual.ElevationGainM += numericOrZero(a.ElevationGainM)
	}

	keys := make([]string, 0, len(buckets))
	for k := range buckets {
		keys = append(keys, k)
	}
	sort.Strings(keys) // ISO dates: orden lexicografico == orden cronologico
	periods := make([]ComparePeriod, 0, len(keys))
	for _, k := range keys {
		periods = append(periods, *buckets[k])
	}
	return periods, nil
}

// userZone resuelve la zona horaria guardada del usuario con fallback a
// UTC. Una fila de preferencias ausente tambien cae a UTC (no es un
// fallo del endpoint); el resto de errores si se propagan.
func (s *Store) userZone(ctx context.Context, userID uuid.UUID) (*time.Location, error) {
	prefs, err := s.q.GetUserPreferencesByID(ctx, toUUID(userID))
	if errors.Is(err, pgx.ErrNoRows) {
		return time.UTC, nil
	}
	if err != nil {
		return nil, fmt.Errorf("workouts: compare preferences: %w", err)
	}
	return UserLocation(prefs.Timezone), nil
}

// numericOrZero convierte un pgtype.Numeric (nullable) a float64; NULL o
// no representable -> 0. Mismo patron que numericOrZero del worker de
// recalc (internal/jobs).
func numericOrZero(n pgtype.Numeric) float64 {
	if !n.Valid {
		return 0
	}
	f, err := n.Float64Value()
	if err != nil {
		return 0
	}
	return f.Float64
}
