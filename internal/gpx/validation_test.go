package gpx

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestValidatorAcceptsValidTrack(t *testing.T) {
	err := (Validator{}).Validate(2048, &Track{Points: []Point{{}, {}}})
	require.NoError(t, err)
}

func TestValidatorRejectsLargeFile(t *testing.T) {
	err := (Validator{}).Validate(10*1024*1024+1, &Track{Points: []Point{{}, {}}})
	require.ErrorContains(t, err, "10 MB")
}

func TestValidatorRejectsSmallFile(t *testing.T) {
	err := (Validator{}).Validate(1023, &Track{Points: []Point{{}, {}}})
	require.ErrorContains(t, err, "1 KB")
}

func TestValidatorRejectsNilTrack(t *testing.T) {
	err := (Validator{}).Validate(2048, nil)
	require.ErrorContains(t, err, "track is required")
}

func TestValidatorRejectsTooFewPoints(t *testing.T) {
	err := (Validator{}).Validate(2048, &Track{Points: []Point{{}}})
	require.ErrorContains(t, err, "at least 2 trackpoints")
}
