// Capa de proveedores de IA: interfaz común, registro de proveedores y
// clasificación de fallos.
//
// Este archivo NO conoce el dominio: solo habla con proveedores HTTP a través
// de la interfaz Analyzer. El dominio (jobs, API) solo ve Analyzer, Output y
// el error clasificado, así que añadir un proveedor es añadir un archivo.
//
// Reglas transversales que aplican a todos los adaptadores:
//
//   - Una llamada de Analyze = UN intento HTTP. No hay reintentos internos:
//     quien reintenta es el job, y decide con Error.Kind.
//   - No hay failover implícito: un fallo de un proveedor nunca salta a otro
//     proveedor por su cuenta (ADR 0002, fase 1.5 · S7).
//   - La credencial viaja solo en la cabecera de autenticación estándar del
//     proveedor, y las redirecciones no se siguen: la credencial nunca sale
//     del host del proveedor.
//   - El cuerpo de la respuesta está acotado y tiene que ser JSON.
//   - Los mensajes de error no contienen ni el payload, ni la credencial, ni
//     el cuerpo devuelto por el proveedor.

package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net"
	"net/http"
	"strings"
	"time"
)

// Provider identifica un adaptador de proveedor de IA.
type Provider string

// Proveedores soportados, en el orden que fija ADR 0002.
const (
	// ProviderOpenAI es el API de chat completions de OpenAI.
	ProviderOpenAI Provider = "openai"
	// ProviderClaude es el API de messages de Anthropic (Claude).
	ProviderClaude Provider = "claude"
	// ProviderOpenRouter es el API de chat completions de OpenRouter.
	ProviderOpenRouter Provider = "openrouter"
)

// providerOrder es el orden de soporte (ADR 0002): OpenAI, Claude, OpenRouter.
var providerOrder = []Provider{ProviderOpenAI, ProviderClaude, ProviderOpenRouter}

// Límites de la capa de proveedores.
const (
	// MaxResponseBytes acota el cuerpo de la respuesta que se lee del
	// proveedor (1 MiB). Una respuesta mayor se descarta sin leerla entera.
	MaxResponseBytes = 1 << 20
	// DefaultHTTPTimeout es el timeout por defecto del cliente HTTP cuando
	// el caller no inyecta uno.
	DefaultHTTPTimeout = 60 * time.Second
)

// SupportedProviders devuelve la lista ordenada de proveedores soportados.
//
// El orden es el de ADR 0002 (OpenAI → Claude → OpenRouter). La lista es una
// copia: el caller puede modificarla sin afectar al paquete.
func SupportedProviders() []Provider {
	out := make([]Provider, len(providerOrder))
	copy(out, providerOrder)
	return out
}

// ParseProvider convierte el nombre de proveedor de la configuración en un
// Provider. Devuelve error si el nombre no es uno de los soportados.
func ParseProvider(name string) (Provider, error) {
	for _, provider := range providerOrder {
		if string(provider) == name {
			return provider, nil
		}
	}
	return "", fmt.Errorf("ai: unsupported provider %q (supported: %s)", name, strings.Join(providerNames(), ", "))
}

// providerNames devuelve los nombres de los proveedores soportados, en orden.
func providerNames() []string {
	names := make([]string, 0, len(providerOrder))
	for _, provider := range providerOrder {
		names = append(names, string(provider))
	}
	return names
}

// Analyzer analiza un payload con un único proveedor.
//
// La interfaz no expone nada del proveedor concreto: el dominio puede pedir
// un análisis y recibir siempre el mismo Output validado.
type Analyzer interface {
	// Provider devuelve el nombre del proveedor que implementa el analizador.
	Provider() Provider
	// Analyze envía el payload al proveedor y devuelve la salida validada.
	//
	// Hace un único intento HTTP. Si el proveedor falla, devuelve un *Error
	// que dice si el fallo es reintentable. No cambia de proveedor nunca.
	Analyze(ctx context.Context, payload Payload) (Output, error)
}

// ProviderConfig es la configuración que necesita un adaptador de proveedor.
type ProviderConfig struct {
	// Provider es el proveedor configurado. Vacío o desconocido es error.
	Provider Provider
	// Model es el modelo concreto. Es obligatorio: no hay modelo por
	// defecto, porque un nombre inventado quemaría créditos y escondería el
	// error de configuración detrás de un 404 del proveedor.
	Model string
	// APIKey es la credencial del proveedor. Es obligatoria y solo viaja en
	// la cabecera de autenticación estándar.
	APIKey string
	// HTTPClient es opcional; los tests inyectan el suyo para apuntar a un
	// servidor httptest. En producción es nil y se usa el cliente por
	// defecto con timeout.
	HTTPClient *http.Client
}

// NewAnalyzer devuelve el adaptador del proveedor configurado.
//
// No hay failover: el adaptador devuelto es exactamente el proveedor pedido,
// o un error si la configuración no sirve.
func NewAnalyzer(cfg ProviderConfig) (Analyzer, error) {
	switch cfg.Provider {
	case ProviderOpenAI:
		return NewOpenAI(cfg.Model, cfg.APIKey, cfg.HTTPClient)
	case ProviderClaude:
		return NewClaude(cfg.Model, cfg.APIKey, cfg.HTTPClient)
	case ProviderOpenRouter:
		return NewOpenRouter(cfg.Model, cfg.APIKey, cfg.HTTPClient)
	default:
		return nil, fmt.Errorf("ai: unsupported provider %q (supported: %s)", cfg.Provider, strings.Join(providerNames(), ", "))
	}
}

// ErrorKind clasifica un fallo de proveedor para que el caller decida si
// reintenta.
type ErrorKind string

const (
	// ErrorKindRetryable marca un fallo que puede resolverse solo:
	// 429, 5xx, error de red o timeout.
	ErrorKindRetryable ErrorKind = "retryable"
	// ErrorKindPermanent marca un fallo que no se arregla reintentando:
	// 4xx distintos de 429, modelo inválido, rechazo del modelo o respuesta
	// que no cumple el esquema.
	ErrorKindPermanent ErrorKind = "permanent"
)

// Reason explica por qué falló la llamada, sin filtrar contenido.
type Reason string

const (
	// ReasonRateLimited: el proveedor devolvió 429.
	ReasonRateLimited Reason = "rate_limited"
	// ReasonProviderError: el proveedor devolvió 5xx.
	ReasonProviderError Reason = "provider_error"
	// ReasonNetwork: fallo de transporte (DNS, conexión, TLS).
	ReasonNetwork Reason = "network"
	// ReasonTimeout: se agotó el deadline del contexto o el timeout del cliente.
	ReasonTimeout Reason = "timeout"
	// ReasonCanceled: quien llamó canceló el contexto.
	ReasonCanceled Reason = "canceled"
	// ReasonBadRequest: el proveedor rechazó la petición (4xx).
	ReasonBadRequest Reason = "bad_request"
	// ReasonUnauthorized: credencial rechazada (401).
	ReasonUnauthorized Reason = "unauthorized"
	// ReasonForbidden: permiso denegado (403).
	ReasonForbidden Reason = "forbidden"
	// ReasonInvalidModel: el proveedor no conoce el endpoint o el modelo (404).
	ReasonInvalidModel Reason = "invalid_model"
	// ReasonRefusal: el modelo se negó a responder.
	ReasonRefusal Reason = "refusal"
	// ReasonTruncated: la respuesta se cortó por límite de tokens.
	ReasonTruncated Reason = "truncated"
	// ReasonInvalidJSON: la respuesta no es JSON válido.
	ReasonInvalidJSON Reason = "invalid_json"
	// ReasonNonJSONResponse: la respuesta no declara content-type JSON.
	ReasonNonJSONResponse Reason = "non_json_response"
	// ReasonOversizedBody: la respuesta excede MaxResponseBytes.
	ReasonOversizedBody Reason = "oversized_body"
	// ReasonRedirect: el proveedor respondió con una redirección.
	ReasonRedirect Reason = "redirect"
	// ReasonUnexpected: la respuesta no tiene la forma que el proveedor
	// debería devolver.
	ReasonUnexpected Reason = "unexpected_response"
	// ReasonSchemaViolation: la respuesta es JSON válido pero no cumple el
	// esquema de salida compartido.
	ReasonSchemaViolation Reason = "schema_violation"
)

// Error es el error que devuelven todos los adaptadores.
//
// El mensaje es deliberadamente pobre: no contiene el payload enviado, ni la
// credencial, ni el cuerpo devuelto por el proveedor. La causa original
// queda en Err para inspección programática (errors.Is / errors.As), pero no
// se imprime.
type Error struct {
	// Provider es el proveedor que falló.
	Provider Provider
	// Kind dice si el fallo es reintentable.
	Kind ErrorKind
	// Reason explica el fallo.
	Reason Reason
	// Status es el código HTTP, o 0 si no hubo respuesta.
	Status int
	// Err es la causa original, si la hubo. Puede contener detalle técnico;
	// no se incluye en el mensaje del error.
	Err error
}

// Error implementa error con un mensaje sanitizado.
func (e *Error) Error() string {
	if e.Status > 0 {
		return fmt.Sprintf("ai: %s: %s (http %d)", e.Provider, e.Reason, e.Status)
	}
	return fmt.Sprintf("ai: %s: %s", e.Provider, e.Reason)
}

// Unwrap expone la causa original.
func (e *Error) Unwrap() error { return e.Err }

// Retryable dice si el fallo se puede reintentar.
func (e *Error) Retryable() bool { return e.Kind == ErrorKindRetryable }

// Retryable dice si err es un fallo de proveedor que se puede reintentar.
//
// El job lo usa para decidir el backoff: si err no es un *Error de este
// paquete, no se reintenta (es un error de programación, no del proveedor).
func Retryable(err error) bool {
	var providerErr *Error
	if errors.As(err, &providerErr) {
		return providerErr.Retryable()
	}
	return false
}

// newProviderError construye un error de proveedor sanitizado.
func newProviderError(provider Provider, kind ErrorKind, reason Reason, status int, cause error) *Error {
	return &Error{
		Provider: provider,
		Kind:     kind,
		Reason:   reason,
		Status:   status,
		Err:      cause,
	}
}

// providerClient devuelve el cliente con el que se habla con el proveedor.
//
// Las redirecciones no se siguen nunca: así la credencial no puede viajar a
// otro host. Un 3xx se convierte en un error permanente.
func providerClient(injected *http.Client) *http.Client {
	base := injected
	if base == nil {
		base = &http.Client{Timeout: DefaultHTTPTimeout}
	}
	client := *base
	client.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		return http.ErrUseLastResponse
	}
	return &client
}

// analysisInstruction es la instrucción que reciben todos los proveedores.
//
// No lleva datos del usuario: solo explica qué hacer. Los únicos datos que
// viajan son los agregados numéricos del payload.
//
// Ojo con el vocabulario: la respuesta se valida contra una lista de términos
// vetados (diagnósticos, enfermedades…), así que la instrucción no los usa
// para que el modelo no pueda citarlos y que su copia se descarte.
const analysisInstruction = "Eres un analista de entrenamiento. Recibes agregados numéricos " +
	"de una sesión o de un periodo y devuelves una interpretación.\n" +
	"Reglas estrictas:\n" +
	"- Usa solo los números que aparecen en los agregados: no calcules porcentajes ni inventes métricas.\n" +
	"- No emitas afirmaciones médicas.\n" +
	"- No menciones datos que no estén en los agregados.\n" +
	"- Responde en español."

// payloadMessage construye el mensaje con los agregados, sin instrucción.
func payloadMessage(payload Payload) (string, error) {
	raw, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("ai: cannot serialize payload: %w", err)
	}
	return "Agregados:\n" + string(raw), nil
}

// userMessage construye el mensaje de usuario: la instrucción más el JSON
// del payload, que ya va por una lista blanca (payloadJSON).
func userMessage(payload Payload) (string, error) {
	aggregates, err := payloadMessage(payload)
	if err != nil {
		return "", err
	}
	return analysisInstruction + "\n\n" + aggregates, nil
}

// newProviderRequest construye la petición HTTP de un análisis.
//
// Se fija Content-Type y Accept a JSON: solo se habla JSON con el proveedor.
func newProviderRequest(ctx context.Context, provider Provider, endpoint string, body []byte, headers map[string]string) (*http.Request, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("ai: %s: cannot build request: %w", provider, err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	for name, value := range headers {
		req.Header.Set(name, value)
	}
	return req, nil
}

// performRequest hace UN intento HTTP y devuelve el cuerpo de la respuesta.
//
// Devuelve un *Error ya clasificado en cuanto la respuesta no es usable.
func performRequest(provider Provider, client *http.Client, req *http.Request) ([]byte, error) {
	resp, err := client.Do(req)
	if err != nil {
		return nil, transportError(provider, err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := readCappedBody(resp.Body)
	if err != nil {
		switch {
		case errors.Is(err, errOversizedBody):
			return nil, newProviderError(provider, ErrorKindPermanent, ReasonOversizedBody, resp.StatusCode, err)
		case errors.Is(err, context.DeadlineExceeded):
			return nil, newProviderError(provider, ErrorKindRetryable, ReasonTimeout, resp.StatusCode, err)
		default:
			return nil, newProviderError(provider, ErrorKindRetryable, ReasonNetwork, resp.StatusCode, err)
		}
	}

	switch {
	case resp.StatusCode >= 300 && resp.StatusCode < 400:
		// No seguimos redirecciones: la credencial no sale del host.
		return nil, newProviderError(provider, ErrorKindPermanent, ReasonRedirect, resp.StatusCode, nil)
	case resp.StatusCode == http.StatusTooManyRequests:
		return nil, newProviderError(provider, ErrorKindRetryable, ReasonRateLimited, resp.StatusCode, nil)
	case resp.StatusCode >= 500:
		return nil, newProviderError(provider, ErrorKindRetryable, ReasonProviderError, resp.StatusCode, nil)
	case resp.StatusCode >= 400:
		return nil, newProviderError(provider, ErrorKindPermanent, reasonForStatus(resp.StatusCode), resp.StatusCode, nil)
	}

	if !isJSONContentType(resp.Header.Get("Content-Type")) {
		return nil, newProviderError(provider, ErrorKindPermanent, ReasonNonJSONResponse, resp.StatusCode, nil)
	}
	return body, nil
}

// reasonForStatus mapea un 4xx a su motivo, sin mirar el cuerpo.
func reasonForStatus(status int) Reason {
	switch status {
	case http.StatusUnauthorized:
		return ReasonUnauthorized
	case http.StatusForbidden:
		return ReasonForbidden
	case http.StatusNotFound:
		return ReasonInvalidModel
	default:
		return ReasonBadRequest
	}
}

// readCappedBody lee el cuerpo con el tope de MaxResponseBytes.
func readCappedBody(body io.Reader) ([]byte, error) {
	limited := io.LimitReader(body, MaxResponseBytes+1)
	raw, err := io.ReadAll(limited)
	if err != nil {
		return nil, err
	}
	if len(raw) > MaxResponseBytes {
		return nil, errOversizedBody
	}
	return raw, nil
}

// errOversizedBody marca una respuesta que excede el tope.
var errOversizedBody = errors.New("ai: response body too large")

// transportError clasifica un fallo de transporte.
func transportError(provider Provider, err error) error {
	switch {
	case errors.Is(err, context.DeadlineExceeded):
		return newProviderError(provider, ErrorKindRetryable, ReasonTimeout, 0, err)
	case errors.Is(err, context.Canceled):
		return newProviderError(provider, ErrorKindPermanent, ReasonCanceled, 0, err)
	}
	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return newProviderError(provider, ErrorKindRetryable, ReasonTimeout, 0, err)
	}
	return newProviderError(provider, ErrorKindRetryable, ReasonNetwork, 0, err)
}

// isJSONContentType dice si la cabecera declara JSON.
func isJSONContentType(header string) bool {
	if header == "" {
		return false
	}
	mediaType, _, err := mime.ParseMediaType(header)
	if err != nil {
		return false
	}
	return mediaType == "application/json"
}
