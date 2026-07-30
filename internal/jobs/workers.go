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
// It fetches activities from Strava within the given time window and persists them to the database.
func (w *ImportStravaWorker) Work(ctx context.Context, job *river.Job[ImportStravaArgs]) error {
	// Get configured dependencies
	refresher := GetTokenRefresher()
	querier := GetTokenQuerier()
	cipherKey := GetCipherKey()

	if refresher == nil || querier == nil || len(cipherKey) == 0 {
		return ErrTokenRefresherNotConfigured
	}

	// Cast globalClient to ActivityFetcher (strava.Client implements it)
	fetcher, ok := refresher.(ActivityFetcher)
	if !ok {
		return errors.New("jobs: token refresher does not implement ActivityFetcher")
	}

	// Create a SyncSessionStore from querier (sqlc.Queries implements it)
	sessionStore, ok := querier.(SyncSessionStore)
	if !ok {
		return errors.New("jobs: token querier does not implement SyncSessionStore")
	}

	// Create an ActivityInserter from querier (sqlc.Queries implements it)
	activityStore, ok := querier.(ActivityInserter)
	if !ok {
		return errors.New("jobs: token querier does not implement ActivityInserter")
	}

	// Create implementation instance and execute
	impl := &importStravaWorkerImpl{
		fetcher:        fetcher,
		sessionStore:   sessionStore,
		activityStore:  activityStore,
		cipherKey:      cipherKey,
		tokenQuerier:   querier,
		tokenRefresher: refresher,
	}

	return impl.execute(ctx, job.Args.UserID, job.Args.WindowStart, job.Args.WindowEnd)
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
	UserID     string
	ActivityID int64
}

// Kind returns the job kind identifier for IngestActivityEventArgs.
func (a IngestActivityEventArgs) Kind() string {
	return "ingest_activity_event"
}

// Work processes an IngestActivityEvent job.
// It fetches the full activity data from Strava API and persists it to the database.
func (w *IngestActivityEventWorker) Work(ctx context.Context, job *river.Job[IngestActivityEventArgs]) error {
	// Get configured dependencies
	refresher := GetTokenRefresher()
	querier := GetTokenQuerier()
	cipherKey := GetCipherKey()

	if refresher == nil || querier == nil || len(cipherKey) == 0 {
		return ErrTokenRefresherNotConfigured
	}

	// Cast globalClient to ActivityFetcher (strava.Client implements it)
	fetcher, ok := refresher.(ActivityFetcher)
	if !ok {
		return errors.New("jobs: token refresher does not implement ActivityFetcher")
	}

	// Create an ActivityInserter from querier (sqlc.Queries implements it)
	activityStore, ok := querier.(ActivityInserter)
	if !ok {
		return errors.New("jobs: token querier does not implement ActivityInserter")
	}

	// Create implementation instance and execute
	impl := &ingestActivityEventWorkerImpl{
		fetcher:        fetcher,
		activityStore:  activityStore,
		cipherKey:      cipherKey,
		tokenQuerier:   querier,
		tokenRefresher: refresher,
	}

	return impl.execute(ctx, job.Args.UserID, job.Args.ActivityID)
}
