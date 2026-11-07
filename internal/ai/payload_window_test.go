package ai

import (
	"strings"
	"testing"
)

// The payload contract has two families of analyses: an activity analysis
// describes ONE event, and a window analysis (weekly, load) describes the
// TOTALS of a bounded window. The ceilings of the aggregates therefore
// cannot be fixed constants: a window of N days has to admit the totals of
// N days and reject anything beyond that, while a single activity keeps
// its own tight ceilings.

// TestActivityCeilingsRejectOutOfRangeValues checks that the per-activity
// ceilings still reject the out of range values, and that declaring a
// window does not widen them: an activity is one event, not a window.
func TestActivityCeilingsRejectOutOfRangeValues(t *testing.T) {
	cases := []struct {
		name  string
		apply func(*Builder) *Builder
	}{
		{"distance_meters", func(b *Builder) *Builder { return b.DistanceMeters(MaxDistanceMeters + 1) }},
		{"duration_seconds", func(b *Builder) *Builder { return b.DurationSeconds(MaxDurationSeconds + 1) }},
		{"elevation_gain_meters", func(b *Builder) *Builder { return b.ElevationGainMeters(MaxElevationGainMeters + 1) }},
		{"avg_heart_rate", func(b *Builder) *Builder { return b.AvgHeartRate(MaxAvgHeartRate + 1) }},
		{"training_load", func(b *Builder) *Builder { return b.TrainingLoad(MaxTrainingLoad + 1) }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			for _, days := range []int{0, 1, 30} {
				_, err := tc.apply(NewBuilder(KindActivity).WindowDays(days)).Build()
				if err == nil {
					t.Fatalf("Build() with out of bounds %s (window_days=%d): expected error", tc.name, days)
				}
			}
		})
	}
}

// TestWindowCeilingsScaleWithWindowDays checks the scaling rule itself:
// the four summed aggregates grow with the window and the average heart
// rate never does.
func TestWindowCeilingsScaleWithWindowDays(t *testing.T) {
	for _, kind := range []Kind{KindWeekly, KindLoad} {
		for _, days := range []int{1, 7, 28, MaxWindowDays} {
			factor := float64(days)
			limits := LimitsFor(kind, days)

			if limits.DistanceMeters != MaxDistanceMeters*factor {
				t.Errorf("LimitsFor(%q, %d).DistanceMeters = %g, want %g", kind, days, limits.DistanceMeters, MaxDistanceMeters*factor)
			}
			if limits.DurationSeconds != MaxDurationSeconds*factor {
				t.Errorf("LimitsFor(%q, %d).DurationSeconds = %g, want %g", kind, days, limits.DurationSeconds, MaxDurationSeconds*factor)
			}
			if limits.ElevationGainMeters != MaxElevationGainMeters*factor {
				t.Errorf("LimitsFor(%q, %d).ElevationGainMeters = %g, want %g", kind, days, limits.ElevationGainMeters, MaxElevationGainMeters*factor)
			}
			if limits.TrainingLoad != MaxTrainingLoad*factor {
				t.Errorf("LimitsFor(%q, %d).TrainingLoad = %g, want %g", kind, days, limits.TrainingLoad, MaxTrainingLoad*factor)
			}
			if limits.AvgHeartRate != MaxAvgHeartRate {
				t.Errorf("LimitsFor(%q, %d).AvgHeartRate = %g, want %g (an average never scales)", kind, days, limits.AvgHeartRate, MaxAvgHeartRate)
			}
		}
	}
}

// TestBuildAcceptsWindowTotalsProportionalToWindowDays checks the positive
// case: a window whose totals are proportional to its days is a valid
// payload, including the totals that the old fixed ceilings rejected.
func TestBuildAcceptsWindowTotalsProportionalToWindowDays(t *testing.T) {
	const days = 28
	factor := float64(days)

	for _, kind := range []Kind{KindWeekly, KindLoad} {
		t.Run(string(kind), func(t *testing.T) {
			// Totals exactly at the proportional ceiling.
			mustBuild(t, NewBuilder(kind).WindowDays(days).
				DistanceMeters(MaxDistanceMeters*factor).
				DurationSeconds(MaxDurationSeconds*factor).
				ElevationGainMeters(MaxElevationGainMeters*factor).
				AvgHeartRate(MaxAvgHeartRate).
				TrainingLoad(MaxTrainingLoad*factor))

			// Totals above the old fixed ceilings but proportional to the
			// window: 28 days of training load, distance and moving time.
			mustBuild(t, NewBuilder(kind).WindowDays(days).
				DistanceMeters(1_400_000).
				DurationSeconds(840_000).
				ElevationGainMeters(56_000).
				AvgHeartRate(150).
				TrainingLoad(1_400))
		})
	}
}

// TestBuildRejectsWindowTotalsAboveProportionalCeiling checks that the
// scaling does not open the contract: a window whose totals exceed what
// its days can hold is still rejected, with a safe error.
func TestBuildRejectsWindowTotalsAboveProportionalCeiling(t *testing.T) {
	const days = 28
	factor := float64(days)

	cases := []struct {
		name  string
		apply func(*Builder) *Builder
	}{
		{"distance_meters", func(b *Builder) *Builder { return b.DistanceMeters(MaxDistanceMeters*factor + 1) }},
		{"duration_seconds", func(b *Builder) *Builder { return b.DurationSeconds(MaxDurationSeconds*factor + 1) }},
		{"elevation_gain_meters", func(b *Builder) *Builder { return b.ElevationGainMeters(MaxElevationGainMeters*factor + 1) }},
		{"training_load", func(b *Builder) *Builder { return b.TrainingLoad(MaxTrainingLoad*factor + 1) }},
	}
	for _, kind := range []Kind{KindWeekly, KindLoad} {
		for _, tc := range cases {
			t.Run(string(kind)+"/"+tc.name, func(t *testing.T) {
				_, err := tc.apply(NewBuilder(kind).WindowDays(days)).Build()
				if err == nil {
					t.Fatalf("Build() with out of bounds %s: expected error", tc.name)
				}
				if !strings.Contains(err.Error(), tc.name) {
					t.Errorf("Build() error = %q, want it to name %s", err, tc.name)
				}
			})
		}
	}
}

// TestBuildRejectsWindowLongerThanMaxWindowDays checks that the window
// length itself stays bounded: the scaling never widens the window.
func TestBuildRejectsWindowLongerThanMaxWindowDays(t *testing.T) {
	for _, kind := range []Kind{KindWeekly, KindLoad} {
		_, err := NewBuilder(kind).WindowDays(MaxWindowDays + 1).Build()
		if err == nil {
			t.Fatalf("Build() with window_days %d (%s): expected error", MaxWindowDays+1, kind)
		}
		if !strings.Contains(err.Error(), "window_days") {
			t.Errorf("Build() error = %q, want it to name window_days", err)
		}
		if _, err := NewBuilder(kind).WindowDays(MaxWindowDays).Build(); err != nil {
			t.Errorf("Build() with window_days %d (%s): unexpected error: %v", MaxWindowDays, kind, err)
		}
	}
}

// TestWindowScalingDoesNotWeakenActivityCeilings checks that the activity
// ceilings are exactly the documented ones, no matter how long the window
// the caller declares.
func TestWindowScalingDoesNotWeakenActivityCeilings(t *testing.T) {
	want := Limits{
		DistanceMeters:      MaxDistanceMeters,
		DurationSeconds:     MaxDurationSeconds,
		ElevationGainMeters: MaxElevationGainMeters,
		AvgHeartRate:        MaxAvgHeartRate,
		TrainingLoad:        MaxTrainingLoad,
	}
	for _, days := range []int{0, 1, 7, MaxWindowDays} {
		if got := LimitsFor(KindActivity, days); got != want {
			t.Errorf("LimitsFor(activity, %d) = %+v, want %+v", days, got, want)
		}
	}
}

// TestAverageHeartRateNeverScalesWithWindow checks that the average heart
// rate keeps the same ceiling in a window: it is an average, not a sum.
func TestAverageHeartRateNeverScalesWithWindow(t *testing.T) {
	if _, err := NewBuilder(KindWeekly).WindowDays(MaxWindowDays).AvgHeartRate(MaxAvgHeartRate + 1).Build(); err == nil {
		t.Fatalf("Build() with avg heart rate %g: expected error", MaxAvgHeartRate+1)
	}
	mustBuild(t, NewBuilder(KindWeekly).WindowDays(MaxWindowDays).AvgHeartRate(MaxAvgHeartRate))
}
