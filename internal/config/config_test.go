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
	t.Setenv("CLERK_ISSUER", "https://clerk.example.com")
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
	t.Setenv("CLERK_ISSUER", "https://clerk.example.com")
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
	t.Setenv("CLERK_ISSUER", "https://clerk.example.com")
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
	t.Setenv("CLERK_ISSUER", "https://clerk.example.com")
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
	t.Setenv("CLERK_ISSUER", "https://clerk.example.com")
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
	t.Setenv("CLERK_ISSUER", "https://clerk.example.com")
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
	t.Setenv("CLERK_ISSUER", "https://clerk.example.com")
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
	t.Setenv("CLERK_ISSUER", "https://clerk.example.com")
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
	t.Setenv("CLERK_ISSUER", "https://clerk.example.com")
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
	t.Setenv("CLERK_ISSUER", "https://clerk.example.com")
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
	t.Setenv("CLERK_ISSUER", "https://clerk.example.com")
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

// TestLoad_ClerkIssuerRequired verifica que CLERK_ISSUER es obligatoria
// (issue #167). Sin el emisor, un token firmado por cualquier clave del
// JWKS configurado entraría sin pertenecer al tenant.
func TestLoad_ClerkIssuerRequired(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://test:test@localhost/test")
	t.Setenv("ENV", "production")
	t.Setenv("CLERK_JWKS_URL", "https://clerk.example.com/jwks")
	// CLERK_ISSUER deliberadamente ausente.
	unsetPoolEnv(t)
	unsetStravaEnv(t)

	_, err := Load()
	if err == nil {
		t.Fatal("Load() debería fallar cuando CLERK_ISSUER está vacía")
	}
	if !strings.Contains(err.Error(), "CLERK_ISSUER") {
		t.Errorf("error = %q, debería mencionar CLERK_ISSUER", err)
	}
}

// TestLoad_ClerkConfigValues verifica que CLERK_JWKS_URL y CLERK_AUDIENCE se leen correctamente.
func TestLoad_ClerkConfigValues(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://test:test@localhost/test")
	t.Setenv("ENV", "production")
	t.Setenv("CLERK_JWKS_URL", "https://clerk.example.com/.well-known/jwks.json")
	t.Setenv("CLERK_ISSUER", "https://clerk.example.com")
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
	t.Setenv("CLERK_ISSUER", "https://clerk.example.com")
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
	t.Setenv("CLERK_ISSUER", "https://clerk.example.com")
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
	t.Setenv("CLERK_ISSUER", "https://clerk.example.com")
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
	t.Setenv("CLERK_ISSUER", "https://clerk.example.com")
	unsetPoolEnv(t)
	unsetStravaEnv(t)

	t.Setenv("STRAVA_CLIENT_ID", "12345")
	t.Setenv("STRAVA_CLIENT_SECRET", "secret")
	t.Setenv("STRAVA_REDIRECT_URL", "http://localhost/callback")
	t.Setenv("STRAVA_SCOPES", "read")
	t.Setenv("STRAVA_CIPHER_KEY", stravaKeyB64)
	t.Setenv("STRAVA_WEBHOOK_VERIFY_TOKEN", "verify-token-123")

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
	if cfg.Strava.WebhookVerifyToken != "verify-token-123" {
		t.Errorf("WebhookVerifyToken = %q, want 'verify-token-123'", cfg.Strava.WebhookVerifyToken)
	}
}

func TestLoad_StravaConfigDefaultScopes(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://test:test@localhost/test")
	t.Setenv("ENV", "production")
	t.Setenv("CLERK_JWKS_URL", "https://clerk.example.com/jwks")
	t.Setenv("CLERK_ISSUER", "https://clerk.example.com")
	unsetPoolEnv(t)
	unsetStravaEnv(t)

	t.Setenv("STRAVA_CLIENT_ID", "id")
	t.Setenv("STRAVA_CLIENT_SECRET", "secret")
	t.Setenv("STRAVA_CIPHER_KEY", stravaKeyB64)
	t.Setenv("STRAVA_WEBHOOK_VERIFY_TOKEN", "verify-token")
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
	t.Setenv("CLERK_ISSUER", "https://clerk.example.com")
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

func TestLoad_StravaConfigRequiresWebhookVerifyToken(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://test:test@localhost/test")
	t.Setenv("ENV", "production")
	t.Setenv("CLERK_JWKS_URL", "https://clerk.example.com/jwks")
	t.Setenv("CLERK_ISSUER", "https://clerk.example.com")
	unsetPoolEnv(t)
	unsetStravaEnv(t)
	t.Setenv("STRAVA_CLIENT_ID", "id")
	t.Setenv("STRAVA_CLIENT_SECRET", "secret")
	t.Setenv("STRAVA_CIPHER_KEY", stravaKeyB64)

	_, err := Load()
	if err == nil || !strings.Contains(err.Error(), "STRAVA_WEBHOOK_VERIFY_TOKEN") {
		t.Fatalf("err = %v, want an error mentioning STRAVA_WEBHOOK_VERIFY_TOKEN", err)
	}
}

func TestLoad_StravaConfigInvalidBase64CipherKey(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://test:test@localhost/test")
	t.Setenv("ENV", "production")
	t.Setenv("CLERK_JWKS_URL", "https://clerk.example.com/jwks")
	t.Setenv("CLERK_ISSUER", "https://clerk.example.com")
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
	t.Setenv("CLERK_ISSUER", "https://clerk.example.com")
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
		"STRAVA_WEBHOOK_VERIFY_TOKEN",
	} {
		t.Setenv(k, "")
	}
}

// ─────────────────────────────────────────────────────────────────────────
// TrustedProxies (issue #64)
// ─────────────────────────────────────────────────────────────────────────

func TestParseTrustedProxies_Empty(t *testing.T) {
	if got := parseTrustedProxies(""); got != nil {
		t.Errorf("parseTrustedProxies(\"\") = %v, quería nil (fail-closed)", got)
	}
}

func TestParseTrustedProxies_SingleCIDR(t *testing.T) {
	got := parseTrustedProxies("10.0.0.0/8")
	if len(got) != 1 || got[0] != "10.0.0.0/8" {
		t.Errorf("got %v, quería [10.0.0.0/8]", got)
	}
}

func TestParseTrustedProxies_MultipleCIDRsTrimmed(t *testing.T) {
	got := parseTrustedProxies(" 10.0.0.0/8 , 172.16.0.0/12 , 192.168.0.0/16 ")
	want := []string{"10.0.0.0/8", "172.16.0.0/12", "192.168.0.0/16"}
	if len(got) != len(want) {
		t.Fatalf("len(got) = %d, quería %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("got[%d] = %q, quería %q", i, got[i], want[i])
		}
	}
}

func TestParseTrustedProxies_SkipsEmptyEntries(t *testing.T) {
	got := parseTrustedProxies("10.0.0.0/8,,, 172.16.0.0/12 ,")
	want := []string{"10.0.0.0/8", "172.16.0.0/12"}
	if len(got) != len(want) {
		t.Fatalf("len(got) = %d, quería %d (got=%v)", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("got[%d] = %q, quería %q", i, got[i], want[i])
		}
	}
}

func TestLoad_PortNonNumericFails(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://test:***@localhost/test")
	t.Setenv("ENV", "production")
	t.Setenv("CLERK_JWKS_URL", "https://clerk.example.com/jwks")
	t.Setenv("CLERK_ISSUER", "https://clerk.example.com")
	unsetPoolEnv(t)
	unsetStravaEnv(t)
	t.Setenv("PORT", "abc")

	_, err := Load()
	if err == nil {
		t.Fatal("Load() debería fallar con PORT=abc")
	}
	if !strings.Contains(err.Error(), "PORT") {
		t.Errorf("error = %q, debería mencionar PORT", err.Error())
	}
}

func TestLoad_PortOutOfRangeFails(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://test:***@localhost/test")
	t.Setenv("ENV", "production")
	t.Setenv("CLERK_JWKS_URL", "https://clerk.example.com/jwks")
	t.Setenv("CLERK_ISSUER", "https://clerk.example.com")
	unsetPoolEnv(t)
	unsetStravaEnv(t)
	t.Setenv("PORT", "99999")

	_, err := Load()
	if err == nil {
		t.Fatal("Load() debería fallar con PORT=99999 (> 65535)")
	}
	if !strings.Contains(err.Error(), "fuera de rango") {
		t.Errorf("error = %q, debería mencionar 'fuera de rango'", err.Error())
	}
}

func TestLoad_TrustedProxiesDefaultEmpty(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://test:***@localhost/test")
	t.Setenv("ENV", "production")
	t.Setenv("CLERK_JWKS_URL", "https://clerk.example.com/jwks")
	t.Setenv("CLERK_ISSUER", "https://clerk.example.com")
	unsetPoolEnv(t)
	unsetStravaEnv(t)
	t.Setenv("TRUSTED_PROXIES", "")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if len(cfg.TrustedProxies) != 0 {
		t.Errorf("TrustedProxies por defecto debería estar vacío; got %v", cfg.TrustedProxies)
	}
}

func TestLoad_TrustedProxiesParsed(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://test:***@localhost/test")
	t.Setenv("ENV", "production")
	t.Setenv("CLERK_JWKS_URL", "https://clerk.example.com/jwks")
	t.Setenv("CLERK_ISSUER", "https://clerk.example.com")
	unsetPoolEnv(t)
	unsetStravaEnv(t)
	t.Setenv("TRUSTED_PROXIES", "10.0.0.0/8,127.0.0.1/32")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if len(cfg.TrustedProxies) != 2 ||
		cfg.TrustedProxies[0] != "10.0.0.0/8" ||
		cfg.TrustedProxies[1] != "127.0.0.1/32" {
		t.Errorf("TrustedProxies = %v, quería [10.0.0.0/8 127.0.0.1/32]", cfg.TrustedProxies)
	}
}

// ─────────────────────────────────────────────────────────────────────────
// SecurityHeaders + MaxBodyBytes (issue #26)
// ─────────────────────────────────────────────────────────────────────────

func TestLoad_SecurityHeadersDefaults(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://test:***@localhost/test")
	t.Setenv("ENV", "production")
	t.Setenv("CLERK_JWKS_URL", "https://clerk.example.com/jwks")
	t.Setenv("CLERK_ISSUER", "https://clerk.example.com")
	unsetPoolEnv(t)
	unsetStravaEnv(t)
	t.Setenv("SECURITY_HSTS_ENABLED", "")
	t.Setenv("SECURITY_CSP_REPORT_ONLY", "")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if cfg.Security.HSTSEnabled {
		t.Errorf("Security.HSTSEnabled = true, quería false (default)")
	}
	if cfg.Security.CSPReportOnly {
		t.Errorf("Security.CSPReportOnly = true, quería false (default)")
	}
	if cfg.MaxBodyBytes != 10<<20 {
		t.Errorf("MaxBodyBytes = %d, quería %d (default 10 MiB)", cfg.MaxBodyBytes, 10<<20)
	}
}

func TestLoad_SecurityHeadersHSTSEnabled(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://test:***@localhost/test")
	t.Setenv("ENV", "production")
	t.Setenv("CLERK_JWKS_URL", "https://clerk.example.com/jwks")
	t.Setenv("CLERK_ISSUER", "https://clerk.example.com")
	unsetPoolEnv(t)
	unsetStravaEnv(t)
	t.Setenv("SECURITY_HSTS_ENABLED", "true")
	t.Setenv("SECURITY_CSP_REPORT_ONLY", "true")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if !cfg.Security.HSTSEnabled {
		t.Errorf("Security.HSTSEnabled = false, quería true")
	}
	if !cfg.Security.CSPReportOnly {
		t.Errorf("Security.CSPReportOnly = false, quería true")
	}
}

func TestLoad_MaxBodyBytesOverride(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://test:***@localhost/test")
	t.Setenv("ENV", "production")
	t.Setenv("CLERK_JWKS_URL", "https://clerk.example.com/jwks")
	t.Setenv("CLERK_ISSUER", "https://clerk.example.com")
	unsetPoolEnv(t)
	unsetStravaEnv(t)
	t.Setenv("MAX_BODY_BYTES", "5242880")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if cfg.MaxBodyBytes != 5*1024*1024 {
		t.Errorf("MaxBodyBytes = %d, quería %d", cfg.MaxBodyBytes, 5*1024*1024)
	}
}

func TestLoad_MaxBodyBytesInvalidFallsBackToDefault(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://test:***@localhost/test")
	t.Setenv("ENV", "production")
	t.Setenv("CLERK_JWKS_URL", "https://clerk.example.com/jwks")
	t.Setenv("CLERK_ISSUER", "https://clerk.example.com")
	unsetPoolEnv(t)
	unsetStravaEnv(t)
	t.Setenv("MAX_BODY_BYTES", "esto no es un número")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() no debería fallar con MAX_BODY_BYTES inválido (usa default): %v", err)
	}
	if cfg.MaxBodyBytes != 10<<20 {
		t.Errorf("MaxBodyBytes = %d, quería default %d tras valor inválido", cfg.MaxBodyBytes, 10<<20)
	}
}

// ─────────────────────────────────────────────────────────────────────────
// AUTH_DISABLED (bypass local de Clerk)
// ─────────────────────────────────────────────────────────────────────────

func TestLoad_AuthDisabledDefaultsFalse(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://test:***@localhost/test")
	t.Setenv("ENV", "production")
	t.Setenv("CLERK_JWKS_URL", "https://clerk.example.com/jwks")
	t.Setenv("CLERK_ISSUER", "https://clerk.example.com")
	unsetPoolEnv(t)
	unsetStravaEnv(t)
	t.Setenv("AUTH_DISABLED", "")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if cfg.AuthDisabled {
		t.Errorf("AuthDisabled = true sin env var, quería false (fail-closed)")
	}
}

func TestLoad_AuthDisabledTruthyValues(t *testing.T) {
	cases := []string{"1", "true", "TRUE", "yes", "Yes", "on"}
	for _, v := range cases {
		t.Run(v, func(t *testing.T) {
			t.Setenv("DATABASE_URL", "postgres://test:***@localhost/test")
			t.Setenv("ENV", "production")
			t.Setenv("CLERK_JWKS_URL", "https://clerk.example.com/jwks")
			t.Setenv("CLERK_ISSUER", "https://clerk.example.com")
			unsetPoolEnv(t)
			unsetStravaEnv(t)
			t.Setenv("AUTH_DISABLED", v)

			cfg, err := Load()
			if err != nil {
				t.Fatalf("Load() error: %v", err)
			}
			if !cfg.AuthDisabled {
				t.Errorf("AuthDisabled = false para AUTH_DISABLED=%q, quería true", v)
			}
		})
	}
}

// ─────────────────────────────────────────────────────────────────────────
// AI config (fase 1.5 · issue #17, unidad T4)
// ─────────────────────────────────────────────────────────────────────────

// unsetAIEnv limpia todas las variables de entorno de la IA para que cada
// test empiece desde un estado conocido (el entorno de CI puede tenerlas
// pre-seteadas).
func unsetAIEnv(t *testing.T) {
	t.Helper()
	for _, k := range []string{
		"AI_ENABLED",
		"AI_PROVIDER",
		"AI_MODEL",
		"AI_OPENAI_API_KEY",
		"AI_CLAUDE_API_KEY",
		"AI_OPENROUTER_API_KEY",
		"AI_TIMEOUT",
		"AI_MAX_RETRIES",
	} {
		t.Setenv(k, "")
	}
}

// prepareAIEnv deja el entorno con el mínimo obligatorio (DATABASE_URL,
// Clerk) y sin ninguna variable de IA.
func prepareAIEnv(t *testing.T) {
	t.Helper()
	t.Setenv("DATABASE_URL", "postgres://test:***@localhost/test")
	t.Setenv("ENV", "production")
	t.Setenv("CLERK_JWKS_URL", "https://clerk.example.com/jwks")
	t.Setenv("CLERK_ISSUER", "https://clerk.example.com")
	unsetPoolEnv(t)
	unsetStravaEnv(t)
	unsetAIEnv(t)
}

// loadConfigWithAI carga la configuración. Falla el test si Load() devuelve
// error: una misconfiguración de IA nunca puede impedir el arranque (S7).
func loadConfigWithAI(t *testing.T) *Config {
	t.Helper()
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() no debería fallar con la configuración de IA de este test: %v", err)
	}
	return cfg
}

// TestLoad_AI_DisabledByDefault comprueba que el flag global de la IA
// está apagado por defecto y que la IA queda en estado "disabled".
func TestLoad_AI_DisabledByDefault(t *testing.T) {
	prepareAIEnv(t)

	cfg := loadConfigWithAI(t)

	if cfg.AI.Enabled {
		t.Error("AI.Enabled = true sin AI_ENABLED, quería false (flag global apagado por defecto)")
	}
	if cfg.AI.Available {
		t.Error("AI.Available = true con el flag apagado, quería false")
	}
	if got := cfg.AI.Status(); got != AIStatusDisabled {
		t.Errorf("AI.Status() = %q, quería %q", got, AIStatusDisabled)
	}
	if cfg.AI.APIKey != "" {
		t.Errorf("AI.APIKey = %q con el flag apagado, quería vacío", cfg.AI.APIKey)
	}
	if cfg.AI.UnavailableReason == "" {
		t.Error("AI.UnavailableReason vacío: el estado no disponible debe poder explicarse")
	}
}

// TestLoad_AI_EnabledWithValidProviderAndKey comprueba la ruta feliz:
// flag + proveedor válido + credencial => IA disponible con proveedor,
// modelo y clave cargados.
func TestLoad_AI_EnabledWithValidProviderAndKey(t *testing.T) {
	prepareAIEnv(t)
	t.Setenv("AI_ENABLED", "true")
	t.Setenv("AI_PROVIDER", "openai")
	t.Setenv("AI_MODEL", "gpt-test-model")
	t.Setenv("AI_OPENAI_API_KEY", "sk-test-key")

	cfg := loadConfigWithAI(t)
	if !cfg.AI.Enabled {
		t.Error("AI.Enabled = false con AI_ENABLED=true")
	}
	if !cfg.AI.Available {
		t.Errorf("AI.Available = false con configuración válida; avisos: %v", cfg.AI.Warnings)
	}
	if got := cfg.AI.Status(); got != AIStatusReady {
		t.Errorf("AI.Status() = %q, quería %q", got, AIStatusReady)
	}
	if cfg.AI.Provider != AIProviderOpenAI {
		t.Errorf("AI.Provider = %q, quería %q", cfg.AI.Provider, AIProviderOpenAI)
	}
	if cfg.AI.Model != "gpt-test-model" {
		t.Errorf("AI.Model = %q, quería %q", cfg.AI.Model, "gpt-test-model")
	}
	if cfg.AI.APIKey != "sk-test-key" {
		t.Errorf("AI.APIKey = %q, quería %q", cfg.AI.APIKey, "sk-test-key")
	}
	if len(cfg.AI.Warnings) != 0 {
		t.Errorf("AI.Warnings = %v, quería sin avisos con configuración válida", cfg.AI.Warnings)
	}
}

// TestLoad_AI_InvalidProviderDoesNotBlockBoot comprueba que un
// proveedor desconocido degrada la IA a "unavailable" con un aviso
// explícito, sin impedir el arranque (S7).
func TestLoad_AI_InvalidProviderDoesNotBlockBoot(t *testing.T) {
	prepareAIEnv(t)
	t.Setenv("AI_ENABLED", "true")
	t.Setenv("AI_PROVIDER", "gemini")
	t.Setenv("AI_OPENAI_API_KEY", "sk-test-key")

	cfg := loadConfigWithAI(t)
	if !cfg.AI.Enabled {
		t.Error("AI.Enabled = false: el flag sigue siendo la fuente de verdad")
	}
	if cfg.AI.Available {
		t.Error("AI.Available = true con un proveedor inválido, quería false")
	}
	if got := cfg.AI.Status(); got != AIStatusUnavailable {
		t.Errorf("AI.Status() = %q, quería %q", got, AIStatusUnavailable)
	}
	if cfg.AI.Provider != "" {
		t.Errorf("AI.Provider = %q, quería vacío con un proveedor inválido", cfg.AI.Provider)
	}
	if !containsWarning(cfg.AI.Warnings, "AI_PROVIDER") {
		t.Errorf("AI.Warnings = %v, quería un aviso que mencione AI_PROVIDER", cfg.AI.Warnings)
	}
	if !containsWarning(cfg.AI.Warnings, "gemini") {
		t.Errorf("AI.Warnings = %v, quería un aviso que nombre el proveedor inválido", cfg.AI.Warnings)
	}
}

// TestLoad_AI_MissingProviderWithFlagOnIsUnavailable comprueba que el
// flag encendido sin proveedor deja la IA no disponible y avisa.
func TestLoad_AI_MissingProviderWithFlagOnIsUnavailable(t *testing.T) {
	prepareAIEnv(t)
	t.Setenv("AI_ENABLED", "true")

	cfg := loadConfigWithAI(t)
	if cfg.AI.Available {
		t.Error("AI.Available = true sin proveedor configurado, quería false")
	}
	if got := cfg.AI.Status(); got != AIStatusUnavailable {
		t.Errorf("AI.Status() = %q, quería %q", got, AIStatusUnavailable)
	}
	if !containsWarning(cfg.AI.Warnings, "AI_PROVIDER") {
		t.Errorf("AI.Warnings = %v, quería un aviso que mencione AI_PROVIDER", cfg.AI.Warnings)
	}
}

// TestLoad_AI_MissingKeyForSelectedProviderMarksUnavailable comprueba
// que la falta de credencial del proveedor seleccionado no impide el
// arranque y marca la IA como no disponible.
func TestLoad_AI_MissingKeyForSelectedProviderMarksUnavailable(t *testing.T) {
	prepareAIEnv(t)
	t.Setenv("AI_ENABLED", "true")
	t.Setenv("AI_PROVIDER", "claude")
	t.Setenv("AI_MODEL", "claude-test-model")
	// Credencial de otro proveedor: no sirve para claude.
	t.Setenv("AI_OPENAI_API_KEY", "sk-test-key")

	cfg := loadConfigWithAI(t)
	if cfg.AI.Available {
		t.Error("AI.Available = true sin la credencial del proveedor seleccionado, quería false")
	}
	if got := cfg.AI.Status(); got != AIStatusUnavailable {
		t.Errorf("AI.Status() = %q, quería %q", got, AIStatusUnavailable)
	}
	if cfg.AI.APIKey != "" {
		t.Errorf("AI.APIKey = %q, quería vacío (la clave de otro proveedor no se usa)", cfg.AI.APIKey)
	}
	if !containsWarning(cfg.AI.Warnings, "AI_CLAUDE_API_KEY") {
		t.Errorf("AI.Warnings = %v, quería un aviso que mencione AI_CLAUDE_API_KEY", cfg.AI.Warnings)
	}
}

// TestLoad_AI_IgnoresKeysOfOtherProviders comprueba que sólo se carga la
// credencial del proveedor seleccionado.
func TestLoad_AI_IgnoresKeysOfOtherProviders(t *testing.T) {
	t.Run("openai seleccionado", func(t *testing.T) {
		prepareAIEnv(t)
		t.Setenv("AI_ENABLED", "true")
		t.Setenv("AI_PROVIDER", "openai")
		t.Setenv("AI_MODEL", "openai-test-model")
		t.Setenv("AI_OPENAI_API_KEY", "sk-openai")
		t.Setenv("AI_CLAUDE_API_KEY", "sk-claude")
		t.Setenv("AI_OPENROUTER_API_KEY", "sk-openrouter")

		cfg := loadConfigWithAI(t)
		if cfg.AI.APIKey != "sk-openai" {
			t.Errorf("AI.APIKey = %q, quería %q (sólo la del proveedor seleccionado)", cfg.AI.APIKey, "sk-openai")
		}
	})

	t.Run("claude seleccionado", func(t *testing.T) {
		prepareAIEnv(t)
		t.Setenv("AI_ENABLED", "true")
		t.Setenv("AI_PROVIDER", "claude")
		t.Setenv("AI_MODEL", "claude-test-model")
		t.Setenv("AI_OPENAI_API_KEY", "sk-openai")
		t.Setenv("AI_OPENROUTER_API_KEY", "sk-openrouter")

		cfg := loadConfigWithAI(t)
		if cfg.AI.APIKey != "" {
			t.Errorf("AI.APIKey = %q, quería vacío (no hay credencial de claude)", cfg.AI.APIKey)
		}
		if cfg.AI.Available {
			t.Error("AI.Available = true sin credencial de claude, quería false")
		}
	})
}

// TestLoad_AI_DisabledDoesNotRequireCredential comprueba que con el flag
// apagado no hace falta ninguna credencial y que el producto no queda
// con avisos por la falta de claves.
func TestLoad_AI_DisabledDoesNotRequireCredential(t *testing.T) {
	prepareAIEnv(t)
	t.Setenv("AI_ENABLED", "false")
	// Ni proveedor ni credenciales: es el estado por defecto de un
	// despliegue sin IA.

	cfg := loadConfigWithAI(t)
	if cfg.AI.Available {
		t.Error("AI.Available = true con el flag apagado, quería false")
	}
	if got := cfg.AI.Status(); got != AIStatusDisabled {
		t.Errorf("AI.Status() = %q, quería %q", got, AIStatusDisabled)
	}
	for _, w := range cfg.AI.Warnings {
		if strings.Contains(w, "API_KEY") {
			t.Errorf("con la IA apagada no debe haber avisos sobre credenciales: %q", w)
		}
	}
}

// TestLoad_AI_InvalidTimeoutFallsBackToDefault comprueba que una
// duración inválida degrada al valor por defecto con un aviso, sin
// impedir el arranque.
func TestLoad_AI_InvalidTimeoutFallsBackToDefault(t *testing.T) {
	prepareAIEnv(t)
	t.Setenv("AI_TIMEOUT", "5 minutos")

	cfg := loadConfigWithAI(t)
	if cfg.AI.Timeout != DefaultAITimeout {
		t.Errorf("AI.Timeout = %v, quería el valor por defecto %v", cfg.AI.Timeout, DefaultAITimeout)
	}
	if !containsWarning(cfg.AI.Warnings, "AI_TIMEOUT") {
		t.Errorf("AI.Warnings = %v, quería un aviso que mencione AI_TIMEOUT", cfg.AI.Warnings)
	}
}

// TestLoad_AI_TimeoutOverride comprueba que AI_TIMEOUT se lee.
func TestLoad_AI_TimeoutOverride(t *testing.T) {
	prepareAIEnv(t)
	t.Setenv("AI_TIMEOUT", "45s")

	cfg := loadConfigWithAI(t)
	if cfg.AI.Timeout != 45*time.Second {
		t.Errorf("AI.Timeout = %v, quería 45s", cfg.AI.Timeout)
	}
}

// TestLoad_AI_InvalidMaxRetriesFallsBackToDefault comprueba que un
// entero inválido degrada al valor por defecto con un aviso.
func TestLoad_AI_InvalidMaxRetriesFallsBackToDefault(t *testing.T) {
	prepareAIEnv(t)
	t.Setenv("AI_MAX_RETRIES", "muchos")

	cfg := loadConfigWithAI(t)
	if cfg.AI.MaxRetries != DefaultAIMaxRetries {
		t.Errorf("AI.MaxRetries = %d, quería el valor por defecto %d", cfg.AI.MaxRetries, DefaultAIMaxRetries)
	}
	if !containsWarning(cfg.AI.Warnings, "AI_MAX_RETRIES") {
		t.Errorf("AI.Warnings = %v, quería un aviso que mencione AI_MAX_RETRIES", cfg.AI.Warnings)
	}
}

// TestLoad_AI_MaxRetriesClampedToMaximum comprueba que un número de
// reintentos fuera de rango se acota (nunca es fatal).
func TestLoad_AI_MaxRetriesClampedToMaximum(t *testing.T) {
	prepareAIEnv(t)
	t.Setenv("AI_MAX_RETRIES", "99")

	cfg := loadConfigWithAI(t)
	if cfg.AI.MaxRetries != MaxAIMaxRetries {
		t.Errorf("AI.MaxRetries = %d, quería el máximo %d", cfg.AI.MaxRetries, MaxAIMaxRetries)
	}
	if !containsWarning(cfg.AI.Warnings, "AI_MAX_RETRIES") {
		t.Errorf("AI.Warnings = %v, quería un aviso que mencione AI_MAX_RETRIES", cfg.AI.Warnings)
	}
}

// TestLoad_AI_WarningsNeverContainCredentials comprueba que ningún aviso
// puede filtrar la clave del proveedor.
func TestLoad_AI_WarningsNeverContainCredentials(t *testing.T) {
	prepareAIEnv(t)
	t.Setenv("AI_ENABLED", "true")
	t.Setenv("AI_PROVIDER", "openai")
	t.Setenv("AI_OPENAI_API_KEY", "sk-secret-value")
	// Modelos y timeouts inválidos generan avisos junto al de modelo vacío.
	t.Setenv("AI_TIMEOUT", "no-es-una-duración")

	cfg := loadConfigWithAI(t)
	if len(cfg.AI.Warnings) == 0 {
		t.Fatal("se esperaba al menos un aviso con la configuración de este test")
	}
	for _, w := range cfg.AI.Warnings {
		if strings.Contains(w, "sk-secret-value") {
			t.Errorf("un aviso filtró la credencial: %q", w)
		}
	}
}

// TestLoad_AI_MissingModelWarnsButKeepsUnavailable comprueba que un modelo
// vacío deja la IA no disponible: no se asume ningún modelo por defecto,
// porque un nombre inventado fallaría en la llamada al proveedor.
func TestLoad_AI_MissingModelWarnsButKeepsUnavailable(t *testing.T) {
	prepareAIEnv(t)
	t.Setenv("AI_ENABLED", "true")
	t.Setenv("AI_PROVIDER", "openrouter")
	t.Setenv("AI_OPENROUTER_API_KEY", "sk-test-key")
	// AI_MODEL vacío.

	cfg := loadConfigWithAI(t)
	if cfg.AI.Available {
		t.Errorf("AI.Available = true sin modelo: no debe asumirse ningún modelo por defecto")
	}
	if cfg.AI.Model != "" {
		t.Errorf("AI.Model = %q, quería vacío", cfg.AI.Model)
	}
	if !containsWarning(cfg.AI.Warnings, "AI_MODEL") {
		t.Errorf("AI.Warnings = %v, quería un aviso que mencione AI_MODEL", cfg.AI.Warnings)
	}
}

// containsWarning dice si algún aviso contiene el fragmento dado.
func containsWarning(warnings []string, fragment string) bool {
	for _, w := range warnings {
		if strings.Contains(w, fragment) {
			return true
		}
	}
	return false
}
