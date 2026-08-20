package jobs

import (
	"context"
	"fmt"
	"math/big"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"

	"github.com/fgjcarlos/ghamusinos/internal/config"
	"github.com/fgjcarlos/ghamusinos/internal/db/sqlc"
	"github.com/fgjcarlos/ghamusinos/internal/strava"
)

// Deps is the constructor-injected dependency bundle for River workers.
//
// AUD-04 (issue #164) replaces the four package-globals + ConfigureTokenRefresher
// that used to be set once at startup and silently forgotten by app.Run. Every
// worker now takes them by constructor; missing any one is a startup error,
// not a runtime nil-deref.
//
// Pool and Config are required — River itself needs a pgx pool, and the
// import worker reads Config.Strava.BackfillDays for the backfill window.
// Strava and CipherKey are required when cfg.Strava is set (production
// always sets it). When cfg.Strava is absent (developer running without
// Strava credentials), the Strava-bound workers are not registered and
// these fields may be nil — see NewRiverWorkers.
type Deps struct {
	Pool      *pgxpool.Pool
	Config    *config.Config
	Strava    *strava.Client
	CipherKey []byte
}

// validate returns a non-nil error if a required dependency is missing.
// Pool and Config are always required; Strava and CipherKey are required
// when RegisterStravaWorkers is true. AUD-04 AC: "NewRiverWorkers returns
// error if any dependency is missing".
func (d Deps) validate(registerStravaWorkers bool) error {
	if d.Pool == nil {
		return fmt.Errorf("jobs: Deps.Pool is required")
	}
	if d.Config == nil {
		return fmt.Errorf("jobs: Deps.Config is required")
	}
	if !registerStravaWorkers {
		return nil
	}
	if d.Strava == nil {
		return fmt.Errorf("jobs: Deps.Strava is required when RegisterStravaWorkers is true")
	}
	if len(d.CipherKey) == 0 {
		return fmt.Errorf("jobs: Deps.CipherKey is required when RegisterStravaWorkers is true")
	}
	return nil
}

// importDeps is the per-worker dependency set derived from Deps. Keeping this
// as a separate struct lets each worker's fields be filled by NewRiverWorkers
// without dragging the Pool/CipherKey through the whole worker tree.
//
// stravaFetcher and stravaRefresher are typed as the narrower interfaces
// (not *strava.Client) so that:
//   - Production *strava.Client satisfies both (it has GetActivities and Refresh).
//   - Tests can substitute smaller mocks that implement only the interface
//     they exercise without dragging the full Strava client surface in.
type importDeps struct {
	queries         *sqlc.Queries
	stravaFetcher   ActivityFetcher
	stravaRefresher TokenRefresher
	cipher          []byte
	backfill        int32
}

// NewRiverWorkers creates a river.Workers instance with every handler bound to
// its concrete dependencies. AUD-04: dependencies come from the constructor,
// not from package globals. AUD-04 AC: "Every worker registered has all its
// dependencies non-nil."
//
// registerStravaWorkers toggles whether the Strava-bound workers
// (ImportStrava, RefreshStravaToken, ImportStravaStreams) are registered.
// Set to true when cfg.Strava is configured; set to false in dev / tests
// that do not have Strava credentials. The IngestActivityEventWorker is
// registered regardless but its eventLoader is nil — see the ponytail note
// in NewRiverWorkers body for the AUD-03 dependency.
func NewRiverWorkers(d Deps, registerStravaWorkers bool) (*river.Workers, error) {
	if err := d.validate(registerStravaWorkers); err != nil {
		return nil, err
	}

	workers := river.NewWorkers()
	queries := sqlc.New(d.Pool)

	if registerStravaWorkers {
		id := importDeps{
			queries:         queries,
			stravaFetcher:   d.Strava, // *strava.Client satisfies ActivityFetcher
			stravaRefresher: d.Strava, // *strava.Client satisfies TokenRefresher
			cipher:          d.CipherKey,
			backfill:        int32(d.Config.Strava.BackfillDays),
		}
		river.AddWorker(workers, NewImportStravaWorker(id))
		river.AddWorker(workers, NewRefreshStravaTokenWorker(id))
		river.AddWorker(workers, &ImportStravaStreamsWorker{
			fetcher:   d.Strava,
			inserter:  queries,
			locator:   queries,
			querier:   queries,
			refresher: d.Strava,
			zoneStore: queries,
			cipherKey: d.CipherKey,
		})
	}

	// ponytail: eventLoader is nil here because sqlc.Queries does not yet
	// implement GetActivityEventByExternalID — that lands in AUD-03 (issue
	// #165). The worker is registered so the kind is known to River, but
	// any job routed to it will fail in Work() with a nil-deref; that is
	// acceptable while AUD-03 is open because no production code enqueues
	// IngestActivityEventArgs yet. Replace nil with id.queries in the AUD-03 PR.
	river.AddWorker(workers, &IngestActivityEventWorker{
		refresher: d.Strava,
		cipher:    d.CipherKey,
	})
	return workers, nil
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
//
// AUD-04: dependencies are set once at construction; the Work method no
// longer reads from package globals and no longer guards "if nil" per field.
type ImportStravaWorker struct {
	river.WorkerDefaults[ImportStravaArgs]
	fetcher   ActivityFetcher
	refresher TokenRefresher
	store     SyncSessionStore
	inserter  ActivityInserter
	querier   TokenQuerier
	cipher    []byte
	backfill  int32
}

// NewImportStravaWorker builds an ImportStravaWorker wired to the supplied
// dependencies. id.strava must be a Strava client implementing both
// ActivityFetcher (for the paginated GetActivities) and TokenRefresher (for
// refreshing the OAuth token before each call). The production *strava.Client
// satisfies both.
func NewImportStravaWorker(id importDeps) *ImportStravaWorker {
	return &ImportStravaWorker{
		fetcher:   id.stravaFetcher,
		refresher: id.stravaRefresher,
		store:     id.queries,
		inserter:  id.queries,
		querier:   id.queries,
		cipher:    id.cipher,
		backfill:  id.backfill,
	}
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

	// Get valid access token (handles refresh internally if needed)
	accessToken, err := GetValidToken(ctx, w.querier, w.cipher, w.refresher, userID)
	if err != nil {
		return fmt.Errorf("jobs: failed to get valid token: %w", err)
	}

	// Get or create sync session
	syncSession, err := w.store.GetLatestSyncSession(ctx, userID)
	if err != nil {
		// Session doesn't exist; create one
		syncSession, err = w.store.CreateSyncSession(ctx, sqlc.CreateSyncSessionParams{
			UserID:     userID,
			WindowDays: w.backfill,
		})
		if err != nil {
			return fmt.Errorf("jobs: failed to create sync session: %w", err)
		}
	}

	// Calculate backfill window. WindowDays is set at construction time
	// from config.Strava.BackfillDays; passing it to BackfillWindow keeps
	// the existing arithmetic in one place.
	after, before := BackfillWindow(int(w.backfill))

	// Fetch activities in pages and upsert each
	page := 1
	perPage := 50
	totalProcessed := int32(0)

	for {
		activities, err := w.fetcher.GetActivities(ctx, accessToken, after, before, page, perPage)
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

			_, err := w.inserter.UpsertActivity(ctx, params)
			if err != nil {
				return fmt.Errorf("jobs: failed to upsert activity %d: %w", activity.ID, err)
			}
			totalProcessed++
		}

		// Update progress
		_, err = w.store.UpdateSyncSessionProgress(ctx, sqlc.UpdateSyncSessionProgressParams{
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
	_, err = w.store.UpdateSyncSessionStatus(ctx, sqlc.UpdateSyncSessionStatusParams{
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
//
// AUD-04: dependencies are set at construction; the Work method no longer
// reads GetTokenRefresher / GetTokenQuerier / GetCipherKey from globals.
type RefreshStravaTokenWorker struct {
	river.WorkerDefaults[RefreshStravaTokenArgs]
	querier   TokenQuerier
	refresher TokenRefresher
	cipher    []byte
}

// NewRefreshStravaTokenWorker builds a RefreshStravaTokenWorker with the
// supplied dependencies. querier is what reads/upserts the encrypted token;
// refresher is the Strava client (which implements TokenRefresher).
func NewRefreshStravaTokenWorker(id importDeps) *RefreshStravaTokenWorker {
	return &RefreshStravaTokenWorker{
		querier:   id.queries,
		refresher: id.stravaRefresher,
		cipher:    id.cipher,
	}
}

// Work processes a RefreshStravaToken job.
// It refreshes the access token using the stored refresh token and the Strava API.
// If the token is valid for more than 5 minutes, no action is taken.
func (w *RefreshStravaTokenWorker) Work(ctx context.Context, job *river.Job[RefreshStravaTokenArgs]) error {
	// Parse UserID from string to pgtype.UUID
	var userID pgtype.UUID
	err := userID.Scan(job.Args.UserID)
	if err != nil {
		return err
	}

	// Get the valid token (this handles refresh internally if needed)
	_, err = GetValidToken(ctx, w.querier, w.cipher, w.refresher, userID)
	if err != nil {
		// Return the error to River for retry
		return err
	}

	return nil
}

// IngestActivityEventWorker handles ingesting activity events from Strava webhooks.
//
// AUD-04: dependencies are set at construction. The "if nil { return ... }"
// guards that used to return ErrTokenRefresherNotConfigured are gone —
// because the constructor guarantees the fields are non-nil. AUD-04 AC:
// "ErrTokenRefresherNotConfigured ya no existe."
//
// NOTE: the eventLoader field is left nil by NewRiverWorkers today because
// sqlc.Queries does not implement ActivityEventLoader — GetActivityEventByExternalID
// is the missing query that AUD-03 (issue #165) ships. Until that lands no
// production code enqueues IngestActivityEventArgs, so a nil eventLoader is
// unreachable. The constructor below exists for tests and for AUD-03.
type IngestActivityEventWorker struct {
	river.WorkerDefaults[IngestActivityEventArgs]
	eventLoader   ActivityEventLoader
	detailFetcher ActivityDetailFetcher
	inserter      ActivityInserter
	querier       TokenQuerier
	cipher        []byte
	refresher     TokenRefresher
}

// NewIngestActivityEventWorker builds an IngestActivityEventWorker with each
// dependency passed explicitly. In production NewRiverWorkers does not call
// this constructor (it constructs the worker struct literal with a nil
// eventLoader because sqlc.Queries cannot implement the missing query yet),
// but tests use this helper to assemble the full dependency set.
func NewIngestActivityEventWorker(
	eventLoader ActivityEventLoader,
	detailFetcher ActivityDetailFetcher,
	inserter ActivityInserter,
	querier TokenQuerier,
	cipher []byte,
	refresher TokenRefresher,
) *IngestActivityEventWorker {
	return &IngestActivityEventWorker{
		eventLoader:   eventLoader,
		detailFetcher: detailFetcher,
		inserter:      inserter,
		querier:       querier,
		cipher:        cipher,
		refresher:     refresher,
	}
}

// IngestActivityEventArgs are the arguments for processing an activity event.
type IngestActivityEventArgs struct {
	EventID string
}

// Kind returns the job kind for IngestActivityEventArgs.
func (a IngestActivityEventArgs) Kind() string {
	return "ingest_activity_event"
}

// Work processes an IngestActivityEvent job.
// It loads the activity event, fetches full activity details from Strava, and upserts to database.
func (w *IngestActivityEventWorker) Work(ctx context.Context, job *river.Job[IngestActivityEventArgs]) error {
	// Load the activity event from database
	activityEvent, err := w.eventLoader.GetActivityEventByExternalID(ctx, job.Args.EventID)
	if err != nil {
		return fmt.Errorf("jobs: failed to load activity event %s: %w", job.Args.EventID, err)
	}

	// Parse userID from event
	userID := activityEvent.UserID

	// Get valid access token (handles refresh internally if needed)
	accessToken, err := GetValidToken(ctx, w.querier, w.cipher, w.refresher, userID)
	if err != nil {
		return fmt.Errorf("jobs: failed to get valid token for user %s: %w", userID.String(), err)
	}

	// Fetch full activity details from Strava API
	activity, err := w.detailFetcher.GetActivity(ctx, accessToken, activityEvent.ObjectID)
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
	_, err = w.inserter.UpsertActivity(ctx, params)
	if err != nil {
		return fmt.Errorf("jobs: failed to upsert activity %d: %w", activity.ID, err)
	}

	// Mark the event as processed
	err = w.eventLoader.MarkActivityEventProcessed(ctx, activityEvent.ID)
	if err != nil {
		return fmt.Errorf("jobs: failed to mark event %s as processed: %w", job.Args.EventID, err)
	}

	return nil
}

// ImportStravaStreamsWorker handles importing Strava activity streams (HR, watts, etc.) and calculating HR zones.
// Dependencies are injected at construction time via NewRiverWorkers.
type ImportStravaStreamsWorker struct {
	river.WorkerDefaults[ImportStravaStreamsArgs]
	fetcher   StreamFetcher
	inserter  StreamInserter
	zoneStore HRZoneStore
	locator   ActivityLocator
	querier   TokenQuerier
	refresher TokenRefresher
	cipherKey []byte
}

// _ keeps the pgx import referenced for type-check on the river.Workers
// generic parameter in NewClient (river.Client[pgx.Tx]).
var _ = (*pgx.Tx)(nil)
