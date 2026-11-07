// Pruebas del adaptador de Claude (Anthropic): contrato de cable y
// clasificación de las respuestas que puede devolver el API.

package ai

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"testing"
)

// TestClaudeAnalyzeSendsExpectedRequest comprueba la petición que sale hacia
// Anthropic: método, ruta, cabeceras y cuerpo con la herramienta forzada.
func TestClaudeAnalyzeSendsExpectedRequest(t *testing.T) {
	const key = "clave-claude-de-prueba"

	srv, capture := capturingServer(t, providerReply(t, ProviderClaude, canonicalAnalysis))
	analyzer := newTestAnalyzer(t, ProviderClaude, "modelo-claude", key, srv)

	if _, err := analyzer.Analyze(context.Background(), testPayload(t)); err != nil {
		t.Fatalf("Analyze(): %v", err)
	}

	method, path, header, body := capture.get()

	if method != http.MethodPost {
		t.Errorf("método = %q, want POST", method)
	}
	if path != "/v1/messages" {
		t.Errorf("ruta = %q, want /v1/messages", path)
	}
	if got := header.Get("Authorization"); got != "Bearer "+key {
		t.Errorf("Authorization = %q, want %q", got, "Bearer "+key)
	}
	if got := header.Get("anthropic-version"); got != ClaudeAPIVersion {
		t.Errorf("anthropic-version = %q, want %q", got, ClaudeAPIVersion)
	}
	if got := header.Get("Content-Type"); got != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", got)
	}
	if got := header.Get("x-api-key"); got != key {
		t.Errorf("x-api-key = %q, want %q", got, key)
	}
	for name, values := range header {
		if strings.EqualFold(name, "Authorization") || strings.EqualFold(name, "x-api-key") {
			continue
		}
		for _, value := range values {
			if strings.Contains(value, key) {
				t.Errorf("la credencial viaja también en la cabecera %q", name)
			}
		}
	}

	request := decodeRequestBody(t, body)
	assertExactKeys(t, request, "model", "max_tokens", "system", "messages", "tools", "tool_choice")
	if request["model"] != "modelo-claude" {
		t.Errorf("model = %v, want modelo-claude", request["model"])
	}
	if request["max_tokens"] != float64(ClaudeMaxTokens) {
		t.Errorf("max_tokens = %v, want %d", request["max_tokens"], ClaudeMaxTokens)
	}
	if system, _ := request["system"].(string); system != analysisInstruction {
		t.Errorf("system = %q, want la instrucción común", system)
	}

	tools, ok := request["tools"].([]any)
	if !ok || len(tools) != 1 {
		t.Fatalf("tools = %v, want una herramienta", request["tools"])
	}
	tool, ok := tools[0].(map[string]any)
	if !ok {
		t.Fatalf("la herramienta no es un objeto: %v", tools[0])
	}
	if tool["name"] != OutputSchemaName {
		t.Errorf("tools[0].name = %v, want %q", tool["name"], OutputSchemaName)
	}
	if _, ok := tool["description"].(string); !ok {
		t.Errorf("tools[0].description ausente: %v", tool)
	}
	assertSameSchema(t, tool["input_schema"])

	toolChoice, ok := request["tool_choice"].(map[string]any)
	if !ok {
		t.Fatalf("tool_choice ausente: %v", request["tool_choice"])
	}
	if toolChoice["type"] != "tool" {
		t.Errorf("tool_choice.type = %v, want tool", toolChoice["type"])
	}
	if toolChoice["name"] != OutputSchemaName {
		t.Errorf("tool_choice.name = %v, want %q", toolChoice["name"], OutputSchemaName)
	}

	content := messageContent(t, ProviderClaude, body)
	sent := extractPayloadFromContent(t, content)
	if sent["kind"] != string(KindActivity) {
		t.Errorf("kind enviado = %v, want %q", sent["kind"], KindActivity)
	}
}

// TestClaudeAnalyzeParsesToolUseReply comprueba que el input de la
// herramienta forzada se valida con el contrato compartido.
func TestClaudeAnalyzeParsesToolUseReply(t *testing.T) {
	srv, _ := capturingServer(t, providerReply(t, ProviderClaude, canonicalAnalysis))
	analyzer := newTestAnalyzer(t, ProviderClaude, "modelo-claude", "clave", srv)

	out, err := analyzer.Analyze(context.Background(), testPayload(t))
	if err != nil {
		t.Fatalf("Analyze(): %v", err)
	}
	if out.Summary != "Sesion de 10000 m en 3600 s." {
		t.Errorf("Summary = %q", out.Summary)
	}
	if len(out.Observations) != 1 || len(out.Cautions) != 1 {
		t.Errorf("observaciones/cautions = %d/%d, want 1/1", len(out.Observations), len(out.Cautions))
	}
}

// TestClaudeRejectionIsPermanent comprueba que un rechazo del modelo es un
// fallo permanente.
func TestClaudeRejectionIsPermanent(t *testing.T) {
	reply := []byte(`{"stop_reason":"refusal","content":[{"type":"text","text":"no"}]}`)

	srv, _ := capturingServer(t, reply)
	analyzer := newTestAnalyzer(t, ProviderClaude, "modelo-claude", "clave", srv)

	_, err := analyzer.Analyze(context.Background(), testPayload(t))
	var providerErr *Error
	if !errors.As(err, &providerErr) {
		t.Fatalf("Analyze() devolvió %v (%T), se esperaba *ai.Error", err, err)
	}
	if providerErr.Kind != ErrorKindPermanent {
		t.Errorf("Kind = %s, want %s", providerErr.Kind, ErrorKindPermanent)
	}
	if providerErr.Reason != ReasonRefusal {
		t.Errorf("Reason = %s, want %s", providerErr.Reason, ReasonRefusal)
	}
}

// TestClaudeTruncatedReplyIsPermanent comprueba que una respuesta cortada por
// max_tokens es un fallo permanente.
func TestClaudeTruncatedReplyIsPermanent(t *testing.T) {
	reply := []byte(`{"stop_reason":"max_tokens","content":[{"type":"tool_use","name":"training_analysis","input":{}}]}`)

	srv, _ := capturingServer(t, reply)
	analyzer := newTestAnalyzer(t, ProviderClaude, "modelo-claude", "clave", srv)

	_, err := analyzer.Analyze(context.Background(), testPayload(t))
	var providerErr *Error
	if !errors.As(err, &providerErr) {
		t.Fatalf("Analyze() devolvió %v (%T), se esperaba *ai.Error", err, err)
	}
	if providerErr.Reason != ReasonTruncated {
		t.Errorf("Reason = %s, want %s", providerErr.Reason, ReasonTruncated)
	}
}

// TestClaudeMissingToolUseIsPermanent comprueba que una respuesta sin el
// bloque tool_use esperado es un fallo permanente.
func TestClaudeMissingToolUseIsPermanent(t *testing.T) {
	reply := []byte(`{"stop_reason":"end_turn","content":[{"type":"text","text":"listo"}]}`)

	srv, _ := capturingServer(t, reply)
	analyzer := newTestAnalyzer(t, ProviderClaude, "modelo-claude", "clave", srv)

	_, err := analyzer.Analyze(context.Background(), testPayload(t))
	var providerErr *Error
	if !errors.As(err, &providerErr) {
		t.Fatalf("Analyze() devolvió %v (%T), se esperaba *ai.Error", err, err)
	}
	if providerErr.Kind != ErrorKindPermanent {
		t.Errorf("Kind = %s, want %s", providerErr.Kind, ErrorKindPermanent)
	}
	if providerErr.Reason != ReasonUnexpected {
		t.Errorf("Reason = %s, want %s", providerErr.Reason, ReasonUnexpected)
	}
}
