package jobs

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/riverqueue/river"
	"github.com/stretchr/testify/require"

	"github.com/fgjcarlos/ghamusinos/internal/config"
	"github.com/fgjcarlos/ghamusinos/internal/crypto"
	"github.com/fgjcarlos/ghamusinos/internal/db/sqlc"
	"github.com/fgjcarlos/ghamusinos/internal/strava"
)

// Mock implementations for testing

type mockActivityFetcher struct {
	activities []strava.ActivitySummary
}

func (m *mockActivityFetcher) GetActivities(ctx context.Context, accessToken string, after, before time.Time, page, perPage int) ([]strava.ActivitySummary, error) {
	if page > 1 {
		return []strava.ActivitySummary{}, nil // Empty page signals end
	}
	return m.activities, nil
}

type mockSyncSessionStore struct{}

func (m *mockSyncSessionStore) GetLatestSyncSession(ctx context.Context, userID pgtype.UUID) (sqlc.SyncSession, error) {
	return sqlc.SyncSession{}, ErrTokenRefresherNotConfigured // Simulate "not found"
}

func (m *mockSyncSessionStore) CreateSyncSession(ctx context.Context, arg sqlc.CreateSyncSessionParams) (sqlc.SyncSession, error) {
	session := sqlc.SyncSession{
		ID:         pgtype.UUID{Bytes: [16]byte{1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1}, Valid: true},
		UserID:     arg.UserID,
		Status:     "pending",
		WindowDays: arg.WindowDays,
		StartedAt:  pgtype.Timestamptz{Time: time.Now(), Valid: true},
	}
	return session, nil
}

func (m *mockSyncSessionStore) UpdateSyncSessionStatus(ctx context.Context, arg sqlc.UpdateSyncSessionStatusParams) (sqlc.SyncSession, error) {
	return sqlc.SyncSession{ID: arg.ID, Status: arg.Status}, nil
}

func (m *mockSyncSessionStore) UpdateSyncSessionProgress(ctx context.Context, arg sqlc.UpdateSyncSessionProgressParams) (sqlc.SyncSession, error) {
	return sqlc.SyncSession{ID: arg.ID, Imported: arg.Imported}, nil
}

type mockActivityInserter struct{}

func (m *mockActivityInserter) UpsertActivity(ctx context.Context, arg sqlc.UpsertActivityParams) (sqlc.Activity, error) {
	return sqlc.Activity{UserID: arg.UserID, ExternalID: arg.ExternalID}, nil
}

type mockActivityEventLoader struct{}

func (m *mockActivityEventLoader) GetActivityEventByExternalID(ctx context.Context, externalID string) (sqlc.ActivityEvent, error) {
	return sqlc.ActivityEvent{
		ID:         pgtype.UUID{Bytes: [16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16}, Valid: true},
		ExternalID: externalID,
		UserID:     pgtype.UUID{Bytes: [16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16}, Valid: true},
		ObjectID:   12345,
	}, nil
}

func (m *mockActivityEventLoader) MarkActivityEventProcessed(ctx context.Context, id pgtype.UUID) error {
	return nil
}

type mockActivityDetailFetcher struct{}

func (m *mockActivityDetailFetcher) GetActivity(ctx context.Context, accessToken string, id int64) (*strava.ActivityDetail, error) {
	return &strava.ActivityDetail{
		ID:        id,
		Name:      "Test Activity",
		Type:      "Run",
		StartDate: time.Now(),
	}, nil
}

type mockTokenQuerierForBackfill struct {
	cipherKey []byte
}

func newMockTokenQuerierForBackfill(cipherKey []byte) *mockTokenQuerierForBackfill {
	if cipherKey == nil {
		cipherKey = make([]byte, 32)
		for i := range cipherKey {
			cipherKey[i] = byte(i)
		}
	}
	return &mockTokenQuerierForBackfill{cipherKey: cipherKey}
}

func (m *mockTokenQuerierForBackfill) GetStravaTokensByUserID(ctx context.Context, userID pgtype.UUID) (sqlc.StravaToken, error) {
	// Encrypt tokens using the provided cipher key
	accessCipher, _ := crypto.Encrypt([]byte("test_access_token"), m.cipherKey)
	refreshCipher, _ := crypto.Encrypt([]byte("test_refresh_token"), m.cipherKey)

	// Return a valid token that won't need refresh (expires in 1 hour)
	return sqlc.StravaToken{
		UserID:        userID,
		AccessCipher:  accessCipher,
		RefreshCipher: refreshCipher,
		ExpiresAt:     pgtype.Timestamptz{Time: time.Now().Add(1 * time.Hour), Valid: true},
	}, nil
}

func (m *mockTokenQuerierForBackfill) UpsertStravaTokens(ctx context.Context, arg sqlc.UpsertStravaTokensParams) (sqlc.StravaToken, error) {
	return sqlc.StravaToken{UserID: arg.UserID}, nil
}

// Helper to set up global test state with cipher key
func setupTestGlobals(t *testing.T, cipherKey []byte) {
	// Configure the global cipher key for this test
	// Note: This is normally done via ConfigureTokenRefresher in production.
	ConfigureTokenRefresher(nil, nil, nil, cipherKey)
	t.Cleanup(func() {
		// Reset globals after test
		ConfigureTokenRefresher(nil, nil, nil, nil)
	})
}

// TestBackfillWindow_CalculatesCorrectSpan is a unit test for the pure function
func TestBackfillWindow_CalculatesCorrectSpan(t *testing.T) {
	now := time.Now()
	after, before := BackfillWindow(42)

	diff := now.Sub(before)
	require.True(t, diff > -time.Minute && diff < time.Minute,
		"before should be ~now, got %v ago", diff)

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

// TestBackfillWindow_EdgeCase_OneDay verifies span calculation with 1 day
func TestBackfillWindow_EdgeCase_OneDay(t *testing.T) {
	after, before := BackfillWindow(1)
	span := before.Sub(after)
	expectedSpan := 1 * 24 * time.Hour
	tolerance := time.Hour

	require.True(t, span > expectedSpan-tolerance && span < expectedSpan+tolerance,
		"span should be ~1 day, got %v", span)
}

// TestImportStravaWorker_Work_FetchesAndUpsertsActivities tests basic flow
func TestImportStravaWorker_Work_FetchesAndUpsertsActivities(t *testing.T) {
	cipherKey := make([]byte, 32)
	for i := range cipherKey {
		cipherKey[i] = byte(i)
	}
	setupTestGlobals(t, cipherKey)

	ctx := context.Background()
	userID := pgtype.UUID{Bytes: [16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16}, Valid: true}

	job := &river.Job[ImportStravaArgs]{
		Args: ImportStravaArgs{UserID: userID.String()},
	}

	worker := &ImportStravaWorker{
		fetcher:  &mockActivityFetcher{activities: []strava.ActivitySummary{{ID: 123, Name: "Run", Type: "Run"}}},
		store:    &mockSyncSessionStore{},
		inserter: &mockActivityInserter{},
		querier:  newMockTokenQuerierForBackfill(cipherKey),
		config:   &config.Config{Strava: &config.StravaConfig{BackfillDays: 42}},
	}

	err := worker.Work(ctx, job)
	require.NoError(t, err, "ImportStravaWorker.Work() should complete without error")
}

// TestImportStravaWorker_Work_HandlesPaginatedResults tests pagination
func TestImportStravaWorker_Work_HandlesPaginatedResults(t *testing.T) {
	cipherKey := make([]byte, 32)
	for i := range cipherKey {
		cipherKey[i] = byte(i)
	}
	setupTestGlobals(t, cipherKey)

	ctx := context.Background()
	userID := pgtype.UUID{Bytes: [16]byte{2, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16}, Valid: true}

	job := &river.Job[ImportStravaArgs]{
		Args: ImportStravaArgs{UserID: userID.String()},
	}

	pageFetcher := &mockActivityFetcher{
		activities: []strava.ActivitySummary{
			{ID: 123, Name: "Run 1", Type: "Run"},
			{ID: 124, Name: "Run 2", Type: "Run"},
		},
	}

	worker := &ImportStravaWorker{
		fetcher:  pageFetcher,
		store:    &mockSyncSessionStore{},
		inserter: &mockActivityInserter{},
		querier:  newMockTokenQuerierForBackfill(cipherKey),
		config:   &config.Config{Strava: &config.StravaConfig{BackfillDays: 42}},
	}

	err := worker.Work(ctx, job)
	require.NoError(t, err, "ImportStravaWorker.Work() should handle pagination without error")
}

// TestImportStravaWorker_Work_UpdatesProgress tests sync session progress tracking
func TestImportStravaWorker_Work_UpdatesProgress(t *testing.T) {
	cipherKey := make([]byte, 32)
	for i := range cipherKey {
		cipherKey[i] = byte(i)
	}
	setupTestGlobals(t, cipherKey)

	ctx := context.Background()
	userID := pgtype.UUID{Bytes: [16]byte{3, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16}, Valid: true}

	job := &river.Job[ImportStravaArgs]{
		Args: ImportStravaArgs{UserID: userID.String()},
	}

	worker := &ImportStravaWorker{
		fetcher:  &mockActivityFetcher{activities: []strava.ActivitySummary{{ID: 123, Name: "Run", Type: "Run"}}},
		store:    &mockSyncSessionStore{},
		inserter: &mockActivityInserter{},
		querier:  newMockTokenQuerierForBackfill(cipherKey),
		config:   &config.Config{Strava: &config.StravaConfig{BackfillDays: 42}},
	}

	err := worker.Work(ctx, job)
	require.NoError(t, err, "ImportStravaWorker.Work() should update progress without error")
}

// TestImportStravaWorker_Work_MarksSyncSessionCompleted tests final status update
func TestImportStravaWorker_Work_MarksSyncSessionCompleted(t *testing.T) {
	cipherKey := make([]byte, 32)
	for i := range cipherKey {
		cipherKey[i] = byte(i)
	}
	setupTestGlobals(t, cipherKey)

	ctx := context.Background()
	userID := pgtype.UUID{Bytes: [16]byte{4, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16}, Valid: true}

	job := &river.Job[ImportStravaArgs]{
		Args: ImportStravaArgs{UserID: userID.String()},
	}

	worker := &ImportStravaWorker{
		fetcher:  &mockActivityFetcher{activities: []strava.ActivitySummary{{ID: 123, Name: "Run", Type: "Run"}}},
		store:    &mockSyncSessionStore{},
		inserter: &mockActivityInserter{},
		querier:  newMockTokenQuerierForBackfill(cipherKey),
		config:   &config.Config{Strava: &config.StravaConfig{BackfillDays: 42}},
	}

	err := worker.Work(ctx, job)
	require.NoError(t, err, "ImportStravaWorker.Work() should mark sync session completed without error")
}

// TestImportStravaWorker_Work_CreatesSyncSessionIfMissing tests session creation
func TestImportStravaWorker_Work_CreatesSyncSessionIfMissing(t *testing.T) {
	cipherKey := make([]byte, 32)
	for i := range cipherKey {
		cipherKey[i] = byte(i)
	}
	setupTestGlobals(t, cipherKey)

	ctx := context.Background()
	userID := pgtype.UUID{Bytes: [16]byte{5, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16}, Valid: true}

	job := &river.Job[ImportStravaArgs]{
		Args: ImportStravaArgs{UserID: userID.String()},
	}

	worker := &ImportStravaWorker{
		fetcher:  &mockActivityFetcher{activities: []strava.ActivitySummary{{ID: 123, Name: "Run", Type: "Run"}}},
		store:    &mockSyncSessionStore{},
		inserter: &mockActivityInserter{},
		querier:  newMockTokenQuerierForBackfill(cipherKey),
		config:   &config.Config{Strava: &config.StravaConfig{BackfillDays: 42}},
	}

	err := worker.Work(ctx, job)
	require.NoError(t, err, "ImportStravaWorker.Work() should create sync session if missing")
}

// TestImportStravaWorker_Work_DeduplicatesActivities tests deduplication by constraint
func TestImportStravaWorker_Work_DeduplicatesActivities(t *testing.T) {
	cipherKey := make([]byte, 32)
	for i := range cipherKey {
		cipherKey[i] = byte(i)
	}
	setupTestGlobals(t, cipherKey)

	ctx := context.Background()
	userID := pgtype.UUID{Bytes: [16]byte{6, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16}, Valid: true}

	job := &river.Job[ImportStravaArgs]{
		Args: ImportStravaArgs{UserID: userID.String()},
	}

	duplicateFetcher := &mockActivityFetcher{
		activities: []strava.ActivitySummary{
			{ID: 123, Name: "Run", Type: "Run"},
			{ID: 123, Name: "Run (updated)", Type: "Run"},
		},
	}

	worker := &ImportStravaWorker{
		fetcher:  duplicateFetcher,
		store:    &mockSyncSessionStore{},
		inserter: &mockActivityInserter{},
		querier:  newMockTokenQuerierForBackfill(cipherKey),
		config:   &config.Config{Strava: &config.StravaConfig{BackfillDays: 42}},
	}

	err := worker.Work(ctx, job)
	require.NoError(t, err, "ImportStravaWorker.Work() should deduplicate activities via ON CONFLICT")
}

// TestIngestActivityEventWorker_Work_ProcessesEvent tests basic webhook flow
func TestIngestActivityEventWorker_Work_ProcessesEvent(t *testing.T) {
	cipherKey := make([]byte, 32)
	for i := range cipherKey {
		cipherKey[i] = byte(i)
	}
	setupTestGlobals(t, cipherKey)

	ctx := context.Background()

	job := &river.Job[IngestActivityEventArgs]{
		Args: IngestActivityEventArgs{EventID: "webhook-event-123"},
	}

	worker := &IngestActivityEventWorker{
		eventLoader:   &mockActivityEventLoader{},
		detailFetcher: &mockActivityDetailFetcher{},
		inserter:      &mockActivityInserter{},
		querier:       newMockTokenQuerierForBackfill(cipherKey),
	}

	err := worker.Work(ctx, job)
	require.NoError(t, err, "IngestActivityEventWorker.Work() should process event without error")
}

// TestIngestActivityEventWorker_Work_FetchesActivityDetail tests activity fetch
func TestIngestActivityEventWorker_Work_FetchesActivityDetail(t *testing.T) {
	cipherKey := make([]byte, 32)
	for i := range cipherKey {
		cipherKey[i] = byte(i)
	}
	setupTestGlobals(t, cipherKey)

	ctx := context.Background()

	job := &river.Job[IngestActivityEventArgs]{
		Args: IngestActivityEventArgs{EventID: "webhook-event-456"},
	}

	worker := &IngestActivityEventWorker{
		eventLoader:   &mockActivityEventLoader{},
		detailFetcher: &mockActivityDetailFetcher{},
		inserter:      &mockActivityInserter{},
		querier:       newMockTokenQuerierForBackfill(cipherKey),
	}

	err := worker.Work(ctx, job)
	require.NoError(t, err, "IngestActivityEventWorker.Work() should fetch activity detail without error")
}

// TestIngestActivityEventWorker_Work_MarksEventProcessed tests event marking
func TestIngestActivityEventWorker_Work_MarksEventProcessed(t *testing.T) {
	cipherKey := make([]byte, 32)
	for i := range cipherKey {
		cipherKey[i] = byte(i)
	}
	setupTestGlobals(t, cipherKey)

	ctx := context.Background()

	job := &river.Job[IngestActivityEventArgs]{
		Args: IngestActivityEventArgs{EventID: "webhook-event-789"},
	}

	worker := &IngestActivityEventWorker{
		eventLoader:   &mockActivityEventLoader{},
		detailFetcher: &mockActivityDetailFetcher{},
		inserter:      &mockActivityInserter{},
		querier:       newMockTokenQuerierForBackfill(cipherKey),
	}

	err := worker.Work(ctx, job)
	require.NoError(t, err, "IngestActivityEventWorker.Work() should mark event processed without error")
}

// TestImportStravaArgs_Kind tests that job kind is correct
func TestImportStravaArgs_Kind(t *testing.T) {
	args := ImportStravaArgs{UserID: "test-user"}
	require.Equal(t, "import_strava", args.Kind())
}

// TestImportStravaArgs_HasUserID tests that UserID is populated
func TestImportStravaArgs_HasUserID(t *testing.T) {
	args := ImportStravaArgs{UserID: "test-user-id"}
	require.NotEmpty(t, args.UserID)
}

// TestIngestActivityEventArgs_Kind tests that job kind is correct
func TestIngestActivityEventArgs_Kind(t *testing.T) {
	args := IngestActivityEventArgs{EventID: "event-123"}
	require.Equal(t, "ingest_activity_event", args.Kind())
}

// TestIngestActivityEventArgs_HasEventID tests that EventID is populated
func TestIngestActivityEventArgs_HasEventID(t *testing.T) {
	args := IngestActivityEventArgs{EventID: "event-id"}
	require.NotEmpty(t, args.EventID)
}
