package metrics

import (
	"math"
	"testing"
)

func TestCardiacDrift(t *testing.T) {
	for _, tt := range []struct {
		name string
		start, end float64
		want float64
	}{
		{name: "ten percent drift", start: 100, end: 110, want: 10},
		{name: "zero start", start: 0, end: 110, want: 0},
		{name: "negative start", start: -1, end: 110, want: 0},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if got := CardiacDrift(tt.start, tt.end); math.Abs(got-tt.want) > 1e-9 {
				t.Fatalf("CardiacDrift(%v, %v) = %v, want %v", tt.start, tt.end, got, tt.want)
			}
		})
	}
}

func TestCardiacDriftSeries(t *testing.T) {
	for _, tt := range []struct {
		name string
		samples []int
		want float64
	}{
		{name: "empty", samples: nil, want: 0},
		{name: "single sample", samples: []int{100}, want: 0},
		{name: "equal quartiles", samples: []int{100, 100, 100, 100}, want: 0},
		{name: "five percent drift", samples: []int{100, 100, 100, 105}, want: 5},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if got := CardiacDriftSeries(tt.samples); math.Abs(got-tt.want) > 1e-9 {
				t.Fatalf("CardiacDriftSeries(%v) = %v, want %v", tt.samples, got, tt.want)
			}
		})
	}
}
