package auth

import (
	"context"
	"errors"

	"github.com/fgjcarlos/ghamusinos/internal/db/sqlc"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
)

// UserResolver resolves Claims to internal User records.
// Implements lazy user creation: if user doesn't exist, creates with invite_status=pending.
type UserResolver interface {
	// Resolve returns the internal user for the given Claims.
	// Creates the user lazily if not found (invite_status = pending).
	// Race-safe: handles unique constraint violation by re-fetching.
	Resolve(ctx context.Context, claims *Claims) (*sqlc.User, error)
}

// dbUserResolver implements UserResolver using a sqlc.Querier.
type dbUserResolver struct {
	q sqlc.Querier
}

// NewUserResolver creates a new user resolver backed by the provided querier.
func NewUserResolver(q sqlc.Querier) UserResolver {
	return &dbUserResolver{q: q}
}

// Resolve implements UserResolver.
func (r *dbUserResolver) Resolve(ctx context.Context, claims *Claims) (*sqlc.User, error) {
	// Try to fetch existing user
	user, err := r.q.GetUserByClerkID(ctx, claims.Subject)
	if err == nil {
		return &user, nil // Found
	}

	// User not found; attempt to create
	displayName := pgtype.Text{String: claims.Name, Valid: claims.Name != ""}
	params := sqlc.CreateUserParams{
		ClerkUserID:  claims.Subject,
		Email:        claims.Email,
		DisplayName:  displayName,
		InviteStatus: "pending",
	}

	user, err = r.q.CreateUser(ctx, params)
	if err == nil {
		return &user, nil // Created successfully
	}

	// Check if it was a unique constraint violation (race condition)
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) {
		// Not a PG error; return it as-is
		return nil, err
	}

	if pgErr.Code != "23505" { // UNIQUE_VIOLATION
		// Some other PG error
		return nil, err
	}

	// Distinguish the two UNIQUE constraints on users (issue #168, A1):
	//
	//   users_clerk_user_id_key — race condition we already handle: someone
	//     else inserted the same Clerk subject. Re-fetch wins.
	//
	//   idx_users_email — a different Clerk account (different sub, possibly
	//     social-login + email/password) is already bound to the same email.
	//     This is a real collision, not a race: returning ErrEmailTaken lets
	//     the middleware translate to a 409 with a meaningful message instead
	//     of an opaque 500.
	switch pgErr.ConstraintName {
	case "idx_users_email":
		return nil, ErrEmailTaken
	case "users_clerk_user_id_key":
		// Re-fetch the user we lost the race against.
		user, err = r.q.GetUserByClerkID(ctx, claims.Subject)
		if err != nil {
			// If re-fetch also fails, return the original create error
			return nil, err
		}
		return &user, nil
	default:
		// Unknown UNIQUE constraint — be conservative and surface the raw
		// error rather than silently treating it as a race.
		return nil, err
	}
}
