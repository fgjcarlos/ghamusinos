// Package config gestiona la configuración central de la aplicación,
// leyendo variables de entorno y validando los valores requeridos.
package config

import (
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"

	"github.com/fgjcarlos/ghamusinos/internal/db"
)

// SecurityHeadersConfig configura las cabeceras defensivas aplicadas por
// internal/http.SecurityHeaders. Issue #26.
type SecurityHeadersConfig struct {
	// HSTSEnabled activa Strict-Transport-Security. Apagado por defecto
	// (binario corre sobre http en local); activar detrás de un proxy TLS.
	HSTSEnabled bool
	// CSPReportOnly aplica CSP en modo report-only (monitorizar sin
	// romper durante despliegue inicial).
	CSPReportOnly bool
}

// Config contiene toda la configuración de la aplicación.
type Config struct {
	// Env es el entorno de ejecución: "development", "production", etc.
	Env string
	// Port es el puerto TCP en el que escucha el servidor HTTP.
	Port string
	// DatabaseURL es la cadena de conexión a PostgreSQL (obligatoria).
	DatabaseURL string
	// Pool contiene los parámetros de tuning de pgxpool.
	Pool db.PoolConfig
	// ClerkJWKSURL es la URL del endpoint JWKS para verificar firmas JWT de Clerk (obligatoria).
	ClerkJWKSURL string
	// ClerkAudience es el valor esperado del claim 'aud' en Clerk JWTs (opcional).
	ClerkAudience string
	// FrontendURL es la URL base del frontend para redirecciones OAuth (default http://localhost:5173).
	FrontendURL string
	// TrustedProxies lista de CIDRs de proxies de confianza para parsear
	// X-Forwarded-For. Si está vacía, NO se monta el middleware de IP real
	// y los logs usan r.RemoteAddr (fail-closed). Configurar con
	// TRUSTED_PROXIES="10.0.0.0/8,172.16.0.0/12" en despliegues detrás
	// de un reverse proxy conocido. Issue #64.
	TrustedProxies []string
	Security       SecurityHeadersConfig
	// MaxBodyBytes es el límite global del cuerpo HTTP en bytes (default
	// 10 MiB = 10<<20). Configurar con MAX_BODY_BYTES. Issue #26.
	MaxBodyBytes int64
	// Strava contiene la configuración de la integración con Strava
	// (fase 1.2, issue #14). Es nil si las variables de entorno no
	// están definidas; en ese caso los handlers OAuth no se montan.
	Strava *StravaConfig
}

// StravaConfig agrupa las credenciales de la app Strava global (ADR 0001)
// y la clave AES-256-GCM usada para cifrar los tokens persistidos.
//
// Los tokens de Strava (STRAVA_CLIENT_ID / SECRET) son obligatorios para
// que la integración funcione. STRAVA_REDIRECT_URL debe coincidir con la
// URL configurada en la app de Strava. STRAVA_CIPHER_KEY es la clave
// AES-256 (32 bytes) codificada en base64 estándar.
// STRAVA_WEBHOOK_SECRET es el secreto compartido para validar firmas
// HMAC-SHA256 en los webhooks de Strava.
// STRAVA_BACKFILL_DAYS es el número de días atrás para la sincronización inicial
// (default 42, aproximadamente 6 semanas).
type StravaConfig struct {
	ClientID      string
	ClientSecret  string
	RedirectURL   string
	Scopes        string
	CipherKey     []byte
	WebhookSecret string
	BackfillDays  int
}

// Load lee las variables de entorno y devuelve un Config validado.
// En entorno de desarrollo intenta cargar el fichero .env de forma
// best-effort (lo ignora si no existe).
func Load() (*Config, error) {
	// Carga .env de forma best-effort en desarrollo (antes de leer las vars).
	env := os.Getenv("ENV")
	if env == "" {
		env = "development"
	}
	if env == "development" {
		_ = godotenv.Load() // ignora el error si .env no existe
	}

	defaults := db.DefaultPoolConfig()

	pool, err := loadPoolConfig(defaults)
	if err != nil {
		return nil, err
	}
	if err := validatePool(pool); err != nil {
		return nil, err
	}

	cfg := &Config{
		Env:            getEnv("ENV", "development"),
		Port:           getEnv("PORT", "8080"),
		DatabaseURL:    os.Getenv("DATABASE_URL"),
		Pool:           pool,
		ClerkJWKSURL:   os.Getenv("CLERK_JWKS_URL"),
		ClerkAudience:  getEnv("CLERK_AUDIENCE", ""),
		FrontendURL:    getEnv("FRONTEND_URL", "http://localhost:5173"),
		TrustedProxies: parseTrustedProxies(os.Getenv("TRUSTED_PROXIES")),
		Security: SecurityHeadersConfig{
			HSTSEnabled:   getEnv("SECURITY_HSTS_ENABLED", "") == "true",
			CSPReportOnly: getEnv("SECURITY_CSP_REPORT_ONLY", "") == "true",
		},
		MaxBodyBytes: parseMaxBodyBytes(os.Getenv("MAX_BODY_BYTES")),
	}

	if cfg.DatabaseURL == "" {
		return nil, errors.New("config: DATABASE_URL es obligatoria y está vacía")
	}
	if cfg.ClerkJWKSURL == "" {
		return nil, errors.New("config: CLERK_JWKS_URL es obligatoria y está vacía")
	}

	strava, err := loadStravaConfig()
	if err != nil {
		return nil, err
	}
	cfg.Strava = strava

	return cfg, nil
}

// loadStravaConfig lee las variables de Strava. Devuelve nil si la
// integración no está configurada (faltan CLIENT_ID o CLIENT_SECRET).
// Devuelve error solo si las variables están definidas pero malformadas
// (cipher key con base64 inválido, longitud incorrecta, etc.).
func loadStravaConfig() (*StravaConfig, error) {
	id := os.Getenv("STRAVA_CLIENT_ID")
	secret := os.Getenv("STRAVA_CLIENT_SECRET")
	if id == "" || secret == "" {
		return nil, nil
	}

	cipherB64 := os.Getenv("STRAVA_CIPHER_KEY")
	if cipherB64 == "" {
		return nil, errors.New("config: STRAVA_CIPHER_KEY es obligatoria cuando STRAVA_CLIENT_ID/SECRET están definidas")
	}
	key, err := base64.StdEncoding.DecodeString(cipherB64)
	if err != nil {
		return nil, fmt.Errorf("config: STRAVA_CIPHER_KEY no es base64 válido: %w", err)
	}
	if len(key) != 32 {
		return nil, fmt.Errorf("config: STRAVA_CIPHER_KEY debe decodificar a 32 bytes (AES-256), got %d", len(key))
	}

	webhookSecret := os.Getenv("STRAVA_WEBHOOK_SECRET")
	if webhookSecret == "" {
		return nil, errors.New("config: STRAVA_WEBHOOK_SECRET es obligatoria cuando STRAVA_CLIENT_ID/SECRET están definidas")
	}

	backfillDays, err := getEnvInt32("STRAVA_BACKFILL_DAYS", 42)
	if err != nil {
		return nil, fmt.Errorf("config: STRAVA_BACKFILL_DAYS parsing error: %w", err)
	}

	return &StravaConfig{
		ClientID:      id,
		ClientSecret:  secret,
		RedirectURL:   os.Getenv("STRAVA_REDIRECT_URL"),
		Scopes:        getEnv("STRAVA_SCOPES", "read,activity:read"),
		CipherKey:     key,
		WebhookSecret: webhookSecret,
		BackfillDays:  int(backfillDays),
	}, nil
}

// validatePool comprueba que los valores del pool son consistentes.
// Errores aquí son fatales porque un pool mal configurado es peor que
// no arrancar: puede colgarse en ConnectTimeout o rechazar trabajo.
func validatePool(p db.PoolConfig) error {
	if p.MaxConns <= 0 {
		return errors.New("config: DB_POOL_MAX_CONNS debe ser > 0")
	}
	if p.MinConns < 0 {
		return errors.New("config: DB_POOL_MIN_CONNS debe ser >= 0")
	}
	if p.MinConns > p.MaxConns {
		return fmt.Errorf("config: DB_POOL_MIN_CONNS (%d) no puede ser mayor que DB_POOL_MAX_CONNS (%d)", p.MinConns, p.MaxConns)
	}
	if p.ConnectTimeout <= 0 {
		return errors.New("config: DB_POOL_CONNECT_TIMEOUT debe ser > 0 (ej: 5s)")
	}
	if p.MaxConnLifetime < 0 {
		return errors.New("config: DB_POOL_MAX_CONN_LIFETIME debe ser >= 0")
	}
	if p.MaxConnIdleTime < 0 {
		return errors.New("config: DB_POOL_MAX_CONN_IDLE_TIME debe ser >= 0")
	}
	if p.HealthCheckPeriod < 0 {
		return errors.New("config: DB_POOL_HEALTH_CHECK_PERIOD debe ser >= 0")
	}
	return nil
}

// getEnv devuelve el valor de la variable de entorno key, o defaultVal si
// la variable no está definida o está vacía.
func getEnv(key, defaultVal string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultVal
}

// loadPoolConfig lee los parámetros del pool desde variables de entorno
// usando defaults como fallback. Si una variable está definida pero
// mal formada, devuelve un error claro (en vez de aplicar el default o
// devolver 0, que daría mensajes de validación confusos).
func loadPoolConfig(defaults db.PoolConfig) (db.PoolConfig, error) {
	maxConns, err := getEnvInt32("DB_POOL_MAX_CONNS", defaults.MaxConns)
	if err != nil {
		return db.PoolConfig{}, err
	}
	minConns, err := getEnvInt32("DB_POOL_MIN_CONNS", defaults.MinConns)
	if err != nil {
		return db.PoolConfig{}, err
	}
	maxConnLifetime, err := getEnvDuration("DB_POOL_MAX_CONN_LIFETIME", defaults.MaxConnLifetime)
	if err != nil {
		return db.PoolConfig{}, err
	}
	maxConnIdleTime, err := getEnvDuration("DB_POOL_MAX_CONN_IDLE_TIME", defaults.MaxConnIdleTime)
	if err != nil {
		return db.PoolConfig{}, err
	}
	connectTimeout, err := getEnvDuration("DB_POOL_CONNECT_TIMEOUT", defaults.ConnectTimeout)
	if err != nil {
		return db.PoolConfig{}, err
	}
	healthCheckPeriod, err := getEnvDuration("DB_POOL_HEALTH_CHECK_PERIOD", defaults.HealthCheckPeriod)
	if err != nil {
		return db.PoolConfig{}, err
	}
	return db.PoolConfig{
		MaxConns:          maxConns,
		MinConns:          minConns,
		MaxConnLifetime:   maxConnLifetime,
		MaxConnIdleTime:   maxConnIdleTime,
		ConnectTimeout:    connectTimeout,
		HealthCheckPeriod: healthCheckPeriod,
	}, nil
}

// getEnvInt32 devuelve el valor entero de key, o defaultVal si está
// vacía. Si está definida pero no parsea, devuelve un error con el
// nombre de la variable y el valor recibido.
func getEnvInt32(key string, defaultVal int32) (int32, error) {
	v := os.Getenv(key)
	if v == "" {
		return defaultVal, nil
	}
	n, err := strconv.ParseInt(v, 10, 32)
	if err != nil {
		return 0, fmt.Errorf("config: %s=%q no es un entero válido: %w", key, v, err)
	}
	return int32(n), nil
}

// getEnvDuration devuelve la duración de key, o defaultVal si está
// vacía. Acepta el formato de time.ParseDuration (e.g. "5s", "1h30m",
// "500ms").
func getEnvDuration(key string, defaultVal time.Duration) (time.Duration, error) {
	v := os.Getenv(key)
	if v == "" {
		return defaultVal, nil
	}
	d, err := time.ParseDuration(v)
	if err != nil {
		return 0, fmt.Errorf("config: %s=%q no es una duración válida (espera formato Go, e.g. 5s, 1h30m): %w", key, v, err)
	}
	return d, nil
}

// parseTrustedProxies lee la lista de CIDRs separados por coma. Devuelve
// slice vacío si la variable está vacía (fail-closed: sin proxies de
// confianza declarados, no se monta el middleware XFF). Espacios
// alrededor de cada CIDR se trimean. Issue #64.
func parseTrustedProxies(raw string) []string {
	if raw == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

// parseMaxBodyBytes lee MAX_BODY_BYTES como entero (bytes). Vacío o
// valor no numérico → default 10 MiB (10<<20). Issue #26.
func parseMaxBodyBytes(raw string) int64 {
	const defaultBytes = 10 << 20 // 10 MiB
	if raw == "" {
		return defaultBytes
	}
	n, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || n <= 0 {
		return defaultBytes
	}
	return n
}
