package config

import (
	"testing"
)

func TestLoad_FailsWithoutDatabaseURL(t *testing.T) {
	t.Setenv("DATABASE_URL", "")
	t.Setenv("ENV", "production") // evita que cargue .env en disco

	_, err := Load()
	if err == nil {
		t.Fatal("Load() debería fallar cuando DATABASE_URL está vacía")
	}
}

func TestLoad_DefaultValues(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://test:test@localhost/test")
	t.Setenv("ENV", "production")
	t.Setenv("PORT", "")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error inesperado: %v", err)
	}

	if cfg.Port != "8080" {
		t.Errorf("Port default = %q, quería %q", cfg.Port, "8080")
	}
	if cfg.Env != "production" {
		t.Errorf("Env = %q, quería %q", cfg.Env, "production")
	}
}

func TestLoad_RespectsEnvValues(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://user:pass@host:5432/db")
	t.Setenv("ENV", "production")
	t.Setenv("PORT", "9090")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error inesperado: %v", err)
	}

	if cfg.DatabaseURL != "postgres://user:pass@host:5432/db" {
		t.Errorf("DatabaseURL = %q, quería %q", cfg.DatabaseURL, "postgres://user:pass@host:5432/db")
	}
	if cfg.Port != "9090" {
		t.Errorf("Port = %q, quería %q", cfg.Port, "9090")
	}
	if cfg.Env != "production" {
		t.Errorf("Env = %q, quería %q", cfg.Env, "production")
	}
}

func TestLoad_EnvDefaultIsDevelopment(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://test:test@localhost/test")
	t.Setenv("ENV", "")
	// No importa si carga .env; el resultado debe ser "development"
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error inesperado: %v", err)
	}
	if cfg.Env != "development" {
		t.Errorf("Env default = %q, quería %q", cfg.Env, "development")
	}
}
