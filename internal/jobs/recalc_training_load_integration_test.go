//go:build integration

package jobs

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/fgjcarlos/ghamusinos/internal/db/sqlc"
)

// TestRecalcTrainingLoad_EndToEnd_AgainstRealTimescale verifies that the
// worker, run against a live Postgres+TimescaleDB, writes rows to
// training_load_daily for a user's activities. The test exercises the
// happy path with idempotency: two consecutive runs must NOT duplicate
// rows thanks to the ON CONFLICT (user_id, day) DO UPDATE clause.
//
// Run with: `go test -tags=integration ./internal/jobs/...`
//
// Required: a Postgres/Timescale instance reachable via TEST_DATABASE_URL
// (defaults to localhost:5432 dev settings). The test inserts and removes
// a unique user row to avoid polluting other tests.
func TestRecalcTrainingLoad_EndToEnd_AgainstRealTimescale(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		dsn = "postgres://ghamusinos:ghamusinos@localhost:5432/ghamusinos?sslmode=disable"
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Skipf("skipping: cannot connect to TEST_DATABASE_URL: %v", err)
	}
	defer pool.Close()

	queries := sqlc.New(pool)

	// Seed a user row + two activities.
	userID, cleanup := seedUserAndActivities(t, ctx, queries)
	defer cleanup()

	store := queries // *sqlc.Queries satisfies TrainingLoadStore subset.
	actLoader := NewSQLCActivityRangeAdapter(queries)
	usrLoader := NewSQLCUserPhysiologyAdapter(queries)

	w := NewRecalcTrainingLoadWorker(store, actLoader, usrLoader, nil, nil, WindowFromDashboardMetadata)

	// First run.
	args := RecalcTrainingLoadArgs{UserID: userID.String()}
	now := time.Now()
	if err := w.recomputeForUser(ctx, userID, args); err != nil {
		t.Fatalf("first run: %v", err)
	}

	// Verify rows were written.
	rows, err := queries.ListTrainingLoadRange(ctx, sqlc.ListTrainingLoadRangeParams{
		UserID: userID,
		Day:    pgtype.Date{Time: now.AddDate(-1, 0, 0), Valid: true},
		Day_2:  pgtype.Date{Time: now.AddDate(0, 0, 1), Valid: true},
	})
	if err != nil {
		t.Fatalf("ListTrainingLoadRange: %v", err)
	}
	if len(rows) < 1 {
		t.Fatalf("expected at least 1 row after first run, got %d", len(rows))
	}

	firstCount := len(rows)
	for _, r := range rows {
		if r.Tss <= 0 {
			t.Errorf("expected TSS > 0, got row %v with tss=%v", r, r.Tss)
		}
	}

	// Second run should not duplicate rows.
	if err := w.recomputeForUser(ctx, userID, args); err != nil {
		t.Fatalf("second run: %v", err)
	}
	rows2, err := queries.ListTrainingLoadRange(ctx, sqlc.ListTrainingLoadRangeParams{
		UserID: userID,
		Day:    pgtype.Date{Time: now.AddDate(-1, 0, 0), Valid: true},
		Day_2:  pgtype.Date{Time: now.AddDate(0, 0, 1), Valid: true},
	})
	if err != nil {
		t.Fatalf("ListTrainingLoadRange second: %v", err)
	}
	if len(rows2) != firstCount {
		t.Errorf("idempotency: expected %d rows after second run, got %d", firstCount, len(rows2))
	}
}
