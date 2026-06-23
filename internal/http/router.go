// Package http construye el router HTTP y el middleware base del servidor.
package http

import (
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
//   - RequestIDHeader: propaga el request ID al header de respuesta.
//   - RealIP:    IP real del cliente tras proxies.
//   - Recoverer: recupera ante panics y devuelve 500 sin tumbar el servidor.
//   - RequestLogger: log de cada petición con estructura JSON.
func (s *Server) Router() http.Handler {
	r := chi.NewRouter()

	r.Use(middleware.RequestID)
	r.Use(RequestIDHeader)
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
	r.Use(middleware.Recoverer)
	r.Use(RequestLogger)

	// Liveness: responde sin tocar la base de datos.
	r.Get("/healthz", handlers.Health)

	// Readiness: refleja el estado real de la base de datos.
	r.Get("/readyz", handlers.Readyz(s.pool))

	// Grupo de API. TODAS las rutas /api/* están protegidas por autenticación.
	// Las rutas específicas de cada versión (v1, v2, etc.) se definen dentro.
	r.Route("/api", func(r chi.Router) {
		// Wire auth middleware for ALL /api routes
		jwksCache := auth.NewJWKSCache(s.cfg.ClerkJWKSURL, time.Hour)
		validator := auth.NewJWTValidator(jwksCache, s.cfg.ClerkAudience)
		resolver := auth.NewUserResolver(s.queries)

		r.Use(auth.AuthMiddleware(validator))
		r.Use(auth.ResolveMiddleware(resolver))
		r.Use(auth.InviteGateMiddleware(s.queries))

		// v1 API routes
		r.Route("/v1", func(r chi.Router) {
			r.Get("/me", handlers.Me(s.queries).ServeHTTP)

			// Custom NotFound and MethodNotAllowed for v1 API (RFC 9457 ProblemDetail)
			r.NotFound(func(w http.ResponseWriter, r *http.Request) {
				requestID := middleware.GetReqID(r.Context())
				problem := handlers.NewNotFound("endpoint not found", requestID)
				handlers.WriteProblem(w, problem)
			})
			r.MethodNotAllowed(func(w http.ResponseWriter, r *http.Request) {
				requestID := middleware.GetReqID(r.Context())
				problem := handlers.ProblemDetail{
					Type:     "about:blank",
					Title:    "Method Not Allowed",
					Status:   http.StatusMethodNotAllowed,
					Detail:   "method " + r.Method + " not allowed for this endpoint",
					Instance: requestID,
				}
				handlers.WriteProblem(w, problem)
			})
		})

		// Catch unknown API versions and return 404
		r.NotFound(func(w http.ResponseWriter, r *http.Request) {
			requestID := middleware.GetReqID(r.Context())
			problem := handlers.NewNotFound("API version not found", requestID)
			handlers.WriteProblem(w, problem)
		})
	})

	// SPA: cualquier otra ruta (incluida /) sirve la SPA embebida con fallback
	// a index.html para el client-side routing.
	r.Handle("/*", frontend.Handler())

	return r
}
