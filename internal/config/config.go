// Package config gestiona la configuración central de la aplicación,
// leyendo variables de entorno y validando los valores requeridos.
package config

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/joho/godotenv"

	"github.com/fgjcarlos/ghamusinos/internal/db"
)

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
	}

	if cfg.DatabaseURL == "" {
		return nil, errors.New("config: DATABASE_URL es obligatoria y está vacía")
	}
	if cfg.ClerkJWKSURL == "" {
		return nil, errors.New("config: CLERK_JWKS_URL es obligatoria y está vacía")
	}

	return cfg, nil
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
