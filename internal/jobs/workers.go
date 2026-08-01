package jobs

import (
	"context"
	"errors"
	"fmt"
	"math/big"

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
	fetcher  ActivityFetcher
	store    SyncSessionStore
	inserter ActivityInserter
	querier  TokenQuerier
	config   interface{} // *config.Config, injected at registration
}

// Work processes an ImportStrava job.
// It fetches activities from Strava within a backfill window and upserts them to the database.
func (w *ImportStravaWorker) Work(ctx context.Context, job *river.Job[ImportStravaArgs]) error {
	// Parse userID from job args
	var userID pgtype.UUID
	err := userID.Scan(job.Args.UserID)
	if err != nil {
		return fmt.Errorf("jobs: failed to parse userID: %w", err)
	}

	// Get dependencies: if not injected, use global singletons
	querier := w.querier
	if querier == nil {
		querier = GetTokenQuerier()
	}
	fetcher := w.fetcher
	if fetcher == nil {
		// In production, this would be the strava.Client; for now, tests use injected mocks
		return ErrTokenRefresherNotConfigured // Placeholder error if no fetcher
	}
	inserter := w.inserter
	if inserter == nil {
		return ErrTokenRefresherNotConfigured // Placeholder error if no inserter
	}
	store := w.store
	if store == nil {
		return ErrTokenRefresherNotConfigured // Placeholder error if no store
	}

	// Get the config (for backfill window)
	cfg, ok := w.config.(*config.Config)
	if !ok || cfg == nil {
		cfg = globalConfig
	}
	if cfg == nil {
		return ErrTokenRefresherNotConfigured // Placeholder error if no config
	}

	// Get valid access token (handles refresh internally if needed)
	cipherKey := GetCipherKey()
	refresher := GetTokenRefresher()
	accessToken, err := GetValidToken(ctx, querier, cipherKey, refresher, userID)
	if err != nil {
		return fmt.Errorf("jobs: failed to get valid token: %w", err)
	}

	// Get or create sync session
	syncSession, err := store.GetLatestSyncSession(ctx, userID)
	if err != nil {
		// Session doesn't exist; create one
		backfillDays := int32(cfg.Strava.BackfillDays)
		syncSession, err = store.CreateSyncSession(ctx, sqlc.CreateSyncSessionParams{
			UserID:     userID,
			WindowDays: backfillDays,
		})
		if err != nil {
			return fmt.Errorf("jobs: failed to create sync session: %w", err)
		}
	}

	// Calculate backfill window
	after, before := BackfillWindow(cfg.Strava.BackfillDays)

	// Fetch activities in pages and upsert each
	page := 1
	perPage := 50
	totalProcessed := int32(0)

	for {
		activities, err := fetcher.GetActivities(ctx, accessToken, after, before, page, perPage)
		if err != nil {
			return fmt.Errorf("jobs: failed to fetch activities (page %d): %w", page, err)
		}

		// If no activities on this page, we're done
		if len(activities) == 0 {
			break
		}

		// Upsert each activity
		for _, activity := range activities {
			// Convert strava.ActivitySummary to UpsertActivityParams
			params := sqlc.UpsertActivityParams{
				UserID:         userID,
				ExternalSource: "strava",
				ExternalID:     activity.ID,
				Name:           activity.Name,
				SportType:      activity.Type,
				StartedAt:      pgtype.Timestamptz{Time: activity.StartDate, Valid: true},
				ElapsedSeconds: int32(activity.ElapsedTime),
				MovingSeconds:  int32(activity.MovingTime),
				DistanceMeters: pgtype.Numeric{Int: big.NewInt(int64(activity.Distance * 1000)), Valid: true}, // Convert km to m
				ElevationGainM: pgtype.Numeric{Int: big.NewInt(int64(activity.TotalElevationGain)), Valid: true},
			}

			_, err := inserter.UpsertActivity(ctx, params)
			if err != nil {
				return fmt.Errorf("jobs: failed to upsert activity %d: %w", activity.ID, err)
			}
			totalProcessed++
		}

		// Update progress
		_, err = store.UpdateSyncSessionProgress(ctx, sqlc.UpdateSyncSessionProgressParams{
			ID:              syncSession.ID,
			TotalActivities: int32(len(activities)),
			Imported:        totalProcessed,
			Skipped:         0,
		})
		if err != nil {
			return fmt.Errorf("jobs: failed to update progress: %w", err)
		}

		// Move to next page
		page++
	}

	// Mark sync session as completed
	_, err = store.UpdateSyncSessionStatus(ctx, sqlc.UpdateSyncSessionStatusParams{
		ID:     syncSession.ID,
		Status: "completed",
		Error:  pgtype.Text{Valid: false},
	})
	if err != nil {
		return fmt.Errorf("jobs: failed to mark sync session completed: %w", err)
	}

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
// This is stub in Slice 4 (webhook enqueue), fully implemented in Slice 5a.
type IngestActivityEventWorker struct {
	river.WorkerDefaults[IngestActivityEventArgs]
	eventLoader   ActivityEventLoader
	detailFetcher ActivityDetailFetcher
	inserter      ActivityInserter
	querier       TokenQuerier
	store         SyncSessionStore
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
// It loads the activity event, fetches full activity details from Strava, and upserts to database.
func (w *IngestActivityEventWorker) Work(ctx context.Context, job *river.Job[IngestActivityEventArgs]) error {
	// Get dependencies: if not injected, use global singletons
	eventLoader := w.eventLoader
	if eventLoader == nil {
		return ErrTokenRefresherNotConfigured // Placeholder error if no eventLoader
	}
	detailFetcher := w.detailFetcher
	if detailFetcher == nil {
		return ErrTokenRefresherNotConfigured // Placeholder error if no detailFetcher
	}
	inserter := w.inserter
	if inserter == nil {
		return ErrTokenRefresherNotConfigured // Placeholder error if no inserter
	}
	querier := w.querier
	if querier == nil {
		querier = GetTokenQuerier()
	}

	// Load the activity event from database
	activityEvent, err := eventLoader.GetActivityEventByExternalID(ctx, job.Args.EventID)
	if err != nil {
		return fmt.Errorf("jobs: failed to load activity event %s: %w", job.Args.EventID, err)
	}

	// Parse userID from event
	userID := activityEvent.UserID

	// Get valid access token (handles refresh internally if needed)
	cipherKey := GetCipherKey()
	refresher := GetTokenRefresher()
	accessToken, err := GetValidToken(ctx, querier, cipherKey, refresher, userID)
	if err != nil {
		return fmt.Errorf("jobs: failed to get valid token for user %s: %w", userID.String(), err)
	}

	// Fetch full activity details from Strava API
	activity, err := detailFetcher.GetActivity(ctx, accessToken, activityEvent.ObjectID)
	if err != nil {
		return fmt.Errorf("jobs: failed to fetch activity details for %d: %w", activityEvent.ObjectID, err)
	}

	// Convert strava.ActivityDetail to UpsertActivityParams
	params := sqlc.UpsertActivityParams{
		UserID:         userID,
		ExternalSource: "strava",
		ExternalID:     activity.ID,
		Name:           activity.Name,
		SportType:      activity.Type,
		StartedAt:      pgtype.Timestamptz{Time: activity.StartDate, Valid: true},
		ElapsedSeconds: int32(activity.ElapsedTime),
		MovingSeconds:  int32(activity.MovingTime),
		DistanceMeters: pgtype.Numeric{Int: big.NewInt(int64(activity.Distance * 1000)), Valid: true}, // Convert km to m
		ElevationGainM: pgtype.Numeric{Int: big.NewInt(int64(activity.TotalElevationGain)), Valid: true},
	}

	// Upsert the activity to database
	_, err = inserter.UpsertActivity(ctx, params)
	if err != nil {
		return fmt.Errorf("jobs: failed to upsert activity %d: %w", activity.ID, err)
	}

	// Mark the event as processed
	err = eventLoader.MarkActivityEventProcessed(ctx, activityEvent.ID)
	if err != nil {
		return fmt.Errorf("jobs: failed to mark event %s as processed: %w", job.Args.EventID, err)
	}

	return nil
}
