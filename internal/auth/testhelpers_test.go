package auth

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"encoding/json"
	"testing"
	"time"

	"github.com/lestrrat-go/jwx/v2/jwa"
	"github.com/lestrrat-go/jwx/v2/jws"
	"github.com/lestrrat-go/jwx/v2/jwt"
)

// TestKeyPair holds a private key for signing and public key for verification in tests.
type TestKeyPair struct {
	PrivateKey crypto.PrivateKey
	PublicKey  crypto.PublicKey
	Kid        string
}

// GenerateTestKeyPair creates a new RSA key pair for test token signing.
// The key pair is generated fresh for each test.
func GenerateTestKeyPair(t *testing.T, kid string) *TestKeyPair {
	t.Helper()
	privKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("failed to generate test RSA key: %v", err)
	}
	return &TestKeyPair{
		PrivateKey: privKey,
		PublicKey:  &privKey.PublicKey,
		Kid:        kid,
	}
}

// SignToken signs a JWT token with the test private key.
// Claims should be a map of claim key-value pairs.
// kid, iss, aud, exp, nbf are added automatically if not provided.
func (kp *TestKeyPair) SignToken(t *testing.T, claims map[string]interface{}, overrides map[string]interface{}) string {
	t.Helper()

	// Merge defaults with overrides
	finalClaims := map[string]interface{}{
		"iss": "https://accounts.clerk.test",
		"aud": "test-audience",
		"exp": time.Now().Add(1 * time.Hour).Unix(),
		"iat": time.Now().Unix(),
		"nbf": time.Now().Unix(),
	}

	// Merge provided claims
	for k, v := range claims {
		finalClaims[k] = v
	}

	// Apply overrides
	for k, v := range overrides {
		finalClaims[k] = v
	}

	// Build the JWT token
	tok := jwt.New()
	for k, v := range finalClaims {
		if err := tok.Set(k, v); err != nil {
			t.Fatalf("failed to set JWT claim %s: %v", k, err)
		}
	}

	// Serialize token to JSON for the JWS payload
	payload, err := json.Marshal(finalClaims)
	if err != nil {
		t.Fatalf("failed to marshal claims: %v", err)
	}

	// Create headers with kid
	hdrs := jws.NewHeaders()
	if err := hdrs.Set("kid", kp.Kid); err != nil {
		t.Fatalf("failed to set kid in header: %v", err)
	}

	// Sign the payload with the header containing kid
	signed, err := jws.Sign(payload,
		jws.WithKey(jwa.RS256, kp.PrivateKey, jws.WithProtectedHeaders(hdrs)),
	)
	if err != nil {
		t.Fatalf("failed to sign JWT: %v", err)
	}

	return string(signed)
}
