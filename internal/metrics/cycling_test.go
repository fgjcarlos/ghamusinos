package metrics

import (
	"math"
	"testing"
)

func TestTSSCycling(t *testing.T) {
	tests := []struct {
		name     string
		ftp      int
		duration int
		np       int
		want     float64
	}{
		{name: "one hour at FTP", ftp: 250, duration: 3600, np: 250, want: 100},
		{name: "half hour at FTP", ftp: 250, duration: 1800, np: 250, want: 50},
		{name: "zero duration", ftp: 250, duration: 0, np: 250, want: 0},
		{name: "zero FTP", ftp: 0, duration: 3600, np: 250, want: 0},
		{name: "zero normalized power", ftp: 250, duration: 3600, np: 0, want: 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := TSSCycling(tt.ftp, tt.duration, tt.np); math.Abs(got-tt.want) > 1e-9 {
				t.Fatalf("TSSCycling(%d, %d, %d) = %v, want %v", tt.ftp, tt.duration, tt.np, got, tt.want)
			}
		})
	}
}
