package auth

import "errors"

var (
	// ErrUnauthenticated is returned when JWT validation fails.
	ErrUnauthenticated = errors.New("unauthorized")

	// ErrForbidden is returned when the user is not authorized for the requested resource.
	ErrForbidden = errors.New("forbidden")

	// ErrExpiredToken is returned when the JWT has expired or is not yet valid.
	ErrExpiredToken = errors.New("token expired")

	// ErrInvalidSignature is returned when the JWT signature verification fails.
	ErrInvalidSignature = errors.New("invalid signature")

	// ErrMissingClaims is returned when required JWT claims are missing or invalid.
	ErrMissingClaims = errors.New("missing required claims")

	// ErrNoActiveInvite is returned when a user has no valid invitation.
	ErrNoActiveInvite = errors.New("no active invite")

	// ErrEmailTaken is returned when a Clerk account tries to register
	// with an email that another account already owns (issue #168).
	// The middleware translates this to a 409 with a clear message
	// instead of an opaque 500.
	ErrEmailTaken = errors.New("email already in use by another account")
)
