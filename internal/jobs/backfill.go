package jobs

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/fgjcarlos/ghamusinos/internal/db/sqlc"
	"github.com/fgjcarlos/ghamusinos/internal/strava"
)

// ActivityFetcher is what ImportStravaWorker needs from the Strava client.
// It fetches activity summaries and detailed activity information.
// Interface Segregation: activity fetching is separate from token management.
type ActivityFetcher interface {
	GetActivities(ctx context.Context, accessToken string, after, before time.Time, page, perPage int) ([]strava.ActivitySummary, error)
	GetActivity(ctx context.Context, accessToken string, id int64) (*strava.ActivityDetail, error)
}

// SyncSessionStore is what ImportStravaWorker needs from the DB layer.
// It manages the lifecycle of sync sessions (state, progress tracking).
// Interface Segregation: sync session management is separate from activity storage.
type SyncSessionStore interface {
	GetLatestSyncSession(ctx context.Context, userID pgtype.UUID) (sqlc.SyncSession, error)
	CreateSyncSession(ctx context.Context, arg sqlc.CreateSyncSessionParams) (sqlc.SyncSession, error)
	UpdateSyncSessionStatus(ctx context.Context, arg sqlc.UpdateSyncSessionStatusParams) (sqlc.SyncSession, error)
	UpdateSyncSessionProgress(ctx context.Context, arg sqlc.UpdateSyncSessionProgressParams) (sqlc.SyncSession, error)
}

// ActivityInserter is what both ImportStravaWorker and IngestActivityEventWorker need.
// It persists activities to the database (ON CONFLICT DO NOTHING for deduplication).
// Interface Segregation: activity insertion is separate from other concerns.
type ActivityInserter interface {
	UpsertActivity(ctx context.Context, arg sqlc.UpsertActivityParams) (sqlc.Activity, error)
}

// backfill.go contains helper functions and implementations for Slice 5a.
// The worker types themselves (ImportStravaWorker, IngestActivityEventWorker) are defined in workers.go
// to comply with River's worker registration.

// importStravaWorkerImpl contains the actual implementation logic for ImportStravaWorker.
// This is separated from the river.Worker type to allow dependency injection in tests.
type importStravaWorkerImpl struct {
	fetcher        ActivityFetcher
	sessionStore   SyncSessionStore
	activityStore  ActivityInserter
	cipherKey      []byte
	tokenQuerier   TokenQuerier
	tokenRefresher TokenRefresher
}

// executeImportStrava performs the actual Strava import with injected dependencies.
func (impl *importStravaWorkerImpl) execute(ctx context.Context, userIDStr string, windowStart, windowEnd time.Time) error {
	// Parse UserID from string to pgtype.UUID
	var userID pgtype.UUID
	err := userID.Scan(userIDStr)
	if err != nil {
		return fmt.Errorf("jobs: failed to parse user ID: %w", err)
	}

	// Get valid access token (refreshes if needed)
	accessToken, err := GetValidToken(ctx, impl.tokenQuerier, impl.cipherKey, impl.tokenRefresher, userID)
	if err != nil {
		return fmt.Errorf("jobs: failed to get valid token: %w", err)
	}

	// Get or create sync session
	syncSession, err := impl.sessionStore.GetLatestSyncSession(ctx, userID)
	if err != nil {
		// No sync session exists; create one
		syncSession, err = impl.sessionStore.CreateSyncSession(ctx, sqlc.CreateSyncSessionParams{
			UserID: userID,
		})
		if err != nil {
			return fmt.Errorf("jobs: failed to create sync session: %w", err)
		}
	}

	// Mark sync_session as running
	_, err = impl.sessionStore.UpdateSyncSessionStatus(ctx, sqlc.UpdateSyncSessionStatusParams{
		ID:     syncSession.ID,
		Status: "running",
	})
	if err != nil {
		return fmt.Errorf("jobs: failed to update sync session status: %w", err)
	}

	// Fetch activities in pages
	page := 1
	perPage := 50
	var imported int32 = 0

	for {
		// Fetch one page of activities
		activities, err := impl.fetcher.GetActivities(ctx, accessToken, windowStart, windowEnd, page, perPage)
		if err != nil {
			return fmt.Errorf("jobs: failed to fetch activities page %d: %w", page, err)
		}

		// If no activities in this page, we're done paginating
		if len(activities) == 0 {
			break
		}

		// Upsert each activity
		for _, activity := range activities {
			_, err := impl.activityStore.UpsertActivity(ctx, sqlc.UpsertActivityParams{
				UserID:         userID,
				ExternalID:     activity.ID,
				ExternalSource: "strava",
				SportType:      activity.Type,
				Name:           activity.Name,
				StartedAt:      pgtype.Timestamptz{Time: activity.StartDate, Valid: true},
				DistanceMeters: pgtype.Numeric{}, // Would be populated from activity if needed
			})
			if err != nil {
				return fmt.Errorf("jobs: failed to upsert activity: %w", err)
			}
			imported++
		}

		// Update progress after each page
		_, err = impl.sessionStore.UpdateSyncSessionProgress(ctx, sqlc.UpdateSyncSessionProgressParams{
			ID:       syncSession.ID,
			Imported: imported,
		})
		if err != nil {
			return fmt.Errorf("jobs: failed to update sync session progress: %w", err)
		}

		// Move to next page
		page++
	}

	// Mark sync_session as completed
	_, err = impl.sessionStore.UpdateSyncSessionStatus(ctx, sqlc.UpdateSyncSessionStatusParams{
		ID:     syncSession.ID,
		Status: "completed",
	})
	if err != nil {
		return fmt.Errorf("jobs: failed to mark sync session completed: %w", err)
	}

	return nil
}

// ingestActivityEventWorkerImpl contains the actual implementation logic for IngestActivityEventWorker.
type ingestActivityEventWorkerImpl struct {
	fetcher        ActivityFetcher
	activityStore  ActivityInserter
	cipherKey      []byte
	tokenQuerier   TokenQuerier
	tokenRefresher TokenRefresher
}

// executeIngestActivityEvent performs the actual activity ingestion with injected dependencies.
func (impl *ingestActivityEventWorkerImpl) execute(ctx context.Context, userIDStr string, activityID int64) error {
	// Parse UserID from string to pgtype.UUID
	var userID pgtype.UUID
	err := userID.Scan(userIDStr)
	if err != nil {
		return fmt.Errorf("jobs: failed to parse user ID: %w", err)
	}

	// Get valid access token (refreshes if needed)
	accessToken, err := GetValidToken(ctx, impl.tokenQuerier, impl.cipherKey, impl.tokenRefresher, userID)
	if err != nil {
		return fmt.Errorf("jobs: failed to get valid token: %w", err)
	}

	// Fetch the full activity from Strava
	activityDetail, err := impl.fetcher.GetActivity(ctx, accessToken, activityID)
	if err != nil {
		return fmt.Errorf("jobs: failed to fetch activity %d: %w", activityID, err)
	}

	// Upsert activity to database
	_, err = impl.activityStore.UpsertActivity(ctx, sqlc.UpsertActivityParams{
		UserID:         userID,
		ExternalID:     activityDetail.ID,
		ExternalSource: "strava",
		SportType:      activityDetail.Type,
		Name:           activityDetail.Name,
		StartedAt:      pgtype.Timestamptz{Time: activityDetail.StartDate, Valid: true},
		DistanceMeters: pgtype.Numeric{
			// Parse distance from activity if present
		},
	})
	if err != nil {
		return fmt.Errorf("jobs: failed to upsert activity: %w", err)
	}

	return nil
}
