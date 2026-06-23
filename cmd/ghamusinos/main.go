// Command ghamusinos es el entrypoint del binario único: sirve la API HTTP,
// la SPA embebida y, en fases posteriores, los workers de background.
package main

import (
	"log/slog"
	"os"

	"github.com/fgjcarlos/ghamusinos/internal/app"
	"github.com/fgjcarlos/ghamusinos/internal/logging"
)

func main() {
	// Initialize structured logging
	logging.Setup(os.Getenv("ENV"), os.Stdout)

	if err := app.Run(); err != nil {
		slog.Error("ghamusinos failed", slog.String("error", err.Error()))
		os.Exit(1)
	}
}
