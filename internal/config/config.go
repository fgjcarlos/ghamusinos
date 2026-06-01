// Package config gestiona la configuración central de la aplicación,
// leyendo variables de entorno y validando los valores requeridos.
package config

import (
	"errors"
	"os"

	"github.com/joho/godotenv"
)

// Config contiene toda la configuración de la aplicación.
type Config struct {
	// Env es el entorno de ejecución: "development", "production", etc.
	Env string
	// Port es el puerto TCP en el que escucha el servidor HTTP.
	Port string
	// DatabaseURL es la cadena de conexión a PostgreSQL (obligatoria).
	DatabaseURL string
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

	cfg := &Config{
		Env:         getEnv("ENV", "development"),
		Port:        getEnv("PORT", "8080"),
		DatabaseURL: os.Getenv("DATABASE_URL"),
	}

	if cfg.DatabaseURL == "" {
		return nil, errors.New("config: DATABASE_URL es obligatoria y está vacía")
	}

	return cfg, nil
}

// getEnv devuelve el valor de la variable de entorno key, o defaultVal si
// la variable no está definida o está vacía.
func getEnv(key, defaultVal string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultVal
}
