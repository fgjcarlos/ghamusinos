// Adaptador de OpenRouter (chat completions con salida estructurada estricta).
//
// Contrato de cable (https://openrouter.ai/docs/guides/features/structured-outputs,
// https://openrouter.ai/docs/guides/routing/provider-selection):
//
//	POST https://openrouter.ai/api/v1/chat/completions
//	Authorization: Bearer <key>
//	{"model":…,"messages":[{"role":"user","content":…}],
//	 "response_format":{"type":"json_schema",
//	                    "json_schema":{"name":…,"strict":true,"schema":{…}}},
//	 "provider":{"require_parameters":true,"allow_fallbacks":false}}
//
// OpenRouter habla el mismo sobre que OpenAI, así que reutiliza los tipos de
// chat completions. El bloque provider es lo que garantiza que solo respondan
// proveedores capaces de honourar el esquema: sin él, OpenRouter podría
// enrutar a un modelo que devuelve texto libre.

package ai

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
)

// OpenRouterEndpoint es el endpoint de producción del API de chat completions de OpenRouter.
const OpenRouterEndpoint = "https://openrouter.ai/api/v1/chat/completions"

// OpenRouterAnalyzer habla con el API de chat completions de OpenRouter.
type OpenRouterAnalyzer struct {
	model    string
	apiKey   string
	client   *http.Client
	endpoint string
}

// NewOpenRouter devuelve el adaptador de OpenRouter.
//
// El modelo y la credencial son obligatorios: no hay modelo por defecto.
func NewOpenRouter(model, apiKey string, client *http.Client) (*OpenRouterAnalyzer, error) {
	if model == "" {
		return nil, errors.New("ai: openrouter: model is required")
	}
	if apiKey == "" {
		return nil, errors.New("ai: openrouter: api key is required")
	}
	return &OpenRouterAnalyzer{
		model:    model,
		apiKey:   apiKey,
		client:   providerClient(client),
		endpoint: OpenRouterEndpoint,
	}, nil
}

// Provider implementa Analyzer.
func (a *OpenRouterAnalyzer) Provider() Provider { return ProviderOpenRouter }

// Analyze implementa Analyzer: un intento HTTP y una salida validada.
func (a *OpenRouterAnalyzer) Analyze(ctx context.Context, payload Payload) (Output, error) {
	content, err := userMessage(payload)
	if err != nil {
		return Output{}, err
	}

	body, err := json.Marshal(openRouterRequest{
		Model: a.model,
		Messages: []chatCompletionsMessage{
			{Role: "user", Content: content},
		},
		ResponseFormat: chatCompletionsResponseFormat{
			Type:       "json_schema",
			JSONSchema: OutputSchema(),
		},
		Provider: openRouterProviderSelection{
			RequireParameters: true,
			AllowFallbacks:    false,
		},
	})
	if err != nil {
		return Output{}, fmt.Errorf("ai: openrouter: cannot serialize request: %w", err)
	}

	req, err := newProviderRequest(ctx, ProviderOpenRouter, a.endpoint, body, map[string]string{
		"Authorization": "Bearer " + a.apiKey,
	})
	if err != nil {
		return Output{}, err
	}

	raw, err := performRequest(ProviderOpenRouter, a.client, req)
	if err != nil {
		return Output{}, err
	}

	analysis, err := extractChatCompletionsContent(ProviderOpenRouter, raw)
	if err != nil {
		return Output{}, err
	}

	out, err := ParseOutput(analysis, payload)
	if err != nil {
		return Output{}, newProviderError(ProviderOpenRouter, ErrorKindPermanent, ReasonSchemaViolation, http.StatusOK, err)
	}
	return out, nil
}

// openRouterRequest es el cuerpo de la petición de OpenRouter: chat
// completions más la selección de proveedor.
type openRouterRequest struct {
	Model          string                        `json:"model"`
	Messages       []chatCompletionsMessage      `json:"messages"`
	ResponseFormat chatCompletionsResponseFormat `json:"response_format"`
	Provider       openRouterProviderSelection   `json:"provider"`
}

// openRouterProviderSelection fija el enrutado de OpenRouter.
type openRouterProviderSelection struct {
	RequireParameters bool `json:"require_parameters"`
	AllowFallbacks    bool `json:"allow_fallbacks"`
}
