package db

import (
	"context"
	"os"
	"testing"
	"time"
)

// TestDefaultPoolConfig_Values verifica que los defaults son los
// documentados y razonables para el binario embebido de Ghamusinos.
func TestDefaultPoolConfig_Values(t *testing.T) {
	got := DefaultPoolConfig()

	if got.MaxConns != 20 {
		t.Errorf("MaxConns default = %d, quería 20", got.MaxConns)
	}
	if got.MinConns != 2 {
		t.Errorf("MinConns default = %d, quería 2", got.MinConns)
	}
	if got.MaxConnLifetime != time.Hour {
		t.Errorf("MaxConnLifetime default = %v, quería 1h", got.MaxConnLifetime)
	}
	if got.MaxConnIdleTime != 30*time.Minute {
		t.Errorf("MaxConnIdleTime default = %v, quería 30m", got.MaxConnIdleTime)
	}
	if got.ConnectTimeout != 5*time.Second {
		t.Errorf("ConnectTimeout default = %v, quería 5s", got.ConnectTimeout)
	}
	if got.HealthCheckPeriod != time.Minute {
		t.Errorf("HealthCheckPeriod default = %v, quería 1m", got.HealthCheckPeriod)
	}
}

// TestConnect_RespectsConnectTimeout verifica que un ConnectTimeout corto
// hace fallar el connect rápido cuando el host no responde, en vez de
// bloquearse esperando el timeout del SO (varios minutos).
//
// Usa 192.0.2.1 (RFC 5737 TEST-NET-1) que no es enrutable en internet
// público, así que la conexión TCP no se completa: solo el ConnectTimeout
// puede cortarla.
func TestConnect_RespectsConnectTimeout(t *testing.T) {
	cfg := PoolConfig{
		MaxConns:          5,
		MinConns:          0,
		MaxConnLifetime:   time.Hour,
		MaxConnIdleTime:   30 * time.Minute,
		ConnectTimeout:    100 * time.Millisecond,
		HealthCheckPeriod: time.Minute,
	}
	url := "postgres://user:pass@192.0.2.1:5432/db?sslmode=disable"

	start := time.Now()
	_, err := Connect(context.Background(), url, cfg)
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("Connect con URL no enrutable debería fallar")
	}
	// Damos un margen generoso: el ConnectTimeout es 100ms pero la
	// resolución de DNS, el SYN sin respuesta, etc., pueden añadir
	// un poco más. Lo que NO debe pasar es que tarde más de 2s.
	if elapsed > 2*time.Second {
		t.Fatalf("Connect tardó %v con ConnectTimeout=100ms, debería haber fallado en <2s", elapsed)
	}
	t.Logf("Connect falló en %v (ConnectTimeout=100ms): %v", elapsed, err)
}

// TestConnect_RejectsBadURL verifica que una URL malformada falla en
// ParseConfig (sin intentar siquiera la conexión).
func TestConnect_RejectsBadURL(t *testing.T) {
	cfg := DefaultPoolConfig()
	cfg.ConnectTimeout = 100 * time.Millisecond

	// ":::" no es un host válido; ParseConfig debe rechazarlo.
	_, err := Connect(context.Background(), "postgres://user:pass@:::/db", cfg)
	if err == nil {
		t.Fatal("Connect con URL inválida debería fallar")
	}
}

// TestConnect_RealDB verifica que Connect funciona contra una base de
// datos PostgreSQL real. Se salta si DATABASE_URL_TEST no está definida,
// así el test corre en CI (donde el servicio de postgres está arriba)
// y en local solo si el dev lo pide.
func TestConnect_RealDB(t *testing.T) {
	url := os.Getenv("DATABASE_URL_TEST")
	if url == "" {
		t.Skip("DATABASE_URL_TEST no está definida; test de integración con BD real, saltando")
	}

	cfg := DefaultPoolConfig()
	cfg.ConnectTimeout = 5 * time.Second

	pool, err := Connect(context.Background(), url, cfg)
	if err != nil {
		t.Fatalf("Connect con BD real: %v", err)
	}
	defer pool.Close()

	// Verificamos que el pool responde a un Ping.
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := pool.Ping(ctx); err != nil {
		t.Fatalf("Ping: %v", err)
	}

	// Verificamos que Stat reporta MaxConns aplicado. No expone
	// MinConns/lifetimes/idle/etc., pero MaxConns sí.
	stat := pool.Stat()
	if stat.MaxConns() != cfg.MaxConns {
		t.Errorf("Stat.MaxConns() = %d, quería %d", stat.MaxConns(), cfg.MaxConns)
	}
}
