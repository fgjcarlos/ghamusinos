package auth

import (
	"errors"
	"testing"
)

func TestErrorSentinels(t *testing.T) {
	tests := []struct {
		name    string
		err     error
		message string
	}{
		{
			name:    "ErrUnauthenticated",
			err:     ErrUnauthenticated,
			message: "unauthorized",
		},
		{
			name:    "ErrForbidden",
			err:     ErrForbidden,
			message: "forbidden",
		},
		{
			name:    "ErrExpiredToken",
			err:     ErrExpiredToken,
			message: "token expired",
		},
		{
			name:    "ErrInvalidSignature",
			err:     ErrInvalidSignature,
			message: "invalid signature",
		},
		{
			name:    "ErrMissingClaims",
			err:     ErrMissingClaims,
			message: "missing required claims",
		},
		{
			name:    "ErrNoActiveInvite",
			err:     ErrNoActiveInvite,
			message: "no active invite",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Error should implement error interface
			if tt.err == nil || tt.Error() == "" {
				t.Fatalf("error sentinel %s does not implement error interface or has empty message", tt.name)
			}

			// Error message should be human-readable
			if tt.err.Error() != tt.message {
				t.Errorf("error message mismatch: got %q, want %q", tt.err.Error(), tt.message)
			}

			// errors.Is() should distinguish sentinels
			if !errors.Is(tt.err, tt.err) {
				t.Errorf("errors.Is(%s, %s) should be true", tt.name, tt.name)
			}

			// Ensure distinct sentinels don't match each other
			other := ErrUnauthenticated
			if tt.err != other {
				if errors.Is(tt.err, other) {
					t.Errorf("errors.Is(%s, ErrUnauthenticated) should be false for different sentinels", tt.name)
				}
			}
		})
	}
}

func TestErrorIsDistinct(t *testing.T) {
	// Verify that each sentinel error can be checked independently
	err1 := ErrUnauthenticated
	err2 := ErrExpiredToken

	if errors.Is(err1, err2) {
		t.Error("ErrUnauthenticated should not match ErrExpiredToken")
	}
	if errors.Is(err2, err1) {
		t.Error("ErrExpiredToken should not match ErrUnauthenticated")
	}
}
