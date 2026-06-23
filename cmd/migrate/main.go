// Comando migrate ejecuta migraciones de base de datos usando goose como
// librería con migraciones SQL embebidas. No depende del CLI de goose.
//
// Uso:
//
//	DATABASE_URL=... go run ./cmd/migrate [up|down|status]
//
// El argumento por defecto es "up" si no se especifica ninguno.
package main

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"os"

	_ "github.com/jackc/pgx/v5/stdlib" // driver database/sql para pgx
	"github.com/pressly/goose/v3"

	"github.com/fgjcarlos/ghamusinos/internal/db"
	"github.com/fgjcarlos/ghamusinos/internal/logging"
)

func main() {
	// Initialize structured logging
	logging.Setup(os.Getenv("ENV"), os.Stdout)

	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		slog.Error("migrate: DATABASE_URL is required and empty")
		os.Exit(1)
	}

	command := "up"
	if len(os.Args) > 1 {
		command = os.Args[1]
	}

	if err := run(databaseURL, command); err != nil {
		slog.Error("migrate failed", slog.String("error", err.Error()))
		os.Exit(1)
	}
}

func run(databaseURL, command string) error {
	sqlDB, err := sql.Open("pgx", databaseURL)
	if err != nil {
		return fmt.Errorf("error al abrir la conexión SQL: %w", err)
	}
	defer func() { _ = sqlDB.Close() }()

	if err := sqlDB.PingContext(context.Background()); err != nil {
		return fmt.Errorf("error al verificar la conexión: %w", err)
	}

	// Configura goose para usar el FS embebido.
	goose.SetBaseFS(db.MigrationsFS)

	if err := goose.SetDialect("postgres"); err != nil {
		return fmt.Errorf("error al configurar el dialecto: %w", err)
	}

	if err := goose.RunContext(context.Background(), command, sqlDB, "migrations"); err != nil {
		return fmt.Errorf("error al ejecutar '%s': %w", command, err)
	}

	return nil
}
