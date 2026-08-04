package gpx

import (
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func fixture(t *testing.T, name string) *os.File {
	t.Helper()
	f, err := os.Open("testdata/" + name)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, f.Close()) })
	return f
}

func TestParserParsesGPX11(t *testing.T) {
	track, err := (Parser{}).Parse(fixture(t, "valid_11.gpx"))
	require.NoError(t, err)
	require.Equal(t, "Madrid trail", track.Name)
	require.Len(t, track.Points, 2)
	require.InDelta(t, 650, *track.Points[0].Ele, 0.001)
	require.NotNil(t, track.Points[0].Time)
}

func TestParserParsesGPX10(t *testing.T) {
	track, err := (Parser{}).Parse(fixture(t, "valid_10.gpx"))
	require.NoError(t, err)
	require.Equal(t, "GPX 1.0", track.Name)
	require.Len(t, track.Points, 2)
}

func TestParserPreservesMissingElevation(t *testing.T) {
	track, err := (Parser{}).Parse(fixture(t, "no_elevation.gpx"))
	require.NoError(t, err)
	require.Nil(t, track.Points[0].Ele)
}

func TestParserPreservesMissingTime(t *testing.T) {
	track, err := (Parser{}).Parse(fixture(t, "no_time.gpx"))
	require.NoError(t, err)
	require.Nil(t, track.Points[0].Time)
}

func TestParserRejectsMalformedXML(t *testing.T) {
	_, err := (Parser{}).Parse(strings.NewReader(`<gpx><trk>`))
	require.ErrorContains(t, err, "invalid GPX")
}

func TestParserRejectsNoTrackpoints(t *testing.T) {
	_, err := (Parser{}).Parse(strings.NewReader(`<gpx version="1.1" creator="test"></gpx>`))
	require.ErrorContains(t, err, "no trackpoints")
}

func TestParserReturnsReaderError(t *testing.T) {
	_, err := (Parser{}).Parse(failingReader{err: errors.New("read failed")})
	require.ErrorContains(t, err, "invalid GPX: read")
}
