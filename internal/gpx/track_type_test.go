package gpx

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDetectTrackTypeClassifiesCircularBelowHundredMeters(t *testing.T) {
	track := &Track{Points: []Point{
		testPoint(0, 100), {Lat: 0, Lon: 0.001, Ele: floatPointer(100)}, testPoint(80, 100),
	}}

	result, err := (TrackTypeService{}).Detect(track)
	require.NoError(t, err)
	require.Equal(t, "circular", result.Type)
	require.NotEmpty(t, result.Direction)
}

func TestDetectTrackTypeClassifiesPointToPointAtHundredMeters(t *testing.T) {
	track := &Track{Points: []Point{testPoint(0, 100), testPoint(101, 100)}}

	result, err := (TrackTypeService{}).Detect(track)
	require.NoError(t, err)
	require.Equal(t, "point-to-point", result.Type)
	require.Empty(t, result.Direction)
}

func TestDetectTrackTypeComputesOppositeLoopDirections(t *testing.T) {
	clockwise := &Track{Points: []Point{
		{Lat: 0, Lon: 0}, {Lat: 0.001, Lon: 0}, {Lat: 0.001, Lon: 0.001}, {Lat: 0, Lon: 0.001}, {Lat: 0, Lon: 0},
	}}
	counter := &Track{Points: []Point{
		{Lat: 0, Lon: 0}, {Lat: 0, Lon: 0.001}, {Lat: 0.001, Lon: 0.001}, {Lat: 0.001, Lon: 0}, {Lat: 0, Lon: 0},
	}}

	clockwiseResult, err := (TrackTypeService{}).Detect(clockwise)
	require.NoError(t, err)
	counterResult, err := (TrackTypeService{}).Detect(counter)
	require.NoError(t, err)
	require.Equal(t, "clockwise", clockwiseResult.Direction)
	require.Equal(t, "counterclockwise", counterResult.Direction)
	require.NotEqual(t, clockwiseResult.Direction, counterResult.Direction)
}

func floatPointer(value float64) *float64 { return &value }
