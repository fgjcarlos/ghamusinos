package logging

import (
	"bytes"
	"context"
	"log/slog"
	"testing"
)

// TestSetup_ProductionJSONHandler tests that Setup returns a JSONHandler in production env.
// SPEC: Scenario "Production JSON handler"
func TestSetup_ProductionJSONHandler(t *testing.T) {
	// Save the current default logger
	oldDefault := slog.Default()
	defer slog.SetDefault(oldDefault)

	buf := &bytes.Buffer{}
	Setup("production", buf)

	handler := slog.Default().Handler()
	if _, ok := handler.(*slog.JSONHandler); !ok {
		t.Errorf("expected *slog.JSONHandler in production, got %T", handler)
	}
}

// TestSetup_DevelopmentTextHandler tests that Setup returns a TextHandler in non-production env.
// SPEC: Scenario "Development text handler"
func TestSetup_DevelopmentTextHandler(t *testing.T) {
	// Save the current default logger
	oldDefault := slog.Default()
	defer slog.SetDefault(oldDefault)

	buf := &bytes.Buffer{}
	Setup("development", buf)

	handler := slog.Default().Handler()
	if _, ok := handler.(*slog.TextHandler); !ok {
		t.Errorf("expected *slog.TextHandler in development, got %T", handler)
	}
}

// TestFromContext_WithoutLogger returns the default logger when context has no logger.
// SPEC: Scenario "FromContext fallback"
func TestFromContext_WithoutLogger(t *testing.T) {
	ctx := context.Background()
	logger := FromContext(ctx)

	if logger == nil {
		t.Fatal("expected non-nil logger from FromContext")
	}

	// Should return the default (not nil)
	if logger != slog.Default() {
		t.Error("expected FromContext to return slog.Default() when context has no logger")
	}
}

// TestFromContext_WithLoggerInContext returns the logger from context if set.
// SPEC: Scenario "FromContext with logger in context"
func TestFromContext_WithLoggerInContext(t *testing.T) {
	oldDefault := slog.Default()
	defer slog.SetDefault(oldDefault)

	buf := &bytes.Buffer{}
	Setup("development", buf)
	ctxLogger := slog.Default()

	// Create a context with the logger (using custom key)
	ctx := contextWithLogger(ctxLogger)
	logger := FromContext(ctx)

	if logger != ctxLogger {
		t.Error("expected FromContext to return the logger from context")
	}
}

// Helper to set a logger in context. This will be tested via our FromContext implementation.
func contextWithLogger(logger *slog.Logger) context.Context {
	return context.WithValue(context.Background(), loggerKey, logger)
}
