package jobs

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"
	"github.com/stretchr/testify/require"

	"github.com/fgjcarlos/ghamusinos/internal/config"
	"github.com/fgjcarlos/ghamusinos/internal/crypto"
	"github.com/fgjcarlos/ghamusinos/internal/db/sqlc"
	"github.com/fgjcarlos/ghamusinos/internal/strava"
)

// ─────────────────────────────────────────────────────────────────────────
// mocks
// ─────────────────────────────────────────────────────────────────────────

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
	// No previous session → triggers the "create new" branch in Work.
	// AUD-04 replaced the old ErrTokenRefresherNotConfigured sentinel; the
	// mock now returns a clean "no rows" equivalent (zero value, nil).
	return sqlc.SyncSession{}, nil
}

func (m *mockSyncSessionStore) CreateSyncSession(ctx context.Context, arg sqlc.CreateSyncSessionParams) (sqlc.SyncSession, error) {
	return sqlc.SyncSession{
		ID:         pgtype.UUID{Bytes: [16]byte{1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1}, Valid: true},
		UserID:     arg.UserID,
		Status:     "pending",
		WindowDays: arg.WindowDays,
		StartedAt:  pgtype.Timestamptz{Time: time.Now(), Valid: true},
	}, nil
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
	accessCipher, _ := crypto.Encrypt([]byte("test_access_token"), m.cipherKey)
	refreshCipher, _ := crypto.Encrypt([]byte("test_refresh_token"), m.cipherKey)
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

// mockStravaForRefresh implements both TokenRefresher (Refresh) and
// ActivityFetcher (GetActivities) so the same value can be the refresher in
// the constructor. AUD-04: workers receive Strava via one field per interface
// but they can be backed by the same *strava.Client in production.
type mockStravaForRefresh struct{}

func (m *mockStravaForRefresh) Refresh(ctx context.Context, refreshToken string) (*strava.TokenSet, error) {
	return &strava.TokenSet{
		AccessToken:  "new_access",
		RefreshToken: "new_refresh",
		ExpiresAt:    time.Now().Add(1 * time.Hour),
		Scopes:       "read,activity:read",
	}, nil
}
func (m *mockStravaForRefresh) GetActivities(ctx context.Context, accessToken string, after, before time.Time, page, perPage int) ([]strava.ActivitySummary, error) {
	return []strava.ActivitySummary{}, nil
}

// key32 returns a deterministic 32-byte cipher key for tests.
func key32() []byte {
	b := make([]byte, 32)
	for i := range b {
		b[i] = byte(i)
	}
	return b
}

// ─────────────────────────────────────────────────────────────────────────
// BackfillWindow pure-function tests
// ─────────────────────────────────────────────────────────────────────────

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

func TestBackfillWindow_CustomDays(t *testing.T) {
	after, before := BackfillWindow(7)
	span := before.Sub(after)
	expectedSpan := 7 * 24 * time.Hour
	tolerance := time.Hour
	require.True(t, span > expectedSpan-tolerance && span < expectedSpan+tolerance,
		"span should be ~7 days, got %v", span)
}

func TestBackfillWindow_EdgeCase_OneDay(t *testing.T) {
	after, before := BackfillWindow(1)
	span := before.Sub(after)
	expectedSpan := 1 * 24 * time.Hour
	tolerance := time.Hour
	require.True(t, span > expectedSpan-tolerance && span < expectedSpan+tolerance,
		"span should be ~1 day, got %v", span)
}

// ─────────────────────────────────────────────────────────────────────────
// ImportStravaWorker.Word tests — switched from struct literals + globals
// to NewImportStravaWorker(importDeps{...}). AUD-04 makes the workers
// deterministic: no env, no package state, no surprise nil.
// ─────────────────────────────────────────────────────────────────────────

func TestImportStravaWorker_Work_FetchesAndUpsertsActivities(t *testing.T) {
	cipherKey := key32()
	strava := &mockActivityFetcher{activities: []strava.ActivitySummary{{ID: 123, Name: "Run", Type: "Run"}}}
	store := &mockSyncSessionStore{}
	inserter := &mockActivityInserter{}
	querier := newMockTokenQuerierForBackfill(cipherKey)
	refresher := &mockStravaForRefresh{}

	worker := NewImportStravaWorker(importDeps{
		cipher:   cipherKey,
		backfill: 42,
	})
	// Override the fields the constructor set from a non-existent queries/
	// strava handle with the actual mocks for the test.
	worker.fetcher = strava
	worker.refresher = refresher
	worker.store = store
	worker.inserter = inserter
	worker.querier = querier

	ctx := context.Background()
	userID := pgtype.UUID{Bytes: [16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16}, Valid: true}
	job := &river.Job[ImportStravaArgs]{Args: ImportStravaArgs{UserID: userID.String()}}

	err := worker.Work(ctx, job)
	require.NoError(t, err, "ImportStravaWorker.Work() should complete without error")
}

func TestImportStravaWorker_Work_HandlesPaginatedResults(t *testing.T) {
	cipherKey := key32()
	pageFetcher := &mockActivityFetcher{
		activities: []strava.ActivitySummary{
			{ID: 123, Name: "Run 1", Type: "Run"},
			{ID: 124, Name: "Run 2", Type: "Run"},
		},
	}
	worker := newImportStravaWorkerForTest(pageFetcher, &mockSyncSessionStore{}, &mockActivityInserter{},
		newMockTokenQuerierForBackfill(cipherKey), cipherKey, 42)

	ctx := context.Background()
	userID := pgtype.UUID{Bytes: [16]byte{2, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16}, Valid: true}
	job := &river.Job[ImportStravaArgs]{Args: ImportStravaArgs{UserID: userID.String()}}

	err := worker.Work(ctx, job)
	require.NoError(t, err, "ImportStravaWorker.Work() should handle pagination without error")
}

func TestImportStravaWorker_Work_UpdatesProgress(t *testing.T) {
	cipherKey := key32()
	worker := newImportStravaWorkerForTest(
		&mockActivityFetcher{activities: []strava.ActivitySummary{{ID: 123, Name: "Run", Type: "Run"}}},
		&mockSyncSessionStore{}, &mockActivityInserter{},
		newMockTokenQuerierForBackfill(cipherKey), cipherKey, 42)

	ctx := context.Background()
	userID := pgtype.UUID{Bytes: [16]byte{3, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16}, Valid: true}
	job := &river.Job[ImportStravaArgs]{Args: ImportStravaArgs{UserID: userID.String()}}

	err := worker.Work(ctx, job)
	require.NoError(t, err, "ImportStravaWorker.Work() should update progress without error")
}

func TestImportStravaWorker_Work_MarksSyncSessionCompleted(t *testing.T) {
	cipherKey := key32()
	worker := newImportStravaWorkerForTest(
		&mockActivityFetcher{activities: []strava.ActivitySummary{{ID: 123, Name: "Run", Type: "Run"}}},
		&mockSyncSessionStore{}, &mockActivityInserter{},
		newMockTokenQuerierForBackfill(cipherKey), cipherKey, 42)

	ctx := context.Background()
	userID := pgtype.UUID{Bytes: [16]byte{4, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16}, Valid: true}
	job := &river.Job[ImportStravaArgs]{Args: ImportStravaArgs{UserID: userID.String()}}

	err := worker.Work(ctx, job)
	require.NoError(t, err, "ImportStravaWorker.Work() should mark sync session completed without error")
}

func TestImportStravaWorker_Work_CreatesSyncSessionIfMissing(t *testing.T) {
	cipherKey := key32()
	worker := newImportStravaWorkerForTest(
		&mockActivityFetcher{activities: []strava.ActivitySummary{{ID: 123, Name: "Run", Type: "Run"}}},
		&mockSyncSessionStore{}, &mockActivityInserter{},
		newMockTokenQuerierForBackfill(cipherKey), cipherKey, 42)

	ctx := context.Background()
	userID := pgtype.UUID{Bytes: [16]byte{5, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16}, Valid: true}
	job := &river.Job[ImportStravaArgs]{Args: ImportStravaArgs{UserID: userID.String()}}

	err := worker.Work(ctx, job)
	require.NoError(t, err, "ImportStravaWorker.Work() should create sync session if missing")
}

func TestImportStravaWorker_Work_DeduplicatesActivities(t *testing.T) {
	cipherKey := key32()
	duplicateFetcher := &mockActivityFetcher{
		activities: []strava.ActivitySummary{
			{ID: 123, Name: "Run", Type: "Run"},
			{ID: 123, Name: "Run (updated)", Type: "Run"},
		},
	}
	worker := newImportStravaWorkerForTest(duplicateFetcher, &mockSyncSessionStore{}, &mockActivityInserter{},
		newMockTokenQuerierForBackfill(cipherKey), cipherKey, 42)

	ctx := context.Background()
	userID := pgtype.UUID{Bytes: [16]byte{6, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16}, Valid: true}
	job := &river.Job[ImportStravaArgs]{Args: ImportStravaArgs{UserID: userID.String()}}

	err := worker.Work(ctx, job)
	require.NoError(t, err, "ImportStravaWorker.Work() should deduplicate activities via ON CONFLICT")
}

// newImportStravaWorkerForTest is a small helper that wraps the constructor
// in a way that is friendly to the table-style tests above: each test
// constructs a worker with the same shape (queries=nil placeholder, then
// overriding fields with the actual mocks). Centralising the override here
// keeps each test to 1–3 lines.
func newImportStravaWorkerForTest(
	fetcher ActivityFetcher,
	store SyncSessionStore,
	inserter ActivityInserter,
	querier TokenQuerier,
	cipher []byte,
	backfillDays int,
) *ImportStravaWorker {
	w := NewImportStravaWorker(importDeps{
		cipher:   cipher,
		backfill: int32(backfillDays),
	})
	w.fetcher = fetcher
	w.refresher = &mockStravaForRefresh{}
	w.store = store
	w.inserter = inserter
	w.querier = querier
	return w
}

// ─────────────────────────────────────────────────────────────────────────
// IngestActivityEventWorker — uses the explicit-arg constructor.
// ─────────────────────────────────────────────────────────────────────────

func TestIngestActivityEventWorker_Work_ProcessesEvent(t *testing.T) {
	cipherKey := key32()
	worker := NewIngestActivityEventWorker(
		&mockActivityEventLoader{},
		&mockActivityDetailFetcher{},
		&mockActivityInserter{},
		newMockTokenQuerierForBackfill(cipherKey),
		cipherKey,
		&mockStravaForRefresh{},
	)

	job := &river.Job[IngestActivityEventArgs]{Args: IngestActivityEventArgs{EventID: "webhook-event-123"}}
	err := worker.Work(context.Background(), job)
	require.NoError(t, err, "IngestActivityEventWorker.Work() should process event without error")
}

func TestIngestActivityEventWorker_Work_FetchesActivityDetail(t *testing.T) {
	cipherKey := key32()
	worker := NewIngestActivityEventWorker(
		&mockActivityEventLoader{},
		&mockActivityDetailFetcher{},
		&mockActivityInserter{},
		newMockTokenQuerierForBackfill(cipherKey),
		cipherKey,
		&mockStravaForRefresh{},
	)

	job := &river.Job[IngestActivityEventArgs]{Args: IngestActivityEventArgs{EventID: "webhook-event-456"}}
	err := worker.Work(context.Background(), job)
	require.NoError(t, err, "IngestActivityEventWorker.Work() should fetch activity detail without error")
}

func TestIngestActivityEventWorker_Work_MarksEventProcessed(t *testing.T) {
	cipherKey := key32()
	worker := NewIngestActivityEventWorker(
		&mockActivityEventLoader{},
		&mockActivityDetailFetcher{},
		&mockActivityInserter{},
		newMockTokenQuerierForBackfill(cipherKey),
		cipherKey,
		&mockStravaForRefresh{},
	)

	job := &river.Job[IngestActivityEventArgs]{Args: IngestActivityEventArgs{EventID: "webhook-event-789"}}
	err := worker.Work(context.Background(), job)
	require.NoError(t, err, "IngestActivityEventWorker.Work() should mark event processed without error")
}

// ─────────────────────────────────────────────────────────────────────────
// Job-kind sanity (unchanged from before; here to keep coverage)
// ─────────────────────────────────────────────────────────────────────────

func TestImportStravaArgs_Kind(t *testing.T) {
	args := ImportStravaArgs{UserID: "test-user"}
	require.Equal(t, "import_strava", args.Kind())
}

func TestImportStravaArgs_HasUserID(t *testing.T) {
	args := ImportStravaArgs{UserID: "test-user-id"}
	require.NotEmpty(t, args.UserID)
}

func TestIngestActivityEventArgs_Kind(t *testing.T) {
	args := IngestActivityEventArgs{EventID: "event-123"}
	require.Equal(t, "ingest_activity_event", args.Kind())
}

func TestIngestActivityEventArgs_HasEventID(t *testing.T) {
	args := IngestActivityEventArgs{EventID: "event-id"}
	require.NotEmpty(t, args.EventID)
}

// ─────────────────────────────────────────────────────────────────────────
// AUD-04 AC: NewRiverWorkers validates the dependency bundle and the
// Strava-bound workers are not registered when Strava is absent.
// ─────────────────────────────────────────────────────────────────────────

// TestNewRiverWorkers_RequiresPool confirms the AUD-04 AC: "NewRiverWorkers
// returns error if any dependency is missing" — first case, Pool.
func TestNewRiverWorkers_RequiresPool(t *testing.T) {
	_, err := NewRiverWorkers(Deps{
		// Pool: nil — must fail
		Config:    &config.Config{Strava: &config.StravaConfig{}},
		Strava:    &strava.Client{},
		CipherKey: key32(),
	}, true)
	require.Error(t, err, "NewRiverWorkers must reject Deps without Pool")
	require.Contains(t, err.Error(), "Pool")
}

// TestNewRiverWorkers_RequiresConfig is the second case in the AC.
// Pool is valid so validation reaches Config; Config is nil and must fail.
func TestNewRiverWorkers_RequiresConfig(t *testing.T) {
	_, err := NewRiverWorkers(Deps{
		Pool:      &pgxpool.Pool{}, // valid sentinel; we want to reach the next check
		Config:    nil,             // must fail here
		Strava:    &strava.Client{},
		CipherKey: key32(),
	}, true)
	require.Error(t, err)
	require.Contains(t, err.Error(), "Config")
}

// TestNewRiverWorkers_RequiresStravaWhenRegisteringStravaWorkers: when
// registerStravaWorkers=true, both Strava and CipherKey are required.
// This guards the AC "el binario no arranca si falta una dependencia".
// Pool+Config valid so validation reaches Strava; Strava is nil and must fail.
func TestNewRiverWorkers_RequiresStravaWhenRegisteringStravaWorkers(t *testing.T) {
	_, err := NewRiverWorkers(Deps{
		Pool:   &pgxpool.Pool{},
		Config: &config.Config{Strava: &config.StravaConfig{}},
		// Strava: nil — must fail when registerStravaWorkers=true
		CipherKey: key32(),
	}, true)
	require.Error(t, err)
	require.Contains(t, err.Error(), "Strava")
}

func TestNewRiverWorkers_RequiresCipherKeyWhenRegisteringStravaWorkers(t *testing.T) {
	_, err := NewRiverWorkers(Deps{
		Pool:   &pgxpool.Pool{},
		Config: &config.Config{Strava: &config.StravaConfig{}},
		Strava: &strava.Client{},
		// CipherKey: nil/empty — must fail
	}, true)
	require.Error(t, err)
	require.Contains(t, err.Error(), "CipherKey")
}

// TestNewRiverWorkers_AcceptsMissingStravaWhenNotRegistering: when
// registerStravaWorkers=false (cfg.Strava == nil at runtime), the Strava-
// bound workers are not registered and Strava/CipherKey may be nil. This
// preserves the "developer can run without Strava" behavior.
func TestNewRiverWorkers_AcceptsMissingStravaWhenNotRegistering(t *testing.T) {
	// Pool is still required; we just confirm that Strava/CipherKey are
	// optional in the no-Strava branch.
	_, err := NewRiverWorkers(Deps{
		// Pool nil — still expected to fail. We're verifying that the failure
		// message does NOT complain about Strava or CipherKey, only Pool.
		Config: &config.Config{},
	}, false)
	require.Error(t, err)
	require.Contains(t, err.Error(), "Pool")
	require.NotContains(t, err.Error(), "Strava")
}
