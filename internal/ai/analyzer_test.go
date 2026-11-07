// Pruebas de la capa común de proveedores: orden, registro, clasificación
// de errores y equivalencia de la salida entre proveedores.

package ai

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// ─────────────────────────────────────────────────────────────────────────
// Ayudas compartidas por los archivos de test del paquete
// ─────────────────────────────────────────────────────────────────────────

// rewriteTransport reescribe el host del proveedor al servidor httptest.
//
// Así los endpoints de producción se quedan fijos en el código (no hay URLs
// configurables) y los tests hablan con un servidor local. Es el mismo truco
// que usa internal/strava.
type rewriteTransport struct {
	fromTo map[string]string
	base   http.RoundTripper
}

// RoundTrip implementa http.RoundTripper.
func (t *rewriteTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if newHost, ok := t.fromTo[req.URL.Host]; ok {
		clone := req.Clone(req.Context())
		clone.URL.Host = newHost
		clone.URL.Scheme = "http"
		return t.base.RoundTrip(clone)
	}
	return t.base.RoundTrip(req)
}

// providerHost devuelve el host de producción del endpoint de un proveedor.
func providerHost(provider Provider) string {
	switch provider {
	case ProviderOpenAI:
		return "api.openai.com"
	case ProviderClaude:
		return "api.anthropic.com"
	case ProviderOpenRouter:
		return "openrouter.ai"
	default:
		return ""
	}
}

// newTestAnalyzer devuelve un analizador cuyas peticiones llegan a srv.
func newTestAnalyzer(t *testing.T, provider Provider, model, key string, srv *httptest.Server) Analyzer {
	t.Helper()

	host := strings.TrimPrefix(srv.URL, "http://")
	client := &http.Client{
		Timeout: 5 * time.Second,
		Transport: &rewriteTransport{
			fromTo: map[string]string{providerHost(provider): host},
			base:   http.DefaultTransport,
		},
	}

	var (
		analyzer Analyzer
		err      error
	)
	switch provider {
	case ProviderOpenAI:
		analyzer, err = NewOpenAI(model, key, client)
	case ProviderClaude:
		analyzer, err = NewClaude(model, key, client)
	case ProviderOpenRouter:
		analyzer, err = NewOpenRouter(model, key, client)
	default:
		t.Fatalf("proveedor no soportado en test: %q", provider)
	}
	if err != nil {
		t.Fatalf("constructor de %s: %v", provider, err)
	}
	return analyzer
}

// testPayload devuelve un payload con agregados conocidos.
func testPayload(t *testing.T) Payload {
	t.Helper()
	payload, err := NewBuilder(KindActivity).
		WindowDays(7).
		DistanceMeters(10000).
		DurationSeconds(3600).
		ElevationGainMeters(150).
		AvgHeartRate(145).
		TrainingLoad(500).
		Build()
	if err != nil {
		t.Fatalf("Build() inesperado: %v", err)
	}
	return payload
}

// canonicalAnalysis es un análisis válido para el payload de test: solo
// menciona números que se enviaron.
const canonicalAnalysis = `{"summary":"Sesion de 10000 m en 3600 s.",` +
	`"observations":["Desnivel de 150 m con frecuencia 145 bpm."],` +
	`"cautions":["Carga de 500: vigila la ventana de 7 dias."]}`

// providerReply construye el cuerpo de respuesta de un proveedor que lleva
// el análisis dado.
func providerReply(t *testing.T, provider Provider, analysis string) []byte {
	t.Helper()
	switch provider {
	case ProviderOpenAI, ProviderOpenRouter:
		content, err := json.Marshal(analysis)
		if err != nil {
			t.Fatalf("json.Marshal del contenido: %v", err)
		}
		return []byte(fmt.Sprintf(
			`{"choices":[{"finish_reason":"stop","message":{"role":"assistant","content":%s}}]}`,
			content,
		))
	case ProviderClaude:
		return []byte(fmt.Sprintf(
			`{"stop_reason":"tool_use","content":[{"type":"tool_use","name":"training_analysis","input":%s}]}`,
			analysis,
		))
	default:
		t.Fatalf("proveedor no soportado en test: %q", provider)
	}
	return nil
}

// jsonServer levanta un servidor httptest que responde siempre con lo mismo.
func jsonServer(t *testing.T, status int, contentType string, body []byte) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", contentType)
		w.WriteHeader(status)
		_, _ = w.Write(body)
	}))
	t.Cleanup(srv.Close)
	return srv
}

// extractPayloadFromContent saca el JSON de agregados del mensaje de usuario
// y lo devuelve como mapa.
func extractPayloadFromContent(t *testing.T, content string) map[string]any {
	t.Helper()
	_, after, found := strings.Cut(content, "Agregados:\n")
	if !found {
		t.Fatalf("el mensaje de usuario no lleva la sección de agregados: %q", content)
	}
	var payload map[string]any
	if err := json.Unmarshal([]byte(after), &payload); err != nil {
		t.Fatalf("el contenido de los agregados no es JSON: %v", err)
	}
	return payload
}

// messageContent saca el mensaje de usuario del cuerpo de la petición,
// sea cual sea la forma del proveedor.
func messageContent(t *testing.T, provider Provider, body []byte) string {
	t.Helper()

	var envelope struct {
		Messages []map[string]any `json:"messages"`
	}
	if err := json.Unmarshal(body, &envelope); err != nil {
		t.Fatalf("el cuerpo de la petición no es JSON: %v", err)
	}
	if len(envelope.Messages) != 1 {
		t.Fatalf("la petición de %s lleva %d mensajes, want 1", provider, len(envelope.Messages))
	}
	assertExactKeys(t, envelope.Messages[0], "role", "content")

	role, _ := envelope.Messages[0]["role"].(string)
	if role != "user" {
		t.Fatalf("el rol del mensaje es %q, want %q", role, "user")
	}
	content, ok := envelope.Messages[0]["content"].(string)
	if !ok {
		t.Fatalf("el contenido del mensaje no es texto: %v", envelope.Messages[0]["content"])
	}
	return content
}

// assertSameSchema comprueba que el schema enviado es el compartido.
//
// Compara el valor JSON, no el texto: el orden de las claves no cuenta.
func assertSameSchema(t *testing.T, sent any) {
	t.Helper()
	raw, err := json.Marshal(sent)
	if err != nil {
		t.Fatalf("json.Marshal del schema enviado: %v", err)
	}
	var got, want any
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatalf("el schema enviado no es JSON: %v", err)
	}
	if err := json.Unmarshal(OutputSchema().Schema, &want); err != nil {
		t.Fatalf("el schema compartido no es JSON: %v", err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("el schema enviado no es el compartido:\n got %s\nwant %s", raw, OutputSchema().Schema)
	}
}

// requestCapture guarda la petición que recibió el proveedor.
type requestCapture struct {
	mu     sync.Mutex
	method string
	path   string
	header http.Header
	body   []byte
}

// record guarda la petición recibida.
func (c *requestCapture) record(r *http.Request) {
	body, _ := io.ReadAll(r.Body)
	c.mu.Lock()
	defer c.mu.Unlock()
	c.method = r.Method
	c.path = r.URL.Path
	c.header = r.Header.Clone()
	c.body = body
}

// get devuelve una copia de lo capturado.
func (c *requestCapture) get() (method, path string, header http.Header, body []byte) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.method, c.path, c.header.Clone(), c.body
}

// capturingServer levanta un servidor que guarda la petición recibida y
// responde siempre con el mismo cuerpo.
func capturingServer(t *testing.T, reply []byte) (*httptest.Server, *requestCapture) {
	t.Helper()
	capture := &requestCapture{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capture.record(r)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(reply)
	}))
	t.Cleanup(srv.Close)
	return srv, capture
}

// assertExactKeys comprueba que el mapa tiene exactamente las claves dadas.
func assertExactKeys(t *testing.T, got map[string]any, want ...string) {
	t.Helper()
	if len(got) != len(want) {
		t.Errorf("el mapa tiene %d claves (%v), want %d (%v)", len(got), keysOf(got), len(want), want)
	}
	for _, key := range want {
		if _, ok := got[key]; !ok {
			t.Errorf("falta la clave %q; claves: %v", key, keysOf(got))
		}
	}
}

// keysOf devuelve las claves de un mapa, ordenadas.
func keysOf(m map[string]any) []string {
	keys := make([]string, 0, len(m))
	for key := range m {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

// decodeRequestBody decodifica el cuerpo de la petición como mapa.
func decodeRequestBody(t *testing.T, body []byte) map[string]any {
	t.Helper()
	var decoded map[string]any
	if err := json.Unmarshal(body, &decoded); err != nil {
		t.Fatalf("el cuerpo de la petición no es JSON: %v", err)
	}
	return decoded
}

// ─────────────────────────────────────────────────────────────────────────
// Orden y registro de proveedores
// ─────────────────────────────────────────────────────────────────────────

// TestSupportedProvidersFollowADR0002 comprueba que el orden soportado es
// OpenAI → Claude → OpenRouter.
func TestSupportedProvidersFollowADR0002(t *testing.T) {
	got := SupportedProviders()
	want := []Provider{ProviderOpenAI, ProviderClaude, ProviderOpenRouter}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("SupportedProviders() = %v, want %v", got, want)
	}
}

// TestSupportedProvidersReturnsACopy comprueba que el caller no puede
// mutar el orden del paquete.
func TestSupportedProvidersReturnsACopy(t *testing.T) {
	got := SupportedProviders()
	got[0] = "mutado"

	if SupportedProviders()[0] != ProviderOpenAI {
		t.Fatal("SupportedProviders() devuelve la lista interna: hay que devolver una copia")
	}
}

// TestParseProviderAcceptsSupportedNames comprueba el parseo del nombre de
// proveedor de la configuración.
func TestParseProviderAcceptsSupportedNames(t *testing.T) {
	for _, provider := range SupportedProviders() {
		got, err := ParseProvider(string(provider))
		if err != nil {
			t.Fatalf("ParseProvider(%q) error inesperado: %v", provider, err)
		}
		if got != provider {
			t.Fatalf("ParseProvider(%q) = %q, want %q", provider, got, provider)
		}
	}
}

// TestParseProviderRejectsUnknownName comprueba que un proveedor desconocido
// es un error de configuración, no un failover.
func TestParseProviderRejectsUnknownName(t *testing.T) {
	for _, name := range []string{"", "OpenAI", "gemini", "openai "} {
		if _, err := ParseProvider(name); err == nil {
			t.Errorf("ParseProvider(%q): se esperaba error", name)
		}
	}
}

// TestNewAnalyzerReturnsExactlyTheConfiguredProvider comprueba que el
// registro devuelve el proveedor pedido y solo ese.
func TestNewAnalyzerReturnsExactlyTheConfiguredProvider(t *testing.T) {
	for _, provider := range SupportedProviders() {
		analyzer, err := NewAnalyzer(ProviderConfig{
			Provider: provider,
			Model:    "modelo-de-prueba",
			APIKey:   "clave-de-prueba",
		})
		if err != nil {
			t.Fatalf("NewAnalyzer(%s): %v", provider, err)
		}
		if analyzer.Provider() != provider {
			t.Errorf("NewAnalyzer(%s).Provider() = %s, want %s", provider, analyzer.Provider(), provider)
		}
	}
}

// TestNewAnalyzerRejectsIncompleteConfiguration comprueba que falta de
// modelo, de credencial o de proveedor es un error.
func TestNewAnalyzerRejectsIncompleteConfiguration(t *testing.T) {
	cases := []struct {
		name string
		cfg  ProviderConfig
	}{
		{"sin proveedor", ProviderConfig{Model: "m", APIKey: "k"}},
		{"proveedor desconocido", ProviderConfig{Provider: "gemini", Model: "m", APIKey: "k"}},
		{"sin modelo", ProviderConfig{Provider: ProviderOpenAI, APIKey: "k"}},
		{"sin clave", ProviderConfig{Provider: ProviderOpenAI, Model: "m"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := NewAnalyzer(tc.cfg); err == nil {
				t.Fatal("NewAnalyzer(): se esperaba error")
			}
		})
	}
}

// TestProviderEndpointsAreFixed comprueba que los endpoints de producción no
// se pueden cambiar por configuración.
func TestProviderEndpointsAreFixed(t *testing.T) {
	openai, err := NewOpenAI("m", "k", nil)
	if err != nil {
		t.Fatalf("NewOpenAI: %v", err)
	}
	if openai.endpoint != "https://api.openai.com/v1/chat/completions" {
		t.Errorf("endpoint de OpenAI = %q", openai.endpoint)
	}

	claude, err := NewClaude("m", "k", nil)
	if err != nil {
		t.Fatalf("NewClaude: %v", err)
	}
	if claude.endpoint != "https://api.anthropic.com/v1/messages" {
		t.Errorf("endpoint de Claude = %q", claude.endpoint)
	}

	openrouter, err := NewOpenRouter("m", "k", nil)
	if err != nil {
		t.Fatalf("NewOpenRouter: %v", err)
	}
	if openrouter.endpoint != "https://openrouter.ai/api/v1/chat/completions" {
		t.Errorf("endpoint de OpenRouter = %q", openrouter.endpoint)
	}
}

// ─────────────────────────────────────────────────────────────────────────
// Clasificación de fallos
// ─────────────────────────────────────────────────────────────────────────

// TestAnalyzeClassifiesHTTPFailures comprueba la clasificación por código
// HTTP: 429 y 5xx reintentables; el resto de 4xx permanentes.
func TestAnalyzeClassifiesHTTPFailures(t *testing.T) {
	cases := []struct {
		name       string
		status     int
		wantKind   ErrorKind
		wantReason Reason
	}{
		{"rate limit", http.StatusTooManyRequests, ErrorKindRetryable, ReasonRateLimited},
		{"error de servidor", http.StatusInternalServerError, ErrorKindRetryable, ReasonProviderError},
		{"gateway caido", http.StatusBadGateway, ErrorKindRetryable, ReasonProviderError},
		{"no autorizado", http.StatusUnauthorized, ErrorKindPermanent, ReasonUnauthorized},
		{"peticion invalida", http.StatusBadRequest, ErrorKindPermanent, ReasonBadRequest},
		{"modelo desconocido", http.StatusNotFound, ErrorKindPermanent, ReasonInvalidModel},
		{"prohibido", http.StatusForbidden, ErrorKindPermanent, ReasonForbidden},
	}

	for _, provider := range SupportedProviders() {
		for _, tc := range cases {
			t.Run(fmt.Sprintf("%s/%s", provider, tc.name), func(t *testing.T) {
				srv := jsonServer(t, tc.status, "application/json", []byte(`{"error":"detalle del proveedor"}`))
				analyzer := newTestAnalyzer(t, provider, "modelo", "clave", srv)

				_, err := analyzer.Analyze(context.Background(), testPayload(t))
				if err == nil {
					t.Fatal("Analyze(): se esperaba error")
				}

				var providerErr *Error
				if !errors.As(err, &providerErr) {
					t.Fatalf("Analyze() devolvió %T, se esperaba *ai.Error", err)
				}
				if providerErr.Kind != tc.wantKind {
					t.Errorf("Kind = %s, want %s", providerErr.Kind, tc.wantKind)
				}
				if providerErr.Reason != tc.wantReason {
					t.Errorf("Reason = %s, want %s", providerErr.Reason, tc.wantReason)
				}
				if providerErr.Status != tc.status {
					t.Errorf("Status = %d, want %d", providerErr.Status, tc.status)
				}
				if Retryable(err) != (tc.wantKind == ErrorKindRetryable) {
					t.Errorf("Retryable(err) = %v, want %v", Retryable(err), tc.wantKind == ErrorKindRetryable)
				}
			})
		}
	}
}

// TestAnalyzeClassifiesInvalidJSONAsPermanent comprueba que una respuesta
// que no es JSON es un fallo permanente.
func TestAnalyzeClassifiesInvalidJSONAsPermanent(t *testing.T) {
	for _, provider := range SupportedProviders() {
		t.Run(string(provider), func(t *testing.T) {
			srv := jsonServer(t, http.StatusOK, "application/json", []byte(`esto no es json`))
			analyzer := newTestAnalyzer(t, provider, "modelo", "clave", srv)

			_, err := analyzer.Analyze(context.Background(), testPayload(t))
			var providerErr *Error
			if !errors.As(err, &providerErr) {
				t.Fatalf("Analyze() devolvió %v (%T), se esperaba *ai.Error", err, err)
			}
			if providerErr.Kind != ErrorKindPermanent {
				t.Errorf("Kind = %s, want %s", providerErr.Kind, ErrorKindPermanent)
			}
			if providerErr.Reason != ReasonInvalidJSON {
				t.Errorf("Reason = %s, want %s", providerErr.Reason, ReasonInvalidJSON)
			}
		})
	}
}

// TestAnalyzeClassifiesSchemaViolationAsPermanent comprueba que una respuesta
// que no cumple el esquema compartido es un fallo permanente.
func TestAnalyzeClassifiesSchemaViolationAsPermanent(t *testing.T) {
	// "12" no se envió al proveedor: el validador compartido lo rechaza.
	invalid := `{"summary":"Has recorrido 12 m","observations":[],"cautions":[]}`

	for _, provider := range SupportedProviders() {
		t.Run(string(provider), func(t *testing.T) {
			srv := jsonServer(t, http.StatusOK, "application/json", providerReply(t, provider, invalid))
			analyzer := newTestAnalyzer(t, provider, "modelo", "clave", srv)

			_, err := analyzer.Analyze(context.Background(), testPayload(t))
			var providerErr *Error
			if !errors.As(err, &providerErr) {
				t.Fatalf("Analyze() devolvió %v (%T), se esperaba *ai.Error", err, err)
			}
			if providerErr.Kind != ErrorKindPermanent {
				t.Errorf("Kind = %s, want %s", providerErr.Kind, ErrorKindPermanent)
			}
			if providerErr.Reason != ReasonSchemaViolation {
				t.Errorf("Reason = %s, want %s", providerErr.Reason, ReasonSchemaViolation)
			}
		})
	}
}

// TestAnalyzeClassifiesNetworkFailureAsRetryable comprueba que un fallo de
// transporte es reintentable.
func TestAnalyzeClassifiesNetworkFailureAsRetryable(t *testing.T) {
	for _, provider := range SupportedProviders() {
		t.Run(string(provider), func(t *testing.T) {
			srv := jsonServer(t, http.StatusOK, "application/json", []byte(`{}`))
			analyzer := newTestAnalyzer(t, provider, "modelo", "clave", srv)
			srv.Close() // nadie escucha: la conexión falla

			_, err := analyzer.Analyze(context.Background(), testPayload(t))
			var providerErr *Error
			if !errors.As(err, &providerErr) {
				t.Fatalf("Analyze() devolvió %v (%T), se esperaba *ai.Error", err, err)
			}
			if providerErr.Kind != ErrorKindRetryable {
				t.Errorf("Kind = %s, want %s", providerErr.Kind, ErrorKindRetryable)
			}
			if providerErr.Reason != ReasonNetwork {
				t.Errorf("Reason = %s, want %s", providerErr.Reason, ReasonNetwork)
			}
		})
	}
}

// TestAnalyzeHonoursContextDeadline comprueba que el deadline del contexto
// se respeta y se clasifica como timeout reintentable.
func TestAnalyzeHonoursContextDeadline(t *testing.T) {
	for _, provider := range SupportedProviders() {
		t.Run(string(provider), func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				time.Sleep(300 * time.Millisecond)
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(`{}`))
			}))
			defer srv.Close()

			analyzer := newTestAnalyzer(t, provider, "modelo", "clave", srv)

			ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
			defer cancel()

			start := time.Now()
			_, err := analyzer.Analyze(ctx, testPayload(t))
			elapsed := time.Since(start)

			var providerErr *Error
			if !errors.As(err, &providerErr) {
				t.Fatalf("Analyze() devolvió %v (%T), se esperaba *ai.Error", err, err)
			}
			if providerErr.Kind != ErrorKindRetryable {
				t.Errorf("Kind = %s, want %s", providerErr.Kind, ErrorKindRetryable)
			}
			if providerErr.Reason != ReasonTimeout {
				t.Errorf("Reason = %s, want %s", providerErr.Reason, ReasonTimeout)
			}
			if elapsed > 250*time.Millisecond {
				t.Errorf("Analyze() tardó %s: no respetó el deadline", elapsed)
			}
		})
	}
}

// TestAnalyzeRejectsOversizedBody comprueba que una respuesta por encima del
// tope se descarta y es un fallo permanente.
func TestAnalyzeRejectsOversizedBody(t *testing.T) {
	for _, provider := range SupportedProviders() {
		t.Run(string(provider), func(t *testing.T) {
			huge := make([]byte, MaxResponseBytes+1)
			for i := range huge {
				huge[i] = 'a'
			}
			srv := jsonServer(t, http.StatusOK, "application/json", huge)
			analyzer := newTestAnalyzer(t, provider, "modelo", "clave", srv)

			_, err := analyzer.Analyze(context.Background(), testPayload(t))
			var providerErr *Error
			if !errors.As(err, &providerErr) {
				t.Fatalf("Analyze() devolvió %v (%T), se esperaba *ai.Error", err, err)
			}
			if providerErr.Kind != ErrorKindPermanent {
				t.Errorf("Kind = %s, want %s", providerErr.Kind, ErrorKindPermanent)
			}
			if providerErr.Reason != ReasonOversizedBody {
				t.Errorf("Reason = %s, want %s", providerErr.Reason, ReasonOversizedBody)
			}
		})
	}
}

// TestAnalyzeRejectsNonJSONResponse comprueba que solo se acepta JSON.
func TestAnalyzeRejectsNonJSONResponse(t *testing.T) {
	for _, provider := range SupportedProviders() {
		t.Run(string(provider), func(t *testing.T) {
			srv := jsonServer(t, http.StatusOK, "text/plain", []byte(canonicalAnalysis))
			analyzer := newTestAnalyzer(t, provider, "modelo", "clave", srv)

			_, err := analyzer.Analyze(context.Background(), testPayload(t))
			var providerErr *Error
			if !errors.As(err, &providerErr) {
				t.Fatalf("Analyze() devolvió %v (%T), se esperaba *ai.Error", err, err)
			}
			if providerErr.Kind != ErrorKindPermanent {
				t.Errorf("Kind = %s, want %s", providerErr.Kind, ErrorKindPermanent)
			}
			if providerErr.Reason != ReasonNonJSONResponse {
				t.Errorf("Reason = %s, want %s", providerErr.Reason, ReasonNonJSONResponse)
			}
		})
	}
}

// TestAnalyzeDoesNotFollowRedirectsNorLeakCredentials comprueba que una
// redirección no se sigue: la credencial nunca viaja a otro host.
func TestAnalyzeDoesNotFollowRedirectsNorLeakCredentials(t *testing.T) {
	const key = "clave-secreta-de-prueba"

	for _, provider := range SupportedProviders() {
		t.Run(string(provider), func(t *testing.T) {
			var targetHits int32
			target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				atomic.AddInt32(&targetHits, 1)
				if got := r.Header.Get("Authorization"); got != "" {
					t.Errorf("la redirección recibió la credencial: %q", got)
				}
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write(providerReply(t, provider, canonicalAnalysis))
			}))
			defer target.Close()

			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				http.Redirect(w, r, target.URL+"/v1/chat/completions", http.StatusTemporaryRedirect)
			}))
			defer srv.Close()

			analyzer := newTestAnalyzer(t, provider, "modelo", key, srv)

			_, err := analyzer.Analyze(context.Background(), testPayload(t))
			var providerErr *Error
			if !errors.As(err, &providerErr) {
				t.Fatalf("Analyze() devolvió %v (%T), se esperaba *ai.Error", err, err)
			}
			if providerErr.Kind != ErrorKindPermanent {
				t.Errorf("Kind = %s, want %s", providerErr.Kind, ErrorKindPermanent)
			}
			if providerErr.Reason != ReasonRedirect {
				t.Errorf("Reason = %s, want %s", providerErr.Reason, ReasonRedirect)
			}
			if hits := atomic.LoadInt32(&targetHits); hits != 0 {
				t.Errorf("el host de destino recibió %d peticiones: no se deben seguir redirecciones", hits)
			}
			if strings.Contains(err.Error(), key) {
				t.Errorf("el mensaje de error filtra la credencial: %q", err.Error())
			}
		})
	}
}

// TestAnalyzePerformsExactlyOneHTTPAttempt comprueba que no hay reintentos
// internos: la decisión de reintentar es del job.
func TestAnalyzePerformsExactlyOneHTTPAttempt(t *testing.T) {
	for _, provider := range SupportedProviders() {
		t.Run(string(provider), func(t *testing.T) {
			var hits int32
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				atomic.AddInt32(&hits, 1)
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusInternalServerError)
			}))
			defer srv.Close()

			analyzer := newTestAnalyzer(t, provider, "modelo", "clave", srv)

			if _, err := analyzer.Analyze(context.Background(), testPayload(t)); err == nil {
				t.Fatal("Analyze(): se esperaba error")
			}
			if got := atomic.LoadInt32(&hits); got != 1 {
				t.Errorf("el proveedor recibió %d peticiones, want 1", got)
			}
		})
	}
}

// TestAnalyzeDoesNotFailOverToAnotherProvider comprueba que un fallo de un
// proveedor no salta a otro: la decisión es del job, no del adaptador.
func TestAnalyzeDoesNotFailOverToAnotherProvider(t *testing.T) {
	var openAIHits, openRouterHits int32

	openAISrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&openAIHits, 1)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer openAISrv.Close()

	openRouterSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&openRouterHits, 1)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(providerReply(t, ProviderOpenRouter, canonicalAnalysis))
	}))
	defer openRouterSrv.Close()

	analyzer := newTestAnalyzer(t, ProviderOpenAI, "modelo", "clave", openAISrv)

	if _, err := analyzer.Analyze(context.Background(), testPayload(t)); err == nil {
		t.Fatal("Analyze(): se esperaba error")
	}
	if got := atomic.LoadInt32(&openAIHits); got != 1 {
		t.Errorf("OpenAI recibió %d peticiones, want 1", got)
	}
	if got := atomic.LoadInt32(&openRouterHits); got != 0 {
		t.Errorf("OpenRouter recibió %d peticiones: no hay failover implícito", got)
	}
}

// ─────────────────────────────────────────────────────────────────────────
// Salida
// ─────────────────────────────────────────────────────────────────────────

// TestInstructionAvoidsVetoedTerms comprueba que la instrucción no usa
// términos vetados por el validador de salida: si el modelo copiase la
// instrucción, la respuesta se descartaría sola.
func TestInstructionAvoidsVetoedTerms(t *testing.T) {
	lower := strings.ToLower(analysisInstruction)
	for _, term := range diagnosisTerms {
		if strings.Contains(lower, term) {
			t.Errorf("la instrucción usa el término vetado %q", term)
		}
	}
}

// TestAnalyzeReturnsIdenticalOutputAcrossProviders comprueba que la salida
// validada no depende del proveedor: la UI no puede distinguir entre ellos.
func TestAnalyzeReturnsIdenticalOutputAcrossProviders(t *testing.T) {
	payload := testPayload(t)

	outputs := make(map[Provider]Output)
	for _, provider := range SupportedProviders() {
		srv := jsonServer(t, http.StatusOK, "application/json", providerReply(t, provider, canonicalAnalysis))
		analyzer := newTestAnalyzer(t, provider, "modelo", "clave", srv)

		out, err := analyzer.Analyze(context.Background(), payload)
		if err != nil {
			t.Fatalf("Analyze(%s): %v", provider, err)
		}
		outputs[provider] = out
	}

	reference := outputs[ProviderOpenAI]
	if reference.Summary == "" {
		t.Fatal("la salida de referencia está vacía")
	}
	for _, provider := range SupportedProviders() {
		if !reflect.DeepEqual(outputs[provider], reference) {
			t.Errorf("la salida de %s difiere de la de OpenAI:\n got %+v\nwant %+v",
				provider, outputs[provider], reference)
		}
	}
}

// TestAnalyzeSendsOnlyAllowlistedAggregates comprueba que en el mensaje al
// proveedor solo viajan los agregados de la lista blanca.
func TestAnalyzeSendsOnlyAllowlistedAggregates(t *testing.T) {
	allowed := map[string]bool{
		"kind": true, "window_days": true, "metrics": true,
		"distance_meters": true, "duration_seconds": true,
		"elevation_gain_meters": true, "avg_heart_rate": true, "training_load": true,
	}

	for _, provider := range SupportedProviders() {
		t.Run(string(provider), func(t *testing.T) {
			var body []byte
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				body, _ = io.ReadAll(r.Body)
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write(providerReply(t, provider, canonicalAnalysis))
			}))
			defer srv.Close()

			analyzer := newTestAnalyzer(t, provider, "modelo", "clave", srv)
			if _, err := analyzer.Analyze(context.Background(), testPayload(t)); err != nil {
				t.Fatalf("Analyze(): %v", err)
			}

			content := messageContent(t, provider, body)
			payload := extractPayloadFromContent(t, content)

			for key := range payload {
				if !allowed[key] {
					t.Errorf("el payload que viaja lleva el campo prohibido %q", key)
				}
			}
			metrics, ok := payload["metrics"].(map[string]any)
			if !ok {
				t.Fatalf("el payload enviado no tiene métricas: %v", payload)
			}
			for key := range metrics {
				if !allowed[key] {
					t.Errorf("las métricas que viajan llevan el campo prohibido %q", key)
				}
			}

			// Ningún dato de identidad o de GPS puede viajar en el cuerpo.
			forbiddenTokens := []string{
				"user_id", "email", "username", "latitude", "longitude",
				"streams", "birthday", "weight", "athlete",
			}
			for _, token := range forbiddenTokens {
				if strings.Contains(string(body), token) {
					t.Errorf("el cuerpo de la petición contiene el token prohibido %q", token)
				}
			}
		})
	}
}

// TestErrorMessageLeaksNothing comprueba que el mensaje de error no lleva el
// payload, la credencial ni el cuerpo devuelto por el proveedor.
func TestErrorMessageLeaksNothing(t *testing.T) {
	const key = "clave-secreta-de-prueba"

	// Números que no chocan con ningún código HTTP, para que el mensaje
	// saneado no los contenga por casualidad.
	payload, err := NewBuilder(KindActivity).
		WindowDays(9).
		DistanceMeters(98765).
		DurationSeconds(87654).
		ElevationGainMeters(321).
		AvgHeartRate(111).
		TrainingLoad(222).
		Build()
	if err != nil {
		t.Fatalf("Build() inesperado: %v", err)
	}

	for _, provider := range SupportedProviders() {
		t.Run(string(provider), func(t *testing.T) {
			srv := jsonServer(t, http.StatusInternalServerError, "application/json",
				[]byte(`{"error":"detalle interno del proveedor"}`))
			analyzer := newTestAnalyzer(t, provider, "modelo", key, srv)

			_, err := analyzer.Analyze(context.Background(), payload)
			if err == nil {
				t.Fatal("Analyze(): se esperaba error")
			}

			message := err.Error()
			forbidden := []string{
				key,
				"detalle interno del proveedor",
				"98765",
				"87654",
				"321",
				"111",
				"222",
			}
			for _, secret := range forbidden {
				if strings.Contains(message, secret) {
					t.Errorf("el mensaje de error filtra %q: %q", secret, message)
				}
			}
		})
	}
}
