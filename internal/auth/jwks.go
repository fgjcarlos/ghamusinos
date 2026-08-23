package auth

import (
	"context"
	"crypto"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/lestrrat-go/jwx/v2/jwk"
	"golang.org/x/sync/singleflight"
)

// JWKSCache defines the interface for fetching and caching JSON Web Key Sets.
type JWKSCache interface {
	// GetKey returns the public key for the given key ID.
	// Fetches and caches the JWKS from the provider if not cached, if TTL
	// expired, or if the kid is unknown and the refetch cooldown has elapsed.
	GetKey(ctx context.Context, kid string) (crypto.PublicKey, error)
}

// refetchCooldown acota la frecuencia con la que se reintenta un fetch
// ante un kid desconocido. Sin este límite, un atacante podría
// amplificar el endpoint de Clerk mandando kids basura en cada request.
// Issue #167.
const refetchCooldown = 30 * time.Second

// inMemoryJWKSCache implements JWKSCache with in-memory storage and TTL.
//
// El hot path usa RLock: un fetch sólo se dispara bajo Lock y se
// coalesce con singleflight para que N peticiones concurrentes con la
// caché fría provoquen UN fetch HTTP. Issue #167.
type inMemoryJWKSCache struct {
	url string
	ttl time.Duration

	mu                  sync.RWMutex
	keyset              jwk.Set
	lastFetch           time.Time
	lastUnknownKidFetch time.Time // último refetch disparado por kid desconocido

	fetch  singleflight.Group
	client *http.Client
}

// NewJWKSCache creates a new in-memory JWKS cache with the given URL and TTL.
func NewJWKSCache(url string, ttl time.Duration) JWKSCache {
	return &inMemoryJWKSCache{
		url: url,
		ttl: ttl,
		// http.DefaultClient no tiene timeout; NewJWKSCache es la pieza
		// que se monta en producción, así que el timeout va aquí.
		client: &http.Client{Timeout: 10 * time.Second},
	}
}

// GetKey returns the public key for the given key ID, fetching if necessary.
func (c *inMemoryJWKSCache) GetKey(ctx context.Context, kid string) (crypto.PublicKey, error) {
	// Camino caliente: lectura bajo RLock. El lock es compartido entre
	// N goroutines que pidan a la vez la misma kid. Issue #167.
	c.mu.RLock()
	if c.keyset != nil && time.Since(c.lastFetch) <= c.ttl {
		if key, ok := c.lookupKey(kid); ok {
			c.mu.RUnlock()
			return key, nil
		}
	}
	// Decidir todo lo necesario bajo el lock: si hay que fetchear
	// (caché vacía o TTL vencido) o si toca refetch por kid
	// desconocido (cooldown vencido). Sacar estas lecturas fuera del
	// lock es una carrera: otro goroutine puede escribir el estado
	// mientras decidimos, y refrescar o no refrescar basándonos en
	// una instantánea vieja. El test concurrente lo cazaba con -race.
	needInitialFetch := c.keyset == nil || time.Since(c.lastFetch) > c.ttl
	// Refetch por kid desconocido: solo cuenta el cooldown de
	// refetches-for-unknown-kid, no el fetch inicial. Sin esto, una
	// rotación de Clerk que cayera justo después del fetch inicial
	// quedaba atrapada en el cooldown durante 30 s.
	needRefetchUnknown := !needInitialFetch && c.keyset != nil && time.Since(c.lastUnknownKidFetch) > refetchCooldown
	c.mu.RUnlock()

	if !needInitialFetch && !needRefetchUnknown {
		return nil, fmt.Errorf("auth: key ID not found: %s", kid)
	}

	if err := c.refresh(ctx, needRefetchUnknown); err != nil {
		return nil, err
	}

	c.mu.RLock()
	defer c.mu.RUnlock()
	key, ok := c.lookupKey(kid)
	if !ok {
		return nil, fmt.Errorf("auth: key ID not found: %s", kid)
	}
	return key, nil
}

// lookupKey extrae la clave pública del keyset. Se llama siempre bajo
// RLock o Lock.
func (c *inMemoryJWKSCache) lookupKey(kid string) (crypto.PublicKey, bool) {
	key, ok := c.keyset.LookupKeyID(kid)
	if !ok {
		return nil, false
	}
	var rawKey interface{}
	if err := key.Raw(&rawKey); err != nil {
		return nil, false
	}
	pubKey, ok := rawKey.(crypto.PublicKey)
	if !ok {
		return nil, false
	}
	return pubKey, true
}

// refresh ejecuta UN fetch HTTP aun con N goroutines concurrentes
// (singleflight) y actualiza el keyset bajo Lock. El caller ya decidió
// que hacía falta un fetch; aquí no se vuelve a comprobar TTL.
//
// triggeredByUnknownKid indica si el fetch lo disparó un kid
// desconocido. Si es así, actualiza lastUnknownKidFetch para que el
// cooldown acote la frecuencia de los siguientes refetches del mismo
// tipo. Si es el fetch inicial, no toca ese timestamp — si no, la
// siguiente rotación de Clerk quedaría bloqueada durante 30 s.
func (c *inMemoryJWKSCache) refresh(ctx context.Context, triggeredByUnknownKid bool) error {
	_, err, _ := c.fetch.Do("jwks", func() (interface{}, error) {
		keyset, fetchErr := c.fetchOnce(ctx)
		if fetchErr != nil {
			return nil, fetchErr
		}
		c.mu.Lock()
		now := time.Now()
		c.keyset = keyset
		c.lastFetch = now
		if triggeredByUnknownKid {
			c.lastUnknownKidFetch = now
		}
		c.mu.Unlock()
		return nil, nil
	})
	return err
}

// fetchOnce hace UNA petición HTTP al endpoint JWKS. Errores de red o
// de parseo se devuelven sin tocar el estado del caché.
func (c *inMemoryJWKSCache) fetchOnce(ctx context.Context) (jwk.Set, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.url, nil)
	if err != nil {
		return nil, fmt.Errorf("auth: failed to create request: %w", err)
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("auth: failed to fetch JWKS: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("auth: JWKS fetch returned status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("auth: failed to read JWKS response: %w", err)
	}

	keyset, err := jwk.Parse(body)
	if err != nil {
		return nil, fmt.Errorf("auth: failed to parse JWKS: %w", err)
	}

	return keyset, nil
}

// errJWKSUnknownKid se devuelve al caller cuando un kid no está y el
// cooldown de refetch sigue vivo. Es interno — el caller lo convierte
// a ErrUnauthenticated o a un error con contexto.
var errJWKSUnknownKid = errors.New("auth: unknown kid, refetch cooldown active")
