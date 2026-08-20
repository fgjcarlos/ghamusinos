// Tests del middleware de seguridad (issue #26).
package http

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestSecurityHeaders_AplicaTodasLasCabeceras verifica que el middleware
// aplica el set defensivo por defecto (X-Content-Type-Options,
// X-Frame-Options, Referrer-Policy, Content-Security-Policy,
// Permissions-Policy). HSTS queda apagado por defecto.
func TestSecurityHeaders_AplicaTodasLasCabeceras(t *testing.T) {
	handler := SecurityHeaders(SecurityHeadersConfig{})(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req, err := http.NewRequestWithContext(t.Context(), "GET", "/probe", nil)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	rec := &httptest.ResponseRecorder{}
	handler.ServeHTTP(rec, req)

	want := map[string]string{
		"X-Content-Type-Options":    "nosniff",
		"X-Frame-Options":           "DENY",
		"Referrer-Policy":           "strict-origin-when-cross-origin",
		"Content-Security-Policy":   "default-src 'self'; script-src 'self'; style-src 'self'; img-src 'self' data:; font-src 'self'; connect-src 'self'; object-src 'none'; base-uri 'self'; frame-ancestors 'none'",
		"Permissions-Policy":        "camera=(), geolocation=(), microphone=(), payment=()",
		"Strict-Transport-Security": "", // apagado por defecto
	}
	for k, v := range want {
		got := rec.Header().Get(k)
		if got != v {
			t.Errorf("header %q = %q, quería %q", k, got, v)
		}
	}
}

// TestSecurityHeaders_HSTSActivable verifica que SECURITY_HSTS_ENABLED=true
// (config.Security.HSTSEnabled) activa la cabecera Strict-Transport-Security.
// Como nuevoServidor usa config por defecto, este test monta su propio router.
func TestSecurityHeaders_HSTSActivable(t *testing.T) {
	// Sin acceso directo a chi en el test público, usamos el helper
	// de Server montando un handler con la cabecera esperada.
	handler := SecurityHeaders(SecurityHeadersConfig{HSTSEnabled: true})(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req, err := http.NewRequestWithContext(t.Context(), "GET", "/probe", nil)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	rec := &httptest.ResponseRecorder{}
	handler.ServeHTTP(rec, req)

	if got := rec.Header().Get("Strict-Transport-Security"); !strings.Contains(got, "max-age=") {
		t.Errorf("Strict-Transport-Security = %q, quería que contenga max-age=", got)
	}
}

// TestSecurityHeaders_CSPReportOnly verifica el modo report-only.
func TestSecurityHeaders_CSPReportOnly(t *testing.T) {
	handler := SecurityHeaders(SecurityHeadersConfig{CSPReportOnly: true})(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req, err := http.NewRequestWithContext(t.Context(), "GET", "/probe", nil)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	rec := &httptest.ResponseRecorder{}
	handler.ServeHTTP(rec, req)

	if got := rec.Header().Get("Content-Security-Policy"); got != "" {
		t.Errorf("CSP no debe estar en modo enforce cuando report-only=true; got %q", got)
	}
	if got := rec.Header().Get("Content-Security-Policy-Report-Only"); got == "" {
		t.Errorf("Content-Security-Policy-Report-Only debería estar set")
	}
}

// TestBodyLimit_PermiteBodyMenor verifica que un body menor que el límite
// pasa a través del middleware sin ser tocado.
func TestBodyLimit_PermiteBodyMenor(t *testing.T) {
	handler := BodyLimit(1024)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = r.Body.Read(make([]byte, 512))
		w.WriteHeader(http.StatusOK)
	}))

	req, err := http.NewRequestWithContext(t.Context(), "POST", "/probe", strings.NewReader(strings.Repeat("x", 512)))
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	rec := &httptest.ResponseRecorder{}
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, quería %d", rec.Code, http.StatusOK)
	}
}

// TestBodyLimit_BloqueaBodyExcedido verifica que http.MaxBytesHandler
// aborta reads que exceden el límite: el handler verá *MaxBytesError al
// leer. Esto valida que el middleware aplica el límite; el handler
// decide cómo responder (en el flujo real, el handler usaría
// handlers.IsBodyLimitError para responder 413).
func TestBodyLimit_BloqueaBodyExcedido(t *testing.T) {
	handler := BodyLimit(10)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		buf := make([]byte, 100)
		_, err := r.Body.Read(buf)
		if err == nil {
			t.Errorf("Read no devolvió error; el body debería haber excedido el límite")
		}
		if !strings.Contains(err.Error(), "http: request body too large") {
			t.Errorf("error = %v, quería que mencione 'http: request body too large'", err)
		}
		w.WriteHeader(http.StatusOK)
	}))

	req, err := http.NewRequestWithContext(t.Context(), "POST", "/probe", strings.NewReader(strings.Repeat("x", 100)))
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	rec := &httptest.ResponseRecorder{}
	handler.ServeHTTP(rec, req)
}
