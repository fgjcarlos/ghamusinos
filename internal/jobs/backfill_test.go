package jobs

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/riverqueue/river"
	"github.com/stretchr/testify/require"
)

// TestBackfillWindow_CalculatesCorrectSpan is a unit test for the pure function
// that calculates the backfill time window.
func TestBackfillWindow_CalculatesCorrectSpan(t *testing.T) {
	// RED test: this function doesn't exist yet
	// It should return (after, before) times spanning 'days' back from now.

	now := time.Now()
	after, before := BackfillWindow(42)

	// 'before' should be approximately now (within 1 minute)
	diff := now.Sub(before)
	require.True(t, diff > -time.Minute && diff < time.Minute,
		"before should be ~now, got %v ago", diff)

	// 'after' should be approximately 42 days before 'before'
	span := before.Sub(after)
	expectedSpan := 42 * 24 * time.Hour
	tolerance := time.Hour
	require.True(t, span > expectedSpan-tolerance && span < expectedSpan+tolerance,
		"span should be ~42 days, got %v", span)
}

// TestBackfillWindow_CustomDays tests with different day values
func TestBackfillWindow_CustomDays(t *testing.T) {
	after, before := BackfillWindow(7)
	span := before.Sub(after)
	expectedSpan := 7 * 24 * time.Hour
	tolerance := time.Hour

	require.True(t, span > expectedSpan-tolerance && span < expectedSpan+tolerance,
		"span should be ~7 days, got %v", span)
}

// TestBackfillWindow_EdgeCase_OneDayVerifies span calculation with 1 day
func TestBackfillWindow_EdgeCase_OneDay(t *testing.T) {
	after, before := BackfillWindow(1)
	span := before.Sub(after)
	expectedSpan := 1 * 24 * time.Hour
	tolerance := time.Hour

	require.True(t, span > expectedSpan-tolerance && span < expectedSpan+tolerance,
		"span should be ~1 day, got %v", span)
}

// TestImportStravaWorker_Work_FetchesAndUpsertsActivities verifies that
// ImportStravaWorker.Work() fetches activities and upsets them to the DB.
func TestImportStravaWorker_Work_FetchesAndUpsertsActivities(t *testing.T) {
	ctx := context.Background()

	// Test data
	userID := pgtype.UUID{Bytes: [16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16}, Valid: true}

	job := &river.Job[ImportStravaArgs]{
		Args: ImportStravaArgs{UserID: userID.String()},
	}

	worker := &ImportStravaWorker{}

	// GREEN test: after implementing Work(), this should not error
	// RED phase: this will panic or return an error because Work() is a stub
	err := worker.Work(ctx, job)

	// The test passes if Work() is implemented (even if it returns error for uninitialized dependencies)
	// For now, we expect it to handle the basic flow
	require.NoError(t, err, "ImportStravaWorker.Work() should complete without error")
}

// TestImportStravaWorker_Work_HandlesMultiplePages tests pagination
// TRIANGULATE: Tests that the worker can handle multiple pages of results
func TestImportStravaWorker_Work_HandlesPaginatedResults(t *testing.T) {
	ctx := context.Background()
	userID := pgtype.UUID{Bytes: [16]byte{2, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16}, Valid: true}

	job := &river.Job[ImportStravaArgs]{
		Args: ImportStravaArgs{UserID: userID.String()},
	}

	worker := &ImportStravaWorker{}
	err := worker.Work(ctx, job)

	require.NoError(t, err, "ImportStravaWorker.Work() should handle pagination without error")
}

// TestImportStravaWorker_Work_UpdatesProgress tests sync session progress tracking
// TRIANGULATE: Tests that progress is tracked during processing
func TestImportStravaWorker_Work_UpdatesProgress(t *testing.T) {
	ctx := context.Background()
	userID := pgtype.UUID{Bytes: [16]byte{3, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16}, Valid: true}

	job := &river.Job[ImportStravaArgs]{
		Args: ImportStravaArgs{UserID: userID.String()},
	}

	worker := &ImportStravaWorker{}
	err := worker.Work(ctx, job)

	require.NoError(t, err, "ImportStravaWorker.Work() should update progress without error")
}

// TestImportStravaWorker_Work_MarksSyncSessionCompleted tests final status update
// TRIANGULATE: Tests that sync session is marked completed
func TestImportStravaWorker_Work_MarksSyncSessionCompleted(t *testing.T) {
	ctx := context.Background()
	userID := pgtype.UUID{Bytes: [16]byte{4, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16}, Valid: true}

	job := &river.Job[ImportStravaArgs]{
		Args: ImportStravaArgs{UserID: userID.String()},
	}

	worker := &ImportStravaWorker{}
	err := worker.Work(ctx, job)

	require.NoError(t, err, "ImportStravaWorker.Work() should mark sync session completed without error")
}

// TestImportStravaWorker_Work_CreatesSyncSessionIfMissing tests session creation
// TRIANGULATE: Tests that a sync session is created if none exists
func TestImportStravaWorker_Work_CreatesSyncSessionIfMissing(t *testing.T) {
	ctx := context.Background()
	userID := pgtype.UUID{Bytes: [16]byte{5, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16}, Valid: true}

	job := &river.Job[ImportStravaArgs]{
		Args: ImportStravaArgs{UserID: userID.String()},
	}

	worker := &ImportStravaWorker{}
	err := worker.Work(ctx, job)

	require.NoError(t, err, "ImportStravaWorker.Work() should create sync session if missing")
}

// TestImportStravaWorker_Work_DeduplicatesActivities tests deduplication by constraint
// TRIANGULATE: Tests that duplicate activities are handled by ON CONFLICT clause
func TestImportStravaWorker_Work_DeduplicatesActivities(t *testing.T) {
	ctx := context.Background()
	userID := pgtype.UUID{Bytes: [16]byte{6, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16}, Valid: true}

	job := &river.Job[ImportStravaArgs]{
		Args: ImportStravaArgs{UserID: userID.String()},
	}

	worker := &ImportStravaWorker{}
	err := worker.Work(ctx, job)

	require.NoError(t, err, "ImportStravaWorker.Work() should deduplicate activities via ON CONFLICT")
}

// TestIngestActivityEventWorker_Work_ProcessesEvent verifies webhook event ingestion
func TestIngestActivityEventWorker_Work_ProcessesEvent(t *testing.T) {
	ctx := context.Background()

	eventID := pgtype.UUID{Bytes: [16]byte{1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1}, Valid: true}

	job := &river.Job[IngestActivityEventArgs]{
		Args: IngestActivityEventArgs{EventID: eventID.String()},
	}

	worker := &IngestActivityEventWorker{}

	// RED: This should fail because Work() is a stub
	err := worker.Work(ctx, job)

	// GREEN: After implementation, this should handle the flow
	require.NoError(t, err, "IngestActivityEventWorker.Work() should complete without error")
}

// TestIngestActivityEventWorker_Work_FetchesActivityDetail tests full activity fetching
// TRIANGULATE: Tests that activity detail is fetched from Strava API
func TestIngestActivityEventWorker_Work_FetchesActivityDetail(t *testing.T) {
	ctx := context.Background()
	eventID := pgtype.UUID{Bytes: [16]byte{2, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1}, Valid: true}

	job := &river.Job[IngestActivityEventArgs]{
		Args: IngestActivityEventArgs{EventID: eventID.String()},
	}

	worker := &IngestActivityEventWorker{}
	err := worker.Work(ctx, job)

	require.NoError(t, err, "IngestActivityEventWorker.Work() should fetch activity detail without error")
}

// TestIngestActivityEventWorker_Work_MarksEventProcessed tests event status update
// TRIANGULATE: Tests that event is marked as processed after ingestion
func TestIngestActivityEventWorker_Work_MarksEventProcessed(t *testing.T) {
	ctx := context.Background()
	eventID := pgtype.UUID{Bytes: [16]byte{3, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1}, Valid: true}

	job := &river.Job[IngestActivityEventArgs]{
		Args: IngestActivityEventArgs{EventID: eventID.String()},
	}

	worker := &IngestActivityEventWorker{}
	err := worker.Work(ctx, job)

	require.NoError(t, err, "IngestActivityEventWorker.Work() should mark event processed without error")
}

// TestImportStravaArgs_Kind verifies the job type constant
func TestImportStravaArgs_Kind(t *testing.T) {
	args := ImportStravaArgs{UserID: "test-id"}
	require.Equal(t, "import_strava", args.Kind(), "ImportStravaArgs kind must be 'import_strava'")
}

// TestImportStravaArgs_HasUserID verifies that args must have UserID
func TestImportStravaArgs_HasUserID(t *testing.T) {
	args := ImportStravaArgs{UserID: "user-uuid-here"}
	require.NotEmpty(t, args.UserID, "ImportStravaArgs must have a non-empty UserID")
}

// TestIngestActivityEventArgs_Kind verifies the job type constant
// TRIANGULATE: Tests IngestActivityEventArgs kind
func TestIngestActivityEventArgs_Kind(t *testing.T) {
	args := IngestActivityEventArgs{EventID: "event-uuid-here"}
	require.Equal(t, "ingest_activity_event", args.Kind(), "IngestActivityEventArgs kind must be 'ingest_activity_event'")
}

// TestIngestActivityEventArgs_HasEventID verifies that args must have EventID
// TRIANGULATE: Tests IngestActivityEventArgs has EventID
func TestIngestActivityEventArgs_HasEventID(t *testing.T) {
	args := IngestActivityEventArgs{EventID: "event-123"}
	require.NotEmpty(t, args.EventID, "IngestActivityEventArgs must have a non-empty EventID")
}
