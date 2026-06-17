// Package db gestiona la conexión a PostgreSQL mediante pgxpool.
package db

import "time"

// PoolConfig agrupa los parámetros de tuning del pool de pgxpool.
// Los valores se aplican explícitamente en Connect para no depender
// de los defaults de la librería y poder documentarlos y verificarlos.
//
// Una PoolConfig vacía NO es válida: Connect aplica los campos tal cual,
// por lo que quien llame debe rellenar todos los valores (típicamente
// desde DefaultPoolConfig o desde la config del binario).
type PoolConfig struct {
	// MaxConns es el tamaño máximo del pool. Default 20.
	MaxConns int32
	// MinConns es el tamaño mínimo (warm pool) que el health check
	// intenta mantener. Default 2.
	MinConns int32
	// MaxConnLifetime cierra conexiones más viejas que esto. Default 1h.
	MaxConnLifetime time.Duration
	// MaxConnIdleTime cierra conexiones que llevan más de este tiempo
	// sin usarse. Default 30m.
	MaxConnIdleTime time.Duration
	// ConnectTimeout limita el tiempo para establecer una conexión TCP
	// nueva. Sin este valor, un arranque puede bloquearse indefinidamente
	// si la base de datos no responde. Default 5s.
	ConnectTimeout time.Duration
	// HealthCheckPeriod es la cadencia con la que el pool valida
	// conexiones idle. Default 1m.
	HealthCheckPeriod time.Duration
}

// DefaultPoolConfig devuelve los valores por defecto del pool.
// Adecuados para el binario embebido de Ghamusinos. Con los workers de
// River en fase 1.2 el pool crece en carga: ajustar antes de producción
// vía variables de entorno (DB_POOL_MAX_CONNS, etc.) o aquí si se decide
// codificarlos.
func DefaultPoolConfig() PoolConfig {
	return PoolConfig{
		MaxConns:          20,
		MinConns:          2,
		MaxConnLifetime:   time.Hour,
		MaxConnIdleTime:   30 * time.Minute,
		ConnectTimeout:    5 * time.Second,
		HealthCheckPeriod: time.Minute,
	}
}
