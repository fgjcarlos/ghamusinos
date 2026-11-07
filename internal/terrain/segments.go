package terrain

import (
	"math"
	"strings"
)

const (
	// sampleSpacingM is the distance between terrain samples along the track.
	sampleSpacingM = 100.0
	// maxWayDistanceM is the largest gap between a sample and a way for the
	// sample to be attributed to that way.
	maxWayDistanceM = 50.0
)

// Segment is a contiguous stretch of the track with the same surface and name.
type Segment struct {
	StartKm      float64 `json:"start_km"`
	EndKm        float64 `json:"end_km"`
	Surface      string  `json:"surface"`
	Technicality int     `json:"technicality"`
	Name         string  `json:"name"`
}

// MapSurface translates OSM surface/highway tags into a surface kind and a
// technicality level (1 = fast and easy, 5 = technical).
//
// This is a deliberate refinement of the old client-side prototype
// (old_ghamusinos/src/lib/utils.ts, mapSurfaceToTechnicality): an explicit
// `surface` tag always wins over the `highway` fallback, the kind labels are
// the stable API values (paved, gravel, trail, technical, unknown) instead of
// the prototype's ad-hoc mixed labels (asphalt, ground, path, grass, stone),
// and several levels were retuned by the feature spec: gravel/fine_gravel
// drops from 3 to 2, track/path highway defaults rise to 3, grass becomes
// trail/3 instead of a dedicated level 4, sand falls back to unknown/2, and
// steps joins rock/scree at level 5.
func MapSurface(surface, highway string) (kind string, technicality int) {
	s := strings.ToLower(strings.TrimSpace(surface))
	h := strings.ToLower(strings.TrimSpace(highway))
	switch s {
	case "asphalt", "paved", "concrete":
		return "paved", 1
	case "gravel", "compacted", "fine_gravel", "unpaved":
		return "gravel", 2
	case "dirt", "earth", "ground", "grass":
		return "trail", 3
	case "steps", "rock", "scree":
		return "technical", 5
	}
	switch h {
	case "primary", "secondary", "tertiary", "service":
		return "paved", 1
	case "path", "footway", "track":
		return "trail", 3
	}
	return "unknown", 2
}

// MapSegments attributes every ~100 m sample of the track to the closest OSM
// way segment within 50 m and merges consecutive samples that share the same
// surface and name. No ways or no matches is a valid, empty result.
func MapSegments(points []LatLon, ways []Way) []Segment {
	segments := make([]Segment, 0, 8)
	if len(points) == 0 || len(ways) == 0 {
		return segments
	}
	samples := sampleTrack(points, sampleSpacingM)
	lastMatched := -2
	for i, sample := range samples {
		way, distance := nearestWay(sample.point, ways)
		if way == nil || distance > maxWayDistanceM {
			continue
		}
		kind, technicality := MapSurface(way.Surface, way.Highway)
		if lastMatched == i-1 && len(segments) > 0 {
			last := &segments[len(segments)-1]
			if last.Surface == kind && last.Name == way.Name {
				last.EndKm = roundKm(sample.endKm)
				lastMatched = i
				continue
			}
		}
		segments = append(segments, Segment{
			StartKm:      roundKm(sample.startKm),
			EndKm:        roundKm(sample.endKm),
			Surface:      kind,
			Technicality: technicality,
			Name:         way.Name,
		})
		lastMatched = i
	}
	return segments
}

// trackSample is one point of the track plus the kilometre interval it covers.
type trackSample struct {
	point   LatLon
	startKm float64
	endKm   float64
}

// sampleTrack walks the polyline, placing a sample every spacingM metres of
// cumulative distance and interpolating between source points.
func sampleTrack(points []LatLon, spacingM float64) []trackSample {
	if len(points) == 0 {
		return nil
	}
	if len(points) == 1 {
		return []trackSample{{point: points[0]}}
	}
	samples := make([]trackSample, 0, 8)
	samples = append(samples, trackSample{point: points[0]})
	totalM := 0.0
	nextAtM := 0.0
	previous := points[0]
	for i := 1; i < len(points); i++ {
		current := points[i]
		segmentStartM := totalM
		segmentLengthM := haversineM(previous, current)
		totalM += segmentLengthM
		if segmentLengthM > 0 {
			for nextAtM+spacingM <= totalM {
				at := nextAtM + spacingM
				fraction := clampUnit((at - segmentStartM) / segmentLengthM)
				samples = append(samples, trackSample{
					point:   interpolate(previous, current, fraction),
					startKm: at / 1000,
				})
				nextAtM = at
			}
		}
		previous = current
	}
	// A sample landing exactly on the final vertex would yield a zero-length
	// trailing interval; drop it.
	if len(samples) > 1 && samples[len(samples)-1].startKm*1000 >= totalM-1e-6 {
		samples = samples[:len(samples)-1]
	}
	for i := range samples {
		startM := samples[i].startKm * 1000
		endM := startM + spacingM
		if endM > totalM {
			endM = totalM
		}
		if endM < startM {
			endM = startM
		}
		samples[i].endKm = endM / 1000
	}
	return samples
}

// nearestWay finds the way whose closest geometry segment is nearest to point.
func nearestWay(point LatLon, ways []Way) (*Way, float64) {
	bestIndex := -1
	bestDistance := math.Inf(1)
	for i := range ways {
		distance, ok := wayDistanceM(point, ways[i])
		if !ok {
			continue
		}
		if distance < bestDistance {
			bestDistance = distance
			bestIndex = i
		}
	}
	if bestIndex < 0 {
		return nil, math.Inf(1)
	}
	return &ways[bestIndex], bestDistance
}

// wayDistanceM is the point-to-segment distance to the closest way segment.
func wayDistanceM(point LatLon, way Way) (float64, bool) {
	switch len(way.Geometry) {
	case 0:
		return 0, false
	case 1:
		return haversineM(point, way.Geometry[0]), true
	}
	bestDistance := math.Inf(1)
	for i := 1; i < len(way.Geometry); i++ {
		distance := pointToSegmentDistanceM(point, way.Geometry[i-1], way.Geometry[i])
		if distance < bestDistance {
			bestDistance = distance
		}
	}
	return bestDistance, true
}

// pointToSegmentDistanceM projects everything into a local metric frame
// centred on point, which is accurate at the 50 m scale we care about.
func pointToSegmentDistanceM(point, a, b LatLon) float64 {
	metersPerDegLat, metersPerDegLon := metersPerDegree(point.Lat)
	ax := (a.Lon - point.Lon) * metersPerDegLon
	ay := (a.Lat - point.Lat) * metersPerDegLat
	bx := (b.Lon - point.Lon) * metersPerDegLon
	by := (b.Lat - point.Lat) * metersPerDegLat
	dx, dy := bx-ax, by-ay
	lengthSquared := dx*dx + dy*dy
	if lengthSquared == 0 {
		return math.Hypot(ax, ay)
	}
	t := clampUnit(-(ax*dx + ay*dy) / lengthSquared)
	return math.Hypot(ax+t*dx, ay+t*dy)
}

func metersPerDegree(lat float64) (metersPerDegLat, metersPerDegLon float64) {
	const metersPerDegreeEquator = 6371000 * math.Pi / 180
	metersPerDegLat = metersPerDegreeEquator
	metersPerDegLon = metersPerDegreeEquator * math.Cos(lat*math.Pi/180)
	return metersPerDegLat, metersPerDegLon
}

func haversineM(a, b LatLon) float64 {
	const earthRadiusM = 6371000.0
	lat1 := a.Lat * math.Pi / 180
	lat2 := b.Lat * math.Pi / 180
	dLat := lat2 - lat1
	dLon := (b.Lon - a.Lon) * math.Pi / 180
	h := math.Sin(dLat/2)*math.Sin(dLat/2) + math.Cos(lat1)*math.Cos(lat2)*math.Sin(dLon/2)*math.Sin(dLon/2)
	if h > 1 {
		h = 1
	}
	return 2 * earthRadiusM * math.Asin(math.Sqrt(h))
}

func interpolate(a, b LatLon, fraction float64) LatLon {
	return LatLon{
		Lat: a.Lat + (b.Lat-a.Lat)*fraction,
		Lon: a.Lon + (b.Lon-a.Lon)*fraction,
	}
}

func clampUnit(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}

// roundKm keeps response values stable at a 10 m resolution.
func roundKm(km float64) float64 {
	return math.Round(km*100) / 100
}
