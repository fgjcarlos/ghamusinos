package metrics

import (
	"math"
	"testing"
)

func TestGradeAdjustedPace(t *testing.T) {
	for _, tt := range []struct {
		name  string
		grade float64
		pace  float64
		want  float64
	}{
		{name: "level grade preserves pace", grade: 0, pace: 360, want: 360},
		{name: "uphill adjusts equivalent pace", grade: 0.10, pace: 360, want: 596.8214},
		{name: "downhill adjusts equivalent pace", grade: -0.10, pace: 360, want: 215.1706},
		{name: "invalid pace", grade: 0.1, pace: 0, want: 0},
		{name: "grade clamps to physiological bound", grade: 1, pace: 360, want: GradeAdjustedPace(0.35, 360)},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if got := GradeAdjustedPace(tt.grade, tt.pace); math.Abs(got-tt.want) > 1e-6 {
				t.Fatalf("GradeAdjustedPace(%v, %v) = %v, want %v", tt.grade, tt.pace, got, tt.want)
			}
		})
	}
}
