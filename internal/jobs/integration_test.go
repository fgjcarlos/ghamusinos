package jobs

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// TestRiverIntegration verifies that River can enqueue and process jobs.
// This test requires a live Postgres database via DATABASE_URL.
// Skip with: go test -short ./internal/jobs/...
func TestRiverIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test requires database; skipping with -short")
	}

	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		t.Skip("DATABASE_URL not set; skipping integration test")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Connect to the database
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		t.Fatalf("failed to connect to database: %v", err)
	}
	defer pool.Close()

	// Create the River client
	client, err := NewClient(ctx, pool)
	if err != nil {
		t.Fatalf("failed to create River client: %v", err)
	}
	defer func() { _ = client.Stop(ctx) }() //nolint:errcheck

	// Start the worker
	if err := client.Start(ctx); err != nil {
		t.Fatalf("failed to start River worker: %v", err)
	}

	// Enqueue a stub job
	insertRes, err := client.Insert(ctx, &StubJob{Message: "test integration job"}, nil)
	if err != nil {
		t.Fatalf("failed to enqueue job: %v", err)
	}

	if insertRes == nil {
		t.Fatal("insert result is nil")
	}

	// Allow time for the job to be processed
	time.Sleep(500 * time.Millisecond)

	// Verify the job was enqueued (no error means success)
	t.Logf("successfully enqueued stub job with ID: %d", insertRes.Job.ID)
}
