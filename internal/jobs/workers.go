package jobs

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"

	"github.com/fgjcarlos/ghamusinos/internal/config"
	"github.com/fgjcarlos/ghamusinos/internal/db/sqlc"
	"github.com/fgjcarlos/ghamusinos/internal/strava"
)

// Global dependencies for River workers (set at initialization)
var (
	globalPool      *pgxpool.Pool
	globalConfig    *config.Config
	globalClient    *strava.Client
	globalCipherKey []byte
)

// ErrTokenRefresherNotConfigured is returned when RefreshStravaTokenWorker is used without configuration.
var ErrTokenRefresherNotConfigured = errors.New("jobs: token refresher not configured; call ConfigureTokenRefresher first")

// ConfigureTokenRefresher sets up the global dependencies for RefreshStravaTokenWorker.
// This should be called once during application initialization.
func ConfigureTokenRefresher(pool *pgxpool.Pool, cfg *config.Config, client *strava.Client, cipherKey []byte) {
	globalPool = pool
	globalConfig = cfg
	globalClient = client
	globalCipherKey = cipherKey
}

// GetTokenRefresher returns the configured token refresher (Strava client).
func GetTokenRefresher() TokenRefresher {
	return globalClient
}

// GetTokenQuerier returns the configured token querier (sqlc.Queries).
func GetTokenQuerier() TokenQuerier {
	if globalPool == nil {
		return nil
	}
	return sqlc.New(globalPool)
}

// GetCipherKey returns the configured cipher key for token encryption/decryption.
func GetCipherKey() []byte {
	return globalCipherKey
}

// NewRiverWorkers creates a river.Workers instance with all registered job handlers.
func NewRiverWorkers() *river.Workers {
	workers := river.NewWorkers()
	river.AddWorker(workers, &ImportStravaWorker{})
	river.AddWorker(workers, &RefreshStravaTokenWorker{})
	river.AddWorker(workers, &IngestActivityEventWorker{})
	return workers
}

// StubJob is a minimal job type for testing River integration.
type StubJob struct {
	Message string
}

// Kind returns the job kind identifier for StubJob.
func (j StubJob) Kind() string {
	return string(KindStub)
}

// Work is the job handler for StubJob.
func (j StubJob) Work(ctx context.Context) error {
	// Stub job does nothing; used for testing enqueue and execution.
	return nil
}

// StubWorker implements river.Worker[StubJob] for processing stub jobs.
type StubWorker struct {
	river.WorkerDefaults[StubJob]
}

// Work processes a StubJob.
func (w *StubWorker) Work(ctx context.Context, job *river.Job[StubJob]) error {
	// Stub job does nothing; used for testing enqueue and execution.
	return nil
}

// ImportStravaWorker handles importing Strava data for a user.
type ImportStravaWorker struct {
	river.WorkerDefaults[ImportStravaArgs]
}

// Work processes an ImportStrava job.
func (w *ImportStravaWorker) Work(ctx context.Context, job *river.Job[ImportStravaArgs]) error {
	// TODO: Implement Strava API integration
	// This worker will fetch activity data from Strava within the given window
	// and persist it to the database.
	return nil
}

// RefreshStravaTokenWorker handles refreshing a user's Strava OAuth token.
// Dependencies are retrieved from global configuration set at initialization time.
type RefreshStravaTokenWorker struct {
	river.WorkerDefaults[RefreshStravaTokenArgs]
}

// Work processes a RefreshStravaToken job.
// It refreshes the access token using the stored refresh token and the Strava API.
// If the token is valid for more than 5 minutes, no action is taken.
func (w *RefreshStravaTokenWorker) Work(ctx context.Context, job *river.Job[RefreshStravaTokenArgs]) error {
	// Get the configured dependencies
	refresher := GetTokenRefresher()
	querier := GetTokenQuerier()
	cipherKey := GetCipherKey()

	if refresher == nil || querier == nil || len(cipherKey) == 0 {
		// Dependencies not configured; this is a setup error
		return ErrTokenRefresherNotConfigured
	}

	// Parse UserID from string to pgtype.UUID
	var userID pgtype.UUID
	err := userID.Scan(job.Args.UserID)
	if err != nil {
		return err
	}

	// Get the valid token (this handles refresh internally if needed)
	_, err = GetValidToken(ctx, querier, cipherKey, refresher, userID)
	if err != nil {
		// Return the error to River for retry
		return err
	}

	return nil
}

// IngestActivityEventWorker handles ingesting activity events from Strava webhooks.
// This is a stub implementation for Slice 4; Slice 5a will implement the actual
// activity ingestion logic from the Strava API.
type IngestActivityEventWorker struct {
	river.WorkerDefaults[IngestActivityEventArgs]
}

// IngestActivityEventArgs are the arguments for processing an activity event.
type IngestActivityEventArgs struct {
	EventID string
}

// Kind returns the job kind identifier for IngestActivityEventArgs.
func (a IngestActivityEventArgs) Kind() string {
	return "ingest_activity_event"
}

// Work processes an IngestActivityEvent job.
// Stub implementation: just receives the event and returns nil.
// The real implementation in Slice 5a will fetch activity data from Strava
// and persist it to the activities table.
func (w *IngestActivityEventWorker) Work(ctx context.Context, job *river.Job[IngestActivityEventArgs]) error {
	// TODO: Implement activity ingestion from Strava API (Slice 5a)
	// This will:
	// 1. Load the activity event from the database
	// 2. Fetch the full activity data from Strava API
	// 3. Parse and validate the data
	// 4. Persist to the activities table
	// 5. Mark the event as processed
	return nil
}
