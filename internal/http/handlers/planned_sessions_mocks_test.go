package handlers

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

// CreatePlannedSession stub para activitiesMockQuerier.
func (m *activitiesMockQuerier) CreatePlannedSession(ctx context.Context, arg sqlc.CreatePlannedSessionParams) (sqlc.PlannedSession, error) {
	return sqlc.PlannedSession{}, nil
}

// GetPlannedSession stub para activitiesMockQuerier.
func (m *activitiesMockQuerier) GetPlannedSession(ctx context.Context, arg sqlc.GetPlannedSessionParams) (sqlc.PlannedSession, error) {
	return sqlc.PlannedSession{}, nil
}

// ListPlannedSessionsByRange stub para activitiesMockQuerier.
func (m *activitiesMockQuerier) ListPlannedSessionsByRange(ctx context.Context, arg sqlc.ListPlannedSessionsByRangeParams) ([]sqlc.ListPlannedSessionsByRangeRow, error) {
	return nil, nil
}

// UpdatePlannedSession stub para activitiesMockQuerier.
func (m *activitiesMockQuerier) UpdatePlannedSession(ctx context.Context, arg sqlc.UpdatePlannedSessionParams) (sqlc.PlannedSession, error) {
	return sqlc.PlannedSession{}, nil
}

// DeletePlannedSession stub para activitiesMockQuerier.
func (m *activitiesMockQuerier) DeletePlannedSession(ctx context.Context, arg sqlc.DeletePlannedSessionParams) (int64, error) {
	return 0, nil
}

// CreatePlannedSession stub para mockMeQuerier.
func (m *mockMeQuerier) CreatePlannedSession(ctx context.Context, arg sqlc.CreatePlannedSessionParams) (sqlc.PlannedSession, error) {
	return sqlc.PlannedSession{}, nil
}

// GetPlannedSession stub para mockMeQuerier.
func (m *mockMeQuerier) GetPlannedSession(ctx context.Context, arg sqlc.GetPlannedSessionParams) (sqlc.PlannedSession, error) {
	return sqlc.PlannedSession{}, nil
}

// ListPlannedSessionsByRange stub para mockMeQuerier.
func (m *mockMeQuerier) ListPlannedSessionsByRange(ctx context.Context, arg sqlc.ListPlannedSessionsByRangeParams) ([]sqlc.ListPlannedSessionsByRangeRow, error) {
	return nil, nil
}

// UpdatePlannedSession stub para mockMeQuerier.
func (m *mockMeQuerier) UpdatePlannedSession(ctx context.Context, arg sqlc.UpdatePlannedSessionParams) (sqlc.PlannedSession, error) {
	return sqlc.PlannedSession{}, nil
}

// DeletePlannedSession stub para mockMeQuerier.
func (m *mockMeQuerier) DeletePlannedSession(ctx context.Context, arg sqlc.DeletePlannedSessionParams) (int64, error) {
	return 0, nil
}

// CreatePlannedSession stub para preferencesMockQuerier.
func (m *preferencesMockQuerier) CreatePlannedSession(ctx context.Context, arg sqlc.CreatePlannedSessionParams) (sqlc.PlannedSession, error) {
	return sqlc.PlannedSession{}, nil
}

// GetPlannedSession stub para preferencesMockQuerier.
func (m *preferencesMockQuerier) GetPlannedSession(ctx context.Context, arg sqlc.GetPlannedSessionParams) (sqlc.PlannedSession, error) {
	return sqlc.PlannedSession{}, nil
}

// ListPlannedSessionsByRange stub para preferencesMockQuerier.
func (m *preferencesMockQuerier) ListPlannedSessionsByRange(ctx context.Context, arg sqlc.ListPlannedSessionsByRangeParams) ([]sqlc.ListPlannedSessionsByRangeRow, error) {
	return nil, nil
}

// UpdatePlannedSession stub para preferencesMockQuerier.
func (m *preferencesMockQuerier) UpdatePlannedSession(ctx context.Context, arg sqlc.UpdatePlannedSessionParams) (sqlc.PlannedSession, error) {
	return sqlc.PlannedSession{}, nil
}

// DeletePlannedSession stub para preferencesMockQuerier.
func (m *preferencesMockQuerier) DeletePlannedSession(ctx context.Context, arg sqlc.DeletePlannedSessionParams) (int64, error) {
	return 0, nil
}
