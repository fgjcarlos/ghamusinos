package gpx

import "math"

const earthRadiusM = 6371e3

type Analyzer struct{}

func (Analyzer) CalculateDistance(p1, p2 Point) float64 {
	lat1 := degreesToRadians(p1.Lat)
	lat2 := degreesToRadians(p2.Lat)
	deltaLat := degreesToRadians(p2.Lat - p1.Lat)
	deltaLon := degreesToRadians(p2.Lon - p1.Lon)
	a := math.Sin(deltaLat/2)*math.Sin(deltaLat/2) +
		math.Cos(lat1)*math.Cos(lat2)*math.Sin(deltaLon/2)*math.Sin(deltaLon/2)
	return earthRadiusM * 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))
}

func (a Analyzer) CalculatePathDistance(points []Point) float64 {
	var total float64
	for i := 1; i < len(points); i++ {
		total += a.CalculateDistance(points[i-1], points[i])
	}
	return total
}

func (Analyzer) CalculateTotalDPlus(points []Point, threshold float64) float64 {
	if len(points) < 2 || !allElevationsPresent(points) {
		return 0
	}
	start, maximum := *points[0].Ele, *points[0].Ele
	var total float64
	for i := 1; i < len(points); i++ {
		current := *points[i].Ele
		if current > maximum {
			maximum = current
		} else if current < maximum-threshold {
			if gain := maximum - start; gain > 0 {
				total += gain
			}
			start, maximum = current, current
		}
	}
	if gain := maximum - start; gain > 0 {
		total += gain
	}
	return total
}

func (Analyzer) CalculateTotalDMinus(points []Point, threshold float64) float64 {
	if len(points) < 2 || !allElevationsPresent(points) {
		return 0
	}
	start, minimum := *points[0].Ele, *points[0].Ele
	var total float64
	for i := 1; i < len(points); i++ {
		current := *points[i].Ele
		if current < minimum {
			minimum = current
		} else if current > minimum+threshold {
			if loss := start - minimum; loss > 0 {
				total += loss
			}
			start, minimum = current, current
		}
	}
	if loss := start - minimum; loss > 0 {
		total += loss
	}
	return total
}

func (Analyzer) CalculateAverageSlope(distance, dPlus float64) float64 {
	if distance <= 0 {
		return 0
	}
	return dPlus / distance * 100
}

func (Analyzer) CalculateEffortIndex(distanceKm, dPlus float64) float64 {
	return distanceKm + dPlus/100
}

func (Analyzer) CalculateITRAPoints(distanceKm, dPlus float64) float64 {
	points := (distanceKm + dPlus/100) / 5
	return math.Round(points*10) / 10
}

func (Analyzer) CalculateLegBreakerIndex(slopes []float64) float64 {
	if len(slopes) < 2 {
		return 0
	}
	var totalChange, previous float64
	for _, slope := range slopes {
		totalChange += math.Abs(slope - previous)
		previous = slope
	}
	return totalChange / float64(len(slopes))
}

func (Analyzer) CalculateEstimatedVAM(distance, dPlus, timeSec float64) float64 {
	if distance <= 0 || dPlus <= 0 || timeSec <= 0 {
		return 0
	}
	return dPlus / (timeSec / 3600)
}

func (Analyzer) CalculateAdjustedSpeed(baseSpeed, slope float64) float64 {
	if slope > 0 {
		return baseSpeed * math.Max(0.3, 1-slope*0.025)
	}
	if slope < 0 {
		absoluteSlope := math.Abs(slope)
		if absoluteSlope < 15 {
			return baseSpeed * (1 + absoluteSlope*0.02)
		}
		return baseSpeed * math.Max(0.7, 1.3-(absoluteSlope-15)*0.05)
	}
	return baseSpeed
}

func (Analyzer) CalculateDifficulty(distance, dPlus, maxSlope float64) DifficultyLabel {
	if distance <= 0 {
		return DifficultyBeginner
	}
	averageSlope := dPlus / distance * 100
	distanceKm := distance / 1000
	score := math.Min(averageSlope*1.75, 35)
	if distanceKm > 0 {
		score += math.Min(math.Max(0, math.Log10(distanceKm))*25, 40)
	}
	score += math.Min(math.Abs(maxSlope)/2, 25)
	switch {
	case score >= 70:
		return DifficultyPro
	case score >= 50:
		return DifficultyAdvanced
	case score >= 30:
		return DifficultyIntermediate
	default:
		return DifficultyBeginner
	}
}

func (a Analyzer) CalculateRunnabilityIndex(track *Track) float64 {
	if track == nil || len(track.Points) < 2 {
		return 1
	}
	var runnableDistance, totalDistance float64
	for i := 1; i < len(track.Points); i++ {
		p1, p2 := track.Points[i-1], track.Points[i]
		distance := a.CalculateDistance(p1, p2)
		if distance <= 0 {
			continue
		}
		totalDistance += distance
		slope := 0.0
		if p1.Ele != nil && p2.Ele != nil {
			slope = (*p2.Ele - *p1.Ele) / distance * 100
		}
		if math.Abs(slope) <= 15 {
			runnableDistance += distance
		}
	}
	if totalDistance == 0 {
		return 1
	}
	return runnableDistance / totalDistance
}

func (Analyzer) CalculateMovingTime(points []Point, maxDeltaS int) int {
	if maxDeltaS <= 0 {
		return 0
	}
	var total int
	for i := 1; i < len(points); i++ {
		if points[i-1].Time == nil || points[i].Time == nil {
			continue
		}
		delta := int(points[i].Time.Sub(*points[i-1].Time).Seconds())
		if delta > 0 && delta <= maxDeltaS {
			total += delta
		}
	}
	return total
}

func degreesToRadians(value float64) float64 { return value * math.Pi / 180 }

func allElevationsPresent(points []Point) bool {
	for _, point := range points {
		if point.Ele == nil {
			return false
		}
	}
	return true
}
