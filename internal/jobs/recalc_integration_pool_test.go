//go:build integration

package jobs

import (
	"context"
	"fmt"
	"os"

	"github.com/jackc/pgx/v5/pgxpool"
)

func poolExec(ctx context.Context, sql string, args ...any) (pgxpool.Conn, error) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		dsn = "postgres://ghamusinos:ghamusinos@localhost:5432/ghamusinos?sslmode=disable"
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
