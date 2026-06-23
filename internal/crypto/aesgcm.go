// Package crypto provides encryption/decryption utilities for sensitive data like tokens.
package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"errors"
	"fmt"
	"io"
)

// Encrypt encrypts plaintext using AES-256-GCM with a random 12-byte nonce.
// The nonce is prepended to the ciphertext (first 12 bytes).
// Key must be exactly 32 bytes.
func Encrypt(key, plaintext []byte) ([]byte, error) {
	if len(key) != 32 {
		return nil, fmt.Errorf("crypto: key must be 32 bytes, got %d", len(key))
	}

	// Create cipher block
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("crypto: failed to create cipher: %w", err)
	}

	// Create GCM mode
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("crypto: failed to create GCM: %w", err)
	}

	// Generate random nonce (12 bytes for GCM standard)
	nonce := make([]byte, gcm.NonceSize())
	_, err = io.ReadFull(rand.Reader, nonce)
	if err != nil {
		return nil, fmt.Errorf("crypto: failed to generate nonce: %w", err)
	}

	// Encrypt (includes authentication tag)
	ciphertext := gcm.Seal(nil, nonce, plaintext, nil)

	// Prepend nonce to ciphertext
	return append(nonce, ciphertext...), nil
}

// Decrypt decrypts ciphertext using AES-256-GCM.
// The nonce is expected in the first 12 bytes of the ciphertext.
// Key must be exactly 32 bytes.
func Decrypt(key, ciphertext []byte) ([]byte, error) {
	if len(key) != 32 {
		return nil, fmt.Errorf("crypto: key must be 32 bytes, got %d", len(key))
	}

	// Check minimum length (12 bytes nonce + 16 bytes tag)
	if len(ciphertext) < 28 {
		return nil, errors.New("crypto: ciphertext too short")
	}

	// Create cipher block
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("crypto: failed to create cipher: %w", err)
	}

	// Create GCM mode
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("crypto: failed to create GCM: %w", err)
	}

	// Extract nonce (first 12 bytes)
	nonce := ciphertext[:gcm.NonceSize()]
	// Extract sealed data (everything after nonce)
	sealed := ciphertext[gcm.NonceSize():]

	// Decrypt and verify
	plaintext, err := gcm.Open(nil, nonce, sealed, nil)
	if err != nil {
		return nil, fmt.Errorf("crypto: decryption failed: %w", err)
	}

	return plaintext, nil
}

// ConstantTimeCompare performs constant-time comparison of two byte slices.
// This is provided for convenience; users should prefer crypto/subtle.ConstantTimeCompare
// for most cases.
func ConstantTimeCompare(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}

	result := byte(0)
	for i := range a {
		result |= a[i] ^ b[i]
	}
	return result == 0
}
