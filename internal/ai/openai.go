// Adaptador de OpenAI (chat completions con salida estructurada estricta).
//
// Contrato de cable (https://developers.openai.com/api/docs/guides/migrate-to-responses):
//
//	POST https://api.openai.com/v1/chat/completions
//	Authorization: Bearer <key>
//	{"model":…,"messages":[{"role":"user","content":…}],
//	 "response_format":{"type":"json_schema",
//	                    "json_schema":{"name":…,"strict":true,"schema":{…}}}}
//
// La respuesta trae el análisis como texto JSON en choices[0].message.content.
// El sobre de chat completions es el mismo que usa OpenRouter, así que los
// tipos viven aquí y openrouter.go los reutiliza.

package ai

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
)

// OpenAIEndpoint es el endpoint de producción del API de chat completions de OpenAI.
const OpenAIEndpoint = "https://api.openai.com/v1/chat/completions"

// OpenAIAnalyzer habla con el API de chat completions de OpenAI.
type OpenAIAnalyzer struct {
	model    string
	apiKey   string
	client   *http.Client
	endpoint string
}

// NewOpenAI devuelve el adaptador de OpenAI.
//
// El modelo y la credencial son obligatorios: no hay modelo por defecto.
func NewOpenAI(model, apiKey string, client *http.Client) (*OpenAIAnalyzer, error) {
	if model == "" {
		return nil, errors.New("ai: openai: model is required")
	}
	if apiKey == "" {
		return nil, errors.New("ai: openai: api key is required")
	}
	return &OpenAIAnalyzer{
		model:    model,
		apiKey:   apiKey,
		client:   providerClient(client),
		endpoint: OpenAIEndpoint,
	}, nil
}

// Provider implementa Analyzer.
func (a *OpenAIAnalyzer) Provider() Provider { return ProviderOpenAI }

// Analyze implementa Analyzer: un intento HTTP y una salida validada.
func (a *OpenAIAnalyzer) Analyze(ctx context.Context, payload Payload) (Output, error) {
	content, err := userMessage(payload)
	if err != nil {
		return Output{}, err
	}

	body, err := json.Marshal(chatCompletionsRequest{
		Model: a.model,
		Messages: []chatCompletionsMessage{
			{Role: "user", Content: content},
		},
		ResponseFormat: chatCompletionsResponseFormat{
			Type:       "json_schema",
			JSONSchema: OutputSchema(),
		},
	})
	if err != nil {
		return Output{}, fmt.Errorf("ai: openai: cannot serialize request: %w", err)
	}

	req, err := newProviderRequest(ctx, ProviderOpenAI, a.endpoint, body, map[string]string{
		"Authorization": "Bearer " + a.apiKey,
	})
	if err != nil {
		return Output{}, err
	}

	raw, err := performRequest(ProviderOpenAI, a.client, req)
	if err != nil {
		return Output{}, err
	}

	analysis, err := extractChatCompletionsContent(ProviderOpenAI, raw)
	if err != nil {
		return Output{}, err
	}

	out, err := ParseOutput(analysis, payload)
	if err != nil {
		return Output{}, newProviderError(ProviderOpenAI, ErrorKindPermanent, ReasonSchemaViolation, http.StatusOK, err)
	}
	return out, nil
}

// chatCompletionsRequest es el cuerpo de la petición de chat completions.
// Lo usan OpenAI y OpenRouter (mismo formato).
type chatCompletionsRequest struct {
	Model          string                        `json:"model"`
	Messages       []chatCompletionsMessage      `json:"messages"`
	ResponseFormat chatCompletionsResponseFormat `json:"response_format"`
}

// chatCompletionsMessage es un mensaje de la conversación.
type chatCompletionsMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// chatCompletionsResponseFormat exige salida estructurada estricta.
type chatCompletionsResponseFormat struct {
	Type       string `json:"type"`
	JSONSchema Schema `json:"json_schema"`
}

// chatCompletionsResponse es el sobre de respuesta de chat completions.
type chatCompletionsResponse struct {
	Choices []chatCompletionsChoice `json:"choices"`
}

// chatCompletionsChoice es una alternativa de respuesta.
type chatCompletionsChoice struct {
	FinishReason string                      `json:"finish_reason"`
	Message      chatCompletionsReplyMessage `json:"message"`
}

// chatCompletionsReplyMessage es el mensaje de la respuesta: además del
// contenido puede traer un rechazo del modelo.
type chatCompletionsReplyMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
	Refusal string `json:"refusal"`
}

// extractChatCompletionsContent saca el JSON del análisis del sobre de
// chat completions.
//
// Clasifica los fallos que el proveedor puede devolver sin filtrar el
// contenido de la respuesta.
func extractChatCompletionsContent(provider Provider, body []byte) ([]byte, error) {
	var envelope chatCompletionsResponse
	if err := json.Unmarshal(body, &envelope); err != nil {
		return nil, newProviderError(provider, ErrorKindPermanent, ReasonInvalidJSON, http.StatusOK, err)
	}
	if len(envelope.Choices) == 0 {
		return nil, newProviderError(provider, ErrorKindPermanent, ReasonUnexpected, http.StatusOK, nil)
	}

	choice := envelope.Choices[0]
	if choice.Message.Refusal != "" {
		return nil, newProviderError(provider, ErrorKindPermanent, ReasonRefusal, http.StatusOK, nil)
	}

	switch choice.FinishReason {
	case "", "stop":
	case "length":
		return nil, newProviderError(provider, ErrorKindPermanent, ReasonTruncated, http.StatusOK, nil)
	case "content_filter":
		return nil, newProviderError(provider, ErrorKindPermanent, ReasonRefusal, http.StatusOK, nil)
	default:
		return nil, newProviderError(provider, ErrorKindPermanent, ReasonUnexpected, http.StatusOK, nil)
	}

	if choice.Message.Content == "" {
		return nil, newProviderError(provider, ErrorKindPermanent, ReasonUnexpected, http.StatusOK, nil)
	}
	if !json.Valid([]byte(choice.Message.Content)) {
		return nil, newProviderError(provider, ErrorKindPermanent, ReasonInvalidJSON, http.StatusOK, nil)
	}
	return []byte(choice.Message.Content), nil
}
