package ai

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"
)

// OutputSchemaName is the name of the strict structured-output schema. Every
// provider reuses it, so a response is comparable across providers.
const OutputSchemaName = "training_analysis"

// Bounds of the shared output contract.
const (
	// MaxSummaryLength is the maximum length of the summary, in characters.
	MaxSummaryLength = 1200
	// MaxObservationLength is the maximum length of one observation or
	// caution, in characters.
	MaxObservationLength = 500
	// MaxObservations is the maximum number of observations.
	MaxObservations = 10
	// MaxCautions is the maximum number of cautions.
	MaxCautions = 10
)

// Output is the analysis result every provider must return.
//
// It is interpretive only: a summary, some observations and some cautions.
// There is no field for metrics, so a provider cannot report a number it was
// never given, and there is no field for a diagnosis.
type Output struct {
	Summary      string   `json:"summary"`
	Observations []string `json:"observations"`
	Cautions     []string `json:"cautions"`
}

// Schema is the strict structured-output definition a provider receives.
//
// Name and Strict are the fields strict structured-output APIs expect
// (name, strict, schema); Schema is the JSON Schema document itself.
type Schema struct {
	Name   string          `json:"name"`
	Strict bool            `json:"strict"`
	Schema json.RawMessage `json:"schema"`
}

// outputJSONSchema is the JSON Schema (draft 2020-12) shared by every
// provider. It mirrors the Output type field by field and forbids any extra
// property.
const outputJSONSchema = `{
  "type": "object",
  "additionalProperties": false,
  "required": ["summary", "observations", "cautions"],
  "properties": {
    "summary": {
      "type": "string",
      "minLength": 1,
      "maxLength": 1200
    },
    "observations": {
      "type": "array",
      "maxItems": 10,
      "items": {
        "type": "string",
        "minLength": 1,
        "maxLength": 500
      }
    },
    "cautions": {
      "type": "array",
      "maxItems": 10,
      "items": {
        "type": "string",
        "minLength": 1,
        "maxLength": 500
      }
    }
  }
}`

// OutputSchema returns the strict schema every provider must reuse.
func OutputSchema() Schema {
	return Schema{
		Name:   OutputSchemaName,
		Strict: true,
		Schema: json.RawMessage(outputJSONSchema),
	}
}

// outputWire keeps the raw JSON of every field so missing, null and wrongly
// typed values can be told apart.
type outputWire struct {
	Summary      json.RawMessage `json:"summary"`
	Observations json.RawMessage `json:"observations"`
	Cautions     json.RawMessage `json:"cautions"`
}

// ValidateOutput validates a raw provider response against the shared output
// contract.
//
// sent is the payload that was handed to the provider: it bounds the numbers
// the answer may quote, so a metric that was never sent cannot be invented.
func ValidateOutput(raw []byte, sent Payload) error {
	_, err := ParseOutput(raw, sent)
	return err
}

// ParseOutput decodes and validates a raw provider response.
//
// It applies the strict schema (required fields, no unknown fields, no nulls,
// no trailing data), the length bounds, and the interpretation rules: no
// invented metric and no diagnosis.
func ParseOutput(raw []byte, sent Payload) (Output, error) {
	wire, err := decodeOutput(raw)
	if err != nil {
		return Output{}, err
	}

	summary, err := decodeString(wire.Summary, "summary")
	if err != nil {
		return Output{}, err
	}
	observations, err := decodeStringList(wire.Observations, "observations")
	if err != nil {
		return Output{}, err
	}
	cautions, err := decodeStringList(wire.Cautions, "cautions")
	if err != nil {
		return Output{}, err
	}

	if err := checkSummary(summary); err != nil {
		return Output{}, err
	}
	if err := checkList(observations, "observations", MaxObservations); err != nil {
		return Output{}, err
	}
	if err := checkList(cautions, "cautions", MaxCautions); err != nil {
		return Output{}, err
	}

	out := Output{
		Summary:      summary,
		Observations: observations,
		Cautions:     cautions,
	}
	if err := checkInterpretation(out, sent); err != nil {
		return Output{}, err
	}
	return out, nil
}

// decodeOutput decodes the strict shape of the response.
func decodeOutput(raw []byte) (outputWire, error) {
	if isJSONNull(bytes.TrimSpace(raw)) {
		return outputWire{}, fmt.Errorf("ai: invalid analysis output: response must not be null")
	}

	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()

	var wire outputWire
	if err := decoder.Decode(&wire); err != nil {
		return outputWire{}, fmt.Errorf("ai: invalid analysis output: %w", err)
	}

	var extra json.RawMessage
	if err := decoder.Decode(&extra); err != io.EOF {
		return outputWire{}, fmt.Errorf("ai: invalid analysis output: unexpected trailing data")
	}
	return wire, nil
}

// decodeString reads a required string field.
func decodeString(raw json.RawMessage, field string) (string, error) {
	if len(raw) == 0 {
		return "", fmt.Errorf("ai: invalid analysis output: missing required field %q", field)
	}
	if isJSONNull(raw) {
		return "", fmt.Errorf("ai: invalid analysis output: field %q must not be null", field)
	}
	var value string
	if err := json.Unmarshal(raw, &value); err != nil {
		return "", fmt.Errorf("ai: invalid analysis output: field %q must be a string", field)
	}
	return value, nil
}

// decodeStringList reads a required array of strings field.
func decodeStringList(raw json.RawMessage, field string) ([]string, error) {
	if len(raw) == 0 {
		return nil, fmt.Errorf("ai: invalid analysis output: missing required field %q", field)
	}
	if isJSONNull(raw) {
		return nil, fmt.Errorf("ai: invalid analysis output: field %q must not be null", field)
	}
	var values []string
	if err := json.Unmarshal(raw, &values); err != nil {
		return nil, fmt.Errorf("ai: invalid analysis output: field %q must be an array of strings", field)
	}
	if values == nil {
		values = []string{}
	}
	return values, nil
}

// isJSONNull reports whether the raw value is the JSON null literal.
func isJSONNull(raw []byte) bool {
	return string(bytes.TrimSpace(raw)) == "null"
}

// checkSummary validates the summary length and that it says something.
func checkSummary(summary string) error {
	if strings.TrimSpace(summary) == "" {
		return fmt.Errorf("ai: invalid analysis output: field %q must not be empty", "summary")
	}
	if utf8.RuneCountInString(summary) > MaxSummaryLength {
		return fmt.Errorf("ai: invalid analysis output: field %q exceeds %d characters", "summary", MaxSummaryLength)
	}
	return nil
}

// checkList validates the number of items and the length of each one.
func checkList(values []string, field string, maxItems int) error {
	if len(values) > maxItems {
		return fmt.Errorf("ai: invalid analysis output: field %q exceeds %d items", field, maxItems)
	}
	for i, value := range values {
		if strings.TrimSpace(value) == "" {
			return fmt.Errorf("ai: invalid analysis output: field %q item %d must not be empty", field, i)
		}
		if utf8.RuneCountInString(value) > MaxObservationLength {
			return fmt.Errorf("ai: invalid analysis output: field %q item %d exceeds %d characters", field, i, MaxObservationLength)
		}
	}
	return nil
}

// numberPattern matches a numeric literal inside free text.
var numberPattern = regexp.MustCompile(`-?\d+(?:[.,]\d+)*`)

// diagnosisTerms are the medical claims the analysis must not make: the
// product interprets training, it does not diagnose.
var diagnosisTerms = []string{
	"arrhythmia",
	"arritmia",
	"arrítmia",
	"cardiopatia",
	"cardiopatía",
	"cardiomyopathy",
	"diagnosis",
	"diagnostic",
	"diagnostico",
	"diagnóstico",
	"diabetes",
	"disease",
	"enfermedad",
	"heart attack",
	"hipertension",
	"hipertensión",
	"infarto",
	"injury",
	"ischemia",
	"isquemia",
	"lesion",
	"lesión",
	"medication",
	"medicamento",
	"patologia",
	"patología",
	"prescription",
	"sindrome",
	"síndrome",
	"syndrome",
}

// checkInterpretation applies the rules that go beyond the JSON shape: the
// answer may only mention numbers it was given and may not diagnose.
func checkInterpretation(out Output, sent Payload) error {
	allowed := allowedNumbers(sent)

	texts := make([]string, 0, 1+len(out.Observations)+len(out.Cautions))
	texts = append(texts, out.Summary)
	texts = append(texts, out.Observations...)
	texts = append(texts, out.Cautions...)

	for _, text := range texts {
		lower := strings.ToLower(text)
		for _, term := range diagnosisTerms {
			if strings.Contains(lower, term) {
				return fmt.Errorf("ai: invalid analysis output: diagnosis claim %q is not allowed", term)
			}
		}
		for _, literal := range numberPattern.FindAllString(text, -1) {
			value, ok := parseNumber(literal)
			if !ok {
				continue
			}
			if _, known := allowed[value]; !known {
				return fmt.Errorf("ai: invalid analysis output: number %q was never sent to the provider", literal)
			}
		}
	}
	return nil
}

// allowedNumbers collects the numbers the provider was given.
func allowedNumbers(sent Payload) map[float64]struct{} {
	allowed := map[float64]struct{}{}
	for _, value := range []*float64{
		sent.Metrics.DistanceMeters,
		sent.Metrics.DurationSeconds,
		sent.Metrics.ElevationGainMeters,
		sent.Metrics.AvgHeartRate,
		sent.Metrics.TrainingLoad,
	} {
		if value != nil {
			allowed[*value] = struct{}{}
		}
	}
	if sent.WindowDays > 0 {
		allowed[float64(sent.WindowDays)] = struct{}{}
	}
	return allowed
}

// parseNumber reads a numeric literal written in text. It understands the
// decimal comma and the thousand separator used in Spanish.
func parseNumber(literal string) (float64, bool) {
	normalized := literal
	if strings.Contains(normalized, ",") && strings.Contains(normalized, ".") {
		normalized = strings.ReplaceAll(normalized, ",", "")
	} else if strings.Contains(normalized, ",") {
		normalized = strings.ReplaceAll(normalized, ",", ".")
	}
	value, err := strconv.ParseFloat(normalized, 64)
	if err != nil {
		return 0, false
	}
	return value, true
}
