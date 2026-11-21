package auth

import (
	"context"

	"github.com/fgjcarlos/ghamusinos/internal/db/sqlc"
)

// Stubs de la interfaz sqlc.Querier para las consultas de planned_sessions.
// La tabla planned_sessions (migración 00016) añadió cinco métodos a la
// interfaz compartida, así que este mock que la implementa completa
// necesita estos stubs para seguir satisfaciéndola.
//
// Los stubs devuelven el valor cero: los tests de este paquete no usan
// las sesiones planificadas (la API llega en la issue #299).

// CreatePlannedSession stub para mockQuerier.
func (m *mockQuerier) CreatePlannedSession(ctx context.Context, arg sqlc.CreatePlannedSessionParams) (sqlc.PlannedSession, error) {
	return sqlc.PlannedSession{}, nil
}

// GetPlannedSession stub para mockQuerier.
func (m *mockQuerier) GetPlannedSession(ctx context.Context, arg sqlc.GetPlannedSessionParams) (sqlc.PlannedSession, error) {
	return sqlc.PlannedSession{}, nil
}

// ListPlannedSessionsByRange stub para mockQuerier.
func (m *mockQuerier) ListPlannedSessionsByRange(ctx context.Context, arg sqlc.ListPlannedSessionsByRangeParams) ([]sqlc.ListPlannedSessionsByRangeRow, error) {
	return nil, nil
}

// UpdatePlannedSession stub para mockQuerier.
func (m *mockQuerier) UpdatePlannedSession(ctx context.Context, arg sqlc.UpdatePlannedSessionParams) (sqlc.PlannedSession, error) {
	return sqlc.PlannedSession{}, nil
}

// DeletePlannedSession stub para mockQuerier.
func (m *mockQuerier) DeletePlannedSession(ctx context.Context, arg sqlc.DeletePlannedSessionParams) (int64, error) {
	return 0, nil
}

// LinkPlannedSessionActivity stub para mockQuerier (migración 00017, issue #311).
func (m *mockQuerier) LinkPlannedSessionActivity(ctx context.Context, arg sqlc.LinkPlannedSessionActivityParams) (sqlc.PlannedSession, error) {
	return sqlc.PlannedSession{}, nil
}

// UnlinkPlannedSessionActivity stub para mockQuerier (migración 00017, issue #311).
func (m *mockQuerier) UnlinkPlannedSessionActivity(ctx context.Context, arg sqlc.UnlinkPlannedSessionActivityParams) (sqlc.PlannedSession, error) {
	return sqlc.PlannedSession{}, nil
}

// ListCalendarActivities stub para mockQuerier (bandeja del calendario,
// migración de queries de #313, T3).
func (m *mockQuerier) ListCalendarActivities(ctx context.Context, arg sqlc.ListCalendarActivitiesParams) ([]sqlc.ListCalendarActivitiesRow, error) {
	return nil, nil
}
