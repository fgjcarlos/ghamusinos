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

func TestFillMissingDays(t *testing.T) {
	from := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	to := from.AddDate(0, 0, 6)
	raw := []DailyLoad{
		{Day: from.AddDate(0, 0, 1), TSS: 80},
		{Day: from.AddDate(0, 0, 3), TSS: 60},
		{Day: from.AddDate(0, 0, 5), TSS: 90},
	}
	got := FillMissingDays(from, to, raw)
	if len(got) != 7 {
		t.Fatalf("got %d rows, want 7", len(got))
	}
	for i, want := range []float64{0, 80, 0, 60, 0, 90, 0} {
		if !got[i].Day.Equal(from.AddDate(0, 0, i)) || got[i].TSS != want {
			t.Errorf("row %d = (%s, %v), want (%s, %v)", i, got[i].Day, got[i].TSS, from.AddDate(0, 0, i), want)
		}
	}
}

func TestFillMissingDaysEmptyAndReversedRanges(t *testing.T) {
	from := time.Date(2026, 9, 7, 0, 0, 0, 0, time.UTC)
	if got := FillMissingDays(from, from.AddDate(0, 0, -6), nil); len(got) != 0 {
		t.Fatalf("reversed range returned %d rows, want empty", len(got))
	}
	got := FillMissingDays(from, from.AddDate(0, 0, 2), nil)
	if len(got) != 3 {
		t.Fatalf("empty input returned %d rows, want 3", len(got))
	}
	for _, row := range got {
		if row.TSS != 0 {
			t.Errorf("missing date %s has TSS %v, want zero", row.Day, row.TSS)
		}
	}
}
