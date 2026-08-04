package gpx

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDetectSteepZonesDetectsTwentyMeterSegment(t *testing.T) {
	track := &Track{Points: []Point{testPoint(0, 100), testPoint(30, 109)}}

	zones, err := (RiskService{}).DetectSteep(track)
	require.NoError(t, err)
	require.Len(t, zones, 1)
	require.Equal(t, "steep", zones[0].RiskType)
	require.Equal(t, "medium", zones[0].Severity)
	require.Equal(t, 0, zones[0].StartIdx)
	require.Equal(t, 1, zones[0].EndIdx)
}

func TestDetectSteepZonesIgnoresShortSegment(t *testing.T) {
	track := &Track{Points: []Point{testPoint(0, 100), testPoint(10, 103)}}

	zones, err := (RiskService{}).DetectSteep(track)
	require.NoError(t, err)
	require.Empty(t, zones)
}

func TestDetectTechnicalZonesFlagsHighCoefficientVariation(t *testing.T) {
	track := &Track{Points: []Point{
		testPoint(0, 100), testPoint(25, 98.75), testPoint(50, 106.25), testPoint(75, 103.75), testPoint(100, 113.75),
	}}

	zones, err := (RiskService{}).DetectTechnical(track)
	require.NoError(t, err)
	require.Len(t, zones, 1)
	require.Equal(t, "technical", zones[0].RiskType)
	require.Equal(t, "medium", zones[0].Severity)
	require.Equal(t, 0, zones[0].StartIdx)
	require.Equal(t, 4, zones[0].EndIdx)
}

func TestDetectTechnicalZonesIgnoresStableSlope(t *testing.T) {
	track := &Track{Points: []Point{
		testPoint(0, 100), testPoint(25, 102.5), testPoint(50, 105), testPoint(75, 107.5), testPoint(100, 110),
	}}

	zones, err := (RiskService{}).DetectTechnical(track)
	require.NoError(t, err)
	require.Empty(t, zones)
}

func TestDetectExposureZonesDetectsHundredMetersAboveThreshold(t *testing.T) {
	track := &Track{Points: []Point{testPoint(0, 2600), testPoint(100, 2610), testPoint(200, 2620)}}

	zones, err := (RiskService{}).DetectExposure(track)
	require.NoError(t, err)
	require.Len(t, zones, 1)
	require.Equal(t, "exposure", zones[0].RiskType)
	require.Equal(t, "high", zones[0].Severity)
	require.Equal(t, 0, zones[0].StartIdx)
	require.Equal(t, 2, zones[0].EndIdx)
}

func TestDetectExposureZonesIgnoresShortHighAltitudeSegment(t *testing.T) {
	track := &Track{Points: []Point{testPoint(0, 2600), testPoint(99, 2610)}}

	zones, err := (RiskService{}).DetectExposure(track)
	require.NoError(t, err)
	require.Empty(t, zones)
}

func TestRiskDetectorCombinesAllZoneTypes(t *testing.T) {
	track := &Track{Points: []Point{
		testPoint(0, 2600), testPoint(30, 2609), testPoint(100, 2590), testPoint(200, 2630),
	}}

	zones, err := (RiskService{}).Detect(track)
	require.NoError(t, err)
	require.NotEmpty(t, zones)
	require.Contains(t, []string{"steep", "technical", "exposure"}, zones[0].RiskType)
}
