// Pruebas del adaptador de OpenRouter: contrato de cable (incluido el
// enrutado que exige salida estructurada) y clasificación de respuestas.

package ai

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"testing"
)

// TestOpenRouterAnalyzeSendsExpectedRequest comprueba la petición que sale
// hacia OpenRouter: método, ruta, cabeceras y cuerpo.
func TestOpenRouterAnalyzeSendsExpectedRequest(t *testing.T) {
	const key = "clave-openrouter-de-prueba"

	srv, capture := capturingServer(t, providerReply(t, ProviderOpenRouter, canonicalAnalysis))
	analyzer := newTestAnalyzer(t, ProviderOpenRouter, "modelo-openrouter", key, srv)

	if _, err := analyzer.Analyze(context.Background(), testPayload(t)); err != nil {
		t.Fatalf("Analyze(): %v", err)
	}

	method, path, header, body := capture.get()

	if method != http.MethodPost {
		t.Errorf("método = %q, want POST", method)
	}
	if path != "/api/v1/chat/completions" {
		t.Errorf("ruta = %q, want /api/v1/chat/completions", path)
	}
	if got := header.Get("Authorization"); got != "Bearer "+key {
		t.Errorf("Authorization = %q, want %q", got, "Bearer "+key)
	}
	if got := header.Get("Content-Type"); got != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", got)
	}
	for name, values := range header {
		if name == "Authorization" {
			continue
		}
		for _, value := range values {
			if strings.Contains(value, key) {
				t.Errorf("la credencial viaja también en la cabecera %q", name)
			}
		}
	}

	request := decodeRequestBody(t, body)
	assertExactKeys(t, request, "model", "messages", "response_format", "provider")
	if request["model"] != "modelo-openrouter" {
		t.Errorf("model = %v, want modelo-openrouter", request["model"])
	}

	responseFormat, ok := request["response_format"].(map[string]any)
	if !ok {
		t.Fatalf("response_format ausente: %v", request["response_format"])
	}
	if responseFormat["type"] != "json_schema" {
		t.Errorf("response_format.type = %v, want json_schema", responseFormat["type"])
	}
	jsonSchema, ok := responseFormat["json_schema"].(map[string]any)
	if !ok {
		t.Fatalf("response_format.json_schema ausente: %v", responseFormat)
	}
	if jsonSchema["name"] != OutputSchemaName {
		t.Errorf("json_schema.name = %v, want %q", jsonSchema["name"], OutputSchemaName)
	}
	if jsonSchema["strict"] != true {
		t.Errorf("json_schema.strict = %v, want true", jsonSchema["strict"])
	}
	assertSameSchema(t, jsonSchema["schema"])

	provider, ok := request["provider"].(map[string]any)
	if !ok {
		t.Fatalf("provider ausente: %v", request["provider"])
	}
	if provider["require_parameters"] != true {
		t.Errorf("provider.require_parameters = %v, want true", provider["require_parameters"])
	}
	if provider["allow_fallbacks"] != false {
		t.Errorf("provider.allow_fallbacks = %v, want false", provider["allow_fallbacks"])
	}

	content := messageContent(t, ProviderOpenRouter, body)
	if !strings.Contains(content, analysisInstruction) {
		t.Error("el mensaje de usuario no lleva la instrucción común")
	}
	sent := extractPayloadFromContent(t, content)
	if sent["kind"] != string(KindActivity) {
		t.Errorf("kind enviado = %v, want %q", sent["kind"], KindActivity)
	}
}

// TestOpenRouterAnalyzeParsesReply comprueba que la respuesta se valida con
// el contrato compartido.
func TestOpenRouterAnalyzeParsesReply(t *testing.T) {
	srv, _ := capturingServer(t, providerReply(t, ProviderOpenRouter, canonicalAnalysis))
	analyzer := newTestAnalyzer(t, ProviderOpenRouter, "modelo-openrouter", "clave", srv)

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

// TestOpenRouterRejectionIsPermanent comprueba que un rechazo del modelo es
// un fallo permanente.
func TestOpenRouterRejectionIsPermanent(t *testing.T) {
	reply := []byte(`{"choices":[{"finish_reason":"stop","message":{"role":"assistant","content":"","refusal":"no"}}]}`)

	srv, _ := capturingServer(t, reply)
	analyzer := newTestAnalyzer(t, ProviderOpenRouter, "modelo-openrouter", "clave", srv)

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
