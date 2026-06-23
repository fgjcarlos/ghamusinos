// Comando invites crea y gestiona invitaciones de usuario.
//
// Uso:
//
//	DATABASE_URL=... go run ./cmd/invites create --email user@example.com [--expires-in 7d] [--token-length 32]
//
// El comando create genera un token de invitación criptográficamente seguro,
// almacena su hash en la base de datos, e imprime el token original una sola
// vez en stdout para que el administrador lo comparta con el usuario invitado.
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/fgjcarlos/ghamusinos/internal/config"
	"github.com/fgjcarlos/ghamusinos/internal/db/sqlc"
	"github.com/fgjcarlos/ghamusinos/internal/db/status"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "uso: invites <command> [opciones]")
		fmt.Fprintln(os.Stderr, "comandos:")
		fmt.Fprintln(os.Stderr, "  create    Crear una nueva invitación")
		os.Exit(1)
	}

	command := os.Args[1]

	switch command {
	case "create":
		if err := runCreate(os.Args[2:]); err != nil {
			log.Fatalf("invites create: %v", err)
		}
	default:
		log.Fatalf("invites: comando desconocido %q", command)
	}
}

func runCreate(args []string) error {
	fs := flag.NewFlagSet("create", flag.ExitOnError)
	email := fs.String("email", "", "email del usuario invitado (obligatorio)")
	expiresIn := fs.String("expires-in", "7d", "duración de la invitación (e.g. 7d, 24h)")
	tokenLength := fs.Int("token-length", 32, "longitud del token en bytes")

	if err := fs.Parse(args); err != nil {
		return err
	}

	if *email == "" {
		return fmt.Errorf("--email es obligatorio")
	}
	if *tokenLength < 16 {
		return fmt.Errorf("--token-length debe ser >= 16")
	}

	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("error al cargar configuración: %w", err)
	}

	pool, err := pgxpool.New(context.Background(), cfg.DatabaseURL)
	if err != nil {
		return fmt.Errorf("error al conectar a la base de datos: %w", err)
	}
	defer pool.Close()

	queries := sqlc.New(pool)

	token, err := createInvite(context.Background(), queries, *email, *expiresIn, *tokenLength)
	if err != nil {
		return err
	}

	// Imprime el token original una sola vez a stdout.
	// El administrador puede copiarlo y compartirlo con el usuario invitado.
	fmt.Println(token)

	return nil
}

func createInvite(ctx context.Context, queries *sqlc.Queries, email, expiresIn string, tokenLength int) (string, error) {
	// Parseamos la duración.
	duration, err := parseDuration(expiresIn)
	if err != nil {
		return "", fmt.Errorf("duración inválida %q: %w", expiresIn, err)
	}

	// Generamos el token.
	token, tokenHash, err := generateTokenAndHash(tokenLength)
	if err != nil {
		return "", fmt.Errorf("error al generar token: %w", err)
	}

	// Almacenamos en la base de datos.
	expiresAt := timeNowUTC().Add(duration)
	expiresAtPgtype := pgtype.Timestamptz{Time: expiresAt, Valid: true}

	_, err = queries.CreateInvite(ctx, sqlc.CreateInviteParams{
		Email:     email,
		TokenHash: tokenHash,
		Status:    status.StatusPending,
		ExpiresAt: expiresAtPgtype,
	})
	if err != nil {
		return "", fmt.Errorf("error al crear invitación: %w", err)
	}

	return token, nil
}
