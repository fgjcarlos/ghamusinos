package handlers

import (
	"encoding/json"
	"net/http"
)

// ProblemDetail represents an RFC 9457 Problem Details response.
type ProblemDetail struct {
	Type     string `json:"type"`
	Title    string `json:"title"`
	Status   int    `json:"status"`
	Detail   string `json:"detail,omitempty"`
	Instance string `json:"instance,omitempty"`
}

// NewUnauthorized creates a ProblemDetail for 401 Unauthorized.
func NewUnauthorized(detail, instance string) ProblemDetail {
	return ProblemDetail{
		Type:     "about:blank",
		Title:    "Unauthorized",
		Status:   401,
		Detail:   detail,
		Instance: instance,
	}
}

// NewForbidden creates a ProblemDetail for 403 Forbidden.
func NewForbidden(detail, instance string) ProblemDetail {
	return ProblemDetail{
		Type:     "about:blank",
		Title:    "Forbidden",
		Status:   403,
		Detail:   detail,
		Instance: instance,
	}
}

// NewNotFound creates a ProblemDetail for 404 Not Found.
func NewNotFound(detail, instance string) ProblemDetail {
	return ProblemDetail{
		Type:     "about:blank",
		Title:    "Not Found",
		Status:   404,
		Detail:   detail,
		Instance: instance,
	}
}

// NewBadRequest creates a ProblemDetail for 400 Bad Request.
func NewBadRequest(detail, instance string) ProblemDetail {
	return ProblemDetail{
		Type:     "about:blank",
		Title:    "Bad Request",
		Status:   400,
		Detail:   detail,
		Instance: instance,
	}
}

// NewInternalError creates a ProblemDetail for 500 Internal Server Error.
func NewInternalError(detail, instance string) ProblemDetail {
	return ProblemDetail{
		Type:     "about:blank",
		Title:    "Internal Server Error",
		Status:   500,
		Detail:   detail,
		Instance: instance,
	}
}

// WriteProblem writes a ProblemDetail as RFC 9457 JSON to the response writer.
func WriteProblem(w http.ResponseWriter, p ProblemDetail) {
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(p.Status)
	//nolint:errcheck

	json.NewEncoder(w).Encode(p)
}
