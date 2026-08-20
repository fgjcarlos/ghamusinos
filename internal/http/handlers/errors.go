package handlers

import (
	"encoding/json"
	"errors"
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

// NewPayloadTooLarge creates a ProblemDetail for 413 Payload Too Large.
// Used by the body-size middleware (issue #26).
func NewPayloadTooLarge(detail, instance string) ProblemDetail {
	return ProblemDetail{
		Type:     "about:blank",
		Title:    "Payload Too Large",
		Status:   http.StatusRequestEntityTooLarge,
		Detail:   detail,
		Instance: instance,
	}
}

// WriteProblem writes a ProblemDetail as RFC 9457 JSON to the response writer.
func WriteProblem(w http.ResponseWriter, p ProblemDetail) {
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(p.Status)
	_ = json.NewEncoder(w).Encode(p)
}

// IsBodyLimitError devuelve true si err proviene de un http.MaxBytesReader
// (cuando un body excede el límite configurado por BodyLimit, issue #26).
// Útil para handlers que quieran comprobar el error de Read() y responder
// 413 con un ProblemDetail en lugar de propagar como 500.
func IsBodyLimitError(err error) bool {
	if err == nil {
		return false
	}
	var maxErr *http.MaxBytesError
	return errors.As(err, &maxErr)
}
