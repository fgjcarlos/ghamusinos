package metrics

// TSSRunning returns running Training Stress Score for threshold and actual
// paces in seconds per kilometer. Non-positive values return zero.
func TSSRunning(thresholdSecPerKm, durationSec, actualSecPerKm int) float64 {
	if thresholdSecPerKm <= 0 || durationSec <= 0 || actualSecPerKm <= 0 {
		return 0
	}
	paceRatio := float64(thresholdSecPerKm) / float64(actualSecPerKm)
	return (float64(durationSec) / 3600) * paceRatio * paceRatio * 100
}
