//go:build integration

package jobs

import (
	"context"
	"fmt"
	"os"

	"github.com/jackc/pgx/v5/pgxpool"
)

func poolExec(ctx context.Context, sql string, args ...any) (pgxpool.Conn, error) {
	// CI only injects DATABASE_URL (e.g. postgres://postgres:postgres@...).
	// Prefer TEST_DATABASE_URL for local dev, fall back to DATABASE_URL for
	// CI, and only then use a hardcoded local dev default.
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		dsn = os.Getenv("DATABASE_URL")
	}
	if dsn == "" {
		dsn = "postgres://ghamusinos:XL8TBqBWvFqT4bMSveiZe3bi@localhost:5432/ghamusinos?sslmode=disable"
	}
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		return pgxpool.Conn{}, fmt.Errorf("pool new: %w", err)
	}
	defer pool.Close()
	_, err = pool.Exec(ctx, sql, args...)
	if err != nil {
		return pgxpool.Conn{}, fmt.Errorf("exec: %w", err)
	}
	return pgxpool.Conn{}, nil
}
