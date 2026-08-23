// Package http construye el router HTTP y el middleware base del servidor.
package http

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/google/uuid"

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
	// stravaEnqueuer encola el job de backfill tras un OAuth exitoso.
	// AUD-04 AC: "Conectar Strava encola un ImportStravaArgs". El seam
	// lo provee app.Run; aquí lo aceptamos como interfaz para mantener
	// la dirección del grafo (strava no importa jobs).
	stravaEnqueuer strava.RiverEnqueuer
	// Webhook store (optional, phase 1.2 issue #86). Routes are mounted before
	// authentication because Strava calls them directly.
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
// AUD-04: el enqueuer (RiverEnqueuer) lo provee app.Run; si es nil el
// callback no encola el backfill (útil en tests que solo ejercitan el
// intercambio de tokens).
func (s *Server) WithStrava(client *strava.Client, store strava.TokenStore, enqueuer strava.RiverEnqueuer, cipherKey []byte) *Server {
	s.stravaClient = client
	s.stravaStore = store
	s.stravaEnqueuer = enqueuer
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
	// Security headers (CSP, X-Frame-Options, Referrer-Policy,
	// Permissions-Policy, HSTS opcional). Issue #26.
	r.Use(SecurityHeaders(SecurityHeadersConfig{
		HSTSEnabled:   s.cfg.Security.HSTSEnabled,
		CSPReportOnly: s.cfg.Security.CSPReportOnly,
	}))
	// Límite global del cuerpo HTTP. Cada handler que lea r.Body debe
	// usar handlers.IsBodyLimitError para responder 413. Issue #26.
	r.Use(BodyLimit(s.cfg.MaxBodyBytes))

	// Liveness: responde sin tocar la base de datos.
	r.Get("/healthz", handlers.Health)

	// Readiness: refleja el estado real de la base de datos.
	r.Get("/readyz", handlers.Readyz(s.pool))

	// Strava webhooks are public because Strava calls them directly. The GET
	// subscription handshake validates the configured verify token.
	if s.webhookStore != nil && s.cfg.Strava != nil {
		r.Get("/strava/webhook", strava.WebhookChallengeHandler(s.cfg.Strava.WebhookVerifyToken))
		r.Post("/strava/webhook", strava.WebhookHandler(s.webhookStore))
	}

	// Strava OAuth callback (AUD-02, issue #163): también se monta FUERA de /api
	// porque la redirección de Strava es una navegación top-level del navegador,
	// que no envía la cabecera Authorization. El user_id viaja dentro del state
	// firmado (HMAC-SHA256 sobre {uid, nonce, exp}); el handler verifica la
	// firma antes de actuar.
	//
	// AUD-04: tras un intercambio de tokens exitoso el callback encola un
	// ImportStravaArgs (River backfill). El enqueuer lo provee app.go.
	if s.stravaClient != nil && s.stravaStore != nil && s.stravaCipherKey != nil {
		r.Get("/strava/callback", strava.CallbackHandler(
			s.stravaClient, s.stravaStore, s.stravaEnqueuer, s.stravaCipherKey, s.cfg.FrontendURL))
	}

	// Grupo de API. TODAS las rutas /api/* están protegidas por autenticación.
	// Las rutas específicas de cada versión (v1, v2, etc.) se definen dentro.
	r.Route("/api", func(r chi.Router) {
		// Wire auth middleware for ALL /api routes
		jwksCache := auth.NewJWKSCache(s.cfg.ClerkJWKSURL, time.Hour)
		validator := auth.NewJWTValidator(jwksCache, s.cfg.ClerkIssuer, s.cfg.ClerkAudience)
		resolver := auth.NewUserResolver(s.queries)

		if s.cfg.AuthDisabled {
			// Bypass Clerk: inyecta un usuario sintético y se salta el
			// invite gate. SOLO desarrollo local — el binario loguea un
			// warning al arrancar si esto está activo.
			slog.Warn("AUTH_DISABLED=true: Clerk y invite gate desactivados, /api/* usa usuario sintético dev@local")
			r.Use(devUserMiddleware())
		} else {
			r.Use(auth.AuthMiddleware(validator))
			r.Use(auth.ResolveMiddleware(resolver))
			r.Use(auth.InviteGateMiddleware(s.queries))
		}

		// v1 API routes
		r.Route("/v1", func(r chi.Router) {
			r.Get("/me", handlers.Me(s.queries).ServeHTTP)

			if s.gpxStore != nil {
				r.Route("/gpx", func(r chi.Router) {
					r.Post("/upload", handlers.UploadGPX(s.gpxStore, s.gpxParser, s.gpxValidator, s.gpxAnalyzer, s.gpxClimbDetector, s.gpxRiskDetector, s.gpxTypeDetector, s.gpxHasher).ServeHTTP)
					r.Get("/", handlers.ListGPX(s.gpxStore).ServeHTTP)
					r.Get("/{id}", handlers.GetGPX(s.gpxStore).ServeHTTP)
					r.Delete("/{id}", handlers.DeleteGPX(s.gpxStore).ServeHTTP)
					r.Post("/compare", handlers.CompareGPX(s.gpxStore).ServeHTTP)
				})
			}

			// Strava OAuth connect (AUD-02, issue #163): sigue bajo /api porque
			// el frontend lo llama con fetch() y lleva Authorization. Devuelve
			// JSON con authorize_url (el state firmado lleva el user_id por dentro)
			// y el frontend hace window.location.assign(url).
			// El callback está montado arriba, fuera de /api.
			if s.stravaClient != nil && s.stravaStore != nil && s.stravaCipherKey != nil {
				r.Get("/strava/connect", strava.ConnectHandler(s.stravaClient, s.stravaCipherKey))
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

// devUserMiddleware inyecta un usuario sintético en el contexto para
// saltarse Clerk + invite gate cuando cfg.AuthDisabled=true. Solo se monta
// en ese caso (router.go línea ~175). El UUID se genera una vez por
// proceso — los handlers que persistan datos obtendrán un user_id estable
// dentro de la misma ejecución del binario.
//
// ponytail: ID hardcodeado a dev@local; aceptable porque este modo es
// solo dev y ningún handler hace upsert por user_id sin que el operador
// lo sepa (logs explícitos al arrancar).
func devUserMiddleware() func(http.Handler) http.Handler {
	devID := uuid.New().String()
	devUser := &auth.User{
		ID:           devID,
		ClerkUserID:  "dev_clerk_user",
		Email:        "dev@local",
		DisplayName:  "Local Dev",
		InviteStatus: "active",
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := auth.WithAuthUser(r.Context(), devUser)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
