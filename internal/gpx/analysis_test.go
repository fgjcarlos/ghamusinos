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

func TestCalculateElevation_FullCoverage(t *testing.T) {
	a := Analyzer{}
	// 4 puntos en línea recta (~111 m entre pares) con elevación
	// continua. Coverage debe ser 1.0, status Full.
	points := []Point{
		{Lat: 0, Lon: 0, Ele: elevation(100)},
		{Lat: 0.001, Lon: 0, Ele: elevation(150)},
		{Lat: 0.002, Lon: 0, Ele: elevation(125)},
		{Lat: 0.003, Lon: 0, Ele: elevation(180)},
	}
	res := a.CalculateElevation(points, 30)
	require.Equal(t, CoverageFull, res.Status)
	require.InDelta(t, 1.0, res.Coverage, 0.001)
	require.NotNil(t, res.DPlusM)
	require.InDelta(t, 80, *res.DPlusM, 0.001)
	require.NotNil(t, res.DMinusM)
	require.InDelta(t, 0, *res.DMinusM, 0.001)
}

func TestCalculateElevation_NilPointsInsufficient(t *testing.T) {
	a := Analyzer{}
	// 2 puntos sin elevación y a distancia cero → coverage 0, nil D+/D-.
	res := a.CalculateElevation([]Point{{Lat: 0, Lon: 0, Ele: nil}, {Lat: 0, Lon: 0, Ele: nil}}, 30)
	require.Equal(t, CoverageInsufficient, res.Status)
	require.Zero(t, res.Coverage)
	require.Nil(t, res.DPlusM)
	require.Nil(t, res.DMinusM)
}

func TestCalculateElevation_FlatTrack(t *testing.T) {
	a := Analyzer{}
	points := []Point{
		{Lat: 0, Lon: 0, Ele: elevation(100)},
		{Lat: 0.001, Lon: 0, Ele: elevation(100)},
	}
	res := a.CalculateElevation(points, 30)
	require.Equal(t, CoverageFull, res.Status)
	require.InDelta(t, 0, *res.DPlusM, 0.001)
	require.InDelta(t, 0, *res.DMinusM, 0.001)
}

func TestCalculateElevation_TwoPoints(t *testing.T) {
	a := Analyzer{}
	points := []Point{
		{Lat: 0, Lon: 0, Ele: elevation(150)},
		{Lat: 0.001, Lon: 0, Ele: elevation(100)},
	}
	res := a.CalculateElevation(points, 30)
	require.Equal(t, CoverageFull, res.Status)
	require.InDelta(t, 0, *res.DPlusM, 0.001)
	require.InDelta(t, 50, *res.DMinusM, 0.001)
}

// Acceptance: track with one missing elevation in 1000 must produce
// the same D+ as the full track, not zero.
func TestCalculateElevation_OneGapInMiddle(t *testing.T) {
	a := Analyzer{}
	full := make([]Point, 1000)
	for i := range full {
		full[i] = Point{Lat: 40 + float64(i)*0.0001, Lon: -3, Ele: elevation(100 + float64(i)*0.1)}
	}
	fullRes := a.CalculateElevation(full, 30)
	require.Equal(t, CoverageFull, fullRes.Status)

	// Insert a single missing-ele at index 500.
	withGap := append([]Point{}, full...)
	withGap[500] = Point{Lat: withGap[500].Lat, Lon: withGap[500].Lon, Ele: nil}
	gapRes := a.CalculateElevation(withGap, 30)

	require.Equal(t, CoverageFull, gapRes.Status,
		"missing a single point must not drop coverage below Full")
	require.InDelta(t, *fullRes.DPlusM, *gapRes.DPlusM, 1.0,
		"a single gap must not change D+ meaningfully")
}

// Acceptance: a hueco spanning a real elevation jump must not fabricate
// the climb across it. We build two tramos — climb from 100 to 500,
// gap, climb from 500 to 800 — and assert that the across-gap jump
// (100→800) is NOT summed.
func TestCalculateElevation_LargeGapDoesNotFabricate(t *testing.T) {
	a := Analyzer{}
	tramoA := []Point{
		{Lat: 0, Lon: 0, Ele: elevation(100)},
		{Lat: 0.001, Lon: 0, Ele: elevation(300)},
		{Lat: 0.002, Lon: 0, Ele: elevation(500)},
	}
	// Hueco: 2 puntos sin elevación, entre los dos tramos.
	farGap := []Point{
		{Lat: 0.003, Lon: 0, Ele: nil},
		{Lat: 0.004, Lon: 0, Ele: nil},
	}
	tramoB := []Point{
		{Lat: 0.005, Lon: 0, Ele: elevation(500)},
		{Lat: 0.006, Lon: 0, Ele: elevation(700)},
		{Lat: 0.007, Lon: 0, Ele: elevation(800)},
	}
	points := append(append(tramoA, farGap...), tramoB...)

	res := a.CalculateElevation(points, 30)
	// 4 de 6 pares tienen elevación → coverage 4/6 ≈ 0.67, status Partial.
	require.Equal(t, CoveragePartial, res.Status,
		"a partial gap should land in Partial territory")
	require.NotNil(t, res.DPlusM)
	// Tramo A contributed +400 (100→500) and tramo B +300 (500→800).
	// El salto artificial entre el último de A (500) y el primero de B
	// (500) es cero; si el algoritmo sumara cualquier salto cross-gap
	// inventado por error, este test lo cazaría.
	require.InDelta(t, 700, *res.DPlusM, 5.0,
		"D+ should sum 400 (tramoA) + 300 (tramoB), not 400+300+synthetic-gap-fabrication")
}

// Acceptance: coverage reflects the fraction of distance with usable
// elevation. Single punto sin elevación en medio de 4 → 2 de 4 pares
// cubiertos = 0.5.
func TestCalculateElevation_CoverageFraction(t *testing.T) {
	a := Analyzer{}
	points := []Point{
		{Lat: 0, Lon: 0, Ele: elevation(100)},
		{Lat: 0.001, Lon: 0, Ele: elevation(110)},
		{Lat: 0.002, Lon: 0, Ele: nil}, // hueco
		{Lat: 0.003, Lon: 0, Ele: elevation(120)},
		{Lat: 0.004, Lon: 0, Ele: elevation(130)},
	}
	res := a.CalculateElevation(points, 30)
	require.InDelta(t, 0.5, res.Coverage, 0.01,
		"a single mid-track gap affects 2 of 4 pairs (one before, one after)")
	require.Equal(t, CoveragePartial, res.Status)
	require.NotNil(t, res.DPlusM)
}

// Acceptance: with coverage < 0.5 the D+ must be nil, not zero.
func TestCalculateElevation_InsufficientCoverageReturnsNilDPlus(t *testing.T) {
	a := Analyzer{}
	// 1 punto con elevación + 9 sin elevación → 1 de 9 pares cubiertos ≈ 11%.
	points := []Point{{Lat: 0, Lon: 0, Ele: elevation(100)}}
	for i := 1; i < 10; i++ {
		points = append(points, Point{Lat: float64(i) * 0.001, Lon: 0, Ele: nil})
	}
	res := a.CalculateElevation(points, 30)
	require.Equal(t, CoverageInsufficient, res.Status)
	require.Nil(t, res.DPlusM)
	require.Nil(t, res.DMinusM)
}

// Acceptance: a track with no elevation at all → coverage 0, D+ nil.
func TestCalculateElevation_NoElevationInTrack(t *testing.T) {
	a := Analyzer{}
	points := []Point{
		{Lat: 0, Lon: 0, Ele: nil},
		{Lat: 0.001, Lon: 0, Ele: nil},
		{Lat: 0.002, Lon: 0, Ele: nil},
	}
	res := a.CalculateElevation(points, 30)
	require.Equal(t, CoverageInsufficient, res.Status)
	require.Zero(t, res.Coverage)
	require.Nil(t, res.DPlusM)
	require.Nil(t, res.DMinusM)
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

func TestCalculateElevation_RegressionFlat(t *testing.T) {
	points := []Point{
		{Lat: 0, Lon: 0, Ele: elevation(100)},
		{Lat: 0.001, Lon: 0, Ele: elevation(100)},
	}
	res := (Analyzer{}).CalculateElevation(points, 30)
	require.Equal(t, CoverageFull, res.Status)
	require.InDelta(t, 0, *res.DPlusM, 0.001)
	require.InDelta(t, 0, *res.DMinusM, 0.001)
}

func TestCalculateElevation_RegressionTwoPoints(t *testing.T) {
	points := []Point{
		{Lat: 0, Lon: 0, Ele: elevation(150)},
		{Lat: 0.001, Lon: 0, Ele: elevation(100)},
	}
	res := (Analyzer{}).CalculateElevation(points, 30)
	require.Equal(t, CoverageFull, res.Status)
	require.InDelta(t, 0, *res.DPlusM, 0.001)
	require.InDelta(t, 50, *res.DMinusM, 0.001)
}

func TestCalculateAdjustedSpeedExtremeDownhill(t *testing.T) {
	require.InDelta(t, 1.4, (Analyzer{}).CalculateAdjustedSpeed(2, -50), 0.001)
}

func TestCalculateRunnabilityEmptyTrack(t *testing.T) {
	require.InDelta(t, 1, (Analyzer{}).CalculateRunnabilityIndex(nil), 0.001)
}
