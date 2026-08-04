package gpx

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func testPoint(distanceM, elevationM float64) Point {
	ele := elevationM
	return Point{Lat: distanceM / 111195, Lon: 0, Ele: &ele}
}

func TestFindAllClimbsDetectsSeparatedAscents(t *testing.T) {
	track := &Track{Points: []Point{
		testPoint(0, 100), testPoint(100, 155), testPoint(201, 120), testPoint(301, 175),
	}}

	climbs, err := (ClimbService{}).FindAllClimbs(track)
	require.NoError(t, err)
	require.Len(t, climbs, 2)
	require.Equal(t, 0, climbs[0].StartIdx)
	require.Equal(t, 1, climbs[0].EndIdx)
	require.InDelta(t, 55, climbs[0].GainM, 0.01)
	require.Equal(t, 2, climbs[1].StartIdx)
	require.Equal(t, 3, climbs[1].EndIdx)
}

func TestFindAllClimbsUsesThirtyMeterDropBoundary(t *testing.T) {
	track := &Track{Points: []Point{
		testPoint(0, 100), testPoint(100, 150), testPoint(200, 120), testPoint(300, 180),
	}}

	climbs, err := (ClimbService{}).FindAllClimbs(track)
	require.NoError(t, err)
	require.Len(t, climbs, 1)
	require.Equal(t, 0, climbs[0].StartIdx)
	require.Equal(t, 3, climbs[0].EndIdx)
	require.InDelta(t, 80, climbs[0].GainM, 0.01)
}

func TestFindAllClimbsIgnoresFlatAndJitter(t *testing.T) {
	track := &Track{Points: []Point{
		testPoint(0, 100), testPoint(100, 120), testPoint(200, 105), testPoint(300, 119),
	}}

	climbs, err := (ClimbService{}).FindAllClimbs(track)
	require.NoError(t, err)
	require.Empty(t, climbs)
}

func TestFindAllClimbsMergesClimbsLessThanHundredMetersApart(t *testing.T) {
	track := &Track{Points: []Point{
		testPoint(0, 100), testPoint(100, 155), testPoint(150, 120), testPoint(250, 180),
	}}

	climbs, err := (ClimbService{}).FindAllClimbs(track)
	require.NoError(t, err)
	require.Len(t, climbs, 1)
	require.Equal(t, 0, climbs[0].StartIdx)
	require.Equal(t, 3, climbs[0].EndIdx)
	require.InDelta(t, 115, climbs[0].GainM, 0.01)
	require.InDelta(t, 250, climbs[0].DistanceM, 1)
}

func TestFindKingClimbSelectsMaximumGain(t *testing.T) {
	climbs := []Climb{{StartIdx: 0, EndIdx: 1, GainM: 50}, {StartIdx: 2, EndIdx: 4, GainM: 120}, {StartIdx: 5, EndIdx: 7, GainM: 80}}

	king, err := (ClimbService{}).FindKingClimb(&Track{}, climbs)
	require.NoError(t, err)
	require.NotNil(t, king)
	require.Equal(t, 2, king.StartIdx)
	require.InDelta(t, 120, king.GainM, 0.01)
	require.True(t, king.IsKingClimb)
}

func TestFindKingClimbReturnsNilWithoutClimbs(t *testing.T) {
	king, err := (ClimbService{}).FindKingClimb(&Track{}, nil)
	require.NoError(t, err)
	require.Nil(t, king)
}

func TestFindKingClimbCalculatesVAMFromTrackTimes(t *testing.T) {
	start := time.Date(2026, 1, 1, 10, 0, 0, 0, time.UTC)
	end := start.Add(30 * time.Minute)
	points := []Point{testPoint(0, 100), testPoint(1000, 300)}
	points[0].Time, points[1].Time = &start, &end

	king, err := (ClimbService{}).FindKingClimb(&Track{Points: points}, []Climb{{StartIdx: 0, EndIdx: 1, GainM: 200}})
	require.NoError(t, err)
	require.NotNil(t, king)
	require.NotNil(t, king.VAM)
	require.InDelta(t, 400, *king.VAM, 0.01)
}

func TestFindMurosDetectsSustainedWall(t *testing.T) {
	track := &Track{Points: []Point{testPoint(0, 100), testPoint(60, 115)}}

	muros, err := (ClimbService{}).FindMuros(track)
	require.NoError(t, err)
	require.Len(t, muros, 1)
	require.Equal(t, 0, muros[0].StartIdx)
	require.Equal(t, 1, muros[0].EndIdx)
	require.InDelta(t, 25, muros[0].AvgSlopePct, 0.2)
}

func TestFindMurosIgnoresShortWall(t *testing.T) {
	track := &Track{Points: []Point{testPoint(0, 100), testPoint(40, 110)}}

	muros, err := (ClimbService{}).FindMuros(track)
	require.NoError(t, err)
	require.Empty(t, muros)
}

func TestFindRecoveryZonesDetectsFlatPostClimb(t *testing.T) {
	track := &Track{Points: []Point{
		testPoint(0, 100), testPoint(100, 150), testPoint(200, 151), testPoint(300, 152),
	}}

	zones, err := (ClimbService{}).FindRecoveryZones(track, []Climb{{StartIdx: 0, EndIdx: 1, GainM: 50}})
	require.NoError(t, err)
	require.Len(t, zones, 1)
	require.Equal(t, 1, zones[0].StartIdx)
	require.Equal(t, 3, zones[0].EndIdx)
	require.InDelta(t, 200, zones[0].DistanceM, 1)
}

func TestFindRecoveryZonesStopsAtTwoHundredMeters(t *testing.T) {
	track := &Track{Points: []Point{
		testPoint(0, 100), testPoint(100, 150), testPoint(200, 151), testPoint(300, 152), testPoint(400, 153),
	}}

	zones, err := (ClimbService{}).FindRecoveryZones(track, []Climb{{StartIdx: 0, EndIdx: 1, GainM: 50}})
	require.NoError(t, err)
	require.Len(t, zones, 1)
	require.InDelta(t, 200, zones[0].DistanceM, 1)
	require.Equal(t, 3, zones[0].EndIdx)
}

func TestFindKmVerticalUsesModifiedKadane(t *testing.T) {
	track := &Track{Points: []Point{
		testPoint(0, 100), testPoint(100, 160), testPoint(200, 220), testPoint(300, 180), testPoint(400, 250),
	}}

	result, err := (ClimbService{}).FindKmVertical(track)
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Equal(t, 0, result.StartIdx)
	require.Equal(t, 2, result.EndIdx)
	require.InDelta(t, 120, result.GainM, 0.01)
}

func TestFindKmVerticalIgnoresSubStepGain(t *testing.T) {
	track := &Track{Points: []Point{testPoint(0, 100), testPoint(100, 149), testPoint(200, 198)}}

	result, err := (ClimbService{}).FindKmVertical(track)
	require.NoError(t, err)
	require.Nil(t, result)
}

func TestFindKmVerticalReturnsNilForEmptyTrack(t *testing.T) {
	result, err := (ClimbService{}).FindKmVertical(&Track{})
	require.NoError(t, err)
	require.Nil(t, result)
}
