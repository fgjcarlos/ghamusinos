// Comando migrate ejecuta migraciones de base de datos usando goose como
// librería con migraciones SQL embebidas. No depende del CLI de goose.
//
// Uso:
//
//	DATABASE_URL=... go run ./cmd/migrate [up|status]
//
// El argumento por defecto es "up" si no se especifica ninguno.
//
// Por seguridad, ./cmd/migrate aplica una allowlist de subcomandos:
// sólo "up" y "status" se aceptan por defecto. Comandos destructivos
// como "down", "reset", "redo" o "create" se rechazan con error para
// evitar migraciones destructivas accidentales en CI o producción.
// En una emergencia real, fija MIGRATE_ALLOW_DANGEROUS=1 en el entorno
// para saltarse la allowlist (el binario emite un warning explícito).
//
// Issue de seguimiento: #27.
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

// safeMigrateCommands es la allowlist de subcomandos que ./cmd/migrate
// acepta por defecto. Cualquier comando fuera de este conjunto se rechaza
// con error antes de tocar la base de datos.
var safeMigrateCommands = map[string]struct{}{
	"up":     {}, // aplica migraciones pendientes (default)
	"status": {}, // muestra el estado actual sin modificar la DB
}

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

	if err := ValidateMigrateCommand(command); err != nil {
		slog.Error("migrate: comando rechazado",
			slog.String("command", command),
			slog.String("error", err.Error()))
		os.Exit(1)
	}

	if err := run(databaseURL, command); err != nil {
		slog.Error("migrate failed", slog.String("error", err.Error()))
		os.Exit(1)
	}
}

// ValidateMigrateCommand devuelve nil si el comando está en la allowlist,
// o si MIGRATE_ALLOW_DANGEROUS=1 está activo. En caso contrario devuelve
// un error explicativo con la lista de comandos permitidos.
//
// Separamos la validación de la ejecución para poder testearla sin
// necesitar una conexión a base de datos.
func ValidateMigrateCommand(command string) error {
	if _, ok := safeMigrateCommands[command]; ok {
		return nil
	}
	if os.Getenv("MIGRATE_ALLOW_DANGEROUS") == "1" {
		slog.Warn("migrate: comando fuera de la allowlist aceptado por escape hatch",
			slog.String("command", command),
			slog.String("note", "MIGRATE_ALLOW_DANGEROUS=1 estaba activo"))
		return nil
	}
	return fmt.Errorf(
		"comando %q no está en la allowlist (permitidos: up, status). "+
			"Para habilitar comandos destructivos (down, reset, redo, ...) "+
			"fija MIGRATE_ALLOW_DANGEROUS=1 en el entorno",
		command,
	)
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
