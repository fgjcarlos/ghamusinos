// Package http construye el router HTTP y el middleware base del servidor.
package http

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"github.com/fgjcarlos/ghamusinos/internal/auth"
	"github.com/fgjcarlos/ghamusinos/internal/config"
	"github.com/fgjcarlos/ghamusinos/internal/db/sqlc"
	"github.com/fgjcarlos/ghamusinos/internal/frontend"
	"github.com/fgjcarlos/ghamusinos/internal/http/handlers"
)

// Server agrupa las dependencias inyectadas necesarias para construir el router.
// Se amplía con nuevas dependencias (queries SQLC, etc.) sin modificar la firma
// de construcción de cada handler.
type Server struct {
	pool    handlers.DBPinger
	queries sqlc.Querier
	cfg     *config.Config
}

// NewServer crea un Server con el pool de base de datos y configuración proporcionados.
// pool puede ser nil en tests sin base de datos; /readyz responderá 503 en ese caso.
func NewServer(pool handlers.DBPinger, queries sqlc.Querier, cfg *config.Config) *Server {
	return &Server{pool: pool, queries: queries, cfg: cfg}
}

// Router construye el handler HTTP con el middleware base y todas las rutas.
//
// Middleware base:
//   - RequestID: identificador de correlación por petición.
//   - RealIP:    IP real del cliente tras proxies.
//   - Logger:    log de cada petición.
//   - Recoverer: recupera ante panics y devuelve 500 sin tumbar el servidor.
func (s *Server) Router() http.Handler {
	r := chi.NewRouter()

	r.Use(middleware.RequestID)
	// RealIP confía en X-Forwarded-For / X-Real-IP. Asume que el binario se
	// despliega DETRÁS de un reverse proxy de confianza (plataforma o Nginx).
	// Si en algún momento se expone directo a internet, restringir o quitar.
	//
	// SA1019 (staticcheck) marca `middleware.RealIP` como deprecated por
	// vulnerabilidad a IP spoofing (GHSA-3fxj-6jh8-hvhx, GHSA-rjr7-jggh-pgcp,
	// GHSA-9g5q-2w5x-hmxf): muta r.RemoteAddr al primer valor de
	// X-Forwarded-For aunque la cadena de proxies no sea de confianza.
	// Se mantiene temporalmente mientras se evalúa la alternativa (p.ej.
	// `middleware.ForwardedHeader` con lista de IPs de proxy, o quitar
	// y derivar la IP del log desde el peer directo). Tracked in issue
	// de seguimiento abierta desde #62.
	r.Use(middleware.RealIP) //nolint:staticcheck // SA1019: deprecated por IP spoofing, fix trackeado en issue de seguimiento
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	// Liveness: responde sin tocar la base de datos.
	r.Get("/healthz", handlers.Health)

	// Readiness: refleja el estado real de la base de datos.
	r.Get("/readyz", handlers.Readyz(s.pool))

	// Grupo de API. Las rutas privadas (protegidas por auth) se añaden en la
	// fase de autenticación. Tiene su propio NotFound para que /api/inexistente
	// devuelva JSON y NO caiga al handler SPA.
	r.Route("/api", func(r chi.Router) {
		// Wire auth middleware
		jwksCache := auth.NewJWKSCache(s.cfg.ClerkJWKSURL, time.Hour)
		validator := auth.NewJWTValidator(jwksCache, s.cfg.ClerkAudience)
		resolver := auth.NewUserResolver(s.queries)

		r.Use(auth.AuthMiddleware(validator))
		r.Use(auth.ResolveMiddleware(resolver))
		r.Use(auth.InviteGateMiddleware(s.queries))

		// Protected routes
		r.Get("/me", handlers.Me(s.queries).ServeHTTP)

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
