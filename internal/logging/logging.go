// Package logging provides structured logging setup using log/slog.
package logging

import (
	"io"
	"log/slog"
)

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
