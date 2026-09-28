package handlers

import (
	"encoding/json"
	"errors"
	"net/http"
)

// ProblemDetail represents an RFC 9457 Problem Details response.
//
// RFC 9457 §3.1 allows extension members in problem details; we use
// the `errors` member to carry a per-field validation map alongside
// the flat `detail` string. The flat string is kept (with
// `omitempty`) so any consumer that already reads `detail` continues
// to work. See closes-159-strava-disconnect/exploration.md and
// proposal.md for the rationale.
type ProblemDetail struct {
	Type     string            `json:"type"`
	Title    string            `json:"title"`
	Status   int               `json:"status"`
	Detail   string            `json:"detail,omitempty"`
	Instance string            `json:"instance,omitempty"`
	Errors   map[string]string `json:"errors,omitempty"`
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

// NewUnprocessableEntity creates a ProblemDetail for 422 Unprocessable
// Entity. Returned by validation paths when the request body parses but
// the values don't satisfy the schema's CHECK constraints (issue #159).
//
// For single-message validation use this constructor. For per-field
// validation prefer NewUnprocessableEntityFields, which populates the
// `errors` extension member alongside `detail`.
func NewUnprocessableEntity(detail, instance string) ProblemDetail {
	return ProblemDetail{
		Type:     "about:blank",
		Title:    "Unprocessable Entity",
		Status:   http.StatusUnprocessableEntity,
		Detail:   detail,
		Instance: instance,
	}
}

// NewUnprocessableEntityFields creates a 422 ProblemDetail with a
// per-field error map (RFC 9457 §3.1 extension member). The flat
// `detail` string is set to a stable summary so consumers that only
// read `detail` still see something meaningful.
//
// `fields` must be non-empty; callers MUST check `len(fields) > 0`
// before calling this constructor to keep the JSON body honest (no
// empty `errors: {}`).
func NewUnprocessableEntityFields(detail, instance string, fields map[string]string) ProblemDetail {
	return ProblemDetail{
		Type:     "about:blank",
		Title:    "Unprocessable Entity",
		Status:   http.StatusUnprocessableEntity,
		Detail:   detail,
		Instance: instance,
		Errors:   fields,
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
