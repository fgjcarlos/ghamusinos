package gpx

import "math"

const earthRadiusM = 6371e3

type Analyzer struct{}

// CalculateElevation computes D+/D- and coverage per the policy in
// issue #171, A8. Two independent accumulators walk the points:
//
//   - D+: tracks the running max from the last threshold-reset. When
//     the current elevation drops below (max - threshold), the gain
//     (max - start) is flushed and start/max reset to current.
//   - D-: tracks the running min from the last threshold-reset. When
//     the current elevation rises above (min + threshold), the loss
//     (start - min) is flushed and start/min reset to current.
//
// A hueco (any point with Ele == nil in either side of a pair) closes
// both accumulators without adding the jump across the gap. The start
// of a fresh tramo is initialized from prev.Ele (the first point of
// the new tramo), and the running extreme from max(prev.Ele, curr.Ele)
// — so the first segment contributes correctly to D+ or D- depending
// on direction.
//
// Coverage is the fraction of total path distance whose segment had
// usable elevation on both sides. When coverage is below
// CoverageMinThreshold, D+/D- are nil and Status is
// CoverageInsufficient — a zero would be a lie about the data.
//
// Pre-condition: len(points) >= 2. Callers that pass a 0/1-length slice
// get an empty result (coverage 0, no D+/D-).
func (Analyzer) CalculateElevation(points []Point, threshold float64) ElevationResult {
	if len(points) < 2 {
		return ElevationResult{Status: CoverageInsufficient}
	}

	var (
		dPlus, dMinus        float64
		coveredDistance      float64
		totalDistance        float64
		plusStart, plusMax   *float64
		minusStart, minusMin *float64
	)

	flushPlus := func() {
		if plusStart != nil && plusMax != nil {
			if gain := *plusMax - *plusStart; gain > 0 {
				dPlus += gain
			}
		}
		plusStart, plusMax = nil, nil
	}
	flushMinus := func() {
		if minusStart != nil && minusMin != nil {
			if loss := *minusStart - *minusMin; loss > 0 {
				dMinus += loss
			}
		}
		minusStart, minusMin = nil, nil
	}
	openTramo := func(prevEle, currEle float64) {
		// First valid pair of a tramo: start at prev's elevation; the
		// running extreme for each direction picks the right one
		// based on which way the first segment goes.
		plusStart = &prevEle
		if currEle > prevEle {
			plusMax = &currEle
		} else {
			startCopy := prevEle
			plusMax = &startCopy
		}
		minusStart = &prevEle
		if currEle < prevEle {
			minusMin = &currEle
		} else {
			startCopy := prevEle
			minusMin = &startCopy
		}
	}

	for i := 1; i < len(points); i++ {
		prev, curr := points[i-1], points[i]
		dist := haversineDistanceM(prev, curr)
		totalDistance += dist

		if prev.Ele == nil || curr.Ele == nil {
			flushPlus()
			flushMinus()
			continue
		}

		coveredDistance += dist
		currEle := *curr.Ele

		if plusStart == nil {
			// First valid pair of a fresh tramo.
			prevEle := *prev.Ele
			openTramo(prevEle, currEle)
			continue
		}

		// Plus accumulator: threshold-reset on big descents.
		if currEle > *plusMax {
			*plusMax = currEle
		} else if currEle < *plusMax-threshold {
			flushPlus()
			startCopy := currEle
			plusStart = &startCopy
			plusMax = &startCopy
		}

		// Minus accumulator: threshold-reset on big ascents.
		if currEle < *minusMin {
			*minusMin = currEle
		} else if currEle > *minusMin+threshold {
			flushMinus()
			startCopy := currEle
			minusStart = &startCopy
			minusMin = &startCopy
		}
	}

	flushPlus()
	flushMinus()

	var coverage float64
	if totalDistance > 0 {
		coverage = coveredDistance / totalDistance
	}

	var dPlusPtr, dMinusPtr *float64
	status := statusForCoverage(coverage)
	if status != CoverageInsufficient {
		dPlusVal := dPlus
		dMinusVal := dMinus
		dPlusPtr = &dPlusVal
		dMinusPtr = &dMinusVal
	}

	return ElevationResult{
		DPlusM:   dPlusPtr,
		DMinusM:  dMinusPtr,
		Coverage: coverage,
		Status:   status,
	}
}

func statusForCoverage(coverage float64) CoverageStatus {
	switch {
	case coverage >= CoverageFullThreshold:
		return CoverageFull
	case coverage >= CoverageMinThreshold:
		return CoveragePartial
	default:
		return CoverageInsufficient
	}
}

// haversineDistanceM is a small helper to keep the per-segment loop
// readable without making CalculatePathDistance recompute from scratch
// on each call.
func haversineDistanceM(p1, p2 Point) float64 {
	return (Analyzer{}).CalculateDistance(p1, p2)
}

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

// allElevationsPresent is a fast pre-flight for analyses that cannot
// tolerate any missing-ele point (climbs, risk zones). CalculateElevation
// has its own policy for partial coverage and does not use this helper.
func allElevationsPresent(points []Point) bool {
	for _, point := range points {
		if point.Ele == nil {
			return false
		}
	}
	return true
}
