package jobs

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/fgjcarlos/ghamusinos/internal/db/sqlc"
	"github.com/fgjcarlos/ghamusinos/internal/metrics"
)

// sqlcActivityRangeAdapter implementa metrics.ActivityRangeQuerier leyendo
// de la query generada por sqlc (activities_ext.sql: ListActivitiesInRange).
type sqlcActivityRangeAdapter struct {
	queries *sqlc.Queries
}

// NewSQLCActivityRangeAdapter envuelve un *sqlc.Queries para exponer las
// filas de actividad como metrics.ActivitySummary. La traducción se hace
// en runtime; las proyecciones estrechas evitan cargar JSONB ni campos
// que ComputeDailyLoad no usa.
func NewSQLCActivityRangeAdapter(queries *sqlc.Queries) *sqlcActivityRangeAdapter {
	return &sqlcActivityRangeAdapter{queries: queries}
}

func (a *sqlcActivityRangeAdapter) ListActivitiesInRange(ctx context.Context, userID pgtype.UUID, from, to time.Time) ([]metrics.ActivitySummary, error) {
	rows, err := a.queries.ListActivitiesInRange(ctx, sqlc.ListActivitiesInRangeParams{
		UserID:      userID,
		StartedAt:   pgtype.Timestamptz{Time: from, Valid: true},
		StartedAt_2: pgtype.Timestamptz{Time: to, Valid: true},
	})
	if err != nil {
		return nil, fmt.Errorf("sqlc: ListActivitiesInRange: %w", err)
	}
	out := make([]metrics.ActivitySummary, 0, len(rows))
	for _, r := range rows {
		out = append(out, metrics.ActivitySummary{
			ID:             r.ID,
			StartedAt:      r.StartedAt.Time,
			SportType:      r.SportType,
			ElapsedSeconds: r.ElapsedSeconds,
			MovingSeconds:  r.MovingSeconds,
			DistanceMeters: numericOrZero(r.DistanceMeters),
			ElevationGainM: numericOrZero(r.ElevationGainM),
			AvgHR:          r.AvgHr,
			AvgPower:       r.AvgPower,
		})
	}
	return out, nil
}

func (a *sqlcActivityRangeAdapter) FirstActivityForUser(ctx context.Context, userID pgtype.UUID) (pgtype.Timestamptz, error) {
	return a.queries.FirstActivityForUser(ctx, userID)
}

// numericOrZero convierte un pgtype.Numeric (nullable) a float64; NULL → 0.
func numericOrZero(n pgtype.Numeric) float64 {
	if !n.Valid {
		return 0
	}
	f, err := n.Float64Value()
	if err != nil || !f.Valid {
		return 0
	}
	return f.Float64
}

// sqlcUserPhysiologyAdapter implementa metrics.UserLoader leyendo de la
// fila users. Las columnas `ftp`, `running_threshold_sec_per_km`, etc. se
// traducen de pgtype.Int2/SMALLINT a int (sentinel 0 cuando NULL).
type sqlcUserPhysiologyAdapter struct {
	queries *sqlc.Queries
}

// NewSQLCUserPhysiologyAdapter wraps a *sqlc.Queries to expose user
// physiology for ComputeDailyLoad.
func NewSQLCUserPhysiologyAdapter(queries *sqlc.Queries) *sqlcUserPhysiologyAdapter {
	return &sqlcUserPhysiologyAdapter{queries: queries}
}

// LoadUserPhysiology carga la fisiología del usuario. Si el usuario no
// existe, devuelve error.
func (a *sqlcUserPhysiologyAdapter) LoadUserPhysiology(ctx context.Context, userID pgtype.UUID) (metrics.UserPhysiology, error) {
	user, err := a.queries.GetUserByID(ctx, userID)
	if err != nil {
		return metrics.UserPhysiology{}, fmt.Errorf("sqlc: GetUserByID: %w", err)
	}
	return metrics.UserPhysiology{
		UserID:                   userID,
		FTP:                      int16OrZero(user.Ftp),
		HRMax:                    int16OrZero(user.HrMax),
		LTHR:                     int16OrZero(user.Lthr),
		RunningThresholdSecPerKm: int16OrZero(user.RunningThresholdSecPerKm),
		Timezone:                 user.Timezone,
	}, nil
}

func int16OrZero(n pgtype.Int2) int {
	if !n.Valid {
		return 0
	}
	return int(n.Int16)
}
