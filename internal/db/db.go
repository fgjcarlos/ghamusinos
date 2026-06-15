package db

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Connect crea y valida una conexión a PostgreSQL usando pgxpool con los
// parámetros de pc aplicados de forma explícita. Los valores de pc
// sobrescriben cualquier default que pgxpool.ParseConfig haya podido leer
// del connection string (p.ej. ?pool_max_conns=N).
//
// pc.MaxConns debe ser > 0 y pc.ConnectTimeout > 0. Quien llame debe
// rellenar todos los campos (típicamente desde DefaultPoolConfig o desde
// la config del binario).
//
// El caller es responsable de cerrar el pool:
//
//	pool, err := db.Connect(ctx, url, db.DefaultPoolConfig())
//	if err != nil { ... }
//	defer pool.Close()
func Connect(ctx context.Context, databaseURL string, pc PoolConfig) (*pgxpool.Pool, error) {
	pgxCfg, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		return nil, fmt.Errorf("db: error al parsear la URL de conexión: %w", err)
	}

	pgxCfg.MaxConns = pc.MaxConns
	pgxCfg.MinConns = pc.MinConns
	pgxCfg.MaxConnLifetime = pc.MaxConnLifetime
	pgxCfg.MaxConnIdleTime = pc.MaxConnIdleTime
	pgxCfg.ConnConfig.ConnectTimeout = pc.ConnectTimeout
	pgxCfg.HealthCheckPeriod = pc.HealthCheckPeriod

	pool, err := pgxpool.NewWithConfig(ctx, pgxCfg)
	if err != nil {
		return nil, fmt.Errorf("db: error al crear el pool de conexiones: %w", err)
	}

	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("db: error al verificar la conexión (ping): %w", err)
	}

	return pool, nil
}
