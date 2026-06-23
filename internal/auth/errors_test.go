package auth

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
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
			if tt.err == nil || tt.err.Error() == "" {
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

// TestAuthErrorFormat_NoTokenLeak verifies that error responses do not leak JWT details
// SPEC: Requirement "JWT-ERROR-FORMAT" - error body must not contain raw JWT
func TestAuthErrorFormat_NoTokenLeak(t *testing.T) {
	// Create a minimal mock validator that returns a token-related error
	validator := &mockJWTValidator{
		onValidate: func(ctx context.Context, token string) (*Claims, error) {
			return nil, ErrUnauthenticated
		},
	}

	handler := AuthMiddleware(validator)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// Use a realistic but invalid JWT format
	badToken := "eyJhbGciOiJSUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiJ1c2VyXzEyMyJ9.invalid_signature_here"

	req := httptest.NewRequestWithContext(context.Background(), "GET", "/api/test", nil)
	req.Header.Set("Authorization", "Bearer "+badToken)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	// Verify response format
	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", w.Code)
	}

	if w.Header().Get("Content-Type") != "application/json" {
		t.Errorf("expected Content-Type=application/json, got %s", w.Header().Get("Content-Type"))
	}

	var resp map[string]string
	//nolint:errcheck
	//nolint:errcheck
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("response body is not valid JSON: %v", err)
	}

	if resp["error"] != "unauthorized" {
		t.Errorf("expected error='unauthorized', got %q", resp["error"])
	}

	// CRITICAL: Verify no token leakage
	responseStr := w.Body.String()

	// Check for token patterns
	if strings.Contains(responseStr, badToken) {
		t.Error("LEAK: full token found in response body")
	}

	// Check for JWT parts (should not appear in error message)
	if strings.Contains(responseStr, "eyJhbGciOi") {
		t.Error("LEAK: JWT header found in response body")
	}

	if strings.Contains(responseStr, "invalid_signature_here") {
		t.Error("LEAK: JWT signature found in response body")
	}

	// Only the "error" field and its value should be in the body
	if len(resp) != 1 || resp["error"] == "" {
		t.Errorf("response should contain only 'error' field, got: %v", resp)
	}
}
