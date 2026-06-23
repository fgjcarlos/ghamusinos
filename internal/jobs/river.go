package jobs

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/riverdriver/riverpgxv5"
)

// NewClient creates a new River job queue client.
// It configures the job workers and uses the pgx v5 driver with the provided pool.
func NewClient(ctx context.Context, pool *pgxpool.Pool) (*river.Client[pgx.Tx], error) {
	// Create the pgx v5 driver
	driver := riverpgxv5.New(pool)

	// Configure the River client with our workers
	workers := NewRiverWorkers()
	client, err := river.NewClient[pgx.Tx](driver, &river.Config{
		Workers: workers,
	})
	if err != nil {
		return nil, err
	}

	return client, nil
}
