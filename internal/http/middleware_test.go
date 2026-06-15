package http

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5/middleware"
)

// captureLogs reemplaza slog.Default() por un logger que escribe a buf
// y devuelve una función para restaurarlo. Útil para tests que dependen
// del default (como nuestro middleware RequestLogger).
func captureLogs(t *testing.T) (*bytes.Buffer, func()) {
	t.Helper()
	var buf bytes.Buffer
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	})))
	return &buf, func() { slog.SetDefault(prev) }
}

func TestRequestLogger_LogsRequestFields(t *testing.T) {
	buf, restore := captureLogs(t)
	defer restore()

	handler := RequestLogger(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	}))

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	entries := parseLogLines(t, buf)
	if len(entries) != 1 {
		t.Fatalf("esperaba 1 línea de log, obtuve %d: %s", len(entries), buf.String())
	}
	e := entries[0]
	if e["msg"] != "request" {
		t.Errorf("msg = %v, quería \"request\"", e["msg"])
	}
	if e["method"] != "GET" {
		t.Errorf("method = %v, quería \"GET\"", e["method"])
	}
	if e["path"] != "/healthz" {
		t.Errorf("path = %v, quería \"/healthz\"", e["path"])
	}
	// JSON decodifica números como float64.
	if e["status"].(float64) != 200 {
		t.Errorf("status = %v, quería 200", e["status"])
	}
	if _, ok := e["latency_ms"]; !ok {
		t.Errorf("falta latency_ms en log: %v", e)
	}
}

func TestRequestLogger_IncludesRequestID(t *testing.T) {
	buf, restore := captureLogs(t)
	defer restore()

	// Cadena: chi.RequestID → nuestro RequestIDHeader → RequestLogger →
	// handler. Las dos primeras pueblan contexto y header, RequestLogger
	// los usa en el log.
	chain := middleware.RequestID(
		RequestIDHeader(
			RequestLogger(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusOK)
			})),
		),
	)

	req := httptest.NewRequest(http.MethodGet, "/api/whatever", nil)
	rec := httptest.NewRecorder()
	chain.ServeHTTP(rec, req)

	// La respuesta debe llevar X-Request-Id (lo pone RequestIDHeader).
	ridHeader := rec.Header().Get("X-Request-Id")
	if ridHeader == "" {
		t.Fatal("respuesta sin X-Request-Id; RequestIDHeader no se ejecutó")
	}

	entries := parseLogLines(t, buf)
	if len(entries) != 1 {
		t.Fatalf("esperaba 1 línea de log, obtuve %d", len(entries))
	}
	if entries[0]["request_id"] != ridHeader {
		t.Errorf("request_id del log = %v, quería %v (el de la respuesta)", entries[0]["request_id"], ridHeader)
	}
}

func TestRequestIDHeader_SetsResponseHeader(t *testing.T) {
	// El request_id del contexto debe terminar en el header de respuesta.
	// Probamos con un ID que llega en el request (chi lo respeta) y
	// otro generado por chi (sin header de entrada).
	t.Run("ID del request", func(t *testing.T) {
		chain := middleware.RequestID(
			RequestIDHeader(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusOK)
			})),
		)
		req := httptest.NewRequest(http.MethodGet, "/x", nil)
		req.Header.Set("X-Request-Id", "cliente-trace-abc")
		rec := httptest.NewRecorder()
		chain.ServeHTTP(rec, req)

		if got := rec.Header().Get("X-Request-Id"); got != "cliente-trace-abc" {
			t.Errorf("X-Request-Id = %q, quería \"cliente-trace-abc\"", got)
		}
	})

	t.Run("ID generado por chi", func(t *testing.T) {
		chain := middleware.RequestID(
			RequestIDHeader(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusOK)
			})),
		)
		req := httptest.NewRequest(http.MethodGet, "/x", nil)
		rec := httptest.NewRecorder()
		chain.ServeHTTP(rec, req)

		got := rec.Header().Get("X-Request-Id")
		if got == "" {
			t.Error("X-Request-Id no se setteó con ID generado por chi")
		}
	})
}

func TestRequestLogger_LevelByStatus(t *testing.T) {
	tests := []struct {
		name      string
		status    int
		wantLevel string
	}{
		{"200 → INFO", http.StatusOK, "INFO"},
		{"301 → INFO", http.StatusMovedPermanently, "INFO"},
		{"404 → WARN", http.StatusNotFound, "WARN"},
		{"500 → ERROR", http.StatusInternalServerError, "ERROR"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buf, restore := captureLogs(t)
			defer restore()

			handler := RequestLogger(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(tt.status)
			}))
			req := httptest.NewRequest(http.MethodGet, "/x", nil)
			handler.ServeHTTP(httptest.NewRecorder(), req)

			entries := parseLogLines(t, buf)
			if len(entries) != 1 {
				t.Fatalf("esperaba 1 línea, obtuve %d", len(entries))
			}
			if entries[0]["level"] != tt.wantLevel {
				t.Errorf("level = %v, quería %v", entries[0]["level"], tt.wantLevel)
			}
		})
	}
}

func TestRequestLogger_PreservesResponseBody(t *testing.T) {
	// El middleware no debe alterar el body ni los headers de la respuesta.
	buf, restore := captureLogs(t)
	defer restore()

	body := `{"hello":"world"}`
	handler := RequestLogger(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(body))
	}))

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/x", nil))

	if rec.Code != http.StatusCreated {
		t.Errorf("status code = %d, quería 201", rec.Code)
	}
	if rec.Header().Get("Content-Type") != "application/json" {
		t.Errorf("Content-Type perdido: %q", rec.Header().Get("Content-Type"))
	}
	if rec.Body.String() != body {
		t.Errorf("body = %q, quería %q", rec.Body.String(), body)
	}
	// Y al menos una línea de log.
	if !strings.Contains(buf.String(), `"msg":"request"`) {
		t.Errorf("no se logueó la request: %s", buf.String())
	}
}

func TestRequestLogger_HandlesHandlerPanic(t *testing.T) {
	// El Recoverer middleware (que está más arriba en la cadena) es
	// quien debe capturar el panic; aquí solo verificamos que nuestro
	// middleware no rompa cuando el handler hace cosas raras. Llamamos
	// directamente sin Recoverer.
	buf, restore := captureLogs(t)
	defer restore()

	handler := RequestLogger(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Usamos el contexto vacío a propósito: queremos ver que el
		// middleware no se cae si no hay request_id.
		_ = r.Context().Value(context.Background())
		w.WriteHeader(http.StatusNoContent)
	}))

	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	handler.ServeHTTP(httptest.NewRecorder(), req)

	if !strings.Contains(buf.String(), `"msg":"request"`) {
		t.Errorf("no se logueó la request: %s", buf.String())
	}
}

// parseLogLines parsea el contenido de buf como líneas JSON (una por
// entry) y devuelve los mapas correspondientes. Falla el test si alguna
// línea no parsea.
func parseLogLines(t *testing.T, buf *bytes.Buffer) []map[string]any {
	t.Helper()
	var out []map[string]any
	for _, line := range strings.Split(strings.TrimSpace(buf.String()), "\n") {
		if line == "" {
			continue
		}
		var m map[string]any
		if err := json.Unmarshal([]byte(line), &m); err != nil {
			t.Fatalf("línea de log no es JSON válido:\n  %s\n  err: %v", line, err)
		}
		out = append(out, m)
	}
	return out
}
