package jobs

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/fgjcarlos/ghamusinos/internal/db/sqlc"
	"github.com/fgjcarlos/ghamusinos/internal/strava"
)

// BackfillWindow returns the time window for backfilling activities.
// It returns (after, before) where:
// - after: 'days' days before now
// - before: now
//
// This is a pure function (no side effects), making it trivial to test.
func BackfillWindow(days int) (after, before time.Time) {
	before = time.Now()
	after = before.AddDate(0, 0, -days)
	return after, before
}

// ActivityFetcher defines the interface for fetching activities from Strava API.
type ActivityFetcher interface {
	GetActivities(ctx context.Context, accessToken string, after, before time.Time, page, perPage int) ([]strava.ActivitySummary, error)
}

// SyncSessionStore defines the interface for managing sync sessions in the database.
type SyncSessionStore interface {
	GetLatestSyncSession(ctx context.Context, userID pgtype.UUID) (sqlc.SyncSession, error)
	CreateSyncSession(ctx context.Context, arg sqlc.CreateSyncSessionParams) (sqlc.SyncSession, error)
	UpdateSyncSessionStatus(ctx context.Context, arg sqlc.UpdateSyncSessionStatusParams) (sqlc.SyncSession, error)
	UpdateSyncSessionProgress(ctx context.Context, arg sqlc.UpdateSyncSessionProgressParams) (sqlc.SyncSession, error)
}

// ActivityInserter defines the interface for upserting activities to the database.
type ActivityInserter interface {
	UpsertActivity(ctx context.Context, arg sqlc.UpsertActivityParams) (sqlc.Activity, error)
}

// ActivityEventLoader defines the interface for loading activity events from the database.
type ActivityEventLoader interface {
	GetActivityEventByID(ctx context.Context, id pgtype.UUID) (sqlc.ActivityEvent, error)
	MarkActivityEventProcessed(ctx context.Context, id pgtype.UUID) error
}

// ActivityDetailFetcher defines the interface for fetching activity details from Strava API.
type ActivityDetailFetcher interface {
	GetActivity(ctx context.Context, accessToken string, id int64) (*strava.ActivityDetail, error)
}
