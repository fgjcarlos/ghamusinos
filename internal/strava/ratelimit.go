package strava

import (
	"context"
	"sync"
	"time"
)

// rateLimiter implementa el rate limit de Strava: 200 req/15min y 2000/día.
// Son dos ventanas independientes; una request consume una unidad de cada
// una. Si cualquiera está agotada, Acquire bloquea hasta que se libere un
// hueco en esa ventana.
//
// Es seguro para uso concurrente (varios goroutines pueden llamar a
// Acquire/Release a la vez).
//
// # Limitaciones conocidas
//
//   - Estado en memoria: tras un restart del proceso, los contadores se
//     reinician. Strava lleva el contador del lado del servidor y nos
//     devolverá 429 si nos pasamos; el limiter local es una optimización,
//     no la fuente de verdad. Persistirlo (Redis) sería la mejora obvia
//     cuando se ejecuten múltiples réplicas.
//   - Granularidad del refill: usamos tick fijo. Una mejora futura es
//     calcular refill por-request (tiempo desde la última) en lugar de
//     por-tick (más suave bajo carga baja).
//
// ponytail: token bucket en memoria, suficiente para un único proceso.
// Cuando se escale a N réplicas, mover a Redis (mismas dos ventanas).
type rateLimiter struct {
	shortCap   int
	shortWin   time.Duration
	dailyCap   int
	dailyWin   time.Duration

	mu          sync.Mutex
	shortTokens int
	shortStart  time.Time
	dailyTokens int
	dailyStart  time.Time
}

func newRateLimiter(shortCap int, shortWin time.Duration, dailyCap int) *rateLimiter {
	now := time.Now()
	return &rateLimiter{
		shortCap:   shortCap,
		shortWin:   shortWin,
		dailyCap:   dailyCap,
		dailyWin:   24 * time.Hour,

		shortTokens: shortCap,
		shortStart:  now,
		dailyTokens: dailyCap,
		dailyStart:  now,
	}
}

// refill resetea los tokens si la ventana actual ya expiró. Se llama
// bajo mu.
func (r *rateLimiter) refill(now time.Time) {
	if now.Sub(r.shortStart) >= r.shortWin {
		r.shortTokens = r.shortCap
		r.shortStart = now
	}
	if now.Sub(r.dailyStart) >= r.dailyWin {
		r.dailyTokens = r.dailyCap
		r.dailyStart = now
	}
}

// Acquire bloquea hasta que haya un token libre en cada ventana o hasta
// que ctx se cancele. Devuelve nil si consigue un slot o el error de ctx.
func (r *rateLimiter) Acquire(ctx context.Context) error {
	for {
		r.mu.Lock()
		now := time.Now()
		r.refill(now)
		if r.shortTokens > 0 && r.dailyTokens > 0 {
			r.shortTokens--
			r.dailyTokens--
			r.mu.Unlock()
			return nil
		}
		// Calculamos cuándo habrá hueco en la ventana más restrictiva.
		// Si ambas están agotadas, esperamos al reset de la primera
		// que venza.
		wait := r.shortWin - now.Sub(r.shortStart)
		if r.dailyTokens == 0 {
			dWait := r.dailyWin - now.Sub(r.dailyStart)
			if dWait > wait {
				wait = dWait
			}
		}
		if r.shortTokens == 0 {
			sWait := r.shortWin - now.Sub(r.shortStart)
			if sWait > wait {
				wait = sWait
			}
		}
		r.mu.Unlock()

		// Si no hay ni un solo token en ninguna ventana, esperamos el
		// reset. capped a 1 minuto para no dormir eternamente si las
		// ventanas están desincronizadas.
		if wait <= 0 {
			wait = time.Second
		}
		if wait > time.Minute {
			wait = time.Minute
		}

		timer := time.NewTimer(wait)
		select {
		case <-ctx.Done():
			timer.Stop()
			return ctx.Err()
		case <-timer.C:
			// vuelta al loop
		}
	}
}

// Release existe por simetría con Acquire y porque algunos patrones de
// semáforo lo piden. Aquí no hacemos nada: cada Acquire consume un token
// de forma síncrona y no hay "prestamos" entre goroutines. Está en la API
// por si en el futuro pasamos a un modelo con Acquire no-bloqueante +
// Release manual.
func (r *rateLimiter) Release() {}