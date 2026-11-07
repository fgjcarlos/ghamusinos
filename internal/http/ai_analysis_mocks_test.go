package http

import (
	"context"
	"net/http"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/require"

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

// TestRouterMountsAIRoutesViaChiWalk walks the chi router tree and asserts
// that the three AI routes are mounted inside the authenticated /api/v1
// group (phase 1.5, T7).
//
// Without this guard the handlers could exist and be tested while the
// router never exposes them: exactly the failure mode that happened with
// the activities routes (issue #158).
func TestRouterMountsAIRoutesViaChiWalk(t *testing.T) {
	srv := nuevoServidor(t)
	require.NotNil(t, srv)

	mux, ok := srv.Config.Handler.(*chi.Mux)
	require.True(t, ok, "expected *chi.Mux, got %T", srv.Config.Handler)

	var routes []string
	if err := chi.Walk(mux, func(method, route string, _ http.Handler, _ ...func(http.Handler) http.Handler) error {
		routes = append(routes, method+" "+route)
		return nil
	}); err != nil {
		t.Fatalf("walk routes: %v", err)
	}

	for _, want := range []string{
		"GET /api/v1/ai/status",
		"GET /api/v1/ai/analyses",
		"POST /api/v1/ai/analyses",
	} {
		require.Contains(t, routes, want)
	}
}

// TestRouterAIRoutesRequireAuth proves that the AI routes live in the
// authenticated group: without a token they answer 401 and never reach the
// handlers.
func TestRouterAIRoutesRequireAuth(t *testing.T) {
	srv := nuevoServidor(t)

	resp := testGet(t, srv.URL+"/api/v1/ai/status")
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("GET /api/v1/ai/status sin token quería 401, obtuvo %d", resp.StatusCode)
	}
}
