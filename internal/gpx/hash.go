package gpx

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
)

type Hasher struct{}

func (Hasher) Hash(reader io.Reader) (string, error) {
	hash := sha256.New()
	if _, err := io.Copy(hash, reader); err != nil {
		return "", fmt.Errorf("hash GPX: %w", err)
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}
