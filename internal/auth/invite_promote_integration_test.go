//go:build integration

// Integration tests for the transactional invite promoter (issue #168, A1).
// These run against a real PostgreSQL via TEST_DATABASE_URL (or
// DATABASE_URL as fallback) and verify that:
//
//   - MarkInviteAccepted + UpdateUserInviteStatus commit atomically.
//   - When UpdateUserInviteStatus fails (e.g. invalid user UUID), the
//     invite is NOT marked accepted (rollback works).
//
// Run with: go test -tags integration -race ./internal/auth/...
package auth

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/fgjcarlos/ghamusinos/internal/db/sqlc"
	"github.com/fgjcarlos/ghamusinos/internal/db/status"
)

// openTestDB is mirrored after internal/db/sqlc/queries_integration_test.go
// so the auth package can stand on its own when running integration tests.
func openTestDB(t *testing.T) *pgxpool.Pool {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		dsn = os.Getenv("DATABASE_URL")
	}
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL (o DATABASE_URL) no definida; saltando integration test")
	}
	pool, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		t.Fatalf("pgxpool.New: %v", err)
	}
	t.Cleanup(pool.Close)
	return pool
}

func pgtypeUUID(t *testing.T, s string) pgtype.UUID {
	t.Helper()
	var u pgtype.UUID
	if err := u.Scan(s); err != nil {
		t.Fatalf("pgtypeUUID.Scan(%q): %v", s, err)
	}
	return u
}

// TestPGXInvitePromoter_Success drives the promoter against a real DB:
// creates a user + invite, promotes them, and asserts both rows updated.
func TestPGXInvitePromoter_Success(t *testing.T) {
	pool := openTestDB(t)
	q := sqlc.New(pool)
	ctx := context.Background()

	// Seed user with a unique email so the test does not collide with
	// existing rows. Cleanup is best-effort: t.Cleanup removes the rows
	// we created.
	uniqueEmail := "promote-success-" + time.Now().Format("20060102T150405.000") + "@example.test"
	clerkID := "clerk_promote_success_" + time.Now().Format("150405.000000000")

	user, err := q.CreateUser(ctx, sqlc.CreateUserParams{
		ClerkUserID:  clerkID,
		Email:        uniqueEmail,
		InviteStatus: status.InviteStatusPending,
	})
	if err != nil {
		t.Fatalf("seed CreateUser: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM invites WHERE email = $1`, uniqueEmail)
		_, _ = pool.Exec(context.Background(), `DELETE FROM users WHERE id = $1`, user.ID)
	})

	tokenHash := "hash-promote-success-" + time.Now().Format("150405.000000000")
	invite, err := q.CreateInvite(ctx, sqlc.CreateInviteParams{
		Email:     uniqueEmail,
		TokenHash: tokenHash,
		Status:    status.StatusPending,
		ExpiresAt: pgtype.Timestamptz{Time: time.Now().Add(24 * time.Hour), Valid: true},
	})
	if err != nil {
		t.Fatalf("seed CreateInvite: %v", err)
	}

	promoter := NewPGXInvitePromoter(pool)
	updated, err := promoter.PromotePendingInvite(ctx, invite.ID, user.ID)
	if err != nil {
		t.Fatalf("PromotePendingInvite: %v", err)
	}
	if updated.InviteStatus != status.InviteStatusActive {
		t.Errorf("returned user invite_status = %v, want active", updated.InviteStatus)
	}

	// Re-fetch invite: must be accepted.
	gotInvite, err := q.GetInviteByTokenHash(ctx, tokenHash)
	if err != nil {
		t.Fatalf("GetInviteByTokenHash: %v", err)
	}
	if gotInvite.Status != status.StatusAccepted {
		t.Errorf("invite status = %v, want accepted", gotInvite.Status)
	}

	// Re-fetch user: must be active.
	gotUser, err := q.GetUserByClerkID(ctx, clerkID)
	if err != nil {
		t.Fatalf("GetUserByClerkID: %v", err)
	}
	if gotUser.InviteStatus != status.InviteStatusActive {
		t.Errorf("user invite_status = %v, want active", gotUser.InviteStatus)
	}
}

// TestPGXInvitePromoter_RollsBackOnUpdateFailure forces the second UPDATE
// to fail by passing an invalid user UUID. The transaction must roll back,
// so the invite stays pending. This is the AC from issue #168:
// "MarkInviteAccepted y UpdateUserInviteStatus van en la misma
// transacción; si el segundo falla, la invitación sigue pending".
func TestPGXInvitePromoter_RollsBackOnUpdateFailure(t *testing.T) {
	pool := openTestDB(t)
	q := sqlc.New(pool)
	ctx := context.Background()

	uniqueEmail := "promote-rollback-" + time.Now().Format("20060102T150405.000") + "@example.test"
	clerkID := "clerk_promote_rollback_" + time.Now().Format("150405.000000000")

	user, err := q.CreateUser(ctx, sqlc.CreateUserParams{
		ClerkUserID:  clerkID,
		Email:        uniqueEmail,
		InviteStatus: status.InviteStatusPending,
	})
	if err != nil {
		t.Fatalf("seed CreateUser: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM invites WHERE email = $1`, uniqueEmail)
		_, _ = pool.Exec(context.Background(), `DELETE FROM users WHERE id = $1`, user.ID)
	})

	tokenHash := "hash-promote-rollback-" + time.Now().Format("150405.000000000")
	invite, err := q.CreateInvite(ctx, sqlc.CreateInviteParams{
		Email:     uniqueEmail,
		TokenHash: tokenHash,
		Status:    status.StatusPending,
		ExpiresAt: pgtype.Timestamptz{Time: time.Now().Add(24 * time.Hour), Valid: true},
	})
	if err != nil {
		t.Fatalf("seed CreateInvite: %v", err)
	}

	// An all-zero UUID with Valid:false makes the UPDATE fail because
	// no row matches WHERE id = $1. PostgreSQL returns 0 rows; sqlc
	// surfaces ErrNoRows from QueryRow().Scan.
	var bogus pgtype.UUID
	bogus.Valid = false

	promoter := NewPGXInvitePromoter(pool)
	_, err = promoter.PromotePendingInvite(ctx, invite.ID, bogus)
	if err == nil {
		t.Fatal("expected error from PromotePendingInvite, got nil")
	}

	// Invite must still be pending after the rollback.
	gotInvite, err := q.GetInviteByTokenHash(ctx, tokenHash)
	if err != nil {
		t.Fatalf("GetInviteByTokenHash: %v", err)
	}
	if gotInvite.Status != status.StatusPending {
		t.Errorf("invite status after failed promote = %v, want pending (rollback failed)", gotInvite.Status)
	}

	// Original user must still be pending too.
	gotUser, err := q.GetUserByClerkID(ctx, clerkID)
	if err != nil {
		t.Fatalf("GetUserByClerkID: %v", err)
	}
	if gotUser.InviteStatus != status.InviteStatusPending {
		t.Errorf("user invite_status after failed promote = %v, want pending", gotUser.InviteStatus)
	}
}
