package terrain

import (
	"math"
	"testing"

	"github.com/stretchr/testify/require"
)

// terrainMetersPerDegree is the length of one degree of latitude on a sphere
// with the same radius used by the haversine distance, so test tracks built
// with these helpers land exactly on the 100 m sampling grid.
const terrainMetersPerDegree = 6371000 * math.Pi / 180

func terrainNorthTrack(lengthM, stepM float64) []LatLon {
	points := make([]LatLon, 0, int(lengthM/stepM)+1)
	for d := 0.0; d <= lengthM+1e-9; d += stepM {
		points = append(points, LatLon{Lat: d / terrainMetersPerDegree, Lon: 0})
	}
	return points
}

func terrainEastTrack(lat, startLon, lengthM, stepM float64) []LatLon {
	points := make([]LatLon, 0, int(lengthM/stepM)+1)
	for d := 0.0; d <= lengthM+1e-9; d += stepM {
		points = append(points, LatLon{Lat: lat, Lon: startLon + d/terrainMetersPerDegree})
	}
	return points
}

func TestTerrainMapSurfaceMapsOSMTags(t *testing.T) {
	cases := []struct {
		name     string
		surface  string
		highway  string
		wantKind string
		wantTech int
	}{
		{"asphalt", "asphalt", "", "paved", 1},
		{"paved", "paved", "", "paved", 1},
		{"concrete", "concrete", "", "paved", 1},
		{"primary highway", "", "primary", "paved", 1},
		{"secondary highway", "", "secondary", "paved", 1},
		{"tertiary highway", "", "tertiary", "paved", 1},
		{"service highway", "", "service", "paved", 1},
		{"gravel", "gravel", "", "gravel", 2},
		{"compacted", "compacted", "", "gravel", 2},
		{"fine gravel", "fine_gravel", "", "gravel", 2},
		{"unpaved", "unpaved", "", "gravel", 2},
		{"dirt", "dirt", "", "trail", 3},
		{"earth", "earth", "", "trail", 3},
		{"ground", "ground", "", "trail", 3},
		{"grass", "grass", "", "trail", 3},
		{"path highway", "", "path", "trail", 3},
		{"footway highway", "", "footway", "trail", 3},
		{"track highway", "", "track", "trail", 3},
		{"steps", "steps", "", "technical", 5},
		{"rock", "rock", "", "technical", 5},
		{"scree", "scree", "", "technical", 5},
		{"explicit surface wins over highway default", "gravel", "primary", "gravel", 2},
		{"case insensitive", "Asphalt", "SECONDARY", "paved", 1},
		{"unknown surface", "cobblestone", "", "unknown", 2},
		{"unknown highway", "", "construction", "unknown", 2},
		{"nothing", "", "", "unknown", 2},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			kind, technicality := MapSurface(tc.surface, tc.highway)

			require.Equal(t, tc.wantKind, kind)
			require.Equal(t, tc.wantTech, technicality)
		})
	}
}

func TestTerrainMapSegmentsReturnsEmptySliceWithoutWays(t *testing.T) {
	segments := MapSegments(terrainNorthTrack(300, 5), nil)

	require.NotNil(t, segments)
	require.Empty(t, segments)
}

func TestTerrainMapSegmentsReturnsEmptySliceWhenNoWayIsWithinFiftyMeters(t *testing.T) {
	// Way runs parallel to the track, 100 m to the east.
	offset := 100 / terrainMetersPerDegree
	ways := []Way{{
		ID:       1,
		Highway:  "track",
		Surface:  "gravel",
		Geometry: []LatLon{{Lat: 0, Lon: offset}, {Lat: 0.0027, Lon: offset}},
	}}

	segments := MapSegments(terrainNorthTrack(250, 5), ways)

	require.NotNil(t, segments)
	require.Empty(t, segments)
}

func TestTerrainMapSegmentsMergesConsecutiveSamplesIntoOneSegment(t *testing.T) {
	ways := []Way{{
		ID:       1,
		Name:     "Camino",
		Highway:  "secondary",
		Surface:  "asphalt",
		Geometry: []LatLon{{Lat: 0, Lon: 0}, {Lat: 0.00315, Lon: 0}},
	}}

	segments := MapSegments(terrainNorthTrack(350, 5), ways)

	require.Len(t, segments, 1)
	require.InDelta(t, 0, segments[0].StartKm, 1e-9)
	require.InEpsilon(t, 0.35, segments[0].EndKm, 1e-3)
	require.Equal(t, "paved", segments[0].Surface)
	require.Equal(t, 1, segments[0].Technicality)
	require.Equal(t, "Camino", segments[0].Name)
}

func TestTerrainMapSegmentsSplitsOnSurfaceChange(t *testing.T) {
	// Two ways meeting at 260 m: asphalt before, gravel after.
	split := 260 / terrainMetersPerDegree
	end := 450 / terrainMetersPerDegree
	ways := []Way{
		{ID: 1, Name: "Camino", Highway: "secondary", Surface: "asphalt", Geometry: []LatLon{{Lat: 0, Lon: 0}, {Lat: 0, Lon: split}}},
		{ID: 2, Name: "Senda", Highway: "path", Surface: "gravel", Geometry: []LatLon{{Lat: 0, Lon: split}, {Lat: 0, Lon: end}}},
	}

	segments := MapSegments(terrainEastTrack(0, 0, 450, 5), ways)

	require.Len(t, segments, 2)
	require.Equal(t, "paved", segments[0].Surface)
	require.Equal(t, "Camino", segments[0].Name)
	require.InDelta(t, 0, segments[0].StartKm, 1e-9)
	require.InEpsilon(t, 0.3, segments[0].EndKm, 1e-3)
	require.Equal(t, "gravel", segments[1].Surface)
	require.Equal(t, "Senda", segments[1].Name)
	require.InEpsilon(t, 0.3, segments[1].StartKm, 1e-3)
	require.InEpsilon(t, 0.45, segments[1].EndKm, 1e-3)
}

func TestTerrainMapSegmentsUsesPointToSegmentDistance(t *testing.T) {
	// Track runs 22 m north of a long way. The closest way *node* is hundreds
	// of metres away, but the closest way *segment* is only 22 m away.
	trackLat := 22 / terrainMetersPerDegree
	ways := []Way{{
		ID:       1,
		Name:     "Camino",
		Highway:  "secondary",
		Surface:  "asphalt",
		Geometry: []LatLon{{Lat: 0, Lon: 0}, {Lat: 0, Lon: 0.004}},
	}}

	segments := MapSegments(terrainEastTrack(trackLat, 0.001, 220, 5), ways)

	require.Len(t, segments, 1)
	require.Equal(t, "paved", segments[0].Surface)
}

func TestTerrainMapSegmentsSkipsSamplesFartherThanFiftyMetersFromTheWay(t *testing.T) {
	// Way starts 60 m east of the track start; only later samples get close
	// enough once the track bends toward it.
	ways := []Way{{
		ID:       1,
		Highway:  "track",
		Surface:  "gravel",
		Geometry: []LatLon{{Lat: 0, Lon: 0.002}, {Lat: 0, Lon: 0.004}},
	}}
	points := []LatLon{{Lat: 0, Lon: 0}, {Lat: 0, Lon: 0.001}, {Lat: 0, Lon: 0.002}, {Lat: 0, Lon: 0.003}}

	segments := MapSegments(points, ways)

	require.Len(t, segments, 1)
	require.InEpsilon(t, 0.2, segments[0].StartKm, 1e-3)
}

func TestTerrainMapSegmentsHandlesSinglePointTrack(t *testing.T) {
	ways := []Way{{
		ID:       1,
		Highway:  "track",
		Surface:  "gravel",
		Geometry: []LatLon{{Lat: 0, Lon: 0}, {Lat: 0.001, Lon: 0.001}},
	}}

	segments := MapSegments([]LatLon{{Lat: 0, Lon: 0}}, ways)

	require.Len(t, segments, 1)
	require.InDelta(t, 0, segments[0].StartKm, 1e-9)
	require.InDelta(t, 0, segments[0].EndKm, 1e-9)
}
