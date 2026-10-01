package metrics

// CardiacDrift returns the percentage change from start to end heart rate.
// A non-positive starting heart rate returns zero to avoid division by zero.
func CardiacDrift(hrStart, hrEnd float64) float64 {
	if hrStart <= 0 {
		return 0
	}
	return (hrEnd - hrStart) / hrStart * 100
}

// CardiacDriftSeries compares the mean heart rate in the first and third
// quartiles of a stream. Fewer than two samples cannot define a drift.
func CardiacDriftSeries(samples []int) float64 {
	if len(samples) < 2 {
		return 0
	}
	quartileSize := (len(samples) + 3) / 4
	firstMean := mean(samples[:quartileSize])
	thirdMean := mean(samples[len(samples)-quartileSize:])
	return CardiacDrift(firstMean, thirdMean)
}

func mean(values []int) float64 {
	total := 0
	for _, value := range values {
		total += value
	}
	return float64(total) / float64(len(values))
}
