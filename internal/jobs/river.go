package jobs

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/riverdriver/riverpgxv5"
)

// NewClient creates a new River job queue client.
// It configures the job workers and uses the pgx v5 driver with the provided pool.
// Workers are registered via NewRiverWorkers(d Deps, registerStravaWorkers)
// which validates the dependency bundle and returns an error if anything
// required is missing — see AUD-04 (issue #164) for the constructor-injection
// rationale.
func NewClient(ctx context.Context, pool *pgxpool.Pool, d Deps, registerStravaWorkers bool) (*river.Client[pgx.Tx], error) {
	// Create the pgx v5 driver
	driver := riverpgxv5.New(pool)

	// Configure the River client with our workers
	workers, err := NewRiverWorkers(d, registerStravaWorkers)
	if err != nil {
		return nil, fmt.Errorf("jobs: register workers: %w", err)
	}
	client, err := river.NewClient[pgx.Tx](driver, &river.Config{
		Workers: workers,
	})
	if err != nil {
		return nil, err
	}

	return client, nil
}

// RiverEnqueuerAdapter satisfies strava.RiverEnqueuer by delegating
// EnqueueImportStrava to a River client.Insert call. The adapter is the
// production implementation that the OAuth callback uses to schedule the
// backfill after a successful token exchange.
//
// AUD-04 AC: "Conectar Strava encola un ImportStravaArgs".
type RiverEnqueuerAdapter struct {
	Client *river.Client[pgx.Tx]
}

// EnqueueImportStrava inserts an ImportStravaArgs job into the River client
// for the given userID. StravaID, WindowStart and WindowEnd are left zero —
// the import worker derives them from cfg.Strava.BackfillDays via BackfillWindow.
func (a *RiverEnqueuerAdapter) EnqueueImportStrava(ctx context.Context, userID string) error {
	if a.Client == nil {
		return fmt.Errorf("jobs: RiverEnqueuerAdapter.Client is nil")
	}
	_, err := a.Client.Insert(ctx, &ImportStravaArgs{UserID: userID}, nil)
	if err != nil {
		return fmt.Errorf("jobs: enqueue ImportStravaArgs: %w", err)
	}
	return nil
}
