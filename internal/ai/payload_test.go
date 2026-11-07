package ai

import (
	"encoding/json"
	"math"
	"reflect"
	"strings"
	"testing"
)

// mustBuild is a test helper that fails the test when the builder rejects
// a payload the test considers valid.
func mustBuild(t *testing.T, b *Builder) Payload {
	t.Helper()
	p, err := b.Build()
	if err != nil {
		t.Fatalf("Build() unexpected error: %v", err)
	}
	return p
}

// TestBuildRejectsUnknownKind checks that only the three documented kinds
// (activity, weekly, load) are accepted.
func TestBuildRejectsUnknownKind(t *testing.T) {
	for _, kind := range []Kind{"monthly", "", "Activity", "weekly "} {
		if _, err := NewBuilder(kind).Build(); err == nil {
			t.Errorf("Build() with kind %q: expected error", kind)
		}
	}
}

// TestBuildAcceptsDocumentedKinds checks the positive case for the three
// supported analysis kinds.
func TestBuildAcceptsDocumentedKinds(t *testing.T) {
	for _, kind := range []Kind{KindActivity, KindWeekly, KindLoad} {
		p := mustBuild(t, NewBuilder(kind))
		if p.Kind != kind {
			t.Errorf("Build() kind = %q, want %q", p.Kind, kind)
		}
	}
}

// TestBuildRejectsNegativeMetrics checks that a negative aggregate is
// rejected: negative values are impossible for distance, duration,
// elevation gain, heart rate or training load.
func TestBuildRejectsNegativeMetrics(t *testing.T) {
	cases := []struct {
		name  string
		apply func(*Builder) *Builder
	}{
		{"distance_meters", func(b *Builder) *Builder { return b.DistanceMeters(-1) }},
		{"duration_seconds", func(b *Builder) *Builder { return b.DurationSeconds(-0.001) }},
		{"elevation_gain_meters", func(b *Builder) *Builder { return b.ElevationGainMeters(-10) }},
		{"avg_heart_rate", func(b *Builder) *Builder { return b.AvgHeartRate(-1) }},
		{"training_load", func(b *Builder) *Builder { return b.TrainingLoad(-0.5) }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := tc.apply(NewBuilder(KindActivity)).Build(); err == nil {
				t.Fatalf("Build() with negative %s: expected error", tc.name)
			}
		})
	}
}

// TestBuildRejectsNonFiniteMetrics checks that NaN and +/-Inf are rejected:
// they cannot be serialised as valid JSON numbers.
func TestBuildRejectsNonFiniteMetrics(t *testing.T) {
	values := []float64{math.NaN(), math.Inf(1), math.Inf(-1)}
	for _, v := range values {
		if _, err := NewBuilder(KindActivity).DistanceMeters(v).Build(); err == nil {
			t.Errorf("Build() with non finite distance %v: expected error", v)
		}
		if _, err := NewBuilder(KindActivity).TrainingLoad(v).Build(); err == nil {
			t.Errorf("Build() with non finite load %v: expected error", v)
		}
		if _, err := NewBuilder(KindActivity).AvgHeartRate(v).Build(); err == nil {
			t.Errorf("Build() with non finite heart rate %v: expected error", v)
		}
	}
}

// TestBuildRejectsZeroHeartRate checks that a zero heart rate is rejected:
// zero is not a measurement, it is missing data, and missing data must be
// omitted rather than reported as a real value.
func TestBuildRejectsZeroHeartRate(t *testing.T) {
	if _, err := NewBuilder(KindActivity).AvgHeartRate(0).Build(); err == nil {
		t.Fatal("Build() with avg heart rate 0: expected error")
	}
}

// TestBuildRejectsOutOfBoundsMetrics checks the sanity bounds applied to
// every aggregate so a broken upstream computation cannot reach a provider.
func TestBuildRejectsOutOfBoundsMetrics(t *testing.T) {
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
			if _, err := tc.apply(NewBuilder(KindActivity)).Build(); err == nil {
				t.Fatalf("Build() with out of bounds %s: expected error", tc.name)
			}
		})
	}
}

// TestBuildRejectsOutOfBoundsWindow checks the window bound used by the
// weekly and load analyses.
func TestBuildRejectsOutOfBoundsWindow(t *testing.T) {
	for _, days := range []int{-1, MaxWindowDays + 1} {
		if _, err := NewBuilder(KindWeekly).WindowDays(days).Build(); err == nil {
			t.Errorf("Build() with window_days %d: expected error", days)
		}
	}
	if _, err := NewBuilder(KindWeekly).WindowDays(MaxWindowDays).Build(); err != nil {
		t.Errorf("Build() with window_days %d: unexpected error: %v", MaxWindowDays, err)
	}
}

// TestBuildOmitsMissingMetrics checks the "honest missing metric" rule: a
// metric that was never measured is absent from the payload instead of
// being reported as zero.
func TestBuildOmitsMissingMetrics(t *testing.T) {
	p := mustBuild(t, NewBuilder(KindActivity).DistanceMeters(1000))

	raw, err := p.MarshalJSON()
	if err != nil {
		t.Fatalf("MarshalJSON() error: %v", err)
	}
	var decoded struct {
		Metrics map[string]json.RawMessage `json:"metrics"`
	}
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatalf("Unmarshal() error: %v", err)
	}
	if len(decoded.Metrics) != 1 {
		t.Fatalf("metrics = %v, want only distance_meters", decoded.Metrics)
	}
	if _, ok := decoded.Metrics["distance_meters"]; !ok {
		t.Fatalf("metrics = %v, want distance_meters", decoded.Metrics)
	}
	for _, absent := range []string{"duration_seconds", "elevation_gain_meters", "avg_heart_rate", "training_load"} {
		if _, ok := decoded.Metrics[absent]; ok {
			t.Errorf("metrics contains %q, want it omitted", absent)
		}
	}
}

// TestPayloadJSONContainsOnlyAllowlistedKeys checks the privacy allowlist:
// the serialized payload may only contain the analysis kind, the bounded
// window and the five documented numeric aggregates.
func TestPayloadJSONContainsOnlyAllowlistedKeys(t *testing.T) {
	p := mustBuild(t, NewBuilder(KindWeekly).
		WindowDays(7).
		DistanceMeters(12345.5).
		DurationSeconds(3600).
		ElevationGainMeters(120).
		AvgHeartRate(148).
		TrainingLoad(55.25))

	raw, err := p.MarshalJSON()
	if err != nil {
		t.Fatalf("MarshalJSON() error: %v", err)
	}

	var top map[string]json.RawMessage
	if err := json.Unmarshal(raw, &top); err != nil {
		t.Fatalf("Unmarshal() error: %v", err)
	}
	allowedTop := map[string]bool{"kind": true, "window_days": true, "metrics": true}
	for key := range top {
		if !allowedTop[key] {
			t.Errorf("payload contains top level key %q, want only %v", key, allowedTop)
		}
	}

	var metrics map[string]json.RawMessage
	if err := json.Unmarshal(top["metrics"], &metrics); err != nil {
		t.Fatalf("Unmarshal(metrics) error: %v", err)
	}
	allowedMetrics := map[string]bool{
		"distance_meters":       true,
		"duration_seconds":      true,
		"elevation_gain_meters": true,
		"avg_heart_rate":        true,
		"training_load":         true,
	}
	for key := range metrics {
		if !allowedMetrics[key] {
			t.Errorf("payload contains metric %q, want only %v", key, allowedMetrics)
		}
	}
	if len(metrics) != len(allowedMetrics) {
		t.Errorf("metrics = %v, want %d entries", metrics, len(allowedMetrics))
	}
}

// TestPayloadJSONLeaksNoForbiddenContent checks that no identity, title,
// GPS, raw stream or timestamp field can ever reach the provider.
func TestPayloadJSONLeaksNoForbiddenContent(t *testing.T) {
	p := mustBuild(t, NewBuilder(KindActivity).
		WindowDays(1).
		DistanceMeters(1000).
		DurationSeconds(600).
		ElevationGainMeters(10).
		AvgHeartRate(140).
		TrainingLoad(30))

	raw, err := p.MarshalJSON()
	if err != nil {
		t.Fatalf("MarshalJSON() error: %v", err)
	}
	lower := strings.ToLower(string(raw))
	forbidden := []string{
		"user", "email", "name", "title", "gps", "stream", "latitude",
		"longitude", "elevation_meters", "started_at", "timestamp", "date",
		"route", "segment",
	}
	for _, token := range forbidden {
		if strings.Contains(lower, token) {
			t.Errorf("payload leaks %q: %s", token, raw)
		}
	}
}

// TestPayloadMarshalJSONIsUsedByEncodingJSON checks that the standard
// encoder goes through the allowlisted serializer, so a caller cannot
// accidentally leak the struct fields.
func TestPayloadMarshalJSONIsUsedByEncodingJSON(t *testing.T) {
	p := mustBuild(t, NewBuilder(KindLoad).TrainingLoad(42))

	viaMarshal, err := p.MarshalJSON()
	if err != nil {
		t.Fatalf("MarshalJSON() error: %v", err)
	}
	viaEncoder, err := json.Marshal(p)
	if err != nil {
		t.Fatalf("json.Marshal() error: %v", err)
	}
	if string(viaMarshal) != string(viaEncoder) {
		t.Errorf("json.Marshal = %s, want %s", viaEncoder, viaMarshal)
	}
}

// TestPayloadStructExposesNoIdentityFields checks the type level privacy
// guarantee: the payload type only carries the kind, the window and the
// aggregate metrics.
func TestPayloadStructExposesNoIdentityFields(t *testing.T) {
	typ := reflect.TypeOf(Payload{})
	want := map[string]bool{"Kind": true, "Metrics": true, "WindowDays": true}
	if typ.NumField() != len(want) {
		t.Fatalf("Payload has %d fields, want %d", typ.NumField(), len(want))
	}
	for i := 0; i < typ.NumField(); i++ {
		if !want[typ.Field(i).Name] {
			t.Errorf("Payload exposes field %q", typ.Field(i).Name)
		}
	}

	metricsType := reflect.TypeOf(Metrics{})
	wantMetrics := map[string]bool{
		"DistanceMeters":      true,
		"DurationSeconds":     true,
		"ElevationGainMeters": true,
		"AvgHeartRate":        true,
		"TrainingLoad":        true,
	}
	if metricsType.NumField() != len(wantMetrics) {
		t.Fatalf("Metrics has %d fields, want %d", metricsType.NumField(), len(wantMetrics))
	}
	for i := 0; i < metricsType.NumField(); i++ {
		if !wantMetrics[metricsType.Field(i).Name] {
			t.Errorf("Metrics exposes field %q", metricsType.Field(i).Name)
		}
	}
}
