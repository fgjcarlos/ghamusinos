// Package app conecta configuración, servidor HTTP y dependencias del binario.
package app

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	apphttp "github.com/fgjcarlos/ghamusinos/internal/http"
)

const shutdownTimeout = 10 * time.Second

// Run arranca el servidor HTTP y bloquea hasta que se recibe una señal de
// apagado (SIGINT/SIGTERM), momento en el que hace un shutdown ordenado.
func Run() error {
	addr := ":" + port()

	srv := &http.Server{
		Addr:              addr,
		Handler:           apphttp.NewRouter(),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

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
		slog.Info("apagando servidor")
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

// port devuelve el puerto de escucha. La configuración central se formaliza en
// una issue posterior; de momento se lee PORT del entorno con un valor por
// defecto.
func port() string {
	if p := os.Getenv("PORT"); p != "" {
		return p
	}
	return "8080"
}
