package handlers

import (
	"encoding/json"
	"net/http/httptest"
	"testing"
)

// TestNewUnauthorized creates a ProblemDetail with 401 status.
// SPEC: Scenario "NewUnauthorized constructs correctly"
func TestNewUnauthorized(t *testing.T) {
	p := NewUnauthorized("missing token", "req-1")

	if p.Status != 401 {
		t.Errorf("expected Status=401, got %d", p.Status)
	}
	if p.Title != "Unauthorized" {
		t.Errorf("expected Title='Unauthorized', got %q", p.Title)
	}
	if p.Type != "about:blank" {
		t.Errorf("expected Type='about:blank', got %q", p.Type)
	}
	if p.Instance != "req-1" {
		t.Errorf("expected Instance='req-1', got %q", p.Instance)
	}
	if p.Detail != "missing token" {
		t.Errorf("expected Detail='missing token', got %q", p.Detail)
	}
}

// TestNewNotFound creates a ProblemDetail with 404 status.
// SPEC: Scenario "NewNotFound constructs correctly"
func TestNewNotFound(t *testing.T) {
	p := NewNotFound("resource not found", "req-2")

	if p.Status != 404 {
		t.Errorf("expected Status=404, got %d", p.Status)
	}
	if p.Title != "Not Found" {
		t.Errorf("expected Title='Not Found', got %q", p.Title)
	}
	if p.Type != "about:blank" {
		t.Errorf("expected Type='about:blank', got %q", p.Type)
	}
}

// TestNewForbidden creates a ProblemDetail with 403 status.
// SPEC: Scenario "NewForbidden constructs correctly"
func TestNewForbidden(t *testing.T) {
	p := NewForbidden("insufficient permissions", "req-3")

	if p.Status != 403 {
		t.Errorf("expected Status=403, got %d", p.Status)
	}
	if p.Title != "Forbidden" {
		t.Errorf("expected Title='Forbidden', got %q", p.Title)
	}
}

// TestNewBadRequest creates a ProblemDetail with 400 status.
// SPEC: Scenario "NewBadRequest constructs correctly"
func TestNewBadRequest(t *testing.T) {
	p := NewBadRequest("invalid input", "req-4")

	if p.Status != 400 {
		t.Errorf("expected Status=400, got %d", p.Status)
	}
	if p.Title != "Bad Request" {
		t.Errorf("expected Title='Bad Request', got %q", p.Title)
	}
}

// TestNewInternalError creates a ProblemDetail with 500 status.
// SPEC: Scenario "NewInternalError constructs correctly"
func TestNewInternalError(t *testing.T) {
	p := NewInternalError("database error", "req-5")

	if p.Status != 500 {
		t.Errorf("expected Status=500, got %d", p.Status)
	}
	if p.Title != "Internal Server Error" {
		t.Errorf("expected Title='Internal Server Error', got %q", p.Title)
	}
}

// TestWriteProblem outputs RFC 9457 format with correct status and content-type.
// SPEC: Scenario "WriteProblem serializes to wire"
func TestWriteProblem(t *testing.T) {
	rec := httptest.NewRecorder()
	p := NewUnauthorized("missing token", "req-1")

	WriteProblem(rec, p)

	// Check status code
	if rec.Code != 401 {
		t.Errorf("expected status 401, got %d", rec.Code)
	}

	// Check Content-Type header
	contentType := rec.Header().Get("Content-Type")
	if contentType != "application/problem+json" {
		t.Errorf("expected Content-Type 'application/problem+json', got %q", contentType)
	}

	// Check JSON body
	var body ProblemDetail
	//nolint:errcheck
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("failed to unmarshal response body: %v", err)
	}

	if body.Status != 401 || body.Title != "Unauthorized" || body.Instance != "req-1" {
		t.Errorf("response body doesn't match: %+v", body)
	}
}

// TestWriteProblem_404 tests 404 with correct structure.
// SPEC: Scenario "Not found returns problem+json"
func TestWriteProblem_404(t *testing.T) {
	rec := httptest.NewRecorder()
	p := NewNotFound("endpoint not found", "req-6")

	WriteProblem(rec, p)

	if rec.Code != 404 {
		t.Errorf("expected status 404, got %d", rec.Code)
	}

	contentType := rec.Header().Get("Content-Type")
	if contentType != "application/problem+json" {
		t.Errorf("expected Content-Type 'application/problem+json', got %q", contentType)
	}

	var body ProblemDetail
	//nolint:errcheck
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("failed to unmarshal response body: %v", err)
	}

	if body.Status != 404 {
		t.Errorf("expected status 404 in body, got %d", body.Status)
	}
}

// TestWriteProblem_EmptyInstance tests that instance can be empty string.
// SPEC: Scenario "instance field may be empty string"
func TestWriteProblem_EmptyInstance(t *testing.T) {
	rec := httptest.NewRecorder()
	p := ProblemDetail{
		Type:     "about:blank",
		Title:    "Test Error",
		Status:   400,
		Detail:   "test detail",
		Instance: "",
	}

	WriteProblem(rec, p)

	var body ProblemDetail
	//nolint:errcheck
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("failed to unmarshal response body: %v", err)
	}

	if body.Instance != "" {
		t.Errorf("expected empty Instance, got %q", body.Instance)
	}
}

// TestProblemDetail_OmitEmptyDetail tests that Detail is omitted from JSON when empty.
// SPEC: Scenario "omitempty fields"
func TestProblemDetail_OmitEmptyDetail(t *testing.T) {
	rec := httptest.NewRecorder()
	p := ProblemDetail{
		Type:     "about:blank",
		Title:    "Not Found",
		Status:   404,
		Detail:   "",
		Instance: "",
	}

	WriteProblem(rec, p)

	var body map[string]interface{}
	//nolint:errcheck
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("failed to unmarshal response body: %v", err)
	}

	// detail and instance should not be in JSON when empty
	if _, hasDetail := body["detail"]; hasDetail {
		t.Error("expected 'detail' to be omitted from JSON when empty")
	}
}
