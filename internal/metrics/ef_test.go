package metrics

import (
	"math"
	"testing"
)

func TestEfficiencyFactor(t *testing.T) {
	for _, tt := range []struct {
		name string
		power int
		hr int
		want float64
	}{
		{name: "power over heart rate", power: 200, hr: 150, want: 4.0 / 3.0},
		{name: "zero heart rate", power: 200, hr: 0, want: 0},
		{name: "negative heart rate", power: 200, hr: -1, want: 0},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if got := EfficiencyFactor(tt.power, tt.hr); math.Abs(got-tt.want) > 1e-9 {
				t.Fatalf("EfficiencyFactor(%d, %d) = %v, want %v", tt.power, tt.hr, got, tt.want)
			}
		})
	}
}
