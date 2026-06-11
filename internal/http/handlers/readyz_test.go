package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestReadyz_poolNil verifica que con pool nil se devuelve 503 degraded.
func TestReadyz_poolNil(t *testing.T) {
	handler := Readyz(nil)

	req := httptest.NewRequest(http.MethodGet, "/readyz", nil)
	rec := httptest.NewRecorder()

	handler(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, quería %d", rec.Code, http.StatusServiceUnavailable)
	}

	var body map[string]string
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("respuesta no es JSON válido: %v", err)
	}
	if body["status"] != "degraded" {
		t.Fatalf(`status = %q, quería "degraded"`, body["status"])
	}
	if body["db"] != "down" {
		t.Fatalf(`db = %q, quería "down"`, body["db"])
	}

	ct := rec.Header().Get("Content-Type")
	if ct != "application/json" {
		t.Fatalf("Content-Type = %q, quería application/json", ct)
	}
}

// TestReadyz_pingFalla verifica que cuando el ping falla se devuelve 503.
func TestReadyz_pingFalla(t *testing.T) {
	// Usamos una implementación de DBPinger que siempre falla.
	pinger := &fakePinger{err: errPingFailed}
	handler := Readyz(pinger)

	req := httptest.NewRequest(http.MethodGet, "/readyz", nil)
	rec := httptest.NewRecorder()

	handler(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, quería %d", rec.Code, http.StatusServiceUnavailable)
	}

	var body map[string]string
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("respuesta no es JSON válido: %v", err)
	}
	if body["status"] != "degraded" {
		t.Fatalf(`status = %q, quería "degraded"`, body["status"])
	}
	if body["db"] != "down" {
		t.Fatalf(`db = %q, quería "down"`, body["db"])
	}

	ct := rec.Header().Get("Content-Type")
	if ct != "application/json" {
		t.Fatalf("Content-Type = %q, quería application/json", ct)
	}
}

// TestReadyz_pingOK verifica que cuando el ping tiene éxito se devuelve 200.
func TestReadyz_pingOK(t *testing.T) {
	pinger := &fakePinger{err: nil}
	handler := Readyz(pinger)

	req := httptest.NewRequest(http.MethodGet, "/readyz", nil)
	rec := httptest.NewRecorder()

	handler(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, quería %d", rec.Code, http.StatusOK)
	}

	var body map[string]string
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("respuesta no es JSON válido: %v", err)
	}
	if body["status"] != "ok" {
		t.Fatalf(`status = %q, quería "ok"`, body["status"])
	}

	ct := rec.Header().Get("Content-Type")
	if ct != "application/json" {
		t.Fatalf("Content-Type = %q, quería application/json", ct)
	}
}
