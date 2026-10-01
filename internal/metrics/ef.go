package metrics

// EfficiencyFactor returns normalized power divided by average heart rate.
// A non-positive heart rate produces the defensive sentinel zero.
func EfficiencyFactor(npWatts, avgHR int) float64 {
	if avgHR <= 0 {
		return 0
	}
	return float64(npWatts) / float64(avgHR)
}
