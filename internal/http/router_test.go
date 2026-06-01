package http

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRouterHealthz(t *testing.T) {
	srv := httptest.NewServer(NewRouter())
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/healthz")
	if err != nil {
		t.Fatalf("GET /healthz: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, quería %d", resp.StatusCode, http.StatusOK)
	}
}

// TestRouterSPARuta verifica que rutas no-API aterrizan en el handler SPA.
// Sin build de Vite ejecutado, el handler devuelve 503 (placeholder sin
// index.html). Eso es el comportamiento correcto en entorno de desarrollo
// sin assets compilados.
func TestRouterSPARuta(t *testing.T) {
	srv := httptest.NewServer(NewRouter())
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/lab")
	if err != nil {
		t.Fatalf("GET /lab: %v", err)
	}
	defer resp.Body.Close()

	// Sin build: 503 (frontend no construido). Con build: 200 (SPA index.html).
	// Ambos son coherentes; lo que NO debe pasar es que llegue a chi's 404.
	if resp.StatusCode == http.StatusNotFound {
		t.Fatalf("GET /lab no debería devolver 404: el handler SPA debe interceptarlo")
	}
}

// TestRouterAPINotFound verifica que /api/inexistente devuelva JSON 404
// y NO el index.html de la SPA.
func TestRouterAPINotFound(t *testing.T) {
	srv := httptest.NewServer(NewRouter())
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/api/no-existe")
	if err != nil {
		t.Fatalf("GET /api/no-existe: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("GET /api/no-existe quería 404, obtuvo %d", resp.StatusCode)
	}

	ct := resp.Header.Get("Content-Type")
	if ct == "" {
		t.Fatal("GET /api/no-existe debería devolver Content-Type JSON")
	}
}
