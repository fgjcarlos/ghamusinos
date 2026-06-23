package auth

import (
	"context"
	"fmt"

	"github.com/lestrrat-go/jwx/v2/jwa"
	"github.com/lestrrat-go/jwx/v2/jws"
	"github.com/lestrrat-go/jwx/v2/jwt"
)

// JWTValidator defines the interface for validating JWT tokens.
type JWTValidator interface {
	// Validate parses and validates the raw JWT string.
	// Returns parsed claims on success; typed sentinel error on failure.
	Validate(ctx context.Context, rawToken string) (*Claims, error)
}

// clerkJWTValidator implements JWTValidator using lestrrat-go/jwx.
type clerkJWTValidator struct {
	cache    JWKSCache
	audience string // optional; if empty, skip aud validation
}

// NewJWTValidator creates a new JWT validator with the given JWKS cache and audience.
func NewJWTValidator(cache JWKSCache, audience string) JWTValidator {
	return &clerkJWTValidator{
		cache:    cache,
		audience: audience,
	}
}

// Validate parses and validates the raw JWT token.
func (v *clerkJWTValidator) Validate(ctx context.Context, rawToken string) (*Claims, error) {
	// Parse at the JWS level to get the protected headers (including kid)
	msg, err := jws.Parse([]byte(rawToken))
	if err != nil {
		return nil, ErrUnauthenticated
	}

	// Get the kid from the protected headers
	headers := msg.Signatures()[0].ProtectedHeaders()
	kid, ok := headers.Get("kid")
	if !ok {
		kid = "default"
	}
	kidStr := fmt.Sprintf("%v", kid)

	// Get the public key for this kid
	pubKey, err := v.cache.GetKey(ctx, kidStr)
	if err != nil {
		return nil, ErrUnauthenticated
	}

	// Parse and verify the token with the public key
	parseOpts := []jwt.ParseOption{
		jwt.WithKey(jwa.RS256, pubKey),
	}
	if v.audience != "" {
		parseOpts = append(parseOpts, jwt.WithAudience(v.audience))
	}

	verifiedToken, err := jwt.Parse([]byte(rawToken), parseOpts...)
	if err != nil {
		// Map jwt errors to our sentinel errors
		if err.Error() == "exp not satisfied" || err.Error() == "nbf not satisfied" {
			return nil, ErrExpiredToken
		}
		if err.Error() == "aud not satisfied" {
			return nil, ErrMissingClaims
		}
		return nil, ErrUnauthenticated
	}

	// Extract claims
	subject, ok := verifiedToken.Get(jwt.SubjectKey)
	if !ok {
		return nil, ErrMissingClaims
	}
	subStr, ok := subject.(string)
	if !ok || subStr == "" {
		return nil, ErrMissingClaims
	}

	email, _ := verifiedToken.Get("email")
	emailStr, _ := email.(string)

	name, _ := verifiedToken.Get("name")
	nameStr, _ := name.(string)

	return &Claims{
		Subject: subStr,
		Email:   emailStr,
		Name:    nameStr,
	}, nil
}
