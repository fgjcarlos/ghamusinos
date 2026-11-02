package handlers

import (
	"context"

	"github.com/fgjcarlos/ghamusinos/internal/db/sqlc"
)

// Stubs de la interfaz sqlc.Querier para las consultas de ai_analysis.
// La tabla ai_analysis (migración 00015) añadió métodos a la interfaz
// compartida, así que cada mock que la implementa completa necesita estos
// stubs. Los handlers de IA (fase 1.5) usarán mocks propios con
// comportamiento; aquí solo se cumple el contrato de interfaz.

// CreateAIAnalysis stub para activitiesMockQuerier.
func (m *activitiesMockQuerier) CreateAIAnalysis(ctx context.Context, arg sqlc.CreateAIAnalysisParams) (sqlc.AiAnalysis, error) {
	return sqlc.AiAnalysis{}, nil
}

// GetAIAnalysis stub para activitiesMockQuerier.
func (m *activitiesMockQuerier) GetAIAnalysis(ctx context.Context, arg sqlc.GetAIAnalysisParams) (sqlc.AiAnalysis, error) {
	return sqlc.AiAnalysis{}, nil
}

// ListAIAnalyses stub para activitiesMockQuerier.
func (m *activitiesMockQuerier) ListAIAnalyses(ctx context.Context, arg sqlc.ListAIAnalysesParams) ([]sqlc.AiAnalysis, error) {
	return nil, nil
}

// CompleteAIAnalysis stub para activitiesMockQuerier.
func (m *activitiesMockQuerier) CompleteAIAnalysis(ctx context.Context, arg sqlc.CompleteAIAnalysisParams) (sqlc.AiAnalysis, error) {
	return sqlc.AiAnalysis{}, nil
}

// FailAIAnalysis stub para activitiesMockQuerier.
func (m *activitiesMockQuerier) FailAIAnalysis(ctx context.Context, arg sqlc.FailAIAnalysisParams) (sqlc.AiAnalysis, error) {
	return sqlc.AiAnalysis{}, nil
}

// CreateAIAnalysis stub para dashboardMockQuerier.
func (m *dashboardMockQuerier) CreateAIAnalysis(ctx context.Context, arg sqlc.CreateAIAnalysisParams) (sqlc.AiAnalysis, error) {
	return sqlc.AiAnalysis{}, nil
}

// GetAIAnalysis stub para dashboardMockQuerier.
func (m *dashboardMockQuerier) GetAIAnalysis(ctx context.Context, arg sqlc.GetAIAnalysisParams) (sqlc.AiAnalysis, error) {
	return sqlc.AiAnalysis{}, nil
}

// ListAIAnalyses stub para dashboardMockQuerier.
func (m *dashboardMockQuerier) ListAIAnalyses(ctx context.Context, arg sqlc.ListAIAnalysesParams) ([]sqlc.AiAnalysis, error) {
	return nil, nil
}

// CompleteAIAnalysis stub para dashboardMockQuerier.
func (m *dashboardMockQuerier) CompleteAIAnalysis(ctx context.Context, arg sqlc.CompleteAIAnalysisParams) (sqlc.AiAnalysis, error) {
	return sqlc.AiAnalysis{}, nil
}

// FailAIAnalysis stub para dashboardMockQuerier.
func (m *dashboardMockQuerier) FailAIAnalysis(ctx context.Context, arg sqlc.FailAIAnalysisParams) (sqlc.AiAnalysis, error) {
	return sqlc.AiAnalysis{}, nil
}

// CreateAIAnalysis stub para preferencesMockQuerier.
func (m *preferencesMockQuerier) CreateAIAnalysis(ctx context.Context, arg sqlc.CreateAIAnalysisParams) (sqlc.AiAnalysis, error) {
	return sqlc.AiAnalysis{}, nil
}

// GetAIAnalysis stub para preferencesMockQuerier.
func (m *preferencesMockQuerier) GetAIAnalysis(ctx context.Context, arg sqlc.GetAIAnalysisParams) (sqlc.AiAnalysis, error) {
	return sqlc.AiAnalysis{}, nil
}

// ListAIAnalyses stub para preferencesMockQuerier.
func (m *preferencesMockQuerier) ListAIAnalyses(ctx context.Context, arg sqlc.ListAIAnalysesParams) ([]sqlc.AiAnalysis, error) {
	return nil, nil
}

// CompleteAIAnalysis stub para preferencesMockQuerier.
func (m *preferencesMockQuerier) CompleteAIAnalysis(ctx context.Context, arg sqlc.CompleteAIAnalysisParams) (sqlc.AiAnalysis, error) {
	return sqlc.AiAnalysis{}, nil
}

// FailAIAnalysis stub para preferencesMockQuerier.
func (m *preferencesMockQuerier) FailAIAnalysis(ctx context.Context, arg sqlc.FailAIAnalysisParams) (sqlc.AiAnalysis, error) {
	return sqlc.AiAnalysis{}, nil
}

// CreateAIAnalysis stub para mockMeQuerier.
func (m *mockMeQuerier) CreateAIAnalysis(ctx context.Context, arg sqlc.CreateAIAnalysisParams) (sqlc.AiAnalysis, error) {
	return sqlc.AiAnalysis{}, nil
}

// GetAIAnalysis stub para mockMeQuerier.
func (m *mockMeQuerier) GetAIAnalysis(ctx context.Context, arg sqlc.GetAIAnalysisParams) (sqlc.AiAnalysis, error) {
	return sqlc.AiAnalysis{}, nil
}

// ListAIAnalyses stub para mockMeQuerier.
func (m *mockMeQuerier) ListAIAnalyses(ctx context.Context, arg sqlc.ListAIAnalysesParams) ([]sqlc.AiAnalysis, error) {
	return nil, nil
}

// CompleteAIAnalysis stub para mockMeQuerier.
func (m *mockMeQuerier) CompleteAIAnalysis(ctx context.Context, arg sqlc.CompleteAIAnalysisParams) (sqlc.AiAnalysis, error) {
	return sqlc.AiAnalysis{}, nil
}

// FailAIAnalysis stub para mockMeQuerier.
func (m *mockMeQuerier) FailAIAnalysis(ctx context.Context, arg sqlc.FailAIAnalysisParams) (sqlc.AiAnalysis, error) {
	return sqlc.AiAnalysis{}, nil
}

// GetActivityForUser stub para activitiesMockQuerier.
func (m *activitiesMockQuerier) GetActivityForUser(ctx context.Context, arg sqlc.GetActivityForUserParams) (sqlc.GetActivityForUserRow, error) {
	return sqlc.GetActivityForUserRow{}, nil
}

// GetActivityForUser stub para dashboardMockQuerier.
func (m *dashboardMockQuerier) GetActivityForUser(ctx context.Context, arg sqlc.GetActivityForUserParams) (sqlc.GetActivityForUserRow, error) {
	return sqlc.GetActivityForUserRow{}, nil
}

// GetActivityForUser stub para preferencesMockQuerier.
func (m *preferencesMockQuerier) GetActivityForUser(ctx context.Context, arg sqlc.GetActivityForUserParams) (sqlc.GetActivityForUserRow, error) {
	return sqlc.GetActivityForUserRow{}, nil
}

// GetActivityForUser stub para mockMeQuerier.
func (m *mockMeQuerier) GetActivityForUser(ctx context.Context, arg sqlc.GetActivityForUserParams) (sqlc.GetActivityForUserRow, error) {
	return sqlc.GetActivityForUserRow{}, nil
}
