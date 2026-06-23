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

	"github.com/fgjcarlos/ghamusinos/internal/config"
	"github.com/fgjcarlos/ghamusinos/internal/db"
	"github.com/fgjcarlos/ghamusinos/internal/db/sqlc"
	apphttp "github.com/fgjcarlos/ghamusinos/internal/http"
	"github.com/fgjcarlos/ghamusinos/internal/jobs"
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

	// Initialize River client for job queue
	riverClient, err := jobs.NewClient(ctx, pool)
	if err != nil {
		return err
	}
	defer func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
		defer cancel()
		riverClient.Stop(shutdownCtx)
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
		Handler:           apphttp.NewServer(pool, queries, cfg).Router(),
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
