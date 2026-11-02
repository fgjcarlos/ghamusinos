package http

import (
	"context"

	"github.com/fgjcarlos/ghamusinos/internal/db/sqlc"
)

// Stubs de la interfaz sqlc.Querier para las consultas de ai_analysis.
// La tabla ai_analysis (migración 00015) añadió métodos a la interfaz
// compartida, así que este mock que la implementa completa necesita estos
// stubs para seguir satisfaciéndola.

// CreateAIAnalysis stub para mockQuerier.
func (m *mockQuerier) CreateAIAnalysis(ctx context.Context, arg sqlc.CreateAIAnalysisParams) (sqlc.AiAnalysis, error) {
	return sqlc.AiAnalysis{}, nil
}

// GetAIAnalysis stub para mockQuerier.
func (m *mockQuerier) GetAIAnalysis(ctx context.Context, arg sqlc.GetAIAnalysisParams) (sqlc.AiAnalysis, error) {
	return sqlc.AiAnalysis{}, nil
}

// ListAIAnalyses stub para mockQuerier.
func (m *mockQuerier) ListAIAnalyses(ctx context.Context, arg sqlc.ListAIAnalysesParams) ([]sqlc.AiAnalysis, error) {
	return nil, nil
}

// CompleteAIAnalysis stub para mockQuerier.
func (m *mockQuerier) CompleteAIAnalysis(ctx context.Context, arg sqlc.CompleteAIAnalysisParams) (sqlc.AiAnalysis, error) {
	return sqlc.AiAnalysis{}, nil
}

// FailAIAnalysis stub para mockQuerier.
func (m *mockQuerier) FailAIAnalysis(ctx context.Context, arg sqlc.FailAIAnalysisParams) (sqlc.AiAnalysis, error) {
	return sqlc.AiAnalysis{}, nil
}

// GetActivityForUser stub para mockQuerier.
//
// La consulta devuelve la actividad de un usuario (fase 1.5, T5). Los
// mocks de este archivo no la usan: sólo cumplen el contrato de la
// interfaz sqlc.Querier.
func (m *mockQuerier) GetActivityForUser(ctx context.Context, arg sqlc.GetActivityForUserParams) (sqlc.GetActivityForUserRow, error) {
	return sqlc.GetActivityForUserRow{}, nil
}
