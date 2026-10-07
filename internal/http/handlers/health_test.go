package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestHealthPublicShape verifies the public /healthz contract:
// without the X-Internal-Health header the response is exactly
// {status:"ok"} and the status is 200 (issue #16 PR3, DA-005
// "public request without the header keeps the minimal shape").
func TestHealthPublicShape(t *testing.T) {
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()

	// Healthy DB, public request → {status:"ok"}.
	Health(okPinger{}, nil).ServeHTTP(rec, req)

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
	if len(body) != 1 {
		t.Fatalf("public /healthz debe contener solo `status`, obtuvo %d keys: %v", len(body), body)
	}
}
