package app

import (
	"testing"

	"github.com/fgjcarlos/ghamusinos/internal/ai"
	"github.com/fgjcarlos/ghamusinos/internal/config"
)

// TestBuildAIAnalyzer_DisabledWhenAIOff verifica que sin el flag global no
// se construye ningún analizador: la IA desactivada no envía nada a ningún
// proveedor y el producto sigue igual (S7).
func TestBuildAIAnalyzer_DisabledWhenAIOff(t *testing.T) {
	cfg := &config.Config{}
	cfg.AI.Enabled = false
	cfg.AI.Available = false

	if analyzer := buildAIAnalyzer(cfg); analyzer != nil {
		t.Errorf("buildAIAnalyzer con la IA desactivada = %v, quiere nil", analyzer)
	}
}

// TestBuildAIAnalyzer_DisabledWhenUnavailable verifica que una IA
// configurada pero no disponible (sin credencial, sin modelo o con un
// proveedor inválido) tampoco construye analizador, y por tanto no puede
// Romper el arranque de la aplicación (S7).
func TestBuildAIAnalyzer_DisabledWhenUnavailable(t *testing.T) {
	t.Run("proveedor inválido", func(t *testing.T) {
		cfg := &config.Config{}
		cfg.AI.Enabled = true
		cfg.AI.Available = true
		cfg.AI.Provider = "proveedor-inexistente"
		cfg.AI.Model = "modelo"
		cfg.AI.APIKey = "clave"

		if analyzer := buildAIAnalyzer(cfg); analyzer != nil {
			t.Errorf("buildAIAnalyzer con proveedor inválido = %v, quiere nil", analyzer)
		}
	})

	t.Run("modelo vacío", func(t *testing.T) {
		cfg := &config.Config{}
		cfg.AI.Enabled = true
		cfg.AI.Available = true
		cfg.AI.Provider = config.AIProviderOpenAI
		cfg.AI.Model = ""
		cfg.AI.APIKey = "clave"

		if analyzer := buildAIAnalyzer(cfg); analyzer != nil {
			t.Errorf("buildAIAnalyzer sin modelo = %v, quiere nil", analyzer)
		}
	})
}

// TestBuildAIAnalyzer_ReadyWhenConfigured verifica que con la IA activa y
// bien configurada se construye el analizador del proveedor elegido.
func TestBuildAIAnalyzer_ReadyWhenConfigured(t *testing.T) {
	cfg := &config.Config{}
	cfg.AI.Enabled = true
	cfg.AI.Available = true
	cfg.AI.Provider = config.AIProviderClaude
	cfg.AI.Model = "claude-test-model"
	cfg.AI.APIKey = "clave-de-prueba"

	analyzer := buildAIAnalyzer(cfg)
	if analyzer == nil {
		t.Fatal("buildAIAnalyzer con la IA configurada = nil, quiere un analizador")
	}
	if analyzer.Provider() != ai.ProviderClaude {
		t.Errorf("Provider() = %q, quiere %q", analyzer.Provider(), ai.ProviderClaude)
	}
}
