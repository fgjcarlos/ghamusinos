// Package db gestiona la conexión a PostgreSQL mediante pgxpool.
package db

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Connect crea y valida una conexión a PostgreSQL usando pgxpool.
//
// El caller es responsable de cerrar el pool cuando ya no lo necesite:
//
//	pool, err := db.Connect(ctx, databaseURL)
//	if err != nil { ... }
//	defer pool.Close()
func Connect(ctx context.Context, databaseURL string) (*pgxpool.Pool, error) {
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		return nil, fmt.Errorf("db: error al crear el pool de conexiones: %w", err)
	}

	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("db: error al verificar la conexión (ping): %w", err)
	}

	return pool, nil
}
