package metrics

import (
	"math"
	"testing"
)

func TestTSSRunning(t *testing.T) {
	tests := []struct {
		name              string
		thresholdSecPerKm int
		durationSec       int
		actualSecPerKm    int
		want              float64
	}{
		{name: "one hour at threshold", thresholdSecPerKm: 240, durationSec: 3600, actualSecPerKm: 240, want: 100},
		{name: "twenty minutes at threshold", thresholdSecPerKm: 240, durationSec: 1200, actualSecPerKm: 240, want: 100.0 / 3.0},
		{name: "faster than threshold", thresholdSecPerKm: 240, durationSec: 3600, actualSecPerKm: 200, want: 144},
		{name: "slower than threshold", thresholdSecPerKm: 240, durationSec: 3600, actualSecPerKm: 300, want: 64},
		{name: "zero duration", thresholdSecPerKm: 240, durationSec: 0, actualSecPerKm: 240, want: 0},
		{name: "invalid pace", thresholdSecPerKm: 0, durationSec: 3600, actualSecPerKm: 240, want: 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := TSSRunning(tt.thresholdSecPerKm, tt.durationSec, tt.actualSecPerKm)
			if math.Abs(got-tt.want) > 1e-9 {
				t.Fatalf("TSSRunning(%d, %d, %d) = %v, want %v", tt.thresholdSecPerKm, tt.durationSec, tt.actualSecPerKm, got, tt.want)
			}
		})
	}
}
