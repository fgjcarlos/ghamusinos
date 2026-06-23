package crypto

import (
	"testing"
)

// TestEncryptDecryptRoundtrip verifies that Encrypt followed by Decrypt recovers the original plaintext.
func TestEncryptDecryptRoundtrip(t *testing.T) {
	key := make([]byte, 32) // 32 bytes of zeros for testing
	for i := range key {
		key[i] = byte(i % 256)
	}

	plaintext := []byte("test token: secret_access_token")

	// Encrypt
	ciphertext, err := Encrypt(key, plaintext)
	if err != nil {
		t.Fatalf("Encrypt failed: %v", err)
	}
	if len(ciphertext) < 12 {
		t.Fatalf("ciphertext too short, expected at least 12 bytes (nonce), got %d", len(ciphertext))
	}

	// Decrypt
	recovered, err := Decrypt(key, ciphertext)
	if err != nil {
		t.Fatalf("Decrypt failed: %v", err)
	}

	if string(recovered) != string(plaintext) {
		t.Fatalf("plaintext mismatch: want %q, got %q", plaintext, recovered)
	}
}

// TestDecryptWithWrongKey verifies that decryption fails with a different key.
func TestDecryptWithWrongKey(t *testing.T) {
	key1 := make([]byte, 32)
	for i := range key1 {
		key1[i] = byte(i % 256)
	}

	key2 := make([]byte, 32)
	for i := range key2 {
		key2[i] = byte((i + 1) % 256)
	}

	plaintext := []byte("secret data")

	// Encrypt with key1
	ciphertext, err := Encrypt(key1, plaintext)
	if err != nil {
		t.Fatalf("Encrypt failed: %v", err)
	}

	// Try to decrypt with key2 (should fail)
	_, err = Decrypt(key2, ciphertext)
	if err == nil {
		t.Fatal("Decrypt with wrong key should fail but didn't")
	}
}

// TestDecryptTamperedCiphertext verifies that decryption fails if the ciphertext is tampered.
func TestDecryptTamperedCiphertext(t *testing.T) {
	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(i % 256)
	}

	plaintext := []byte("tamper test data")

	// Encrypt
	ciphertext, err := Encrypt(key, plaintext)
	if err != nil {
		t.Fatalf("Encrypt failed: %v", err)
	}

	// Tamper with ciphertext (flip a bit after the nonce)
	if len(ciphertext) > 12 {
		ciphertext[12] ^= 0x01
	}

	// Try to decrypt (should fail due to GCM authentication)
	_, err = Decrypt(key, ciphertext)
	if err == nil {
		t.Fatal("Decrypt of tampered ciphertext should fail but didn't")
	}
}

// TestEncryptInvalidKeyLength verifies that Encrypt rejects keys not exactly 32 bytes.
func TestEncryptInvalidKeyLength(t *testing.T) {
	plaintext := []byte("test")

	tests := []struct {
		name    string
		keyLen  int
		wantErr bool
	}{
		{"24 bytes", 24, true},
		{"31 bytes", 31, true},
		{"32 bytes (valid)", 32, false},
		{"33 bytes", 33, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			key := make([]byte, tt.keyLen)
			_, err := Encrypt(key, plaintext)
			if (err != nil) != tt.wantErr {
				t.Fatalf("Encrypt(key len %d): got err %v, want err=%v", tt.keyLen, err, tt.wantErr)
			}
		})
	}
}

// TestDecryptInvalidKeyLength verifies that Decrypt rejects keys not exactly 32 bytes.
func TestDecryptInvalidKeyLength(t *testing.T) {
	tests := []struct {
		name    string
		keyLen  int
		wantErr bool
	}{
		{"24 bytes", 24, true},
		{"31 bytes", 31, true},
		{"33 bytes", 33, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			key := make([]byte, tt.keyLen)
			// Use a dummy ciphertext; with wrong key length, it should fail before validating auth
			ciphertext := make([]byte, 28)
			_, err := Decrypt(key, ciphertext)
			if (err != nil) != tt.wantErr {
				t.Fatalf("Decrypt(key len %d): got err %v, want err=%v", tt.keyLen, err, tt.wantErr)
			}
		})
	}
}

// TestDecryptValidKeyLength verifies that Decrypt works with a valid 32-byte key and valid ciphertext.
func TestDecryptValidKeyLength(t *testing.T) {
	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(i % 256)
	}
	plaintext := []byte("test message")

	// Create a valid ciphertext with the key
	ciphertext, err := Encrypt(key, plaintext)
	if err != nil {
		t.Fatalf("Encrypt setup failed: %v", err)
	}

	// Decrypt with valid key should succeed
	recovered, err := Decrypt(key, ciphertext)
	if err != nil {
		t.Fatalf("Decrypt with valid 32-byte key failed: %v", err)
	}
	if string(recovered) != string(plaintext) {
		t.Fatalf("plaintext mismatch: want %q, got %q", plaintext, recovered)
	}
}

// TestEncryptEmptyPlaintext verifies that encrypting empty plaintext works.
func TestEncryptEmptyPlaintext(t *testing.T) {
	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(i % 256)
	}

	plaintext := []byte{}

	// Encrypt empty plaintext
	ciphertext, err := Encrypt(key, plaintext)
	if err != nil {
		t.Fatalf("Encrypt(empty) failed: %v", err)
	}
	if len(ciphertext) < 12 {
		t.Fatalf("empty plaintext ciphertext too short")
	}

	// Decrypt and verify it's empty
	recovered, err := Decrypt(key, ciphertext)
	if err != nil {
		t.Fatalf("Decrypt failed: %v", err)
	}
	if len(recovered) != 0 {
		t.Fatalf("recovered should be empty, got %v", recovered)
	}
}
