// Package logging configura el handler global de slog para el binario.
// Es un punto único de configuración: cualquier llamada a slog.Info,
// slog.Error, etc. en el árbol principal acabará usando el handler
// instalado aquí.
package logging

import (
	"io"
	"log/slog"
	"os"
)

// Niveles soportados a través de LOG_LEVEL.
const (
	EnvProduction  = "production"
	EnvDevelopment = "development"
)

// NewHandler devuelve un slog.Handler adecuado al entorno:
//   - production: JSONHandler (parseable por agregadores / log shippers).
//   - development: TextHandler (legible por humanos en stdout/stderr).
//
// Acepta un io.Writer para poder testear sin redirigir os.Stderr.
// En el binario se llama con os.Stderr (ver Setup).
func NewHandler(env string, w io.Writer) slog.Handler {
	opts := &slog.HandlerOptions{Level: slog.LevelInfo}
	if env == EnvProduction {
		return slog.NewJSONHandler(w, opts)
	}
	return slog.NewTextHandler(w, opts)
}

// Setup instala el handler por defecto de slog según el entorno.
// Llamar al arrancar el binario, antes de cualquier slog.Info/Error.
//
// Usa os.Stderr: los logs son stderr por convención y para que stdout
// quede libre para datos (p.ej. respuestas del cmd/migrate que escribe
// el estado de las migraciones a stdout en el futuro).
func Setup(env string) {
	slog.SetDefault(slog.New(NewHandler(env, os.Stderr)))
}
