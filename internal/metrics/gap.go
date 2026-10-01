package metrics

import "math"

// GradeAdjustedPace applies the normalized Minetti running-cost curve to a
// pace in seconds per kilometer. Grade is a fraction and is clamped to ±35%.
func GradeAdjustedPace(grade, paceSecPerKm float64) float64 {
	if paceSecPerKm <= 0 {
		return 0
	}
	grade = math.Max(-0.35, math.Min(0.35, grade))
	return paceSecPerKm * minettiCost(grade) / minettiCost(0)
}

func minettiCost(grade float64) float64 {
	g2 := grade * grade
	g3 := g2 * grade
	g4 := g3 * grade
	g5 := g4 * grade
	return 155.4*g5 - 30.4*g4 - 43.3*g3 + 46.3*g2 + 19.5*grade + 3.6
}
