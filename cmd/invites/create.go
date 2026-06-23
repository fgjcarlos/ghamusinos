package main

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"
)

// generateTokenAndHash crea un token criptográficamente seguro y devuelve
// tanto el token original (para imprimir al usuario) como su hash SHA-256
// (para almacenar en la base de datos).
func generateTokenAndHash(length int) (token, hash string, err error) {
	buf := make([]byte, length)
	if _, err := rand.Read(buf); err != nil {
		return "", "", fmt.Errorf("rand.Read: %w", err)
	}

	// Token original en hex.
	token = hex.EncodeToString(buf)

	// Hash SHA-256 del token (también en hex).
	hasher := sha256.New()
	hasher.Write(buf)
	hash = hex.EncodeToString(hasher.Sum(nil))

	return token, hash, nil
}

// parseDuration parsea una duración en formato time.ParseDuration.
// Ejemplos válidos: "7d" no es válido directamente, pero "168h" sí.
// Para simplificar, permitimos "Xd" convirtiéndolo a "X*24h".
func parseDuration(s string) (time.Duration, error) {
	// Si termina con 'd', lo convertimos a horas.
	if len(s) > 1 && s[len(s)-1] == 'd' {
		dayStr := s[:len(s)-1]
		var days int
		if _, err := fmt.Sscanf(dayStr, "%d", &days); err != nil {
			return 0, fmt.Errorf("formato de días inválido: %w", err)
		}
		s = fmt.Sprintf("%dh", days*24)
	}

	return time.ParseDuration(s)
}

// timeNowUTC devuelve la hora actual en UTC.
// Separado en su propia función para facilitar mocking en tests.
func timeNowUTC() time.Time {
	return time.Now().UTC()
}
