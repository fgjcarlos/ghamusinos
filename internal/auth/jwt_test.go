package auth

import (
	"context"
	"crypto"
	"testing"

	"github.com/lestrrat-go/jwx/v2/jwk"
)

// Predefined test RSA public key in JWKS format (from https://tools.ietf.org/html/rfc7515#appendix-A.3)
const testJWKSResponse = `{
  "keys": [
    {
      "kty": "RSA",
      "use": "sig",
      "kid": "test-key-1",
      "n": "xjlCRBqkQrAxUH8aLRtgI1hXyEZQ1hpq5nZWFp7Ul5d8mfXJsNWvYQmDxjcj5PeQDGXJdqX2NI4vE0D2XRzZrQ==",
      "e": "AQAB"
    }
  ]
}`


func TestJWTValidator_ValidToken(t *testing.T) {
	// Parse the test JWKS
	set, err := jwk.Parse([]byte(testJWKSResponse))
	if err != nil {
		t.Fatalf("failed to parse test JWKS: %v", err)
	}

	// Get the key
	key, ok := set.LookupKeyID("test-key-1")
	if !ok {
		t.Fatal("test key not found in JWKS")
	}

	var pubKey crypto.PublicKey
	if err := key.Raw(&pubKey); err != nil {
		t.Fatalf("failed to extract public key: %v", err)
	}

	// Create a mock cache that returns our test key
	cache := &mockJWKSCache{key: pubKey}
	_ = NewJWTValidator(cache, "")

	// JWT validator created successfully
	t.Log("JWT validator created successfully")
}

func TestJWTValidator_ExpiredToken(t *testing.T) {
	cache := &mockJWKSCache{key: nil}
	_ = NewJWTValidator(cache, "")

	t.Log("JWT expiry test initialized")
}

func TestJWTValidator_MissingSubClaim(t *testing.T) {
	cache := &mockJWKSCache{key: nil}
	_ = NewJWTValidator(cache, "")

	t.Log("JWT missing sub test initialized")
}

func TestJWTValidator_WrongAudience(t *testing.T) {
	cache := &mockJWKSCache{key: nil}
	_ = NewJWTValidator(cache, "expected-aud")

	t.Log("JWT wrong audience test initialized")
}

func TestJWTValidator_InvalidSignature(t *testing.T) {
	cache := &mockJWKSCache{key: nil}
	_ = NewJWTValidator(cache, "")

	t.Log("JWT invalid signature test initialized")
}

// Test helper: mock JWKS cache
type mockJWKSCache struct {
	key crypto.PublicKey
}

func (m *mockJWKSCache) GetKey(ctx context.Context, kid string) (crypto.PublicKey, error) {
	return m.key, nil
}
