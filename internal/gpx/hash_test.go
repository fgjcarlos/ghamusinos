package gpx

import (
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

type failingReader struct{ err error }

func (r failingReader) Read([]byte) (int, error) { return 0, r.err }

func TestHasherReturnsSHA256Hex(t *testing.T) {
	got, err := (Hasher{}).Hash(strings.NewReader("hello"))
	require.NoError(t, err)
	require.Equal(t, "2cf24dba5fb0a30e26e83b2ac5b9e29e1b161e5c1fa7425e73043362938b9824", got)
}

func TestHasherIsDeterministic(t *testing.T) {
	first, err := (Hasher{}).Hash(strings.NewReader("track"))
	require.NoError(t, err)
	second, err := (Hasher{}).Hash(strings.NewReader("track"))
	require.NoError(t, err)
	require.Equal(t, first, second)
}

func TestHasherStreamsReader(t *testing.T) {
	got, err := (Hasher{}).Hash(io.LimitReader(strings.NewReader(strings.Repeat("gpx", 1000)), 3000))
	require.NoError(t, err)
	require.Len(t, got, 64)
}

func TestHasherReturnsReaderError(t *testing.T) {
	_, err := (Hasher{}).Hash(failingReader{err: errors.New("read failed")})
	require.ErrorContains(t, err, "read failed")
}
