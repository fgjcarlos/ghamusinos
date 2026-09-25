package logging

import (
	"bytes"
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
