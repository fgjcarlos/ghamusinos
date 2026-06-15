package http

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5/middleware"
)

// RequestIDHeader es un middleware que escribe el header X-Request-Id
// en la respuesta, tomándolo del contexto.
//
// chi.middleware.RequestID genera el ID y lo guarda en el contexto,
// pero NO toca el header de respuesta (a diferencia de chi v4, que
// sí lo hacía). Sin este middleware, el cliente nunca ve el ID.
// Si el cliente envía su propio X-Request-Id, chi lo respeta, así
// que este middleware se limita a propagarlo a la respuesta.
func RequestIDHeader(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if id := middleware.GetReqID(r.Context()); id != "" {
			w.Header().Set(middleware.RequestIDHeader, id)
		}
		next.ServeHTTP(w, r)
	})
}

// RequestLogger es un middleware de access log que escribe una línea
// estructurada por cada request HTTP, con los campos:
//
//   - request_id: extraído del contexto (poblado por middleware.RequestID)
//   - method, path: datos básicos de la request
//   - status: código HTTP devuelto
//   - latency_ms: duración de la request en milisegundos
//
// El nivel de log depende del status:
//   - 5xx → ERROR
//   - 4xx → WARN
//   - resto → INFO
//
// Reemplaza a chi.middleware.Logger (que escribe texto plano a stdout
// y no es parseable por agregadores).
func RequestLogger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		// WrapResponseWriter captura el status code escrito por el
		// handler (y el número de bytes, aunque no lo logueamos).
		ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)

		next.ServeHTTP(ww, r)

		status := ww.Status()
		level := slog.LevelInfo
		switch {
		case status >= 500:
			level = slog.LevelError
		case status >= 400:
			level = slog.LevelWarn
		}

		// LogAttrs evita la asignación de los pares clave=valor en cada
		// llamada (más eficiente que Info/Error con args variádicos).
		slog.LogAttrs(r.Context(), level, "request",
			slog.String("request_id", middleware.GetReqID(r.Context())),
			slog.String("method", r.Method),
			slog.String("path", r.URL.Path),
			slog.Int("status", status),
			slog.Int64("latency_ms", time.Since(start).Milliseconds()),
		)
	})
}
