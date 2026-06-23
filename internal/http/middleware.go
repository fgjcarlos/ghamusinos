package http

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5/middleware"
)

// RequestIDHeader middleware writes the request ID from context to the response header.
func RequestIDHeader(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestID := middleware.GetReqID(r.Context())
		if requestID != "" {
			w.Header().Set("X-Request-Id", requestID)
		}
		next.ServeHTTP(w, r)
	})
}

// wrappedResponseWriter wraps http.ResponseWriter to capture the status code.
type wrappedResponseWriter struct {
	http.ResponseWriter
	statusCode int
	wrote      bool
}

func (w *wrappedResponseWriter) WriteHeader(code int) {
	if !w.wrote {
		w.statusCode = code
		w.wrote = true
		w.ResponseWriter.WriteHeader(code)
	}
}

func (w *wrappedResponseWriter) Write(b []byte) (int, error) {
	if !w.wrote {
		w.statusCode = http.StatusOK
		w.wrote = true
	}
	return w.ResponseWriter.Write(b)
}

// RequestLogger middleware logs structured access logs for each request.
func RequestLogger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		wrapped := &wrappedResponseWriter{ResponseWriter: w, statusCode: http.StatusOK}

		start := time.Now()
		next.ServeHTTP(wrapped, r)
		latencyMs := int64(time.Since(start).Milliseconds())

		requestID := middleware.GetReqID(r.Context())

		// Determine log level based on status code
		level := slog.LevelInfo
		if wrapped.statusCode >= 500 {
			level = slog.LevelError
		} else if wrapped.statusCode >= 400 {
			level = slog.LevelWarn
		}

		slog.LogAttrs(r.Context(), level, "request",
			slog.String("request_id", requestID),
			slog.String("method", r.Method),
			slog.String("path", r.RequestURI),
			slog.Int("status", wrapped.statusCode),
			slog.Int64("latency_ms", latencyMs),
		)
	})
}
