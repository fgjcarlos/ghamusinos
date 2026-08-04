package gpx

import "fmt"

const circularDistanceM = 100.0

type TrackTypeService struct {
	analyzer Analyzer
}

func (s TrackTypeService) Detect(track *Track) (TrackTypeResult, error) {
	if track == nil || len(track.Points) < 2 {
		return TrackTypeResult{}, fmt.Errorf("gpx: at least two points are required")
	}
	points := track.Points
	if s.analyzer.CalculateDistance(points[0], points[len(points)-1]) >= circularDistanceM {
		return TrackTypeResult{Type: "point-to-point"}, nil
	}

	area := 0.0
	for i := 0; i < len(points)-1; i++ {
		area += points[i].Lon*points[i+1].Lat - points[i+1].Lon*points[i].Lat
	}
	direction := "counterclockwise"
	if area < 0 {
		direction = "clockwise"
	}
	return TrackTypeResult{Type: "circular", Direction: direction}, nil
}
