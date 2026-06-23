package strava

import (
	"io"
	"log/slog"
	"strconv"
	"strings"
	"sync"
)

// RateLimitTracker tracks Strava API rate limits.
type RateLimitTracker struct {
	mu sync.RWMutex

	// 15-minute window
	Usage15min int
	Limit15min int

	// Daily window
	UsageDaily int
	LimitDaily int

	logger *slog.Logger
}

// NewRateLimitTracker creates a new rate limit tracker.
func NewRateLimitTracker(logger *slog.Logger) *RateLimitTracker {
	if logger == nil {
		logger = slog.New(slog.NewTextHandler(io.Discard, nil))
	}
	return &RateLimitTracker{
		logger: logger,
	}
}

// UpdateFromHeaders parses X-RateLimit-Usage and X-RateLimit-Limit headers.
// Format: "X-RateLimit-Usage: 160,1000" (15-min, daily) and "X-RateLimit-Limit: 200,2000"
func (r *RateLimitTracker) UpdateFromHeaders(usage, limit string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Parse usage: "160,1000"
	if usage != "" {
		parts := strings.Split(usage, ",")
		if len(parts) >= 2 {
			if u15, err := strconv.Atoi(strings.TrimSpace(parts[0])); err == nil {
				r.Usage15min = u15
			}
			if uDaily, err := strconv.Atoi(strings.TrimSpace(parts[1])); err == nil {
				r.UsageDaily = uDaily
			}
		}
	}

	// Parse limit: "200,2000"
	if limit != "" {
		parts := strings.Split(limit, ",")
		if len(parts) >= 2 {
			if l15, err := strconv.Atoi(strings.TrimSpace(parts[0])); err == nil {
				r.Limit15min = l15
			}
			if lDaily, err := strconv.Atoi(strings.TrimSpace(parts[1])); err == nil {
				r.LimitDaily = lDaily
			}
		}
	}

	// Log warning if 15-minute usage is ≥ 80% of limit
	if r.Limit15min > 0 && r.Usage15min > 0 {
		percentage := float64(r.Usage15min) / float64(r.Limit15min)
		if percentage >= 0.8 {
			r.logger.Warn("Strava rate limit warning",
				slog.Int("usage_15min", r.Usage15min),
				slog.Int("limit_15min", r.Limit15min),
				slog.Float64("percentage", percentage*100))
		}
	}
}

// GetUsage returns current usage (15-min, daily).
func (r *RateLimitTracker) GetUsage() (int, int) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.Usage15min, r.UsageDaily
}

// GetLimits returns current limits (15-min, daily).
func (r *RateLimitTracker) GetLimits() (int, int) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.Limit15min, r.LimitDaily
}
