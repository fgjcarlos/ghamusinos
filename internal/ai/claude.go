// Adaptador de Claude (Anthropic messages API con herramienta forzada).
//
// Contrato de cable (https://platform.claude.com/docs/en/api/messages):
//
//	POST https://api.anthropic.com/v1/messages
//	Content-Type: application/json
//	anthropic-version: 2026-10-21
//	Authorization: Bearer <key>
//	{"model":…,"max_tokens":…,"system":…,
//	 "messages":[{"role":"user","content":…}],
//	 "tools":[{"name":…,"description":…,"input_schema":{…}}],
//	 "tool_choice":{"type":"tool","name":…}}
//
// La respuesta trae un bloque de contenido de tipo tool_use cuyo input es el
// objeto JSON del análisis. La herramienta forzada es la forma que tiene
// Anthropic de pedir salida estructurada estricta.

package ai

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
)

// Parámetros fijos del API de Anthropic.
const (
	// ClaudeEndpoint es el endpoint de producción del API de messages de Anthropic.
	ClaudeEndpoint = "https://api.anthropic.com/v1/messages"
	// ClaudeAPIVersion es la versión del API de Anthropic que usa el adaptador.
	ClaudeAPIVersion = "2026-10-21"
	// ClaudeMaxTokens es el tope de tokens de la respuesta.
	ClaudeMaxTokens = 1024
	// ClaudeToolDescription describe la herramienta forzada al modelo.
	ClaudeToolDescription = "Devuelve el analisis de entrenamiento con el esquema estricto indicado."
)

// ClaudeAnalyzer habla con el API de messages de Anthropic.
type ClaudeAnalyzer struct {
	model    string
	apiKey   string
	client   *http.Client
	endpoint string
}

// NewClaude devuelve el adaptador de Claude.
//
// El modelo y la credencial son obligatorios: no hay modelo por defecto.
func NewClaude(model, apiKey string, client *http.Client) (*ClaudeAnalyzer, error) {
	if model == "" {
		return nil, errors.New("ai: claude: model is required")
	}
	if apiKey == "" {
		return nil, errors.New("ai: claude: api key is required")
	}
	return &ClaudeAnalyzer{
		model:    model,
		apiKey:   apiKey,
		client:   providerClient(client),
		endpoint: ClaudeEndpoint,
	}, nil
}

// Provider implementa Analyzer.
func (a *ClaudeAnalyzer) Provider() Provider { return ProviderClaude }

// Analyze implementa Analyzer: un intento HTTP y una salida validada.
func (a *ClaudeAnalyzer) Analyze(ctx context.Context, payload Payload) (Output, error) {
	content, err := payloadMessage(payload)
	if err != nil {
		return Output{}, err
	}

	schema := OutputSchema()
	body, err := json.Marshal(claudeRequest{
		Model:     a.model,
		MaxTokens: ClaudeMaxTokens,
		System:    analysisInstruction,
		Messages: []claudeMessage{
			{Role: "user", Content: content},
		},
		Tools: []claudeTool{
			{
				Name:        schema.Name,
				Description: ClaudeToolDescription,
				InputSchema: schema.Schema,
			},
		},
		ToolChoice: claudeToolChoice{
			Type: "tool",
			Name: schema.Name,
		},
	})
	if err != nil {
		return Output{}, fmt.Errorf("ai: claude: cannot serialize request: %w", err)
	}

	req, err := newProviderRequest(ctx, ProviderClaude, a.endpoint, body, map[string]string{
		// La documentación oficial muestra las dos formas de autenticar
		// una clave de API (cabecera "x-api-key" y token bearer en
		// "Authorization"). Se envían ambas para que la cuenta
		// configurada acepte la llamada sin depender de una variante.
		"x-api-key":         a.apiKey,
		"Authorization":     "Bearer " + a.apiKey,
		"anthropic-version": ClaudeAPIVersion,
	})
	if err != nil {
		return Output{}, err
	}

	raw, err := performRequest(ProviderClaude, a.client, req)
	if err != nil {
		return Output{}, err
	}

	analysis, err := extractClaudeToolInput(raw)
	if err != nil {
		return Output{}, err
	}

	out, err := ParseOutput(analysis, payload)
	if err != nil {
		return Output{}, newProviderError(ProviderClaude, ErrorKindPermanent, ReasonSchemaViolation, http.StatusOK, err)
	}
	return out, nil
}

// claudeRequest es el cuerpo de la petición de messages.
type claudeRequest struct {
	Model      string           `json:"model"`
	MaxTokens  int              `json:"max_tokens"`
	System     string           `json:"system"`
	Messages   []claudeMessage  `json:"messages"`
	Tools      []claudeTool     `json:"tools"`
	ToolChoice claudeToolChoice `json:"tool_choice"`
}

// claudeMessage es un mensaje de la conversación.
type claudeMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// claudeTool define la herramienta forzada, con el esquema de salida
// compartido como input_schema.
type claudeTool struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	InputSchema json.RawMessage `json:"input_schema"`
}

// claudeToolChoice fuerza el uso de una herramienta concreta.
type claudeToolChoice struct {
	Type string `json:"type"`
	Name string `json:"name"`
}

// claudeResponse es el sobre de respuesta de messages.
type claudeResponse struct {
	StopReason string              `json:"stop_reason"`
	Content    []claudeContentItem `json:"content"`
}

// claudeContentItem es un bloque de contenido de la respuesta.
type claudeContentItem struct {
	Type  string          `json:"type"`
	Name  string          `json:"name"`
	Input json.RawMessage `json:"input"`
}

// extractClaudeToolInput saca el JSON del análisis del bloque tool_use.
func extractClaudeToolInput(body []byte) ([]byte, error) {
	var envelope claudeResponse
	if err := json.Unmarshal(body, &envelope); err != nil {
		return nil, newProviderError(ProviderClaude, ErrorKindPermanent, ReasonInvalidJSON, http.StatusOK, err)
	}

	switch envelope.StopReason {
	case "refusal":
		return nil, newProviderError(ProviderClaude, ErrorKindPermanent, ReasonRefusal, http.StatusOK, nil)
	case "max_tokens":
		return nil, newProviderError(ProviderClaude, ErrorKindPermanent, ReasonTruncated, http.StatusOK, nil)
	}

	for _, item := range envelope.Content {
		if item.Type != "tool_use" || item.Name != OutputSchemaName {
			continue
		}
		if len(item.Input) == 0 {
			return nil, newProviderError(ProviderClaude, ErrorKindPermanent, ReasonUnexpected, http.StatusOK, nil)
		}
		if !json.Valid(item.Input) {
			return nil, newProviderError(ProviderClaude, ErrorKindPermanent, ReasonInvalidJSON, http.StatusOK, nil)
		}
		return item.Input, nil
	}
	return nil, newProviderError(ProviderClaude, ErrorKindPermanent, ReasonUnexpected, http.StatusOK, nil)
}
