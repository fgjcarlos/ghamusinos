// Package app conecta configuración, servidor HTTP y dependencias del binario.
package app

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os/signal"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/fgjcarlos/ghamusinos/internal/config"
	"github.com/fgjcarlos/ghamusinos/internal/db"
	"github.com/fgjcarlos/ghamusinos/internal/db/sqlc"
	"github.com/fgjcarlos/ghamusinos/internal/gpx"
	apphttp "github.com/fgjcarlos/ghamusinos/internal/http"
	"github.com/fgjcarlos/ghamusinos/internal/jobs"
	"github.com/fgjcarlos/ghamusinos/internal/strava"
)

const shutdownTimeout = 10 * time.Second

// Run arranca el servidor HTTP y el River job queue, bloqueando hasta que se
// recibe una señal de apagado (SIGINT/SIGTERM), momento en el que hace un
// shutdown ordenado para ambos.
func Run() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	pool, err := db.Connect(ctx, cfg.DatabaseURL, cfg.Pool)
	if err != nil {
		return err
	}
	defer pool.Close()

	slog.Info("conexión a base de datos establecida")

	// Initialize River client for job queue. AUD-04 (issue #164): the workers
	// are constructed from a Deps bundle instead of package globals; NewClient
	// returns an error if any required dependency is missing so the binary fails
	// fast at startup rather than at the first job. The Strava-bound workers
	// are only registered when cfg.Strava is populated.
	registerStravaWorkers := cfg.Strava != nil
	var stravaForWorkers *strava.Client
	var cipherForWorkers []byte
	if registerStravaWorkers {
		// Build the same Strava client buildRouter uses, so both the HTTP
		// handlers and the workers share one instance.
		sc, scErr := strava.NewClient(strava.Config{
			ClientID:     cfg.Strava.ClientID,
			ClientSecret: cfg.Strava.ClientSecret,
			RedirectURL:  cfg.Strava.RedirectURL,
			Scopes:       cfg.Strava.Scopes,
		})
		if scErr != nil {
			slog.Warn("strava: cliente no inicializado, workers Strava deshabilitados", "err", scErr)
			registerStravaWorkers = false
		} else {
			stravaForWorkers = sc
			cipherForWorkers = cfg.Strava.CipherKey
		}
	}
	riverClient, err := jobs.NewClient(ctx, pool, jobs.Deps{
		Pool:      pool,
		Config:    cfg,
		Strava:    stravaForWorkers,
		CipherKey: cipherForWorkers,
	}, registerStravaWorkers)
	if err != nil {
		return err
	}

	// AUD-04 AC: el callback OAuth encola el backfill con este adapter.
	// Se construye aquí (no en buildRouter) porque necesita el *river.Client
	// que acabamos de crear; buildRouter solo recibe la Server.
	stravaEnqueuer := &jobs.RiverEnqueuerAdapter{Client: riverClient}
	defer func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
		defer cancel()
		_ = riverClient.Stop(shutdownCtx) //nolint:errcheck
	}()

	// Start River workers
	if err := riverClient.Start(ctx); err != nil {
		return err
	}
	slog.Info("River job queue workers iniciados")

	queries := sqlc.New(pool)
	addr := ":" + cfg.Port
	srv := &http.Server{
		Addr:              addr,
		Handler:           buildRouter(cfg, pool, queries, stravaEnqueuer),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		slog.Info("servidor escuchando", "addr", addr)
		err := srv.ListenAndServe()
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
			return
		}
		close(errCh)
	}()

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
		slog.Info("apagando servidor y job queue")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
		defer cancel()
		if err := srv.Shutdown(shutdownCtx); err != nil {
			return err
		}
		// Espera a que la goroutine de ListenAndServe termine limpiamente
		// antes de salir (evita data races en arranques/paradas repetidos).
		<-errCh
		return nil
	}
}

// buildRouter monta el router con todas las dependencias cableadas.
// Se extrae de Run() para que app_test.go pueda construir un server
// equivalente sin arrancarlo.
//
// Strava (issue #14, fase 1.2) se monta solo si cfg.Strava != nil
// (vars de entorno presentes). En su ausencia, /api/v1/strava/* no
// existe como ruta — comportamiento correcto bajo el NotFound del grupo v1.
//
// stravaEnqueuer is the RiverEnqueuer that the OAuth callback uses to
// schedule the backfill after a successful token exchange (AUD-04 AC:
// "Conectar Strava encola un ImportStravaArgs"). It is non-nil because
// app.Run always wires it before calling buildRouter.
func buildRouter(cfg *config.Config, pool *pgxpool.Pool, queries sqlc.Querier, stravaEnqueuer strava.RiverEnqueuer) http.Handler {
	server := apphttp.NewServer(pool, queries, cfg)
	server.WithGPX(
		gpx.NewTransactionalSQLCStore(pool),
		gpx.Parser{},
		gpx.Validator{},
		gpx.Analyzer{},
		gpx.ClimbService{},
		gpx.RiskService{},
		gpx.TrackTypeService{},
		gpx.Hasher{},
	)

	if cfg.Strava != nil {
		stravaClient, err := strava.NewClient(strava.Config{
			ClientID:     cfg.Strava.ClientID,
			ClientSecret: cfg.Strava.ClientSecret,
			RedirectURL:  cfg.Strava.RedirectURL,
			Scopes:       cfg.Strava.Scopes,
		})
		if err != nil {
			// NewClient solo falla si ClientID/Secret están vacíos;
			// loadStravaConfig ya validó eso, así que esto es paranoia
			// defense-in-depth: log y continuamos sin Strava.
			slog.Warn("strava: cliente no inicializado, rutas OAuth deshabilitadas", "err", err)
		} else {
			store := strava.NewSQLCTokenStore(queries)
			server.WithStrava(stravaClient, store, stravaEnqueuer, cfg.Strava.CipherKey)
			slog.Info("strava: rutas OAuth habilitadas", "redirect", cfg.Strava.RedirectURL)
		}
	}

	return server.Router()
}
