// Package http construye el router HTTP y el middleware base del servidor.
package http

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"github.com/fgjcarlos/ghamusinos/internal/http/handlers"
)

// NewRouter crea el router con el middleware base y las rutas iniciales.
//
// Middleware base:
//   - RequestID: identificador de correlación por petición.
//   - RealIP:    IP real del cliente tras proxies.
//   - Logger:    log de cada petición.
//   - Recoverer: recupera ante panics y devuelve 500 sin tumbar el servidor.
func NewRouter() http.Handler {
	r := chi.NewRouter()

	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	// Ruta pública de healthcheck.
	r.Get("/healthz", handlers.Health)

	// Grupo de API. Las rutas privadas (protegidas por auth) se añaden en la
	// fase de autenticación.
	r.Route("/api", func(_ chi.Router) {})

	return r
}
