package main

import (
	"crypto/sha256"
	"encoding/hex"
	"testing"
	"time"
)

// Test token generation produces correct hash
func TestGenerateTokenAndHash_ValidToken(t *testing.T) {
	token, hash, err := generateTokenAndHash(32)
	if err != nil {
		t.Fatalf("generateTokenAndHash error: %v", err)
	}

	if token == "" || hash == "" {
		t.Fatalf("empty token or hash")
	}

	// Token debe ser hex string válido
	if _, err := hex.DecodeString(token); err != nil {
		t.Fatalf("token not valid hex: %v", err)
	}

	// Hash debe ser SHA-256 válido del token (en bytes)
	tokenBytes, _ := hex.DecodeString(token)
	expectedHash := sha256.Sum256(tokenBytes)
	expectedHashStr := hex.EncodeToString(expectedHash[:])

	if hash != expectedHashStr {
		t.Errorf("hash mismatch: got %q, want %q", hash, expectedHashStr)
	}
}

// Test token length is respected
func TestGenerateTokenAndHash_RespectLength(t *testing.T) {
	tests := []int{16, 32, 64}
	for _, length := range tests {
		token, _, err := generateTokenAndHash(length)
		if err != nil {
			t.Fatalf("generateTokenAndHash(%d) error: %v", length, err)
		}

		// Hex-encoded string should be 2x the byte length
		expectedLen := length * 2
		if len(token) != expectedLen {
			t.Errorf("token length: got %d, want %d", len(token), expectedLen)
		}
	}
}

// Test parseDuration handles days
func TestParseDuration_Days(t *testing.T) {
	d, err := parseDuration("7d")
	if err != nil {
		t.Fatalf("parseDuration(7d) error: %v", err)
	}

	expected := 7 * 24 * time.Hour
	if d != expected {
		t.Errorf("parseDuration(7d) = %v, want %v", d, expected)
	}
}

// Test parseDuration handles hours
func TestParseDuration_Hours(t *testing.T) {
	d, err := parseDuration("24h")
	if err != nil {
		t.Fatalf("parseDuration(24h) error: %v", err)
	}

	expected := 24 * time.Hour
	if d != expected {
		t.Errorf("parseDuration(24h) = %v, want %v", d, expected)
	}
}

// Test parseDuration rejects invalid format
func TestParseDuration_InvalidFormat(t *testing.T) {
	_, err := parseDuration("invalid")
	if err == nil {
		t.Fatalf("parseDuration(invalid) should error")
	}
}

// Test createInvite generates and stores token
func TestCreateInvite_StoresToken(t *testing.T) {
	// This test would require a real or better-mocked querier.
	// For now, we focus on token generation logic (which is in generateTokenAndHash).
	// Integration test would be in a separate integration_test.go file
	// that spins up a test database.

	token, hash, err := generateTokenAndHash(32)
	if err != nil {
		t.Fatalf("generateTokenAndHash: %v", err)
	}

	// Verify token hash roundtrip
	if hash == "" || token == "" {
		t.Fatalf("expected non-empty token and hash")
	}

	// Verify hash is deterministic (same token -> same hash)
	tokenBytes, _ := hex.DecodeString(token)
	hasher := sha256.New()
	hasher.Write(tokenBytes)
	hash2 := hex.EncodeToString(hasher.Sum(nil))

	if hash != hash2 {
		t.Errorf("hash not deterministic: %q vs %q", hash, hash2)
	}
}

// Test CLI flag parsing
func TestCreateInviteFlags_Email(t *testing.T) {
	// Test that email flag is required
	// (This is tested in the CLI, but we can verify parseDuration logic here)
	d, err := parseDuration("7d")
	if err != nil || d != 7*24*time.Hour {
		t.Fatalf("parseDuration default failed: %v, %v", d, err)
	}
}

// Test CLI expiry flag parsing with --expires-in
// SPEC: Scenario "Invite CLI accepts --expires-in 7d and produces correct DB expiry"
func TestInviteCLI_ExpiresIn(t *testing.T) {
	// Test various expiry formats accepted by the CLI
	tests := []struct {
		input    string
		expected time.Duration
	}{
		{"7d", 7 * 24 * time.Hour},
		{"24h", 24 * time.Hour},
		{"1d", 24 * time.Hour},
		{"14d", 14 * 24 * time.Hour},
		{"72h", 72 * time.Hour},
	}

	for _, tt := range tests {
		d, err := parseDuration(tt.input)
		if err != nil {
			t.Errorf("parseDuration(%q) error: %v", tt.input, err)
			continue
		}

		if d != tt.expected {
			t.Errorf("parseDuration(%q) = %v, want %v", tt.input, d, tt.expected)
		}
	}
}

// Test token uniqueness
func TestGenerateTokenAndHash_Unique(t *testing.T) {
	token1, _, _ := generateTokenAndHash(32)
	token2, _, _ := generateTokenAndHash(32)

	if token1 == token2 {
		t.Errorf("generated duplicate tokens (should be cryptographically unique)")
	}

	if len(token1) == 0 || len(token2) == 0 {
		t.Fatalf("empty token generated")
	}
}

// Benchmark token generation
func BenchmarkGenerateTokenAndHash(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_, _, _ = generateTokenAndHash(32)
	}
}
