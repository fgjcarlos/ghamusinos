package metrics

// IntensityFactor returns normalized power divided by functional threshold
// power. A non-positive FTP produces the defensive sentinel zero.
func IntensityFactor(ftpWatts, npWatts int) float64 {
	if ftpWatts <= 0 {
		return 0
	}
	return float64(npWatts) / float64(ftpWatts)
}

// IntensityFactorRunning returns actual velocity divided by threshold
// velocity. A non-positive threshold velocity produces zero.
func IntensityFactorRunning(velocity, thresholdVelocity float64) float64 {
	if thresholdVelocity <= 0 {
		return 0
	}
	return velocity / thresholdVelocity
}
