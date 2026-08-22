package auth

import (
	"context"
	"errors"
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
//
// issuer es obligatorio: validar sólo la firma con una clave del JWKS
// permite que cualquier token firmado por una clave de Clerk entre,
// aunque no sea del tenant. Si la verificación del emisor falla,
// Validate devuelve ErrUnauthenticated. Issue #167.
type clerkJWTValidator struct {
	cache    JWKSCache
	issuer   string // obligatorio
	audience string // opcional; si vacío, no se valida aud
}

// NewJWTValidator creates a new JWT validator.
//
// issuer es el emisor esperado (claim `iss`); audience es opcional.
func NewJWTValidator(cache JWKSCache, issuer, audience string) JWTValidator {
	return &clerkJWTValidator{
		cache:    cache,
		issuer:   issuer,
		audience: audience,
	}
}

// Validate parses and validates the raw JWT token.
func (v *clerkJWTValidator) Validate(ctx context.Context, rawToken string) (*Claims, error) {
	// Parse at the JWS level to get the protected headers (including kid).
	// jws.Parse devuelve un mensaje con cero firmas si el formato no es
	// un JWS compact válido (p.ej. un JWE, un JSON malformado). Antes
	// accedíamos a msg.Signatures()[0] sin comprobar, lo que producía un
	// panic alcanzable desde un cliente sin autenticar. Recoverer lo
	// cazaba, pero como 500 evitable. Issue #167.
	msg, err := jws.Parse([]byte(rawToken))
	if err != nil {
		return nil, ErrUnauthenticated
	}
	if len(msg.Signatures()) == 0 {
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

	// Parse and verify the token with the public key.
	// jwt.WithIssuer implementa ParseOption (ValidateOption lo satisface),
	// así que pasa a Parse sin envoltorio. La verificación de iss vive
	// dentro de jwx; si falla, jwt.Parse devuelve un error cuyo tipo
	// satisface errors.Is contra jwx centinelas (no texto). Issue #167.
	parseOpts := []jwt.ParseOption{
		jwt.WithKey(jwa.RS256, pubKey),
		jwt.WithIssuer(v.issuer),
	}
	if v.audience != "" {
		parseOpts = append(parseOpts, jwt.WithAudience(v.audience))
	}

	verifiedToken, err := jwt.Parse([]byte(rawToken), parseOpts...)
	if err != nil {
		// El mapeo por texto ("\"exp\" not satisfied") que había aquí
		// era frágil: se rompía con la siguiente subida de jwx. Ahora
		// distinguimos con errors.Is contra los centinelas de jwx, que
		// son inmutables y están documentados para comparación. Issue #167.
		switch {
		case errors.Is(err, jwt.ErrTokenExpired()),
			errors.Is(err, jwt.ErrTokenNotYetValid()):
			return nil, ErrExpiredToken
		case errors.Is(err, jwt.ErrInvalidAudience()):
			return nil, ErrMissingClaims
		default:
			return nil, ErrUnauthenticated
		}
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
