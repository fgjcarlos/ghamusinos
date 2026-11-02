// Package ai builds the provider-agnostic analysis payload and validates the
// analysis output that every AI provider must respect.
//
// The package is deliberately provider free: it knows how to describe a
// training request (numeric aggregates only) and how to validate the answer
// that comes back, but it never talks to a provider. HTTP adapters are added
// on top of this contract, not inside it.
package ai

import (
	"encoding/json"
	"fmt"
	"math"
)

// Kind identifies the analysis the user requested on demand.
type Kind string

const (
	// KindActivity analyses a single activity.
	KindActivity Kind = "activity"
	// KindWeekly analyses a bounded weekly window.
	KindWeekly Kind = "weekly"
	// KindLoad analyses a bounded training-load window.
	KindLoad Kind = "load"
)

// Sanity bounds for the aggregates that may leave the system. They are
// deliberately generous so weekly and load windows stay valid, but tight
// enough to stop a broken upstream computation from reaching a provider.
const (
	// MaxDistanceMeters is 1000 km.
	MaxDistanceMeters = 1_000_000.0
	// MaxDurationSeconds is 7 days.
	MaxDurationSeconds = 604_800.0
	// MaxElevationGainMeters is 50 km of accumulated climb.
	MaxElevationGainMeters = 50_000.0
	// MaxAvgHeartRate is 240 bpm, above the highest plausible average.
	MaxAvgHeartRate = 240.0
	// MaxTrainingLoad is the top of the calculated load scale.
	MaxTrainingLoad = 1_000.0
	// MaxWindowDays bounds the analysed window (weekly and load analyses).
	MaxWindowDays = 90
)

// Metrics holds the numeric aggregates that may be sent to a provider.
//
// A nil field means "not measured": the metric is omitted from the payload
// instead of being reported as a zero. A non-nil field holds a measured
// value, including an explicit zero (a rest day has a real load of zero).
type Metrics struct {
	DistanceMeters      *float64 `json:"distance_meters,omitempty"`
	DurationSeconds     *float64 `json:"duration_seconds,omitempty"`
	ElevationGainMeters *float64 `json:"elevation_gain_meters,omitempty"`
	AvgHeartRate        *float64 `json:"avg_heart_rate,omitempty"`
	TrainingLoad        *float64 `json:"training_load,omitempty"`
}

// Payload is the request that may leave the system.
//
// It carries the analysis kind, the bounded window and the numeric
// aggregates only. There is no field for identity, names, GPS, raw streams,
// timestamps or free text, so those can never be sent by accident.
type Payload struct {
	Kind       Kind
	Metrics    Metrics
	WindowDays int
}

// payloadJSON is the allowlisted wire shape of a Payload. Only Payload
// serialises through it, so adding a field to Payload cannot leak data.
type payloadJSON struct {
	Kind       Kind    `json:"kind"`
	Metrics    Metrics `json:"metrics"`
	WindowDays int     `json:"window_days,omitempty"`
}

// MarshalJSON serialises the payload through the allowlisted shape.
func (p Payload) MarshalJSON() ([]byte, error) {
	return json.Marshal(payloadJSON(p))
}

// Builder assembles a Payload.
//
// Every setter returns the builder so calls can be chained, and every
// validation happens in Build so a caller always learns about a rejected
// value through the returned error.
type Builder struct {
	kind       Kind
	windowDays int
	metrics    Metrics
}

// NewBuilder returns a builder for the given analysis kind.
func NewBuilder(kind Kind) *Builder {
	return &Builder{kind: kind}
}

// WindowDays sets the analysed window, in days. Zero means "not set" and is
// omitted from the payload.
func (b *Builder) WindowDays(days int) *Builder {
	b.windowDays = days
	return b
}

// DistanceMeters sets the distance travelled, in meters.
func (b *Builder) DistanceMeters(value float64) *Builder {
	b.metrics.DistanceMeters = &value
	return b
}

// DurationSeconds sets the moving time, in seconds.
func (b *Builder) DurationSeconds(value float64) *Builder {
	b.metrics.DurationSeconds = &value
	return b
}

// ElevationGainMeters sets the accumulated climb, in meters.
func (b *Builder) ElevationGainMeters(value float64) *Builder {
	b.metrics.ElevationGainMeters = &value
	return b
}

// AvgHeartRate sets the average heart rate, in bpm.
//
// Zero is rejected: an average over a non-empty sample is never zero, so a
// zero means the value was never measured.
func (b *Builder) AvgHeartRate(value float64) *Builder {
	b.metrics.AvgHeartRate = &value
	return b
}

// TrainingLoad sets the calculated training load.
func (b *Builder) TrainingLoad(value float64) *Builder {
	b.metrics.TrainingLoad = &value
	return b
}

// Build validates the accumulated values and returns the payload.
//
// Metrics that were never set are omitted. Values that are not finite,
// negative or beyond the documented bounds are rejected.
func (b *Builder) Build() (Payload, error) {
	if !b.kind.valid() {
		return Payload{}, fmt.Errorf("ai: unknown analysis kind %q", string(b.kind))
	}
	if b.windowDays < 0 || b.windowDays > MaxWindowDays {
		return Payload{}, fmt.Errorf("ai: window_days %d out of bounds (0..%d)", b.windowDays, MaxWindowDays)
	}

	checks := []struct {
		name     string
		value    *float64
		max      float64
		positive bool
	}{
		{"distance_meters", b.metrics.DistanceMeters, MaxDistanceMeters, false},
		{"duration_seconds", b.metrics.DurationSeconds, MaxDurationSeconds, false},
		{"elevation_gain_meters", b.metrics.ElevationGainMeters, MaxElevationGainMeters, false},
		{"avg_heart_rate", b.metrics.AvgHeartRate, MaxAvgHeartRate, true},
		{"training_load", b.metrics.TrainingLoad, MaxTrainingLoad, false},
	}
	for _, check := range checks {
		if check.value == nil {
			continue
		}
		if err := validateMetric(check.name, *check.value, check.max, check.positive); err != nil {
			return Payload{}, err
		}
	}

	return Payload{
		Kind:       b.kind,
		Metrics:    b.metrics,
		WindowDays: b.windowDays,
	}, nil
}

// valid reports whether the kind is one of the three supported analyses.
func (k Kind) valid() bool {
	switch k {
	case KindActivity, KindWeekly, KindLoad:
		return true
	default:
		return false
	}
}

// validateMetric rejects values that cannot describe a real measurement.
func validateMetric(name string, value, max float64, positive bool) error {
	if math.IsNaN(value) || math.IsInf(value, 0) {
		return fmt.Errorf("ai: %s must be a finite number", name)
	}
	if value < 0 {
		return fmt.Errorf("ai: %s must not be negative", name)
	}
	if positive && value == 0 {
		return fmt.Errorf("ai: %s must be greater than zero: zero is missing data, not a measurement", name)
	}
	if value > max {
		return fmt.Errorf("ai: %s out of bounds: %g (max %g)", name, value, max)
	}
	return nil
}
