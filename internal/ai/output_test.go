package ai

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

// outputPayload returns the payload used as the reference request in the
// output tests. The numbers it carries (1000, 600, 10, 140, 30 and the
// 7 day window) are the only ones a provider may quote.
func outputPayload(t *testing.T) Payload {
	t.Helper()
	payload, err := NewBuilder(KindWeekly).
		WindowDays(7).
		DistanceMeters(1000).
		DurationSeconds(600).
		ElevationGainMeters(10).
		AvgHeartRate(140).
		TrainingLoad(30).
		Build()
	if err != nil {
		t.Fatalf("Build() unexpected error: %v", err)
	}
	return payload
}

// TestParseOutputAcceptsValidResponse checks the happy path: a response that
// respects the contract is parsed into the shared Output type.
func TestParseOutputAcceptsValidResponse(t *testing.T) {
	raw := []byte(`{
		"summary": "La carga de la semana se mantiene estable.",
		"observations": ["El volumen sigue concentrado en dos sesiones."],
		"cautions": ["Considera un descanso extra si la fatiga sube."]
	}`)

	out, err := ParseOutput(raw, outputPayload(t))
	if err != nil {
		t.Fatalf("ParseOutput() unexpected error: %v", err)
	}
	if out.Summary == "" {
		t.Error("ParseOutput() summary is empty, want the provider text")
	}
	if len(out.Observations) != 1 {
		t.Errorf("ParseOutput() observations = %v, want 1 item", out.Observations)
	}
	if len(out.Cautions) != 1 {
		t.Errorf("ParseOutput() cautions = %v, want 1 item", out.Cautions)
	}
}

// TestValidateOutputRejectsMissingRequiredFields checks that every field of
// the shared schema is required.
func TestValidateOutputRejectsMissingRequiredFields(t *testing.T) {
	cases := map[string]string{
		"summary":      `{"observations": [], "cautions": []}`,
		"observations": `{"summary": "ok", "cautions": []}`,
		"cautions":     `{"summary": "ok", "observations": []}`,
	}
	for field, raw := range cases {
		t.Run(field, func(t *testing.T) {
			err := ValidateOutput([]byte(raw), outputPayload(t))
			if err == nil {
				t.Fatalf("ValidateOutput() without %q: expected error", field)
			}
			if !strings.Contains(err.Error(), "missing required field") ||
				!strings.Contains(err.Error(), field) {
				t.Errorf("ValidateOutput() error = %q, want it to report the missing field %q", err, field)
			}
		})
	}
}

// TestValidateOutputRejectsUnknownFields checks the strict schema: a provider
// cannot add fields, and it cannot smuggle metrics back in the answer.
func TestValidateOutputRejectsUnknownFields(t *testing.T) {
	cases := map[string]string{
		"metrics":         `{"summary": "ok", "observations": [], "cautions": [], "metrics": {}}`,
		"distance_meters": `{"summary": "ok", "observations": [], "cautions": [], "distance_meters": 1000}`,
		"diagnosis":       `{"summary": "ok", "observations": [], "cautions": [], "diagnosis": "no"}`,
	}
	for field, raw := range cases {
		t.Run(field, func(t *testing.T) {
			err := ValidateOutput([]byte(raw), outputPayload(t))
			if err == nil {
				t.Fatalf("ValidateOutput() with unknown field %q: expected error", field)
			}
			if !strings.Contains(err.Error(), "unknown field") {
				t.Errorf("ValidateOutput() error = %q, want it to report an unknown field", err)
			}
		})
	}
}

// TestValidateOutputRejectsWrongTypes checks that every field has the type
// declared by the shared schema.
func TestValidateOutputRejectsWrongTypes(t *testing.T) {
	cases := map[string]string{
		"summary as number":       `{"summary": 42, "observations": [], "cautions": []}`,
		"summary as object":       `{"summary": {"text": "ok"}, "observations": [], "cautions": []}`,
		"observations as string":  `{"summary": "ok", "observations": "none", "cautions": []}`,
		"observations as numbers": `{"summary": "ok", "observations": [1, 2], "cautions": []}`,
		"cautions as object":      `{"summary": "ok", "observations": [], "cautions": {}}`,
		"root as array":           `[{"summary": "ok", "observations": [], "cautions": []}]`,
	}
	for name, raw := range cases {
		t.Run(name, func(t *testing.T) {
			err := ValidateOutput([]byte(raw), outputPayload(t))
			if err == nil {
				t.Fatalf("ValidateOutput() with %s: expected error", name)
			}
			if !strings.Contains(err.Error(), "invalid analysis output") {
				t.Errorf("ValidateOutput() error = %q, want an invalid analysis output error", err)
			}
		})
	}
}

// TestValidateOutputRejectsNullFields checks that null is not an accepted
// way to skip a required field.
func TestValidateOutputRejectsNullFields(t *testing.T) {
	cases := map[string]string{
		"summary":      `{"summary": null, "observations": [], "cautions": []}`,
		"observations": `{"summary": "ok", "observations": null, "cautions": []}`,
		"cautions":     `{"summary": "ok", "observations": [], "cautions": null}`,
		"root":         `null`,
	}
	for field, raw := range cases {
		t.Run(field, func(t *testing.T) {
			err := ValidateOutput([]byte(raw), outputPayload(t))
			if err == nil {
				t.Fatalf("ValidateOutput() with null %q: expected error", field)
			}
			if !strings.Contains(err.Error(), "must not be null") {
				t.Errorf("ValidateOutput() error = %q, want it to report the null %q", err, field)
			}
		})
	}
}

// TestValidateOutputRejectsTrailingJSON checks that only one JSON document
// is accepted, so a provider cannot append a second answer.
func TestValidateOutputRejectsTrailingJSON(t *testing.T) {
	cases := map[string]string{
		"second document": `{"summary": "ok", "observations": [], "cautions": []}{"summary": "again"}`,
		"garbage":         `{"summary": "ok", "observations": [], "cautions": []} trailing`,
		"array":           `{"summary": "ok", "observations": [], "cautions": []} []`,
	}
	for name, raw := range cases {
		t.Run(name, func(t *testing.T) {
			err := ValidateOutput([]byte(raw), outputPayload(t))
			if err == nil {
				t.Fatalf("ValidateOutput() with %s: expected error", name)
			}
			if !strings.Contains(err.Error(), "trailing data") {
				t.Errorf("ValidateOutput() error = %q, want it to report trailing data", err)
			}
		})
	}
}

// TestValidateOutputRejectsUnboundedStrings checks the length bounds of the
// shared schema.
func TestValidateOutputRejectsUnboundedStrings(t *testing.T) {
	longSummary := strings.Repeat("a", MaxSummaryLength+1)
	longItem := strings.Repeat("a", MaxObservationLength+1)
	manyObservations := `{"summary": "ok", "observations": [`
	for i := 0; i < MaxObservations+1; i++ {
		if i > 0 {
			manyObservations += ","
		}
		manyObservations += `"item"`
	}
	manyObservations += `], "cautions": []}`
	manyCautions := `{"summary": "ok", "observations": [], "cautions": [`
	for i := 0; i < MaxCautions+1; i++ {
		if i > 0 {
			manyCautions += ","
		}
		manyCautions += `"item"`
	}
	manyCautions += `]}`

	cases := map[string]string{
		"long summary":      `{"summary": "` + longSummary + `", "observations": [], "cautions": []}`,
		"long observation":  `{"summary": "ok", "observations": ["` + longItem + `"], "cautions": []}`,
		"long caution":      `{"summary": "ok", "observations": [], "cautions": ["` + longItem + `"]}`,
		"empty summary":     `{"summary": "   ", "observations": [], "cautions": []}`,
		"empty item":        `{"summary": "ok", "observations": ["  "], "cautions": []}`,
		"too many":          manyObservations,
		"too many cautions": manyCautions,
	}
	for name, raw := range cases {
		t.Run(name, func(t *testing.T) {
			err := ValidateOutput([]byte(raw), outputPayload(t))
			if err == nil {
				t.Fatalf("ValidateOutput() with %s: expected error", name)
			}
			if !strings.Contains(err.Error(), "invalid analysis output") {
				t.Errorf("ValidateOutput() error = %q, want an invalid analysis output error", err)
			}
		})
	}
}

// TestValidateOutputRejectsInventedMetrics checks that the answer cannot
// introduce a number the provider was never given.
func TestValidateOutputRejectsInventedMetrics(t *testing.T) {
	cases := map[string]string{
		"unknown number": `{"summary": "Cubriste 2500 metros.", "observations": [], "cautions": []}`,
		"unknown ratio":  `{"summary": "ok", "observations": ["La carga subió 12% respecto a la semana anterior."], "cautions": []}`,
		"unknown value":  `{"summary": "ok", "observations": [], "cautions": ["Descansa al menos 48 horas."]}`,
	}
	for name, raw := range cases {
		t.Run(name, func(t *testing.T) {
			err := ValidateOutput([]byte(raw), outputPayload(t))
			if err == nil {
				t.Fatalf("ValidateOutput() with %s: expected error", name)
			}
			if !strings.Contains(err.Error(), "was never sent to the provider") {
				t.Errorf("ValidateOutput() error = %q, want it to report an invented metric", err)
			}
		})
	}
}

// TestValidateOutputAcceptsSentNumbers checks that quoting a number the
// provider did receive is allowed.
func TestValidateOutputAcceptsSentNumbers(t *testing.T) {
	raw := []byte(`{
		"summary": "La sesion cubrio 1000 metros en 600 segundos.",
		"observations": ["La carga de 30 mantiene el ritmo."],
		"cautions": ["Revisa la fc media de 140 bpm."]
	}`)
	if err := ValidateOutput(raw, outputPayload(t)); err != nil {
		t.Fatalf("ValidateOutput() unexpected error: %v", err)
	}
}

// TestValidateOutputRejectsDiagnosis checks that the analysis interprets
// training and never diagnoses the user.
func TestValidateOutputRejectsDiagnosis(t *testing.T) {
	cases := map[string]string{
		"spanish": `{"summary": "El patron sugiere un diagnostico de cardiopatia.", "observations": [], "cautions": []}`,
		"english": `{"summary": "ok", "observations": ["Possible arrhythmia detected."], "cautions": []}`,
		"injury":  `{"summary": "ok", "observations": [], "cautions": ["Looks like a chronic injury."]}`,
	}
	for name, raw := range cases {
		t.Run(name, func(t *testing.T) {
			err := ValidateOutput([]byte(raw), outputPayload(t))
			if err == nil {
				t.Fatalf("ValidateOutput() with %s: expected error", name)
			}
			if !strings.Contains(err.Error(), "diagnosis claim") {
				t.Errorf("ValidateOutput() error = %q, want it to report a diagnosis claim", err)
			}
		})
	}
}

// TestOutputSchemaIsStrictAndShared checks that every provider receives the
// same strict schema and that the schema matches the Go validator.
func TestOutputSchemaIsStrictAndShared(t *testing.T) {
	schema := OutputSchema()

	if schema.Name != OutputSchemaName {
		t.Errorf("schema name = %q, want %q", schema.Name, OutputSchemaName)
	}
	if !schema.Strict {
		t.Error("schema strict = false, want true")
	}

	var document struct {
		Type                 string `json:"type"`
		AdditionalProperties *bool  `json:"additionalProperties"`
		Required             []string
		Properties           map[string]struct {
			Type     string `json:"type"`
			MaxLen   int    `json:"maxLength"`
			MaxItems int    `json:"maxItems"`
			Items    *struct {
				Type   string `json:"type"`
				MaxLen int    `json:"maxLength"`
			} `json:"items"`
		} `json:"properties"`
	}
	if err := json.Unmarshal(schema.Schema, &document); err != nil {
		t.Fatalf("Unmarshal(schema) error: %v", err)
	}

	if document.Type != "object" {
		t.Errorf("schema type = %q, want object", document.Type)
	}
	if document.AdditionalProperties == nil || *document.AdditionalProperties {
		t.Errorf("schema additionalProperties = %v, want false", document.AdditionalProperties)
	}

	wantRequired := []string{"summary", "observations", "cautions"}
	if !reflect.DeepEqual(document.Required, wantRequired) {
		t.Errorf("schema required = %v, want %v", document.Required, wantRequired)
	}

	// The schema properties must be exactly the JSON fields of Output.
	typ := reflect.TypeOf(Output{})
	gotFields := map[string]bool{}
	for i := 0; i < typ.NumField(); i++ {
		name := strings.Split(typ.Field(i).Tag.Get("json"), ",")[0]
		gotFields[name] = true
	}
	if len(document.Properties) != len(gotFields) {
		t.Errorf("schema properties = %v, want %v", document.Properties, gotFields)
	}
	for name := range gotFields {
		if _, ok := document.Properties[name]; !ok {
			t.Errorf("schema does not describe the Output field %q", name)
		}
	}
	for name := range document.Properties {
		if !gotFields[name] {
			t.Errorf("schema describes %q, which is not an Output field", name)
		}
	}

	// The schema bounds must be the bounds the validator enforces.
	if got := document.Properties["summary"].MaxLen; got != MaxSummaryLength {
		t.Errorf("schema summary maxLength = %d, want %d", got, MaxSummaryLength)
	}
	if got := document.Properties["observations"].MaxItems; got != MaxObservations {
		t.Errorf("schema observations maxItems = %d, want %d", got, MaxObservations)
	}
	if got := document.Properties["cautions"].MaxItems; got != MaxCautions {
		t.Errorf("schema cautions maxItems = %d, want %d", got, MaxCautions)
	}
	if got := document.Properties["observations"].Items; got == nil || got.MaxLen != MaxObservationLength {
		t.Errorf("schema observations item maxLength = %v, want %d", got, MaxObservationLength)
	}
}

// TestOutputSchemaIsStableAcrossCalls checks that every provider gets the
// very same schema document.
func TestOutputSchemaIsStableAcrossCalls(t *testing.T) {
	first := OutputSchema()
	second := OutputSchema()

	if string(first.Schema) != string(second.Schema) {
		t.Error("OutputSchema() returned two different schema documents")
	}
	if first.Name != second.Name || first.Strict != second.Strict {
		t.Error("OutputSchema() returned two different schema envelopes")
	}
}
