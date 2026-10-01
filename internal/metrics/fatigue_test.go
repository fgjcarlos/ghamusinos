package metrics

import (
	"math"
	"testing"
	"time"
)

func TestCTLAndATL(t *testing.T) {
	day := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	constant := make([]DailyLoad, 42)
	for i := range constant {
		constant[i] = DailyLoad{Day: day.AddDate(0, 0, i), TSS: 100}
	}
	if got, want := CTL(constant), 100.0; math.Abs(got-want) > 1e-9 {
		t.Fatalf("CTL(constant 42 days) = %v, want %v", got, want)
	}
	if got, want := ATL(constant[:7]), 100.0; math.Abs(got-want) > 1e-9 {
		t.Fatalf("ATL(constant 7 days) = %v, want %v", got, want)
	}
	if CTL(nil) != 0 || ATL(nil) != 0 {
		t.Fatalf("empty CTL/ATL must be zero: CTL=%v ATL=%v", CTL(nil), ATL(nil))
	}
}

func TestATLDecaysOnZeroLoad(t *testing.T) {
	daily := []DailyLoad{{TSS: 100}}
	for range 7 {
		daily = append(daily, DailyLoad{TSS: 0})
	}
	want := 100 * math.Exp(-7.0/float64(TauATL))
	if got := ATL(daily); math.Abs(got-want) > 1e-9 {
		t.Fatalf("ATL after zero-load days = %v, want %v", got, want)
	}
}

func TestTSB(t *testing.T) {
	if got := TSB(42.1, 31); math.Abs(got-11.1) > 1e-9 {
		t.Fatalf("TSB(42.1, 31) = %v, want 11.1", got)
	}
	if got := TSB(50, 50); got != 0 {
		t.Fatalf("TSB(equal loads) = %v, want 0", got)
	}
}
