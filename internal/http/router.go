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
	// RealIP confía en X-Forwarded-For / X-Real-IP. Asume que el binario se
	// despliega DETRÁS de un reverse proxy de confianza (plataforma o Nginx).
	// Si en algún momento se expone directo a internet, restringir o quitar.
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	// Ruta pública de healthcheck (exacta, sin trailing slash).
	r.Get("/healthz", handlers.Health)

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
