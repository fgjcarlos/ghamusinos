package main

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strconv"
	"time"
)

// generateTokenAndHash crea un token criptográficamente seguro y devuelve
// tanto el token original (para imprimir al usuario) como su hash SHA-256
// (para almacenar en la base de datos).
//
// El hash es SHA-256 de la cadena hex que se entrega al usuario, NO de
// los bytes aleatorios crudos. El contrato observable pasa a ser el
// evidente: `hash == sha256hex(token que el usuario pega)`. Antes
// hasheábamos los bytes crudos, lo que rompe cualquier validación que
// el día de mañana use el token hex — issue #173, M15.
func generateTokenAndHash(length int) (token, hash string, err error) {
	buf := make([]byte, length)
	if _, err := rand.Read(buf); err != nil {
		return "", "", fmt.Errorf("rand.Read: %w", err)
	}

	// Token original en hex: lo que verá el administrador y, eventualmente,
	// el usuario invitado.
	token = hex.EncodeToString(buf)

	// Hash SHA-256 del token en su forma hex (no de los bytes aleatorios).
	hasher := sha256.New()
	hasher.Write([]byte(token))
	hash = hex.EncodeToString(hasher.Sum(nil))

	return token, hash, nil
}

// parseDuration parsea una duración en formato time.ParseDuration.
// Ejemplos válidos: "7d" no es válido directamente, pero "168h" sí.
// Para simplificar, permitimos "Xd" convirtiéndolo a "X*24h".
func parseDuration(s string) (time.Duration, error) {
	// Si termina con 'd', lo convertimos a horas. strconv.Atoi (no
	// fmt.Sscanf) para que "7dx" falle ruidosamente en vez de
	// descartarse — issue #173, bonus.
	if len(s) > 1 && s[len(s)-1] == 'd' {
		dayStr := s[:len(s)-1]
		days, err := strconv.Atoi(dayStr)
		if err != nil {
			return 0, fmt.Errorf("formato de días inválido %q: %w", dayStr, err)
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
