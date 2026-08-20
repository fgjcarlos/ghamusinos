// Package main implements the migrate command with an allowlist of safe
// subcommands. Destructive commands (down, down-to, reset, redo) require
// the explicit --allow-destructive flag to prevent accidental schema
// destruction by deploy scripts or operator typos. Issue #27.
package main

import (
	"context"
	"database/sql"
	"flag"
	"fmt"
	"log/slog"
	"os"

	_ "github.com/jackc/pgx/v5/stdlib" // driver database/sql para pgx
	"github.com/pressly/goose/v3"

	"github.com/fgjcarlos/ghamusinos/internal/db"
	"github.com/fgjcarlos/ghamusinos/internal/logging"
)

// allowedCommands es el conjunto de subcomandos de goose que el binario
// acepta sin flag adicional. Cualquier otro subcomando (incluidos los
// destructivos) se rechaza salvo que se pase --allow-destructive.
var allowedCommands = map[string]struct{}{
	"up":        {},
	"up-by-one": {},
	"status":    {},
	"version":   {},
}

// destructiveCommands requiere --allow-destructive explícito.
var destructiveCommands = map[string]struct{}{
	"down":    {},
	"down-to": {},
	"reset":   {},
	"redo":    {},
}

func main() {
	logging.Setup(os.Getenv("ENV"), os.Stdout)

	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		slog.Error("migrate: DATABASE_URL is required and empty")
		os.Exit(1)
	}

	// Parse flags. El primer arg posicional (subcomando de goose) se extrae
	// de flag.Args() tras el parsing.
	fs := flag.NewFlagSet("migrate", flag.ContinueOnError)
	allowDestructive := fs.Bool("allow-destructive", false, "allow destructive goose commands (down, down-to, reset, redo)")
	if err := fs.Parse(os.Args[1:]); err != nil {
		// ContinueOnError ya imprime el error; salimos con código no-cero.
		os.Exit(2)
	}

	command := "up"
	if args := fs.Args(); len(args) > 0 {
		command = args[0]
	}

	if err := guardCommand(command, *allowDestructive); err != nil {
		slog.Error(err.Error())
		os.Exit(1)
	}

	if err := run(databaseURL, command); err != nil {
		slog.Error("migrate failed", slog.String("error", err.Error()))
		os.Exit(1)
	}
}

// guardCommand valida que `command` esté en la allowlist o, si es
// destructivo, se haya pasado --allow-destructive. Devuelve un error
// con mensaje claro cuando el comando se rechaza.
func guardCommand(command string, allowDestructive bool) error {
	if _, ok := allowedCommands[command]; ok {
		return nil
	}
	if _, destructive := destructiveCommands[command]; destructive {
		if allowDestructive {
			return nil
		}
		return fmt.Errorf("migrate: subcomando destructivo %q bloqueado — requiere --allow-destructive (issue #27)", command)
	}
	return fmt.Errorf("migrate: subcomando %q no está en la allowlist %v (issue #27)", command, mapKeys(allowedCommands))
}

// mapKeys devuelve las claves de un set como slice ordenado (determinista).
func mapKeys(m map[string]struct{}) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	// Orden estable para mensajes de error reproducibles.
	for i := 1; i < len(out); i++ {
		for j := i; j > 0 && out[j-1] > out[j]; j-- {
			out[j-1], out[j] = out[j], out[j-1]
		}
	}
	return out
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

	goose.SetBaseFS(db.MigrationsFS)

	if err := goose.SetDialect("postgres"); err != nil {
		return fmt.Errorf("error al configurar el dialecto: %w", err)
	}

	if err := goose.RunContext(context.Background(), command, sqlDB, "migrations"); err != nil {
		return fmt.Errorf("error al ejecutar '%s': %w", command, err)
	}

	return nil
}
