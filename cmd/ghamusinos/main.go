// Command ghamusinos es el entrypoint del binario único: sirve la API HTTP,
// la SPA embebida y, en fases posteriores, los workers de background.
package main

import (
	"log/slog"
	"os"

	"github.com/fgjcarlos/ghamusinos/internal/app"
)

func main() {
	if err := app.Run(); err != nil {
		// Logging todavía no está configurado (app.Run falla antes
		// si config.Load falla), así que usamos slog.Default con el
		// handler por defecto de Go (texto a stderr).
		slog.Error("ghamusinos: error fatal", "err", err)
		os.Exit(1)
	}
}
