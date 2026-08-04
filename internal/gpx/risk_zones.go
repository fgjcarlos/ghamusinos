package gpx

import "math"

const (
	steepSlopePct      = 25.0
	steepMinimumM      = 20.0
	technicalWindowM   = 100.0
	technicalMinimumCV = 0.5
	exposureElevationM = 2500.0
	exposureMinimumM   = 100.0
	distanceToleranceM = 0.5
)

type RiskService struct {
	analyzer Analyzer
}

func (s RiskService) Detect(track *Track) ([]RiskZone, error) {
	steep, err := s.DetectSteep(track)
	if err != nil {
		return nil, err
	}
	technical, err := s.DetectTechnical(track)
	if err != nil {
		return nil, err
	}
	exposure, err := s.DetectExposure(track)
	if err != nil {
		return nil, err
	}
	return append(append(steep, technical...), exposure...), nil
}

func (s RiskService) DetectSteep(track *Track) ([]RiskZone, error) {
	if track == nil || len(track.Points) < 2 || !allElevationsPresent(track.Points) {
		return []RiskZone{}, nil
	}
	zones := make([]RiskZone, 0)
	start := -1
	var distance, maximumSlope float64
	flush := func(end int) {
		if start >= 0 && distance+distanceToleranceM >= steepMinimumM {
			severity := "medium"
			if maximumSlope >= 40 || distance >= 500 {
				severity = "high"
			}
			zones = append(zones, RiskZone{StartIdx: start, EndIdx: end, RiskType: "steep", Severity: severity})
		}
		start, distance, maximumSlope = -1, 0, 0
	}
	for i := 1; i < len(track.Points); i++ {
		d := s.analyzer.CalculateDistance(track.Points[i-1], track.Points[i])
		slope := 0.0
		if d > 0 {
			slope = math.Abs((*track.Points[i].Ele - *track.Points[i-1].Ele) / d * 100)
		}
		if slope > steepSlopePct {
			if start < 0 {
				start = i - 1
			}
			distance += d
			maximumSlope = math.Max(maximumSlope, slope)
			continue
		}
		flush(i - 1)
	}
	flush(len(track.Points) - 1)
	return zones, nil
}

func (s RiskService) DetectTechnical(track *Track) ([]RiskZone, error) {
	if track == nil || len(track.Points) < 3 || !allElevationsPresent(track.Points) {
		return []RiskZone{}, nil
	}
	zones := make([]RiskZone, 0)
	for start := 0; start < len(track.Points)-1; {
		end, distance := start, 0.0
		slopes := make([]float64, 0)
		for end+1 < len(track.Points) && distance+distanceToleranceM < technicalWindowM {
			d := s.analyzer.CalculateDistance(track.Points[end], track.Points[end+1])
			if d > 0 {
				slopes = append(slopes, (*track.Points[end+1].Ele-*track.Points[end].Ele)/d*100)
				distance += d
			}
			end++
		}
		if distance+distanceToleranceM < technicalWindowM || len(slopes) < 2 {
			break
		}
		if slopeCoefficientVariation(slopes) > technicalMinimumCV {
			zones = append(zones, RiskZone{StartIdx: start, EndIdx: end, RiskType: "technical", Severity: "medium"})
			start = end
			continue
		}
		start++
	}
	return zones, nil
}

func slopeCoefficientVariation(values []float64) float64 {
	var mean float64
	for _, value := range values {
		mean += value
	}
	mean /= float64(len(values))
	var variance float64
	for _, value := range values {
		variance += math.Pow(value-mean, 2)
	}
	deviation := math.Sqrt(variance / float64(len(values)))
	if math.Abs(mean) < 1e-9 {
		if deviation > 0 {
			return math.Inf(1)
		}
		return 0
	}
	return deviation / math.Abs(mean)
}

func (s RiskService) DetectExposure(track *Track) ([]RiskZone, error) {
	if track == nil || len(track.Points) < 2 || !allElevationsPresent(track.Points) {
		return []RiskZone{}, nil
	}
	zones := make([]RiskZone, 0)
	start := -1
	var distance float64
	flush := func(end int) {
		if start >= 0 && distance+distanceToleranceM >= exposureMinimumM {
			zones = append(zones, RiskZone{StartIdx: start, EndIdx: end, RiskType: "exposure", Severity: "high"})
		}
		start, distance = -1, 0
	}
	for i := 1; i < len(track.Points); i++ {
		if *track.Points[i-1].Ele > exposureElevationM && *track.Points[i].Ele > exposureElevationM {
			if start < 0 {
				start = i - 1
			}
			distance += s.analyzer.CalculateDistance(track.Points[i-1], track.Points[i])
			continue
		}
		flush(i - 1)
	}
	flush(len(track.Points) - 1)
	return zones, nil
}
