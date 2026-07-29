package config

import (
	"encoding/base64"
	"strings"
	"testing"
	"time"

	"github.com/fgjcarlos/ghamusinos/internal/db"
)

func TestLoad_FailsWithoutDatabaseURL(t *testing.T) {
	t.Setenv("DATABASE_URL", "")
	t.Setenv("ENV", "production") // evita que cargue .env en disco
	t.Setenv("CLERK_JWKS_URL", "https://clerk.example.com/jwks")
	// Limpiamos también las vars del pool y Strava para que no contaminen este test.
	unsetPoolEnv(t)
	unsetStravaEnv(t)

	_, err := Load()
	if err == nil {
		t.Fatal("Load() debería fallar cuando DATABASE_URL está vacía")
	}
}

func TestLoad_DefaultValues(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://test:test@localhost/test")
	t.Setenv("ENV", "production")
	t.Setenv("PORT", "")
	t.Setenv("CLERK_JWKS_URL", "https://clerk.example.com/jwks")
	unsetPoolEnv(t)
	unsetStravaEnv(t)

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
	t.Setenv("CLERK_JWKS_URL", "https://clerk.example.com/jwks")
	unsetPoolEnv(t)
	unsetStravaEnv(t)

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
	t.Setenv("CLERK_JWKS_URL", "https://clerk.example.com/jwks")
	unsetPoolEnv(t)
	unsetStravaEnv(t)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error inesperado: %v", err)
	}
	if cfg.Env != "development" {
		t.Errorf("Env default = %q, quería %q", cfg.Env, "development")
	}
}

// TestLoad_PoolDefaults comprueba que sin variables de entorno
// relacionadas al pool, Load devuelve DefaultPoolConfig.
func TestLoad_PoolDefaults(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://test:test@localhost/test")
	t.Setenv("ENV", "production")
	t.Setenv("CLERK_JWKS_URL", "https://clerk.example.com/jwks")
	unsetPoolEnv(t)
	unsetStravaEnv(t)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error inesperado: %v", err)
	}

	want := db.DefaultPoolConfig()
	if cfg.Pool != want {
		t.Errorf("Pool = %+v, quería %+v", cfg.Pool, want)
	}
}

// TestLoad_PoolEnvOverrides comprueba que las 6 vars del pool se leen
// correctamente y se exponen en Config.Pool.
func TestLoad_PoolEnvOverrides(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://test:test@localhost/test")
	t.Setenv("ENV", "production")
	t.Setenv("CLERK_JWKS_URL", "https://clerk.example.com/jwks")
	unsetPoolEnv(t)
	unsetStravaEnv(t)

	t.Setenv("DB_POOL_MAX_CONNS", "50")
	t.Setenv("DB_POOL_MIN_CONNS", "5")
	t.Setenv("DB_POOL_MAX_CONN_LIFETIME", "2h")
	t.Setenv("DB_POOL_MAX_CONN_IDLE_TIME", "15m")
	t.Setenv("DB_POOL_CONNECT_TIMEOUT", "10s")
	t.Setenv("DB_POOL_HEALTH_CHECK_PERIOD", "30s")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error inesperado: %v", err)
	}

	want := db.PoolConfig{
		MaxConns:          50,
		MinConns:          5,
		MaxConnLifetime:   2 * time.Hour,
		MaxConnIdleTime:   15 * time.Minute,
		ConnectTimeout:    10 * time.Second,
		HealthCheckPeriod: 30 * time.Second,
	}
	if cfg.Pool != want {
		t.Errorf("Pool = %+v, quería %+v", cfg.Pool, want)
	}
}

// TestLoad_PoolInvalidInt verifica que un valor no-entero en
// DB_POOL_MAX_CONNS produce un error claro, en lugar de aplicar el
// default silenciosamente.
func TestLoad_PoolInvalidInt(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://test:test@localhost/test")
	t.Setenv("ENV", "production")
	t.Setenv("CLERK_JWKS_URL", "https://clerk.example.com/jwks")
	unsetPoolEnv(t)
	unsetStravaEnv(t)

	t.Setenv("DB_POOL_MAX_CONNS", "twenty")

	_, err := Load()
	if err == nil {
		t.Fatal("Load() debería fallar con DB_POOL_MAX_CONNS=twenty")
	}
	if !strings.Contains(err.Error(), "DB_POOL_MAX_CONNS") {
		t.Errorf("error = %q, debería mencionar la variable", err)
	}
}

// TestLoad_PoolInvalidDuration verifica que un valor no-duración en
// DB_POOL_CONNECT_TIMEOUT produce un error claro.
func TestLoad_PoolInvalidDuration(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://test:test@localhost/test")
	t.Setenv("ENV", "production")
	t.Setenv("CLERK_JWKS_URL", "https://clerk.example.com/jwks")
	unsetPoolEnv(t)
	unsetStravaEnv(t)

	t.Setenv("DB_POOL_CONNECT_TIMEOUT", "5 minutos")

	_, err := Load()
	if err == nil {
		t.Fatal("Load() debería fallar con DB_POOL_CONNECT_TIMEOUT=\"5 minutos\"")
	}
	if !strings.Contains(err.Error(), "DB_POOL_CONNECT_TIMEOUT") {
		t.Errorf("error = %q, debería mencionar la variable", err)
	}
}

// TestLoad_PoolInvalidMaxConns verifica que un MaxConns=0 (o negativo)
// es rechazado.
func TestLoad_PoolInvalidMaxConns(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://test:test@localhost/test")
	t.Setenv("ENV", "production")
	t.Setenv("CLERK_JWKS_URL", "https://clerk.example.com/jwks")
	unsetPoolEnv(t)
	unsetStravaEnv(t)

	t.Setenv("DB_POOL_MAX_CONNS", "0")

	_, err := Load()
	if err == nil {
		t.Fatal("Load() debería fallar con DB_POOL_MAX_CONNS=0")
	}
	if !strings.Contains(err.Error(), "DB_POOL_MAX_CONNS") {
		t.Errorf("error = %q, debería mencionar la variable", err)
	}
}

// TestLoad_PoolMinGreaterThanMax verifica que MinConns > MaxConns
// es rechazado.
func TestLoad_PoolMinGreaterThanMax(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://test:test@localhost/test")
	t.Setenv("ENV", "production")
	t.Setenv("CLERK_JWKS_URL", "https://clerk.example.com/jwks")
	unsetPoolEnv(t)
	unsetStravaEnv(t)

	t.Setenv("DB_POOL_MAX_CONNS", "5")
	t.Setenv("DB_POOL_MIN_CONNS", "10")

	_, err := Load()
	if err == nil {
		t.Fatal("Load() debería fallar con MinConns > MaxConns")
	}
	if !strings.Contains(err.Error(), "DB_POOL_MIN_CONNS") {
		t.Errorf("error = %q, debería mencionar la variable", err)
	}
}

// TestLoad_PoolConnectTimeoutRequired verifica que un ConnectTimeout=0
// es rechazado (sin él, el arranque puede colgarse).
func TestLoad_PoolConnectTimeoutRequired(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://test:test@localhost/test")
	t.Setenv("ENV", "production")
	t.Setenv("CLERK_JWKS_URL", "https://clerk.example.com/jwks")
	unsetPoolEnv(t)
	unsetStravaEnv(t)

	t.Setenv("DB_POOL_CONNECT_TIMEOUT", "0s")

	_, err := Load()
	if err == nil {
		t.Fatal("Load() debería fallar con DB_POOL_CONNECT_TIMEOUT=0s")
	}
	if !strings.Contains(err.Error(), "DB_POOL_CONNECT_TIMEOUT") {
		t.Errorf("error = %q, debería mencionar la variable", err)
	}
}

// TestLoad_FailsWithoutClerkJWKSURL verifica que CLERK_JWKS_URL es obligatoria.
func TestLoad_FailsWithoutClerkJWKSURL(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://test:test@localhost/test")
	t.Setenv("ENV", "production")
	t.Setenv("CLERK_JWKS_URL", "")
	unsetPoolEnv(t)
	unsetStravaEnv(t)

	_, err := Load()
	if err == nil {
		t.Fatal("Load() debería fallar cuando CLERK_JWKS_URL está vacía")
	}
	if !strings.Contains(err.Error(), "CLERK_JWKS_URL") {
		t.Errorf("error = %q, debería mencionar CLERK_JWKS_URL", err)
	}
}

// TestLoad_ClerkJWKSURLRequired verifica que Load falla si CLERK_JWKS_URL está vacía.
func TestLoad_ClerkJWKSURLRequired(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://test:test@localhost/test")
	t.Setenv("ENV", "production")
	unsetPoolEnv(t)
	unsetStravaEnv(t)

	_, err := Load()
	if err == nil {
		t.Fatal("Load() debería fallar sin CLERK_JWKS_URL")
	}
}

// TestLoad_ClerkConfigValues verifica que CLERK_JWKS_URL y CLERK_AUDIENCE se leen correctamente.
func TestLoad_ClerkConfigValues(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://test:test@localhost/test")
	t.Setenv("ENV", "production")
	t.Setenv("CLERK_JWKS_URL", "https://clerk.example.com/.well-known/jwks.json")
	t.Setenv("CLERK_AUDIENCE", "my-app-prod")
	unsetPoolEnv(t)
	unsetStravaEnv(t)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error inesperado: %v", err)
	}

	if cfg.ClerkJWKSURL != "https://clerk.example.com/.well-known/jwks.json" {
		t.Errorf("ClerkJWKSURL = %q, quería %q", cfg.ClerkJWKSURL, "https://clerk.example.com/.well-known/jwks.json")
	}
	if cfg.ClerkAudience != "my-app-prod" {
		t.Errorf("ClerkAudience = %q, quería %q", cfg.ClerkAudience, "my-app-prod")
	}
}

// TestLoad_ClerkAudienceOptional verifica que CLERK_AUDIENCE por defecto es vacío.
func TestLoad_ClerkAudienceOptional(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://test:test@localhost/test")
	t.Setenv("ENV", "production")
	t.Setenv("CLERK_JWKS_URL", "https://clerk.example.com/.well-known/jwks.json")
	t.Setenv("CLERK_AUDIENCE", "")
	unsetPoolEnv(t)
	unsetStravaEnv(t)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error inesperado: %v", err)
	}

	if cfg.ClerkAudience != "" {
		t.Errorf("ClerkAudience default = %q, quería vacío", cfg.ClerkAudience)
	}
}

// ─────────────────────────────────────────────────────────────────────────
// Strava config tests (issue #14)
// ─────────────────────────────────────────────────────────────────────────

// stravaKeyB64 es una clave AES-256 cualquiera (32 bytes) en base64.
const stravaKeyB64 = "AAECAwQFBgcICQoLDA0ODxAREhMUFRYXGBkaGxwdHh8="

func TestLoad_StravaConfigNilWhenMissing(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://test:test@localhost/test")
	t.Setenv("ENV", "production")
	t.Setenv("CLERK_JWKS_URL", "https://clerk.example.com/jwks")
	unsetPoolEnv(t)
	unsetStravaEnv(t)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error inesperado: %v", err)
	}
	if cfg.Strava != nil {
		t.Errorf("Strava debería ser nil sin vars; got %+v", cfg.Strava)
	}
}

func TestLoad_StravaConfigNilWhenOnlyIDSet(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://test:test@localhost/test")
	t.Setenv("ENV", "production")
	t.Setenv("CLERK_JWKS_URL", "https://clerk.example.com/jwks")
	unsetPoolEnv(t)
	unsetStravaEnv(t)
	t.Setenv("STRAVA_CLIENT_ID", "12345")
	t.Setenv("STRAVA_CIPHER_KEY", stravaKeyB64)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error inesperado: %v", err)
	}
	if cfg.Strava != nil {
		t.Errorf("Strava debería ser nil con solo ID; got %+v", cfg.Strava)
	}
}

func TestLoad_StravaConfigPopulated(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://test:test@localhost/test")
	t.Setenv("ENV", "production")
	t.Setenv("CLERK_JWKS_URL", "https://clerk.example.com/jwks")
	unsetPoolEnv(t)
	unsetStravaEnv(t)

	t.Setenv("STRAVA_CLIENT_ID", "12345")
	t.Setenv("STRAVA_CLIENT_SECRET", "secret")
	t.Setenv("STRAVA_REDIRECT_URL", "http://localhost/callback")
	t.Setenv("STRAVA_SCOPES", "read")
	t.Setenv("STRAVA_CIPHER_KEY", stravaKeyB64)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error inesperado: %v", err)
	}
	if cfg.Strava == nil {
		t.Fatal("Strava no se cargó")
	}
	if cfg.Strava.ClientID != "12345" || cfg.Strava.ClientSecret != "secret" {
		t.Errorf("credenciales mal leídas: %+v", cfg.Strava)
	}
	if cfg.Strava.RedirectURL != "http://localhost/callback" {
		t.Errorf("RedirectURL = %q", cfg.Strava.RedirectURL)
	}
	if cfg.Strava.Scopes != "read" {
		t.Errorf("Scopes = %q, quería 'read'", cfg.Strava.Scopes)
	}
	// La cipher key debe decodificar a 32 bytes exactos.
	decoded, _ := base64.StdEncoding.DecodeString(stravaKeyB64)
	if len(cfg.Strava.CipherKey) != len(decoded) {
		t.Errorf("CipherKey len = %d, quería %d", len(cfg.Strava.CipherKey), len(decoded))
	}
}

func TestLoad_StravaConfigDefaultScopes(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://test:test@localhost/test")
	t.Setenv("ENV", "production")
	t.Setenv("CLERK_JWKS_URL", "https://clerk.example.com/jwks")
	unsetPoolEnv(t)
	unsetStravaEnv(t)

	t.Setenv("STRAVA_CLIENT_ID", "id")
	t.Setenv("STRAVA_CLIENT_SECRET", "secret")
	t.Setenv("STRAVA_CIPHER_KEY", stravaKeyB64)
	// STRAVA_SCOPES no se setea → debe caer al default.

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error inesperado: %v", err)
	}
	if cfg.Strava == nil || cfg.Strava.Scopes != "read,activity:read" {
		t.Errorf("default scopes no aplicado: %+v", cfg.Strava)
	}
}

func TestLoad_StravaConfigRequiresCipherKey(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://test:test@localhost/test")
	t.Setenv("ENV", "production")
	t.Setenv("CLERK_JWKS_URL", "https://clerk.example.com/jwks")
	unsetPoolEnv(t)
	unsetStravaEnv(t)
	t.Setenv("STRAVA_CLIENT_ID", "id")
	t.Setenv("STRAVA_CLIENT_SECRET", "secret")
	// STRAVA_CIPHER_KEY no se setea.

	_, err := Load()
	if err == nil || !strings.Contains(err.Error(), "STRAVA_CIPHER_KEY") {
		t.Fatalf("err = %v, esperaba error mencionando STRAVA_CIPHER_KEY", err)
	}
}

func TestLoad_StravaConfigInvalidBase64CipherKey(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://test:test@localhost/test")
	t.Setenv("ENV", "production")
	t.Setenv("CLERK_JWKS_URL", "https://clerk.example.com/jwks")
	unsetPoolEnv(t)
	unsetStravaEnv(t)
	t.Setenv("STRAVA_CLIENT_ID", "id")
	t.Setenv("STRAVA_CLIENT_SECRET", "secret")
	t.Setenv("STRAVA_CIPHER_KEY", "esto no es base64 !@#$")

	_, err := Load()
	if err == nil || !strings.Contains(err.Error(), "STRAVA_CIPHER_KEY") {
		t.Fatalf("err = %v, esperaba error de base64", err)
	}
}

func TestLoad_StravaConfigInvalidCipherKeyLength(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://test:test@localhost/test")
	t.Setenv("ENV", "production")
	t.Setenv("CLERK_JWKS_URL", "https://clerk.example.com/jwks")
	unsetPoolEnv(t)
	unsetStravaEnv(t)
	t.Setenv("STRAVA_CLIENT_ID", "id")
	t.Setenv("STRAVA_CLIENT_SECRET", "secret")
	// 16 bytes en base64 = 24 chars pero la validación es sobre el decoded len.
	// "AAAAAAAAAAAAAAAAAAAAAA==" decodifica a 16 bytes (no 32).
	t.Setenv("STRAVA_CIPHER_KEY", "AAAAAAAAAAAAAAAAAAAAAA==")

	_, err := Load()
	if err == nil || !strings.Contains(err.Error(), "32 bytes") {
		t.Fatalf("err = %v, esperaba error mencionando '32 bytes'", err)
	}
}

// unsetPoolEnv limpia todas las variables de entorno relacionadas al pool
// para que cada test empiece desde un estado conocido (en CI el entorno
// puede tenerlas pre-seteadas).
func unsetPoolEnv(t *testing.T) {
	t.Helper()
	for _, k := range []string{
		"DB_POOL_MAX_CONNS",
		"DB_POOL_MIN_CONNS",
		"DB_POOL_MAX_CONN_LIFETIME",
		"DB_POOL_MAX_CONN_IDLE_TIME",
		"DB_POOL_CONNECT_TIMEOUT",
		"DB_POOL_HEALTH_CHECK_PERIOD",
	} {
		t.Setenv(k, "")
	}
}

// unsetStravaEnv limpia las variables de entorno de Strava para que cada
// test empiece desde un estado conocido. Los tests que cubren la config
// de Strava setean sus propias vars; los demás la dejan en nil.
func unsetStravaEnv(t *testing.T) {
	t.Helper()
	for _, k := range []string{
		"STRAVA_CLIENT_ID",
		"STRAVA_CLIENT_SECRET",
		"STRAVA_REDIRECT_URL",
		"STRAVA_SCOPES",
		"STRAVA_CIPHER_KEY",
	} {
		t.Setenv(k, "")
	}
}