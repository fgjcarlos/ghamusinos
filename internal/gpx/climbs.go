package gpx

import "math"

const (
	climbDropThresholdM = 30.0
	minimumClimbGainM   = 30.0
	minimumMuroSlopePct = 20.0
	minimumMuroLengthM  = 50.0
	recoverySlopePct    = 3.0
	maximumRecoveryM    = 200.0
	kmVerticalMinStepM  = 50.0
)

type ClimbService struct {
	analyzer Analyzer
}

func (s ClimbService) distance(p1, p2 Point) float64 {
	return s.analyzer.CalculateDistance(p1, p2)
}

func (s ClimbService) FindAllClimbs(track *Track) ([]Climb, error) {
	if track == nil || len(track.Points) < 2 || !allElevationsPresent(track.Points) {
		return []Climb{}, nil
	}

	points := track.Points
	climbs := make([]Climb, 0)
	start, peak := 0, 0
	for i := 1; i < len(points); i++ {
		if *points[i].Ele > *points[peak].Ele {
			peak = i
		}
		drop := *points[peak].Ele - *points[i].Ele
		if drop > climbDropThresholdM {
			climbs = s.appendClimb(climbs, points, start, peak)
			start, peak = i, i
		}
	}
	climbs = s.appendClimb(climbs, points, start, peak)
	return s.mergeNearbyClimbs(points, climbs), nil
}

func (s ClimbService) mergeNearbyClimbs(points []Point, climbs []Climb) []Climb {
	if len(climbs) < 2 {
		return climbs
	}
	merged := make([]Climb, 0, len(climbs))
	merged = append(merged, climbs[0])
	for _, climb := range climbs[1:] {
		previous := &merged[len(merged)-1]
		gap := s.analyzer.CalculatePathDistance(points[previous.EndIdx : climb.StartIdx+1])
		if gap >= 100 {
			merged = append(merged, climb)
			continue
		}
		previous.EndIdx = climb.EndIdx
		previous.GainM += climb.GainM
		previous.DistanceM = s.analyzer.CalculatePathDistance(points[previous.StartIdx : previous.EndIdx+1])
		if previous.DistanceM > 0 {
			previous.AvgSlopePct = previous.GainM / previous.DistanceM * 100
		}
	}
	return merged
}

func (s ClimbService) appendClimb(climbs []Climb, points []Point, start, end int) []Climb {
	if end <= start {
		return climbs
	}
	gain := *points[end].Ele - *points[start].Ele
	if gain <= minimumClimbGainM {
		return climbs
	}
	distance := s.analyzer.CalculatePathDistance(points[start : end+1])
	average := 0.0
	if distance > 0 {
		average = gain / distance * 100
	}
	return append(climbs, Climb{StartIdx: start, EndIdx: end, GainM: gain, DistanceM: distance, AvgSlopePct: average})
}

func (ClimbService) FindKingClimb(track *Track, climbs []Climb) (*Climb, error) {
	if len(climbs) == 0 {
		return nil, nil
	}
	best := climbs[0]
	for _, climb := range climbs[1:] {
		if climb.GainM > best.GainM {
			best = climb
		}
	}
	best.IsKingClimb = true
	if track != nil && best.StartIdx >= 0 && best.EndIdx < len(track.Points) && best.StartIdx < best.EndIdx {
		start, end := track.Points[best.StartIdx].Time, track.Points[best.EndIdx].Time
		if start != nil && end != nil {
			hours := end.Sub(*start).Hours()
			if hours > 0 {
				vam := best.GainM / hours
				best.VAM = &vam
			}
		}
	}
	return &best, nil
}

func (s ClimbService) FindMuros(track *Track) ([]Muro, error) {
	if track == nil || len(track.Points) < 2 || !allElevationsPresent(track.Points) {
		return []Muro{}, nil
	}
	muros := make([]Muro, 0)
	start := -1
	var distance, gain float64
	flush := func(end int) {
		if start >= 0 && distance+0.5 >= minimumMuroLengthM {
			muros = append(muros, Muro{StartIdx: start, EndIdx: end, GainM: gain, DistanceM: distance, AvgSlopePct: gain / distance * 100})
		}
		start, distance, gain = -1, 0, 0
	}
	for i := 1; i < len(track.Points); i++ {
		segmentDistance := s.distance(track.Points[i-1], track.Points[i])
		segmentGain := *track.Points[i].Ele - *track.Points[i-1].Ele
		slope := 0.0
		if segmentDistance > 0 {
			slope = segmentGain / segmentDistance * 100
		}
		if slope > minimumMuroSlopePct {
			if start < 0 {
				start = i - 1
			}
			distance += segmentDistance
			gain += segmentGain
			continue
		}
		flush(i - 1)
	}
	flush(len(track.Points) - 1)
	return muros, nil
}

func (s ClimbService) FindRecoveryZones(track *Track, climbs []Climb) ([]RecoveryZone, error) {
	if track == nil || len(track.Points) < 2 || !allElevationsPresent(track.Points) {
		return []RecoveryZone{}, nil
	}
	zones := make([]RecoveryZone, 0)
	for _, climb := range climbs {
		if climb.EndIdx < 0 || climb.EndIdx >= len(track.Points)-1 {
			continue
		}
		start, end := -1, climb.EndIdx
		var distance float64
		for i := climb.EndIdx + 1; i < len(track.Points); i++ {
			segmentDistance := s.distance(track.Points[i-1], track.Points[i])
			if segmentDistance <= 0 {
				continue
			}
			remaining := maximumRecoveryM - distance
			if remaining <= 0 {
				break
			}
			slope := math.Abs((*track.Points[i].Ele - *track.Points[i-1].Ele) / segmentDistance * 100)
			if slope >= recoverySlopePct {
				break
			}
			if start < 0 {
				start = i - 1
			}
			if segmentDistance > remaining+0.5 {
				distance = maximumRecoveryM
				break
			}
			distance += segmentDistance
			end = i
		}
		if start >= 0 && distance > 0 {
			zones = append(zones, RecoveryZone{StartIdx: start, EndIdx: end, DistanceM: math.Min(distance, maximumRecoveryM)})
		}
	}
	return zones, nil
}

func (s ClimbService) FindKmVertical(track *Track) (*KmVerticalResult, error) {
	if track == nil || len(track.Points) < 2 || !allElevationsPresent(track.Points) {
		return nil, nil
	}
	var currentGain, bestGain float64
	currentStart, bestStart, bestEnd := 0, 0, 0
	for i := 1; i < len(track.Points); i++ {
		diff := *track.Points[i].Ele - *track.Points[i-1].Ele
		if diff < kmVerticalMinStepM {
			currentGain = 0
			currentStart = i
			continue
		}
		currentGain += diff
		if currentGain > bestGain {
			bestGain, bestStart, bestEnd = currentGain, currentStart, i
		}
	}
	if bestGain == 0 {
		return nil, nil
	}
	return &KmVerticalResult{
		StartIdx: bestStart, EndIdx: bestEnd, GainM: bestGain,
		DistanceM: s.analyzer.CalculatePathDistance(track.Points[bestStart : bestEnd+1]),
	}, nil
}
