// Package crypto ofrece primitivas de cifrado simétrico para secretos
// persistidos (tokens OAuth de Strava, etc.).
//
// Usa AES-256-GCM (AEAD). El sobre es nonce(12) || ciphertext || tag(16)
// codificado en base64 estándar, un único string fácil de guardar en una
// columna TEXT.
//
// # Modelo de amenaza
//
// - NO protege contra un atacante que también tiene la KEY (si la DB está
//   comprometida Y el .env también, no hay defensa).
// - SÍ protege contra robo de backup de la DB, acceso SQL con un usuario
//   de aplicación sin acceso a la KEY, o un dump filtrado.
//
// # Gestión de la KEY
//
// La KEY nunca se persiste en este paquete. La lee el caller (típicamente
// config.Load) y se pasa a Encrypt/Decrypt. Rotar la KEY requiere re-cifrar
// todas las filas (no cubierto aquí; trampolín natural en una migración
// cuando llegue el momento).
package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
)

// KeyLength es el tamaño obligatorio de la clave AES-256 en bytes.
const KeyLength = 32

// nonceLength es el tamaño de nonce que GCM usa por defecto (12 bytes).
// Documentado por si en el futuro queremos cambiarlo.
const nonceLength = 12

// ErrKeyLength se devuelve cuando la clave no tiene KeyLength bytes.
// El error es sentinel para que los callers puedan distinguirlo.
var ErrKeyLength = errors.New("crypto: la clave debe tener 32 bytes (AES-256)")

// Encrypt cifra plaintext con AES-256-GCM usando key y devuelve el sobre
// completo (nonce || ciphertext || tag) codificado en base64 estándar.
//
// Cada llamada genera un nonce aleatorio con crypto/rand. Reutilizar nonce
// con la misma clave rompe GCM — por eso nunca se acepta nonce externo.
func Encrypt(plaintext, key []byte) (string, error) {
	if len(key) != KeyLength {
		return "", ErrKeyLength
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		// aes.NewCipher solo falla si len(key) != 16/24/32, ya filtrado arriba.
		return "", fmt.Errorf("crypto: NewCipher: %w", err)
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("crypto: NewGCM: %w", err)
	}

	nonce := make([]byte, nonceLength)
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", fmt.Errorf("crypto: nonce aleatorio: %w", err)
	}

	// Seal anexa el sobre al primer argumento; pasando nonce + plaintext
	// obtenemos nonce || ciphertext || tag en un único slice.
	sealed := aead.Seal(nonce, nonce, plaintext, nil)
	return base64.StdEncoding.EncodeToString(sealed), nil
}

// Decrypt abre un sobre producido por Encrypt y devuelve el plaintext.
// Falla con error claro si la clave es incorrecta, el sobre está corrupto
// o la autenticación falla (cualquiera de los tres produce error de GCM;
// no se distingue porque revelar "auth ok pero contenido basura" sería
// útil a un atacante que manipula ciphertexts).
func Decrypt(ciphertextB64 string, key []byte) ([]byte, error) {
	if len(key) != KeyLength {
		return nil, ErrKeyLength
	}

	sealed, err := base64.StdEncoding.DecodeString(ciphertextB64)
	if err != nil {
		return nil, fmt.Errorf("crypto: base64: %w", err)
	}
	if len(sealed) < nonceLength {
		return nil, errors.New("crypto: sobre demasiado corto")
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("crypto: NewCipher: %w", err)
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("crypto: NewGCM: %w", err)
	}

	nonce, ct := sealed[:nonceLength], sealed[nonceLength:]
	plaintext, err := aead.Open(nil, nonce, ct, nil)
	if err != nil {
		return nil, fmt.Errorf("crypto: open: %w", err)
	}
	return plaintext, nil
}

// GenerateKey devuelve una clave aleatoria de 32 bytes apta para Encrypt.
// Útil para tests, bootstrapping y comandos de gestión. NO usar para
// derivar claves de passwords (eso es trabajo de scrypt/argon2, fuera de
// alcance de este paquete).
func GenerateKey() ([]byte, error) {
	key := make([]byte, KeyLength)
	if _, err := io.ReadFull(rand.Reader, key); err != nil {
		return nil, fmt.Errorf("crypto: clave aleatoria: %w", err)
	}
	return key, nil
}