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
	"github.com/fgjcarlos/ghamusinos/internal/gpx"
	"github.com/fgjcarlos/ghamusinos/internal/http/handlers"
	"github.com/fgjcarlos/ghamusinos/internal/strava"
)

// Server agrupa las dependencias inyectadas necesarias para construir el router.
// Se amplía con nuevas dependencias (queries SQLC, etc.) sin modificar la firma
// de construcción de cada handler.
type Server struct {
	pool    handlers.DBPinger
	queries sqlc.Querier
	cfg     *config.Config
	// Strava (opcional, fase 1.2 issue #14): si se setea, monta los
	// handlers OAuth bajo /api/v1/strava/* después del middleware de auth.
	stravaClient    *strava.Client
	stravaStore     strava.TokenStore
	stravaCipherKey []byte
	// Webhook store (opcional, fase 1.2 issue #86): para webhooks de Strava.
	// Se monta antes del middleware de auth, es signature-gated.
	webhookStore strava.ActivityEventStore
	// GPX Lab dependencies are optional so focused router tests and commands that
	// do not initialize the feature keep their existing construction path.
	gpxStore         gpx.GPXStore
	gpxParser        gpx.GPXParser
	gpxValidator     gpx.GPXValidator
	gpxAnalyzer      gpx.GPXAnalyzer
	gpxClimbDetector gpx.ClimbDetector
	gpxRiskDetector  gpx.RiskZoneDetector
	gpxTypeDetector  gpx.TrackTypeDetector
	gpxHasher        gpx.GPXHasher
}

// NewServer crea un Server con el pool de base de datos y configuración proporcionados.
// pool puede ser nil en tests sin base de datos; /readyz responderá 503 en ese caso.
func NewServer(pool handlers.DBPinger, queries sqlc.Querier, cfg *config.Config) *Server {
	return &Server{pool: pool, queries: queries, cfg: cfg}
}

// WithStrava cablea los handlers de OAuth Strava al router.
// Se devuelve *Server para fluent chaining. cipherKey debe tener 32 bytes;
// si no, los handlers devolverán error en runtime al primer callback.
func (s *Server) WithStrava(client *strava.Client, store strava.TokenStore, cipherKey []byte) *Server {
	s.stravaClient = client
	s.stravaStore = store
	s.stravaCipherKey = cipherKey
	return s
}

// WithWebhooks cablea los handlers de webhooks Strava al router.
// Se devuelve *Server para fluent chaining. El ActivityEventStore maneja
// tanto la encolada de eventos como la encolada de jobs River.
func (s *Server) WithWebhooks(store strava.ActivityEventStore) *Server {
	s.webhookStore = store
	return s
}

func (s *Server) WithGPX(
	store gpx.GPXStore,
	parser gpx.GPXParser,
	validator gpx.GPXValidator,
	analyzer gpx.GPXAnalyzer,
	climbDetector gpx.ClimbDetector,
	riskDetector gpx.RiskZoneDetector,
	typeDetector gpx.TrackTypeDetector,
	hasher gpx.GPXHasher,
) *Server {
	s.gpxStore = store
	s.gpxParser = parser
	s.gpxValidator = validator
	s.gpxAnalyzer = analyzer
	s.gpxClimbDetector = climbDetector
	s.gpxRiskDetector = riskDetector
	s.gpxTypeDetector = typeDetector
	s.gpxHasher = hasher
	return s
}

// Router construye el handler HTTP con el middleware base y todas las rutas.
//
// Middleware base:
//   - RequestID: identificador de correlación por petición.
//   - RequestIDHeader: propaga el request ID al header de respuesta.
//   - ClientIPFromXFF (opcional): solo si TRUSTED_PROXIES está configurado.
//   - Recoverer: recupera ante panics y devuelve 500 sin tumbar el servidor.
//   - RequestLogger: log de cada petición con estructura JSON.
func (s *Server) Router() http.Handler {
	r := chi.NewRouter()

	r.Use(middleware.RequestID)
	r.Use(RequestIDHeader)
	// IP del cliente: usa middleware.ClientIPFromXFF si hay proxies de
	// confianza declarados en TRUSTED_PROXIES (CIDRs separados por coma);
	// si no, no monta nada y los logs usan r.RemoteAddr (fail-closed).
	// Reemplaza middleware.RealIP (SA1019: vulnerable a IP spoofing,
	// GHSA-3fxj-6jh8-hvhx). Issue #64.
	if len(s.cfg.TrustedProxies) > 0 {
		r.Use(middleware.ClientIPFromXFF(s.cfg.TrustedProxies...))
	}
	r.Use(middleware.Recoverer)
	r.Use(RequestLogger)

	// Liveness: responde sin tocar la base de datos.
	r.Get("/healthz", handlers.Health)

	// Readiness: refleja el estado real de la base de datos.
	r.Get("/readyz", handlers.Readyz(s.pool))

	// Strava webhooks (issue #86, fase 1.2): se monta ANTES del middleware de auth
	// porque los webhooks se validan por firma HMAC-SHA256, no por Clerk JWT.
	// Las rutas quedan fuera de /api para mantenerlas públicas.
	if s.webhookStore != nil && s.cfg.Strava != nil {
		r.Get("/strava/webhook", strava.WebhookChallengeHandler("strava"))
		r.Post("/strava/webhook", strava.WebhookHandler(s.cfg.Strava.WebhookSecret, s.webhookStore))
	}

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

			if s.gpxStore != nil {
				r.Route("/gpx", func(r chi.Router) {
					r.Post("/upload", handlers.UploadGPX(s.gpxStore, s.gpxParser, s.gpxValidator, s.gpxAnalyzer, s.gpxClimbDetector, s.gpxRiskDetector, s.gpxTypeDetector, s.gpxHasher).ServeHTTP)
					r.Get("/{id}", handlers.GetGPX(s.gpxStore).ServeHTTP)
				})
			}

			// Strava OAuth (issue #14, fase 1.2): se monta solo si el wiring
			// inyectó dependencias (cliente + store + cipher key). Las rutas
			// quedan detrás del middleware de auth: el user_id se toma del
			// contexto (inyectado por ResolveMiddleware) en vez de query string.
			if s.stravaClient != nil && s.stravaStore != nil && s.stravaCipherKey != nil {
				r.Route("/strava", func(r chi.Router) {
					r.Get("/connect", strava.ConnectHandler(s.stravaClient))
					r.Get("/callback", strava.CallbackHandler(s.stravaClient, s.stravaStore, s.stravaCipherKey, s.cfg.FrontendURL))
				})
			}

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
