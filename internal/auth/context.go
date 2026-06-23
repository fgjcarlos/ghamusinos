package auth

import "context"

// Claims represents the JWT claims extracted from a Clerk token.
type Claims struct {
	Subject string // sub — Clerk user ID
	Email   string // email claim
	Name    string // name claim (may be empty)
}

// User represents an internal user record resolved from a Clerk identity.
type User struct {
	ID           string // internal UUID
	ClerkUserID  string // Clerk user ID from JWT sub claim
	Email        string // user email
	DisplayName  string // user display name
	InviteStatus string // 'pending', 'active', or 'blocked'
}

type contextKey string

const (
	authClaimsKey contextKey = "auth_claims"
	authUserKey   contextKey = "auth_user"
)

// WithAuthClaims injects authenticated claims into the context.
func WithAuthClaims(ctx context.Context, claims *Claims) context.Context {
	return context.WithValue(ctx, authClaimsKey, claims)
}

// AuthClaims retrieves authenticated claims from the context.
// Returns nil if no claims are present.
func AuthClaims(ctx context.Context) *Claims {
	claims, ok := ctx.Value(authClaimsKey).(*Claims)
	if !ok {
		return nil
	}
	return claims
}

// WithAuthUser injects an authenticated user into the context.
func WithAuthUser(ctx context.Context, user *User) context.Context {
	return context.WithValue(ctx, authUserKey, user)
}

// AuthUser retrieves an authenticated user from the context.
// Returns nil if no user is present.
func AuthUser(ctx context.Context) *User {
	user, ok := ctx.Value(authUserKey).(*User)
	if !ok {
		return nil
	}
	return user
}
