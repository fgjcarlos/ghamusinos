// Pruebas del adaptador de OpenAI: contrato de cable y clasificación de las
// respuestas que puede devolver el API.

package ai

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"testing"
)

// TestOpenAIAnalyzeSendsExpectedRequest comprueba la petición que sale hacia
// OpenAI: método, ruta, cabeceras y cuerpo.
func TestOpenAIAnalyzeSendsExpectedRequest(t *testing.T) {
	const key = "clave-openai-de-prueba"

	srv, capture := capturingServer(t, providerReply(t, ProviderOpenAI, canonicalAnalysis))
	analyzer := newTestAnalyzer(t, ProviderOpenAI, "modelo-openai", key, srv)

	if _, err := analyzer.Analyze(context.Background(), testPayload(t)); err != nil {
		t.Fatalf("Analyze(): %v", err)
	}

	method, path, header, body := capture.get()

	if method != http.MethodPost {
		t.Errorf("método = %q, want POST", method)
	}
	if path != "/v1/chat/completions" {
		t.Errorf("ruta = %q, want /v1/chat/completions", path)
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
	assertExactKeys(t, request, "model", "messages", "response_format")
	if request["model"] != "modelo-openai" {
		t.Errorf("model = %v, want modelo-openai", request["model"])
	}

	responseFormat, ok := request["response_format"].(map[string]any)
	if !ok {
		t.Fatalf("response_format ausente o con otra forma: %v", request["response_format"])
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

	content := messageContent(t, ProviderOpenAI, body)
	if !strings.Contains(content, analysisInstruction) {
		t.Error("el mensaje de usuario no lleva la instrucción común")
	}
	sent := extractPayloadFromContent(t, content)
	if sent["kind"] != string(KindActivity) {
		t.Errorf("kind enviado = %v, want %q", sent["kind"], KindActivity)
	}
}

// TestOpenAIAnalyzeParsesReply comprueba que la respuesta se valida con el
// contrato compartido y devuelve la salida esperada.
func TestOpenAIAnalyzeParsesReply(t *testing.T) {
	srv, _ := capturingServer(t, providerReply(t, ProviderOpenAI, canonicalAnalysis))
	analyzer := newTestAnalyzer(t, ProviderOpenAI, "modelo-openai", "clave", srv)

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

// TestOpenAIRejectionIsPermanent comprueba que un rechazo del modelo es un
// fallo permanente.
func TestOpenAIRejectionIsPermanent(t *testing.T) {
	reply := []byte(`{"choices":[{"finish_reason":"stop","message":{"role":"assistant","content":"","refusal":"no puedo"}}]}`)

	srv, _ := capturingServer(t, reply)
	analyzer := newTestAnalyzer(t, ProviderOpenAI, "modelo-openai", "clave", srv)

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

// TestOpenAITruncatedReplyIsPermanent comprueba que una respuesta cortada
// por el límite de tokens es un fallo permanente.
func TestOpenAITruncatedReplyIsPermanent(t *testing.T) {
	reply := []byte(`{"choices":[{"finish_reason":"length","message":{"role":"assistant","content":"{}"}}]}`)

	srv, _ := capturingServer(t, reply)
	analyzer := newTestAnalyzer(t, ProviderOpenAI, "modelo-openai", "clave", srv)

	_, err := analyzer.Analyze(context.Background(), testPayload(t))
	var providerErr *Error
	if !errors.As(err, &providerErr) {
		t.Fatalf("Analyze() devolvió %v (%T), se esperaba *ai.Error", err, err)
	}
	if providerErr.Kind != ErrorKindPermanent {
		t.Errorf("Kind = %s, want %s", providerErr.Kind, ErrorKindPermanent)
	}
	if providerErr.Reason != ReasonTruncated {
		t.Errorf("Reason = %s, want %s", providerErr.Reason, ReasonTruncated)
	}
}

// TestOpenAIContentFilterIsPermanent comprueba que un bloqueo por política de
// contenido se trata como rechazo permanente.
func TestOpenAIContentFilterIsPermanent(t *testing.T) {
	reply := []byte(`{"choices":[{"finish_reason":"content_filter","message":{"role":"assistant","content":""}}]}`)

	srv, _ := capturingServer(t, reply)
	analyzer := newTestAnalyzer(t, ProviderOpenAI, "modelo-openai", "clave", srv)

	_, err := analyzer.Analyze(context.Background(), testPayload(t))
	var providerErr *Error
	if !errors.As(err, &providerErr) {
		t.Fatalf("Analyze() devolvió %v (%T), se esperaba *ai.Error", err, err)
	}
	if providerErr.Reason != ReasonRefusal {
		t.Errorf("Reason = %s, want %s", providerErr.Reason, ReasonRefusal)
	}
}

// TestOpenAIUnexpectedFinishReasonIsPermanent comprueba que un final de
// respuesta inesperado (por ejemplo tool_calls) es un fallo permanente.
func TestOpenAIUnexpectedFinishReasonIsPermanent(t *testing.T) {
	reply := []byte(`{"choices":[{"finish_reason":"tool_calls","message":{"role":"assistant","content":"{}"}}]}`)

	srv, _ := capturingServer(t, reply)
	analyzer := newTestAnalyzer(t, ProviderOpenAI, "modelo-openai", "clave", srv)

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

// TestOpenAIEmptyChoicesIsPermanent comprueba que una respuesta sin
// alternativas es un fallo permanente.
func TestOpenAIEmptyChoicesIsPermanent(t *testing.T) {
	srv, _ := capturingServer(t, []byte(`{"choices":[]}`))
	analyzer := newTestAnalyzer(t, ProviderOpenAI, "modelo-openai", "clave", srv)

	_, err := analyzer.Analyze(context.Background(), testPayload(t))
	var providerErr *Error
	if !errors.As(err, &providerErr) {
		t.Fatalf("Analyze() devolvió %v (%T), se esperaba *ai.Error", err, err)
	}
	if providerErr.Reason != ReasonUnexpected {
		t.Errorf("Reason = %s, want %s", providerErr.Reason, ReasonUnexpected)
	}
}
