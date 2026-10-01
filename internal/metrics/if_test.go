package metrics

import (
	"math"
	"testing"
)

func TestIntensityFactor(t *testing.T) {
	for _, tt := range []struct {
		name string
		ftp  int
		np   int
		want float64
	}{
		{name: "at FTP", ftp: 250, np: 250, want: 1},
		{name: "seventy percent FTP", ftp: 250, np: 175, want: 0.7},
		{name: "zero FTP", ftp: 0, np: 175, want: 0},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if got := IntensityFactor(tt.ftp, tt.np); math.Abs(got-tt.want) > 1e-9 {
				t.Fatalf("IntensityFactor(%d, %d) = %v, want %v", tt.ftp, tt.np, got, tt.want)
			}
		})
	}
}

func TestIntensityFactorRunning(t *testing.T) {
	if got := IntensityFactorRunning(4, 4); got != 1 {
		t.Fatalf("equal velocities: got %v, want 1", got)
	}
	if got := IntensityFactorRunning(5, 4); got != 1.25 {
		t.Fatalf("faster velocity: got %v, want 1.25", got)
	}
	if got := IntensityFactorRunning(5, 0); got != 0 {
		t.Fatalf("zero threshold velocity: got %v, want 0", got)
	}
}
