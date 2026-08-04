package gpx

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func elevation(value float64) *float64 { return &value }

func TestCalculateDistance(t *testing.T) {
	a := Analyzer{}
	require.InDelta(t, 139.5, a.CalculateDistance(Point{Lat: 40.4168, Lon: -3.7038}, Point{Lat: 40.4178, Lon: -3.7048}), 2)
	require.Zero(t, a.CalculateDistance(Point{Lat: 40, Lon: -3}, Point{Lat: 40, Lon: -3}))
}

func TestCalculatePathDistance(t *testing.T) {
	a := Analyzer{}
	points := []Point{{Lat: 40.4168, Lon: -3.7038}, {Lat: 40.4178, Lon: -3.7048}, {Lat: 40.4188, Lon: -3.7058}}
	require.InDelta(t, 279, a.CalculatePathDistance(points), 4)
	require.Zero(t, a.CalculatePathDistance(points[:1]))
}

func TestCalculateTotalDPlus(t *testing.T) {
	a := Analyzer{}
	points := []Point{{Ele: elevation(100)}, {Ele: elevation(150)}, {Ele: elevation(125)}, {Ele: elevation(180)}}
	require.InDelta(t, 80, a.CalculateTotalDPlus(points, 30), 0.001)
	require.Zero(t, a.CalculateTotalDPlus([]Point{{Ele: nil}, {Ele: nil}}, 30))
}

func TestCalculateTotalDMinus(t *testing.T) {
	a := Analyzer{}
	points := []Point{{Ele: elevation(200)}, {Ele: elevation(150)}, {Ele: elevation(175)}, {Ele: elevation(100)}}
	require.InDelta(t, 100, a.CalculateTotalDMinus(points, 30), 0.001)
	require.Zero(t, a.CalculateTotalDMinus([]Point{{Ele: elevation(100)}, {Ele: elevation(100)}}, 30))
}

func TestCalculateAverageSlope(t *testing.T) {
	a := Analyzer{}
	require.InDelta(t, 5, a.CalculateAverageSlope(1000, 50), 0.001)
	require.Zero(t, a.CalculateAverageSlope(0, 50))
}

func TestCalculateEffortIndex(t *testing.T) {
	a := Analyzer{}
	require.InDelta(t, 15, a.CalculateEffortIndex(10, 500), 0.001)
	require.InDelta(t, 10, a.CalculateEffortIndex(10, 0), 0.001)
}

func TestCalculateITRAPoints(t *testing.T) {
	a := Analyzer{}
	require.InDelta(t, 3, a.CalculateITRAPoints(10, 500), 0.001)
	require.InDelta(t, 1.1, a.CalculateITRAPoints(5, 50), 0.001)
}

func TestCalculateLegBreakerIndex(t *testing.T) {
	a := Analyzer{}
	require.InDelta(t, 10, a.CalculateLegBreakerIndex([]float64{0, 10, -10}), 0.001)
	require.Zero(t, a.CalculateLegBreakerIndex([]float64{5}))
}

func TestCalculateEstimatedVAM(t *testing.T) {
	a := Analyzer{}
	require.InDelta(t, 1000, a.CalculateEstimatedVAM(10000, 1000, 3600), 0.001)
	require.Zero(t, a.CalculateEstimatedVAM(0, 1000, 3600))
}

func TestCalculateAdjustedSpeed(t *testing.T) {
	a := Analyzer{}
	require.InDelta(t, 2, a.CalculateAdjustedSpeed(2, 0), 0.001)
	require.InDelta(t, 1.5, a.CalculateAdjustedSpeed(2, 10), 0.001)
	require.Greater(t, a.CalculateAdjustedSpeed(2, -10), 2.0)
}

func TestCalculateDifficulty(t *testing.T) {
	a := Analyzer{}
	require.Equal(t, DifficultyBeginner, a.CalculateDifficulty(5000, 0, 2))
	require.Equal(t, DifficultyIntermediate, a.CalculateDifficulty(20000, 1000, 10))
	require.Equal(t, DifficultyAdvanced, a.CalculateDifficulty(50000, 2500, 20))
	require.Equal(t, DifficultyPro, a.CalculateDifficulty(100000, 5000, 50))
}

func TestCalculateRunnabilityIndex(t *testing.T) {
	a := Analyzer{}
	flat := &Track{Points: []Point{{Lat: 40, Lon: -3, Ele: elevation(100)}, {Lat: 40.001, Lon: -3, Ele: elevation(100)}}}
	require.InDelta(t, 1, a.CalculateRunnabilityIndex(flat), 0.001)
	steep := &Track{Points: []Point{{Lat: 40, Lon: -3, Ele: elevation(100)}, {Lat: 40.0001, Lon: -3, Ele: elevation(120)}}}
	require.Zero(t, a.CalculateRunnabilityIndex(steep))
}

func TestCalculateMovingTime(t *testing.T) {
	a := Analyzer{}
	start := time.Date(2026, 1, 1, 10, 0, 0, 0, time.UTC)
	t30 := start.Add(30 * time.Second)
	t150 := start.Add(150 * time.Second)
	require.Equal(t, 30, a.CalculateMovingTime([]Point{{Time: &start}, {Time: &t30}, {Time: &t150}}, 60))
	require.Zero(t, a.CalculateMovingTime([]Point{{}, {}}, 60))
}

func TestCalculateTotalDPlusFlatTrack(t *testing.T) {
	points := []Point{{Ele: elevation(100)}, {Ele: elevation(100)}}
	require.Zero(t, (Analyzer{}).CalculateTotalDPlus(points, 30))
}

func TestCalculateTotalDMinusTwoPoints(t *testing.T) {
	points := []Point{{Ele: elevation(150)}, {Ele: elevation(100)}}
	require.InDelta(t, 50, (Analyzer{}).CalculateTotalDMinus(points, 30), 0.001)
}

func TestCalculateAdjustedSpeedExtremeDownhill(t *testing.T) {
	require.InDelta(t, 1.4, (Analyzer{}).CalculateAdjustedSpeed(2, -50), 0.001)
}

func TestCalculateRunnabilityEmptyTrack(t *testing.T) {
	require.InDelta(t, 1, (Analyzer{}).CalculateRunnabilityIndex(nil), 0.001)
}
