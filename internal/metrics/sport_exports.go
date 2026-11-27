package metrics

// IsRunningSport and IsCyclingSport expose the sport classifiers of this
// package to callers that need the same definition (internal/reports).
func IsRunningSport(sportType string) bool { return isRunningSport(sportType) }

func IsCyclingSport(sportType string) bool { return isCyclingSport(sportType) }
