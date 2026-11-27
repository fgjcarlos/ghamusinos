//go:build integration

package app

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"
	"github.com/stretchr/testify/require"

	"github.com/fgjcarlos/ghamusinos/internal/db/sqlc"
	"github.com/fgjcarlos/ghamusinos/internal/db/status"
	"github.com/fgjcarlos/ghamusinos/internal/jobs"
	"github.com/fgjcarlos/ghamusinos/internal/reports"
)

// TestIntegration_GenerateReportTwiceLeavesOneWeekRow covers S4: the
// per-user job is idempotent, so a repeated tick leaves one snapshot per
// period instead of duplicating rows.
func TestIntegration_GenerateReportTwiceLeavesOneWeekRow(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		dsn = os.Getenv("DATABASE_URL")
	}
	if dsn == "" {
		t.Skip("integration requires a migrated test database")
	}
	ctx := t.Context()
	pool, err := pgxpool.New(ctx, dsn)
	require.NoError(t, err)
	t.Cleanup(pool.Close)
	q := sqlc.New(pool)

	key := uuid.NewString()
	user, err := q.CreateUser(ctx, sqlc.CreateUserParams{ClerkUserID: key, Email: key + "@example.test", InviteStatus: status.InviteStatusActive})
	require.NoError(t, err)
	t.Cleanup(func() {
		_, err := pool.Exec(context.Background(), "DELETE FROM users WHERE id = $1", user.ID)
		require.NoError(t, err)
	})

	now := time.Date(2026, 3, 4, 12, 0, 0, 0, time.UTC)
	worker := jobs.NewGenerateReportWorker(reports.NewStore(q), q, func() time.Time { return now })
	job := &river.Job[jobs.GenerateReportArgs]{Args: jobs.GenerateReportArgs{UserID: uuid.UUID(user.ID.Bytes).String()}}
	require.NoError(t, worker.Work(ctx, job))
	require.NoError(t, worker.Work(ctx, job))

	var rows, periods int
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT count(*), count(DISTINCT period_start) FROM report_snapshots WHERE user_id = $1 AND kind = 'week'`,
		user.ID).Scan(&rows, &periods))
	require.Equal(t, 1, rows)
	require.Equal(t, 1, periods)
}
