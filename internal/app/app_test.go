package app

import (
	"testing"
)

func TestPortDefault(t *testing.T) {
	t.Setenv("PORT", "")
	if got := port(); got != "8080" {
		t.Fatalf("port() = %q, quería %q", got, "8080")
	}
}

func TestPortFromEnv(t *testing.T) {
	t.Setenv("PORT", "9090")
	if got := port(); got != "9090" {
		t.Fatalf("port() = %q, quería %q", got, "9090")
	}
}
