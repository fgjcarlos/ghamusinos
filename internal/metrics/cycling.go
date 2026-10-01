package metrics

// TSSCycling returns the TrainingPeaks cycling Training Stress Score.
// Inputs are FTP watts, elapsed seconds, and normalized-power watts.
// Invalid non-positive inputs produce the defensive sentinel zero.
func TSSCycling(ftpWatts, durationSec, npWatts int) float64 {
	if ftpWatts <= 0 || durationSec <= 0 || npWatts <= 0 {
		return 0
	}
	ftp := float64(ftpWatts)
	np := float64(npWatts)
	intensityFactor := np / ftp
	return (float64(durationSec) * np * intensityFactor / (ftp * 3600)) * 100
}
