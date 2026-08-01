package jobs

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

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
// It fetches activities from the Strava API within the specified time window,
// upserts them to the database, and updates the sync session with progress.
// Contract: on success, returns nil; on retriable error (like rate limit), returns error for River retry.
func (w *ImportStravaWorker) Work(ctx context.Context, job *river.Job[ImportStravaArgs]) error {
	// Get configured dependencies
	refresher := GetTokenRefresher()
	querier := GetTokenQuerier()
	cipherKey := GetCipherKey()

	if refresher == nil || querier == nil || len(cipherKey) == 0 {
		return ErrTokenRefresherNotConfigured
	}

	// Parse UserID from string to pgtype.UUID
	var userID pgtype.UUID
	err := userID.Scan(job.Args.UserID)
	if err != nil {
		return fmt.Errorf("jobs: invalid UserID format: %w", err)
	}

	// Get valid access token (with refresh if needed)
	accessToken, err := GetValidToken(ctx, querier, cipherKey, refresher, userID)
	if err != nil {
		return fmt.Errorf("jobs: failed to get valid token: %w", err)
	}

	// Check if sync session exists; if not, create one
	syncSession, err := querier.GetLatestSyncSession(ctx, userID)
	if err != nil {
		// No session found; create one
		windowDays := int32(42) // default backfill window
		if globalConfig != nil && globalConfig.Strava != nil {
			windowDays = int32(globalConfig.Strava.BackfillDays)
		}
		syncSession, err = querier.CreateSyncSession(ctx, sqlc.CreateSyncSessionParams{
			UserID:     userID,
			WindowDays: windowDays,
		})
		if err != nil {
			return fmt.Errorf("jobs: failed to create sync session: %w", err)
		}
	}

	// Transition to 'running' status
	syncSession, err = querier.UpdateSyncSessionStatus(ctx, sqlc.UpdateSyncSessionStatusParams{
		ID:     syncSession.ID,
		Status: "running",
		Error:  pgtype.Text{},
	})
	if err != nil {
		return fmt.Errorf("jobs: failed to update sync session status: %w", err)
	}

	// Paginate through activities (30 per page is Strava API default)
	page := 1
	perPage := 30
	totalImported := int32(0)
	totalSkipped := int32(0)

	for {
		// Fetch activities from Strava API
		activities, err := globalClient.GetActivities(
			ctx,
			accessToken,
			job.Args.WindowStart,
			job.Args.WindowEnd,
			page,
			perPage,
		)
		if err != nil {
			// Return error to River for retry
			return fmt.Errorf("jobs: strava GetActivities failed: %w", err)
		}

		// If we got an empty page, we're done
		if len(activities) == 0 {
			break
		}

		// Upsert each activity
		for _, act := range activities {
			// Serialize raw payload as JSON
			rawJSON, _ := json.Marshal(act)

			// Upsert to database
			_, err := querier.UpsertActivity(ctx, sqlc.UpsertActivityParams{
				UserID:         userID,
				ExternalSource: "strava",
				ExternalID:     act.ID,
				Name:           act.Name,
				SportType:      act.Type,
				StartedAt: pgtype.Timestamptz{
					Time:  act.StartDate,
					Valid: true,
				},
				ElapsedSeconds: int32(act.ElapsedTime),
				MovingSeconds:  int32(act.MovingTime),
				DistanceMeters: pgtype.Numeric{}, // TODO: parse float to numeric
				ElevationGainM: pgtype.Numeric{}, // TODO: parse float to numeric
				AvgHr:          pgtype.Int2{},
				MaxHr:          pgtype.Int2{},
				AvgPower:       pgtype.Int2{},
				RawPayload:     rawJSON,
			})
			if err != nil {
				// Log but continue; the ON CONFLICT constraint will handle dupes
				totalSkipped++
			} else {
				totalImported++
			}
		}

		// Update progress
		_, err = querier.UpdateSyncSessionProgress(ctx, sqlc.UpdateSyncSessionProgressParams{
			ID:              syncSession.ID,
			TotalActivities: int32(totalImported + totalSkipped),
			Imported:        totalImported,
			Skipped:         totalSkipped,
		})
		if err != nil {
			return fmt.Errorf("jobs: failed to update progress: %w", err)
		}

		// Next page
		page++
	}

	// Mark sync session as completed
	_, err = querier.UpdateSyncSessionStatus(ctx, sqlc.UpdateSyncSessionStatusParams{
		ID:     syncSession.ID,
		Status: "completed",
		Error:  pgtype.Text{},
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
// Workflow:
// 1. Load the activity_event row from the database by EventID (external_id).
// 2. Extract user_id and object_id (Strava activity ID) from the event.
// 3. Get valid access token for the user (with refresh if needed).
// 4. Fetch full activity data from Strava API using object_id.
// 5. Upsert the activity to the activities table.
// 6. Mark the event as processed.
//
// Returns nil on success; returns error to River for retry on failures.
func (w *IngestActivityEventWorker) Work(ctx context.Context, job *river.Job[IngestActivityEventArgs]) error {
	// CONTRACT: Webhook Ingestion Flow
	// ================================
	// 1. Strava sends webhook event (e.g., "activity" event with "create" aspect_type)
	// 2. Handler validates signature (STRAVA_WEBHOOK_SECRET) and enqueues IngestActivityEventArgs{EventID: "..."}
	// 3. River executes this worker:
	//    a. Load ActivityEvent by external_id from DB (stores raw event metadata)
	//    b. Extract user_id and object_id (Strava activity ID)
	//    c. Get valid access token for user (decrypt, refresh if needed)
	//    d. Fetch activity detail from Strava API using token
	//    e. Upsert activity to DB (ON CONFLICT handles idempotency)
	//    f. Mark ActivityEvent as processed (update processed_at)
	// Errors at steps a–c are retryable; step d fails only if token is invalid.
	// Idempotent: Upsert activity and mark event processed are safe to repeat.

	// Get configured dependencies
	refresher := GetTokenRefresher()
	querier := GetTokenQuerier()
	cipherKey := GetCipherKey()

	if refresher == nil || querier == nil || len(cipherKey) == 0 {
		return ErrTokenRefresherNotConfigured
	}

	// Step 1: Load the activity event from DB by external_id (EventID)
	event, err := querier.GetActivityEventByExternalID(ctx, job.Args.EventID)
	if err != nil {
		return fmt.Errorf("jobs: failed to load activity event: %w", err)
	}

	// Step 2: Extract user_id and object_id from the event
	userID := event.UserID
	stravaActivityID := event.ObjectID

	// Skip if user_id is null (webhook has not yet resolved the user)
	if !userID.Valid {
		return fmt.Errorf("jobs: activity event %s has no user_id; webhook resolution pending", job.Args.EventID)
	}

	// Step 3: Get valid access token for the user
	accessToken, err := GetValidToken(ctx, querier, cipherKey, refresher, userID)
	if err != nil {
		return fmt.Errorf("jobs: failed to get valid token: %w", err)
	}

	// Step 4: Fetch full activity data from Strava API
	activity, err := globalClient.GetActivity(ctx, accessToken, stravaActivityID)
	if err != nil {
		return fmt.Errorf("jobs: strava GetActivity failed: %w", err)
	}

	// Step 5: Upsert activity to database
	rawJSON, _ := json.Marshal(activity)
	_, err = querier.UpsertActivity(ctx, sqlc.UpsertActivityParams{
		UserID:         userID,
		ExternalSource: "strava",
		ExternalID:     activity.ID,
		Name:           activity.Name,
		SportType:      activity.Type,
		StartedAt: pgtype.Timestamptz{
			Time:  activity.StartDate,
			Valid: true,
		},
		ElapsedSeconds: int32(activity.ElapsedTime),
		MovingSeconds:  int32(activity.MovingTime),
		DistanceMeters: pgtype.Numeric{}, // TODO: parse float to numeric
		ElevationGainM: pgtype.Numeric{}, // TODO: parse float to numeric
		AvgHr:          pgtype.Int2{},
		MaxHr:          pgtype.Int2{},
		AvgPower:       pgtype.Int2{},
		RawPayload:     rawJSON,
	})
	if err != nil {
		return fmt.Errorf("jobs: failed to upsert activity: %w", err)
	}

	// Step 6: Mark event as processed
	err = querier.MarkActivityEventProcessed(ctx, event.ID)
	if err != nil {
		return fmt.Errorf("jobs: failed to mark event processed: %w", err)
	}

	return nil
}
