package auth

import (
	"context"
	"crypto"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/lestrrat-go/jwx/v2/jwk"
)

// JWKSCache defines the interface for fetching and caching JSON Web Key Sets.
type JWKSCache interface {
	// GetKey returns the public key for the given key ID.
	// Fetches and caches the JWKS from the provider if not cached or TTL expired.
	GetKey(ctx context.Context, kid string) (crypto.PublicKey, error)
}

// inMemoryJWKSCache implements JWKSCache with in-memory storage and TTL.
type inMemoryJWKSCache struct {
	url       string
	ttl       time.Duration
	mu        sync.RWMutex
	lastFetch time.Time
	keyset    jwk.Set
}

// NewJWKSCache creates a new in-memory JWKS cache with the given URL and TTL.
func NewJWKSCache(url string, ttl time.Duration) JWKSCache {
	return &inMemoryJWKSCache{
		url: url,
		ttl: ttl,
	}
}

// GetKey returns the public key for the given key ID, fetching if necessary.
func (c *inMemoryJWKSCache) GetKey(ctx context.Context, kid string) (crypto.PublicKey, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Check if we need to refresh the cache
	if time.Since(c.lastFetch) > c.ttl || c.keyset == nil {
		if err := c.fetchKeys(ctx); err != nil {
			return nil, err
		}
	}

	// Find the key in the keyset
	key, ok := c.keyset.LookupKeyID(kid)
	if !ok {
		return nil, fmt.Errorf("key ID not found: %s", kid)
	}

	// Extract the public key
	var rawKey interface{}
	if err := key.Raw(&rawKey); err != nil {
		return nil, fmt.Errorf("failed to extract public key: %w", err)
	}

	pubKey, ok := rawKey.(crypto.PublicKey)
	if !ok {
		return nil, fmt.Errorf("key is not a public key")
	}

	return pubKey, nil
}

// fetchKeys fetches the JWKS from the remote URL and updates the cache.
func (c *inMemoryJWKSCache) fetchKeys(ctx context.Context) error {
	// Create a timeout for the fetch operation
	fetchCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(fetchCtx, http.MethodGet, c.url, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to fetch JWKS: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("JWKS fetch returned status %d", resp.StatusCode)
	}

	// Read the response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read JWKS response: %w", err)
	}

	// Parse the JWKS
	keyset, err := jwk.Parse(body)
	if err != nil {
		return fmt.Errorf("failed to parse JWKS: %w", err)
	}

	c.keyset = keyset
	c.lastFetch = time.Now()
	return nil
}
