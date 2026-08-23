package auth

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/fgjcarlos/ghamusinos/internal/db/sqlc"
	"github.com/fgjcarlos/ghamusinos/internal/db/status"
)

// InvitePromoter promotes a pending user to active after their invitation
// has been accepted. The two writes (MarkInviteAccepted on invites and
// UpdateUserInviteStatus on users) must be committed atomically: if the
// second fails, the invitation must not be marked as accepted, otherwise
// the user loses access the moment expires_at passes (issue #168, A1).
//
// The interface is intentionally narrow so the middleware can stay
// independent of pgxpool. Tests inject a fake promoter.
type InvitePromoter interface {
	// PromotePendingInvite marks the invite as accepted and flips the
	// user's invite_status to active in one transaction. Returns the
	// updated user record on success.
	PromotePendingInvite(ctx context.Context, inviteID, userID pgtype.UUID) (sqlc.User, error)
}

// pgxInvitePromoter is the production InvitePromoter backed by pgxpool.
type pgxInvitePromoter struct {
	pool *pgxpool.Pool
}

// NewPGXInvitePromoter builds a promoter that uses the given pool.
// Returns a non-nil promoter even if pool is nil; nil-pool promoters
// return an error on every call so misconfiguration fails closed.
func NewPGXInvitePromoter(pool *pgxpool.Pool) InvitePromoter {
	return &pgxInvitePromoter{pool: pool}
}

// PromotePendingInvite runs MarkInviteAccepted + UpdateUserInviteStatus
// inside a single transaction. If either write fails the invite is left
// pending and the user is left pending, preserving the audit invariant
// "promotion is durable and atomic".
func (p *pgxInvitePromoter) PromotePendingInvite(ctx context.Context, inviteID, userID pgtype.UUID) (sqlc.User, error) {
	if p == nil || p.pool == nil {
		return sqlc.User{}, errors.New("auth: invite promoter not initialized (nil pool)")
	}

	tx, err := p.pool.Begin(ctx)
	if err != nil {
		return sqlc.User{}, fmt.Errorf("auth: begin invite promote tx: %w", err)
	}
	defer func() {
		// Rollback is a no-op after Commit; we log only the unexpected path.
		if rollbackErr := tx.Rollback(ctx); rollbackErr != nil && !errors.Is(rollbackErr, pgx.ErrTxClosed) {
			slog.Warn("auth: rollback invite promote tx", "error", rollbackErr)
		}
	}()

	q := sqlc.New(tx)
	if err := q.MarkInviteAccepted(ctx, inviteID); err != nil {
		return sqlc.User{}, fmt.Errorf("auth: mark invite accepted: %w", err)
	}

	updated, err := q.UpdateUserInviteStatus(ctx, sqlc.UpdateUserInviteStatusParams{
		ID:           userID,
		InviteStatus: status.InviteStatusActive,
	})
	if err != nil {
		return sqlc.User{}, fmt.Errorf("auth: update user invite status: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return sqlc.User{}, fmt.Errorf("auth: commit invite promote tx: %w", err)
	}
	return updated, nil
}