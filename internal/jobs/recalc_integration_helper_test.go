//go:build integration

package jobs

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/fgjcarlos/ghamusinos/internal/db/sqlc"
	"github.com/fgjcarlos/ghamusinos/internal/db/status"
)

// seedUserAndActivities creates a fresh user with FTP and two cycling
// activities on consecutive days, so the recalc worker has data to
// process. It returns a UUID and a cleanup func that removes every row
// created in the test.
func seedUserAndActivities(t *testing.T, ctx context.Context, q *sqlc.Queries) (pgtype.UUID, func()) {
	t.Helper()

	// Use a clock-derived timestamp and a deterministic random suffix for
	// the clerk_user_id (unique constraint).
	suffix := time.Now().UTC().Format("20060102_150405.000000")
	clerkID := "itest_" + suffix
	uniqueEmail := "itest+" + suffix + "@example.com"

	// CreateInvite + UpdateUserInviteStatus pattern, mirroring
	// auth/invite_promote_integration_test.go. Without this the user row
	// violates users_invite_status_check (NOT NULL constraint).
	tokenHash := "hash-recalc-itest-" + suffix
	if _, err := q.CreateInvite(ctx, sqlc.CreateInviteParams{
		Email:     uniqueEmail,
		TokenHash: tokenHash,
		Status:    status.StatusPending,
		ExpiresAt: pgtype.Timestamptz{Time: time.Now().Add(24 * time.Hour), Valid: true},
	}); err != nil {
		t.Fatalf("seed CreateInvite: %v", err)
	}

	user, err := q.CreateUser(ctx, sqlc.CreateUserParams{
		ClerkUserID:  clerkID,
		Email:        uniqueEmail,
		InviteStatus: status.InviteStatusActive,
	})
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	if !user.ID.Valid {
		t.Fatalf("CreateUser returned invalid UUID")
	}

	if _, err := q.UpdateUserInviteStatus(ctx, sqlc.UpdateUserInviteStatusParams{
		ID:           user.ID,
		InviteStatus: status.InviteStatusActive,
	}); err != nil {
		t.Fatalf("UpdateUserInviteStatus: %v", err)
	}
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	if !user.ID.Valid {
		t.Fatalf("CreateUser returned invalid UUID")
	}

	// Set ftp = 250 so TSS cycling > 0.
	ftp := pgtype.Int2{Int16: 250, Valid: true}
	if _, err := q.UpdateUserPreferences(ctx, sqlc.UpdateUserPreferencesParams{
		ID:        user.ID,
		HrMax:     pgtype.Int2{Valid: false},
		Lthr:      pgtype.Int2{Valid: false},
		Ftp:       ftp,
		Level:     pgtype.Text{Valid: false},
		Timezone:  "UTC",
		AiEnabled: false,
	}); err != nil {
		t.Fatalf("UpdateUserPreferences: %v", err)
	}

	now := time.Now().UTC()
	start1 := time.Date(now.Year(), now.Month(), now.Day()-1, 8, 0, 0, 0, time.UTC)
	start2 := time.Date(now.Year(), now.Month(), now.Day()-2, 8, 0, 0, 0, time.UTC)

	// Two activities on distinct days. Use Strava external IDs.
	if _, err := q.UpsertActivity(ctx, sqlc.UpsertActivityParams{
		UserID:         user.ID,
		ExternalSource: "strava",
		ExternalID:     99000001,
		Name:           "ITest Ride 1",
		SportType:      "Ride",
		StartedAt:      pgtype.Timestamptz{Time: start1, Valid: true},
		ElapsedSeconds: 3600,
		MovingSeconds:  3600,
		DistanceMeters: pgtype.Numeric{Valid: true, Int: nil},
		ElevationGainM: pgtype.Numeric{Valid: true, Int: nil},
		AvgHr:          pgtype.Int2{Valid: false},
		AvgPower:       pgtype.Int2{Int16: 250, Valid: true},
		RawPayload:     []byte(`{}`),
	}); err != nil {
		t.Fatalf("UpsertActivity 1: %v", err)
	}
	if _, err := q.UpsertActivity(ctx, sqlc.UpsertActivityParams{
		UserID:         user.ID,
		ExternalSource: "strava",
		ExternalID:     99000002,
		Name:           "ITest Ride 2",
		SportType:      "Ride",
		StartedAt:      pgtype.Timestamptz{Time: start2, Valid: true},
		ElapsedSeconds: 3600,
		MovingSeconds:  3600,
		DistanceMeters: pgtype.Numeric{Valid: true, Int: nil},
		ElevationGainM: pgtype.Numeric{Valid: true, Int: nil},
		AvgHr:          pgtype.Int2{Valid: false},
		AvgPower:       pgtype.Int2{Int16: 250, Valid: true},
		RawPayload:     []byte(`{}`),
	}); err != nil {
		t.Fatalf("UpsertActivity 2: %v", err)
	}

	cleanup := func() {
		// Best-effort cleanup: the user row is what we created. Cascading
		// FKs handle activities / training_load_daily / dashboard_metadata.
		_, _ = poolExec(ctx, "DELETE FROM invites WHERE email = $1", uniqueEmail)
		_, _ = poolExec(ctx, "DELETE FROM users WHERE id = $1", user.ID)
	}
	return user.ID, cleanup
}
