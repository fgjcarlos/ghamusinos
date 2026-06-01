package frontend

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"testing/fstest"
)

// fsConBuild simula un FS con build real (index.html + un asset).
var fsConBuild = fstest.MapFS{
	"index.html": {
		Data: []byte(`<!DOCTYPE html><html><body>Ghamusinos App</body></html>`),
	},
	"assets/app.js": {
		Data: []byte(`console.log("ghamusinos")`),
	},
}

// fsSinBuild simula un FS placeholder (sin build ejecutado).
var fsSinBuild = fstest.MapFS{
	".gitkeep": {Data: []byte{}},
}

func TestSPAHandler_IndexEnRaiz(t *testing.T) {
	h := NewSPAHandler(fsConBuild)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("GET / quería 200, obtuvo %d", rec.Code)
	}
	body, _ := io.ReadAll(rec.Body)
	if len(body) == 0 {
		t.Fatal("GET / devolvió cuerpo vacío")
	}
}

func TestSPAHandler_FallbackRutaCliente(t *testing.T) {
	h := NewSPAHandler(fsConBuild)
	req := httptest.NewRequest(http.MethodGet, "/lab/entrenos/2024", nil)
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("GET /lab/entrenos/2024 quería 200 (fallback index.html), obtuvo %d", rec.Code)
	}
	body, _ := io.ReadAll(rec.Body)
	if len(body) == 0 {
		t.Fatal("fallback devolvió cuerpo vacío")
	}
}

func TestSPAHandler_AssetExistente(t *testing.T) {
	h := NewSPAHandler(fsConBuild)
	req := httptest.NewRequest(http.MethodGet, "/assets/app.js", nil)
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("GET /assets/app.js quería 200, obtuvo %d", rec.Code)
	}
	body, _ := io.ReadAll(rec.Body)
	expected := `console.log("ghamusinos")`
	if string(body) != expected {
		t.Fatalf("contenido del asset = %q, quería %q", string(body), expected)
	}
}

func TestSPAHandler_SinIndexDevuelve503(t *testing.T) {
	h := NewSPAHandler(fsSinBuild)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("sin index.html quería 503, obtuvo %d", rec.Code)
	}
}
