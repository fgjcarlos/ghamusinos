// Package http construye el router HTTP y el middleware base del servidor.
package http

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"github.com/fgjcarlos/ghamusinos/internal/frontend"
	"github.com/fgjcarlos/ghamusinos/internal/http/handlers"
)

// Server agrupa las dependencias inyectadas necesarias para construir el router.
// Se amplía con nuevas dependencias (queries SQLC, etc.) sin modificar la firma
// de construcción de cada handler.
type Server struct {
	pool handlers.DBPinger
}

// NewServer crea un Server con el pool de base de datos proporcionado.
// pool puede ser nil en tests sin base de datos; /readyz responderá 503 en ese caso.
func NewServer(pool handlers.DBPinger) *Server {
	return &Server{pool: pool}
}

// Router construye el handler HTTP con el middleware base y todas las rutas.
//
// Middleware base (en orden de aplicación):
//   - RequestID (chi): genera o respeta el request_id, lo guarda en el
//     contexto. NO escribe el header de respuesta (lo hace RequestIDHeader
//     a continuación).
//   - RequestIDHeader: propaga el request_id al header X-Request-Id de
//     la respuesta para que el cliente pueda ver su correlation ID.
//   - RealIP (chi): IP real del cliente tras proxies.
//   - Recoverer (chi): recupera ante panics y devuelve 500 sin tumbar
//     el servidor.
//   - RequestLogger: log estructurado (slog) con request_id, method,
//     path, status y latency_ms. Nivel según status (5xx→ERROR,
//     4xx→WARN, resto→INFO). Se aplica ÚLTIMO para que la latencia y
//     el status reflejen lo que el cliente recibió.
func (s *Server) Router() http.Handler {
	r := chi.NewRouter()

	r.Use(middleware.RequestID)
	r.Use(RequestIDHeader)
	// RealIP confía en X-Forwarded-For / X-Real-IP. Asume que el binario se
	// despliega DETRÁS de un reverse proxy de confianza (plataforma o Nginx).
	// Si en algún momento se expone directo a internet, restringir o quitar.
	r.Use(middleware.RealIP)
	r.Use(middleware.Recoverer)
	r.Use(RequestLogger)

	// Liveness: responde sin tocar la base de datos.
	r.Get("/healthz", handlers.Health)

	// Readiness: refleja el estado real de la base de datos.
	r.Get("/readyz", handlers.Readyz(s.pool))

	// Grupo de API. Las rutas privadas (protegidas por auth) se añaden en la
	// fase de autenticación. Tiene su propio NotFound para que /api/inexistente
	// devuelva JSON y NO caiga al handler SPA.
	r.Route("/api", func(r chi.Router) {
		r.NotFound(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusNotFound)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": "not found"})
		})
	})

	// SPA: cualquier otra ruta (incluida /) sirve la SPA embebida con fallback
	// a index.html para el client-side routing.
	r.Handle("/*", frontend.Handler())

	return r
}
