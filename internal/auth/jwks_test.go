package auth

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestJWKSCache_FetchesOnFirstCall(t *testing.T) {
	// Create a test server with a valid JWKS response
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		// Return a minimal but valid JWKS structure
		jwks := map[string]interface{}{
			"keys": []map[string]interface{}{
				{
					"kty": "RSA",
					"use": "sig",
					"kid": "test-key-1",
					"n":   "0vx7agoebGcQSuuPiLJXZptN9nndrQmbXEps2aiAFbWhM78LhWx4cbbfAAtVT86zwu1RK7aPFFxuhDR1L6tSoc_BJECPebWKRXjBZCiFV4n3oknjhMstn64tZ_2W-5JsGY4Hc5n9yBXArwl93lqt7_RN5w6Cf0h4QyQ5v-65YGjQR0_FDW2QvzqY368QQMicAtaSqzs8KJZgnYb9c7d0zgdAZHzu6qMQvRL5hajrn1n91CbOpbISD08qNLyrdkt-bFTWhAI4vMQFh6WeZu0fM4lFd2NcRwr3XPksINHaQ-G_xBniIqbw0Ls1jF44-csFCur-kEgU8awapJzKnqDKgw",
					"e":   "AQAB",
				},
			},
		}
		//nolint:errcheck

		//nolint:errcheck
		//nolint:errcheck
		//nolint:errcheck
		//nolint:errcheck
		json.NewEncoder(w).Encode(jwks)
	}))
	defer server.Close()

	cache := NewJWKSCache(server.URL, time.Hour)

	// First call should fetch
	pubKey, err := cache.GetKey(context.Background(), "test-key-1")
	if err != nil {
		t.Fatalf("failed to get key: %v", err)
	}
	if pubKey == nil {
		t.Fatal("expected non-nil public key")
	}
}

func TestJWKSCache_ReturnsCachedKeys(t *testing.T) {
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.Header().Set("Content-Type", "application/json")
		jwks := map[string]interface{}{
			"keys": []map[string]interface{}{
				{
					"kty": "RSA",
					"kid": "test-key",
					"n":   "0vx7agoebGcQSuuPiLJXZptN9nndrQmbXEps2aiAFbWhM78LhWx4cbbfAAtVT86zwu1RK7aPFFxuhDR1L6tSoc_BJECPebWKRXjBZCiFV4n3oknjhMstn64tZ_2W-5JsGY4Hc5n9yBXArwl93lqt7_RN5w6Cf0h4QyQ5v-65YGjQR0_FDW2QvzqY368QQMicAtaSqzs8KJZgnYb9c7d0zgdAZHzu6qMQvRL5hajrn1n91CbOpbISD08qNLyrdkt-bFTWhAI4vMQFh6WeZu0fM4lFd2NcRwr3XPksINHaQ-G_xBniIqbw0Ls1jF44-csFCur-kEgU8awapJzKnqDKgw",
					"e":   "AQAB",
				},
			},
		}
		//nolint:errcheck

		//nolint:errcheck
		//nolint:errcheck
		//nolint:errcheck
		//nolint:errcheck
		json.NewEncoder(w).Encode(jwks)
	}))
	defer server.Close()

	cache := NewJWKSCache(server.URL, time.Hour)

	// First call
	_, err := cache.GetKey(context.Background(), "test-key")
	if err != nil {
		t.Fatalf("first call failed: %v", err)
	}

	// Second call (within TTL) should use cache
	_, err = cache.GetKey(context.Background(), "test-key")
	if err != nil {
		t.Fatalf("second call failed: %v", err)
	}

	if callCount != 1 {
		t.Errorf("expected 1 HTTP call, got %d (cache not working)", callCount)
	}
}

func TestJWKSCache_RefreshesAfterTTL(t *testing.T) {
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.Header().Set("Content-Type", "application/json")
		jwks := map[string]interface{}{
			"keys": []map[string]interface{}{
				{
					"kty": "RSA",
					"kid": "test-key",
					"n":   "0vx7agoebGcQSuuPiLJXZptN9nndrQmbXEps2aiAFbWhM78LhWx4cbbfAAtVT86zwu1RK7aPFFxuhDR1L6tSoc_BJECPebWKRXjBZCiFV4n3oknjhMstn64tZ_2W-5JsGY4Hc5n9yBXArwl93lqt7_RN5w6Cf0h4QyQ5v-65YGjQR0_FDW2QvzqY368QQMicAtaSqzs8KJZgnYb9c7d0zgdAZHzu6qMQvRL5hajrn1n91CbOpbISD08qNLyrdkt-bFTWhAI4vMQFh6WeZu0fM4lFd2NcRwr3XPksINHaQ-G_xBniIqbw0Ls1jF44-csFCur-kEgU8awapJzKnqDKgw",
					"e":   "AQAB",
				},
			},
		}
		//nolint:errcheck

		//nolint:errcheck
		//nolint:errcheck
		//nolint:errcheck
		//nolint:errcheck
		json.NewEncoder(w).Encode(jwks)
	}))
	defer server.Close()

	// Very short TTL
	cache := NewJWKSCache(server.URL, 10*time.Millisecond)

	// First call
	_, err := cache.GetKey(context.Background(), "test-key")
	if err != nil {
		t.Fatalf("first call failed: %v", err)
	}

	// Wait for TTL to expire
	time.Sleep(20 * time.Millisecond)

	// Second call should refresh
	_, err = cache.GetKey(context.Background(), "test-key")
	if err != nil {
		t.Fatalf("second call failed: %v", err)
	}

	if callCount != 2 {
		t.Errorf("expected 2 HTTP calls after TTL, got %d", callCount)
	}
}

func TestJWKSCache_FetchErrorReturnsError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	cache := NewJWKSCache(server.URL, time.Hour)

	_, err := cache.GetKey(context.Background(), "test-key")
	if err == nil {
		t.Fatal("expected error on HTTP 404")
	}
}

func TestJWKSCache_MissingKidReturnsError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		jwks := map[string]interface{}{
			"keys": []map[string]interface{}{
				{
					"kty": "RSA",
					"kid": "wrong-key",
					"n":   "0vx7agoebGcQSuuPiLJXZptN9nndrQmbXEps2aiAFbWhM78LhWx4cbbfAAtVT86zwu1RK7aPFFxuhDR1L6tSoc_BJECPebWKRXjBZCiFV4n3oknjhMstn64tZ_2W-5JsGY4Hc5n9yBXArwl93lqt7_RN5w6Cf0h4QyQ5v-65YGjQR0_FDW2QvzqY368QQMicAtaSqzs8KJZgnYb9c7d0zgdAZHzu6qMQvRL5hajrn1n91CbOpbISD08qNLyrdkt-bFTWhAI4vMQFh6WeZu0fM4lFd2NcRwr3XPksINHaQ-G_xBniIqbw0Ls1jF44-csFCur-kEgU8awapJzKnqDKgw",
					"e":   "AQAB",
				},
			},
		}
		//nolint:errcheck

		//nolint:errcheck
		//nolint:errcheck
		//nolint:errcheck
		//nolint:errcheck
		json.NewEncoder(w).Encode(jwks)
	}))
	defer server.Close()

	cache := NewJWKSCache(server.URL, time.Hour)

	_, err := cache.GetKey(context.Background(), "missing-key")
	if err == nil {
		t.Fatal("expected error when key ID not found in JWKS")
	}
}

// TestJWKSCache_RefetchesOnUnknownKid simula una rotación de claves
// de Clerk: el primer fetch devuelve la clave A, el segundo la clave B.
// Cuando llega un token con kid B después de un primer kid A, el caché
// debe forzar un refetch y devolver la nueva clave. Sin esto, la
// rotación de Clerk tira 401 durante una hora. Issue #167.
func TestJWKSCache_RefetchesOnUnknownKid(t *testing.T) {
	var callCount int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := atomic.AddInt32(&callCount, 1)
		w.Header().Set("Content-Type", "application/json")
		// Primera respuesta: clave "key-A". Segunda: clave "key-B".
		kid := "key-A"
		if n >= 2 {
			kid = "key-B"
		}
		jwks := map[string]interface{}{
			"keys": []map[string]interface{}{
				{
					"kty": "RSA",
					"kid": kid,
					"n":   "0vx7agoebGcQSuuPiLJXZptN9nndrQmbXEps2aiAFbWhM78LhWx4cbbfAAtVT86zwu1RK7aPFFxuhDR1L6tSoc_BJECPebWKRXjBZCiFV4n3oknjhMstn64tZ_2W-5JsGY4Hc5n9yBXArwl93lqt7_RN5w6Cf0h4QyQ5v-65YGjQR0_FDW2QvzqY368QQMicAtaSqzs8KJZgnYb9c7d0zgdAZHzu6qMQvRL5hajrn1n91CbOpbISD08qNLyrdkt-bFTWhAI4vMQFh6WeZu0fM4lFd2NcRwr3XPksINHaQ-G_xBniIqbw0Ls1jF44-csFCur-kEgU8awapJzKnqDKgw",
					"e":   "AQAB",
				},
			},
		}
		_ = json.NewEncoder(w).Encode(jwks)
	}))
	defer server.Close()

	cache := NewJWKSCache(server.URL, time.Hour)

	// 1) kid A: primer fetch, devuelve key-A.
	if _, err := cache.GetKey(context.Background(), "key-A"); err != nil {
		t.Fatalf("first GetKey(key-A) failed: %v", err)
	}
	// 2) kid B: el caché no la tiene, refetch y devuelve key-B.
	if _, err := cache.GetKey(context.Background(), "key-B"); err != nil {
		t.Fatalf("GetKey(key-B) after rotation failed: %v", err)
	}
	if got := atomic.LoadInt32(&callCount); got != 2 {
		t.Errorf("expected 2 fetches (initial + refetch on unknown kid), got %d", got)
	}
}

// TestJWKSCache_UnknownKidCooldownLimitsAmplification verifica que un
// atacante que mande un kid basura 100 veces seguidas sólo consigue
// DOS fetches HTTP: el inicial (caché vacía) y el primer refetch
// (cooldown vencido). Los 98 siguientes caen en el cooldown. Sin el
// cooldown, el endpoint de Clerk se convierte en un objetivo de
// amplificación. Issue #167.
func TestJWKSCache_UnknownKidCooldownLimitsAmplification(t *testing.T) {
	var callCount int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&callCount, 1)
		w.Header().Set("Content-Type", "application/json")
		// El servidor siempre devuelve una clave distinta a la pedida.
		jwks := map[string]interface{}{
			"keys": []map[string]interface{}{
				{
					"kty": "RSA",
					"kid": "server-key",
					"n":   "0vx7agoebGcQSuuPiLJXZptN9nndrQmbXEps2aiAFbWhM78LhWx4cbbfAAtVT86zwu1RK7aPFFxuhDR1L6tSoc_BJECPebWKRXjBZCiFV4n3oknjhMstn64tZ_2W-5JsGY4Hc5n9yBXArwl93lqt7_RN5w6Cf0h4QyQ5v-65YGjQR0_FDW2QvzqY368QQMicAtaSqzs8KJZgnYb9c7d0zgdAZHzu6qMQvRL5hajrn1n91CbOpbISD08qNLyrdkt-bFTWhAI4vMQFh6WeZu0fM4lFd2NcRwr3XPksINHaQ-G_xBniIqbw0Ls1jF44-csFCur-kEgU8awapJzKnqDKgw",
					"e":   "AQAB",
				},
			},
		}
		_ = json.NewEncoder(w).Encode(jwks)
	}))
	defer server.Close()

	cache := NewJWKSCache(server.URL, time.Hour)

	for i := 0; i < 100; i++ {
		if _, err := cache.GetKey(context.Background(), "attacker-kid"); err == nil {
			t.Fatalf("iteration %d: expected error, got nil", i)
		}
	}

	got := atomic.LoadInt32(&callCount)
	// Caché vacía (1) + primer refetch al pedir el kid desconocido (1) = 2.
	// Los 98 siguientes caen en el cooldown.
	if got > 2 {
		t.Errorf("expected at most 2 fetches for 100 unknown-kid requests, got %d", got)
	}
}

// TestJWKSCache_ConcurrentFetchesCoalesce verifica que 50 goroutines
// llamando a GetKey con la misma kid y la caché fría provocan UN solo
// fetch HTTP. El singleflight dentro de refresh coalesce las llamadas
// concurrentes para no amplificar el endpoint de Clerk al arrancar
// después de un cold start. Issue #167.
func TestJWKSCache_ConcurrentFetchesCoalesce(t *testing.T) {
	var callCount int32
	// Un servidor que bloquea un instante para forzar la concurrencia:
	// si no coalesce, varias goroutines pasan la comprobación de caché
	// vacía y cada una hace su fetch.
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&callCount, 1)
		time.Sleep(50 * time.Millisecond)
		w.Header().Set("Content-Type", "application/json")
		jwks := map[string]interface{}{
			"keys": []map[string]interface{}{
				{
					"kty": "RSA",
					"kid": "shared-kid",
					"n":   "0vx7agoebGcQSuuPiLJXZptN9nndrQmbXEps2aiAFbWhM78LhWx4cbbfAAtVT86zwu1RK7aPFFxuhDR1L6tSoc_BJECPebWKRXjBZCiFV4n3oknjhMstn64tZ_2W-5JsGY4Hc5n9yBXArwl93lqt7_RN5w6Cf0h4QyQ5v-65YGjQR0_FDW2QvzqY368QQMicAtaSqzs8KJZgnYb9c7d0zgdAZHzu6qMQvRL5hajrn1n91CbOpbISD08qNLyrdkt-bFTWhAI4vMQFh6WeZu0fM4lFd2NcRwr3XPksINHaQ-G_xBniIqbw0Ls1jF44-csFCur-kEgU8awapJzKnqDKgw",
					"e":   "AQAB",
				},
			},
		}
		_ = json.NewEncoder(w).Encode(jwks)
	}))
	defer server.Close()

	cache := NewJWKSCache(server.URL, time.Hour)

	const goroutines = 50
	var wg sync.WaitGroup
	wg.Add(goroutines)
	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			if _, err := cache.GetKey(context.Background(), "shared-kid"); err != nil {
				t.Errorf("GetKey failed: %v", err)
			}
		}()
	}
	wg.Wait()

	if got := atomic.LoadInt32(&callCount); got != 1 {
		t.Errorf("expected 1 fetch for %d concurrent GetKey calls, got %d", goroutines, got)
	}
}
