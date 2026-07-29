package crypto

import (
	"bytes"
	"encoding/base64"
	"errors"
	"strings"
	"testing"
)

func mustKey(t *testing.T) []byte {
	t.Helper()
	k, err := GenerateKey()
	if err != nil {
		t.Fatalf("GenerateKey: %v", err)
	}
	return k
}

// TestRoundTrip cubre el camino feliz y los casos de borde de longitud
// (vacío, un byte, bloques grandes). Es el guard mínimo que dice "si esto
// se rompe, los tokens guardados en la DB dejan de poderse descifrar".
func TestRoundTrip(t *testing.T) {
	key := mustKey(t)

	cases := []struct {
		name      string
		plaintext []byte
	}{
		{"vacio", []byte("")},
		{"un_byte", []byte("x")},
		{"corto", []byte("access-token-mock-12345")},
		{"largo", bytes.Repeat([]byte("A"), 4096)},
		{"binario", []byte{0x00, 0x01, 0xff, 0xfe, 0x80}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ct, err := Encrypt(tc.plaintext, key)
			if err != nil {
				t.Fatalf("Encrypt: %v", err)
			}
			if ct == "" {
				t.Fatal("ciphertext vacío")
			}
			pt, err := Decrypt(ct, key)
			if err != nil {
				t.Fatalf("Decrypt: %v", err)
			}
			if !bytes.Equal(pt, tc.plaintext) {
				t.Fatalf("round-trip mismatch: got %q, want %q", pt, tc.plaintext)
			}
		})
	}
}

// TestNonceUniqueness verifica que dos llamadas con la misma clave y
// el mismo plaintext producen ciphertexts distintos (nonces aleatorios).
// Si esto falla, GCM está roto.
func TestNonceUniqueness(t *testing.T) {
	key := mustKey(t)
	plain := []byte("refresh-token-igual")

	ct1, err := Encrypt(plain, key)
	if err != nil {
		t.Fatalf("Encrypt 1: %v", err)
	}
	ct2, err := Encrypt(plain, key)
	if err != nil {
		t.Fatalf("Encrypt 2: %v", err)
	}
	if ct1 == ct2 {
		t.Fatal("dos Encrypt con misma clave/plaintext devolvieron el mismo ciphertext: nonces no aleatorios")
	}
}

// TestKeyLength valida el guard de tamaño de clave. Sin él, una clave
// corta llega a NewCipher y devuelve error genérico confuso.
func TestKeyLength(t *testing.T) {
	for _, n := range []int{0, 16, 24, 31, 33, 64} {
		key := make([]byte, n)

		if _, err := Encrypt([]byte("x"), key); !errors.Is(err, ErrKeyLength) {
			t.Errorf("Encrypt con key len=%d: error = %v, want ErrKeyLength", n, err)
		}
		if _, err := Decrypt("aGVsbG8=", key); !errors.Is(err, ErrKeyLength) {
			t.Errorf("Decrypt con key len=%d: error = %v, want ErrKeyLength", n, err)
		}
	}
}

// TestDecryptRejectsTampered verifica que modificar un bit del ciphertext
// hace fallar la autenticación de GCM. Es la propiedad AEAD más
// importante para nosotros: detecta corrupción Y manipulación.
func TestDecryptRejectsTampered(t *testing.T) {
	key := mustKey(t)
	ct, err := Encrypt([]byte("payload"), key)
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}

	// Flipa el primer bit del payload (justo después del nonce de 12 bytes
	// codificados en base64 → ~16 chars). Tampering debe romper el tag.
	raw, err := decodeB64(ct)
	if err != nil {
		t.Fatalf("decode base64: %v", err)
	}
	if len(raw) <= nonceLength+1 {
		t.Fatal("ciphertext demasiado corto para manipular")
	}
	raw[nonceLength] ^= 0x01
	tampered := encodeB64(raw)

	if _, err := Decrypt(tampered, key); err == nil {
		t.Fatal("Decrypt aceptó ciphertext manipulado: AEAD no detecta tampering")
	} else if !strings.Contains(err.Error(), "open") {
		t.Errorf("error esperado relacionado con AEAD open, got: %v", err)
	}
}

// TestDecryptRejectsWrongKey verifica que cambiar la clave falla de
// forma clara (no devuelve basura).
func TestDecryptRejectsWrongKey(t *testing.T) {
	key1 := mustKey(t)
	key2 := mustKey(t)
	if bytes.Equal(key1, key2) {
		t.Fatal("dos GenerateKey devolvieron la misma clave: random roto")
	}

	ct, err := Encrypt([]byte("payload"), key1)
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}
	if _, err := Decrypt(ct, key2); err == nil {
		t.Fatal("Decrypt aceptó ciphertext con clave incorrecta")
	}
}

// TestDecryptRejectsCorruptedBase64 cubre entradas malformadas.
func TestDecryptRejectsCorruptedBase64(t *testing.T) {
	key := mustKey(t)
	if _, err := Decrypt("@@@ no es base64 @@@", key); err == nil {
		t.Fatal("Decrypt aceptó base64 inválido")
	}
	if _, err := Decrypt("AAAA", key); err == nil {
		t.Fatal("Decrypt aceptó sobre de 3 bytes (más corto que nonce)")
	}
}

// Helpers locales para tampering: el test toca el ciphertext crudo y lo
// re-codifica para verificar que GCM rechaza la manipulación.
func decodeB64(s string) ([]byte, error) {
	return base64.StdEncoding.DecodeString(s)
}

func encodeB64(b []byte) string {
	return base64.StdEncoding.EncodeToString(b)
}