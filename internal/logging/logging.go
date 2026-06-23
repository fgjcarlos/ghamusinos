// Package logging provides structured logging setup using log/slog.
package logging

import (
	"context"
	"io"
	"log/slog"
)

// loggerKey is a private context key to store the request logger.
type contextKey int

const loggerKey contextKey = 0

// Setup initializes the default slog logger based on the environment.
// In production ("production"), it uses JSONHandler; otherwise TextHandler.
// The handler writes to the provided writer.
func Setup(env string, w io.Writer) {
	var handler slog.Handler
	if env == "production" {
		handler = slog.NewJSONHandler(w, nil)
	} else {
		handler = slog.NewTextHandler(w, nil)
	}
	slog.SetDefault(slog.New(handler))
}

// FromContext returns the logger from the request context if present,
// otherwise returns the default logger.
func FromContext(ctx context.Context) *slog.Logger {
	logger, ok := ctx.Value(loggerKey).(*slog.Logger)
	if !ok {
		return slog.Default()
	}
	return logger
}
test re-trigger
