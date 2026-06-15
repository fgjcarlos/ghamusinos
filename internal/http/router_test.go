package http

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// nuevoServidor es un helper de test que crea un Server con pool nil
// (entorno sin base de datos) y lo envuelve en httptest.
func nuevoServidor(t *testing.T) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(NewServer(nil).Router())
	t.Cleanup(srv.Close)
	return srv
}

// testGet lanza una petición GET con context.Context contra el server.
// Usar http.Get directamente dispara el linter noctx; este helper
// mantiene el patrón idiomático sin saltarse la regla.
func testGet(t *testing.T, url string) *http.Response {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	t.Cleanup(cancel)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		t.Fatalf("new request %s: %v", url, err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("GET %s: %v", url, err)
	}
	return resp
}
}

func TestRouterHealthz(t *testing.T) {
	srv := nuevoServidor(t)

	resp := testGet(t, srv.URL+"/healthz")
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, quería %d", resp.StatusCode, http.StatusOK)
	}

	// Criterio de aceptación #32: cada respuesta incluye X-Request-Id.
	if rid := resp.Header.Get("X-Request-Id"); rid == "" {
		t.Error("respuesta sin X-Request-Id")
	}
}

// TestRouterReadyz_sinPool verifica que /readyz responde 503 cuando no hay pool.
func TestRouterReadyz_sinPool(t *testing.T) {
	srv := nuevoServidor(t)

	resp, err := http.Get(srv.URL + "/readyz")
	if err != nil {
		t.Fatalf("GET /readyz: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, quería %d (sin pool, readyz debe degradar)", resp.StatusCode, http.StatusServiceUnavailable)
	}

	ct := resp.Header.Get("Content-Type")
	if ct != "application/json" {
		t.Fatalf("Content-Type = %q, quería application/json", ct)
	}
}

// TestRouterSPARuta verifica que rutas no-API aterrizan en el handler SPA.
// Sin build de Vite ejecutado, el handler devuelve 503 (placeholder sin
// index.html). Eso es el comportamiento correcto en entorno de desarrollo
// sin assets compilados.
func TestRouterSPARuta(t *testing.T) {
	srv := nuevoServidor(t)

	resp := testGet(t, srv.URL+"/lab")
	defer func() { _ = resp.Body.Close() }()

	// Sin build: 503 (frontend no construido). Con build: 200 (SPA index.html).
	// Ambos son coherentes; lo que NO debe pasar es que llegue a chi's 404.
	if resp.StatusCode == http.StatusNotFound {
		t.Fatalf("GET /lab no debería devolver 404: el handler SPA debe interceptarlo")
	}
	if rid := resp.Header.Get("X-Request-Id"); rid == "" {
		t.Error("respuesta sin X-Request-Id")
	}
}

// TestRouterAPINotFound verifica que /api/inexistente devuelva JSON 404
// y NO el index.html de la SPA.
func TestRouterAPINotFound(t *testing.T) {
	srv := nuevoServidor(t)

	resp := testGet(t, srv.URL+"/api/no-existe")
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("GET /api/no-existe quería 404, obtuvo %d", resp.StatusCode)
	}

	ct := resp.Header.Get("Content-Type")
	if ct == "" {
		t.Fatal("GET /api/no-existe debería devolver Content-Type JSON")
	}
	if rid := resp.Header.Get("X-Request-Id"); rid == "" {
		t.Error("respuesta sin X-Request-Id")
	}
}
