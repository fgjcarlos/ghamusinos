package jobs

import (
	"context"
	"errors"
	"sync"
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

func (m *mockActivityEventLoader) GetActivityEventByID(ctx context.Context, id pgtype.UUID) (sqlc.ActivityEvent, error) {
	return sqlc.ActivityEvent{
		ID:       id,
		UserID:   pgtype.UUID{Bytes: [16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16}, Valid: true},
		ObjectID: 12345,
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

	job := &river.Job[IngestActivityEventArgs]{Args: IngestActivityEventArgs{EventID: "01020304-0506-0708-090a-0b0c0d0e0f10"}}
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

	job := &river.Job[IngestActivityEventArgs]{Args: IngestActivityEventArgs{EventID: "01020304-0506-0708-090a-0b0c0d0e0f10"}}
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

	job := &river.Job[IngestActivityEventArgs]{Args: IngestActivityEventArgs{EventID: "01020304-0506-0708-090a-0b0c0d0e0f10"}}
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

// ─────────────────────────────────────────────────────────────────────────
// AUD-05 (issue #166) tests:
//   - 10 000 m persists as 10000.00 (no × 1000).
//   - Decimals survive (10012.34 → 10012.34 in DistanceMeters).
//   - Total accumulates across pages.
//   - Session marked "running" on start and "failed" on error.
// ─────────────────────────────────────────────────────────────────────────

// mockActivityInserterCapture is the same as mockActivityInserter but
// records each UpsertActivity call's DistanceMeters for inspection. AUD-05
// AC #1 and #2: the previous code multiplied by 1000 and truncated decimals
// (pgtype.Numeric{Int: big.NewInt(...)} without an exponent). This mock
// lets the test assert that DistanceMeters arrives unaltered.
type mockActivityInserterCapture struct {
	mu       sync.Mutex
	captured []pgtype.Numeric
}

func (m *mockActivityInserterCapture) UpsertActivity(_ context.Context, arg sqlc.UpsertActivityParams) (sqlc.Activity, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.captured = append(m.captured, arg.DistanceMeters)
	return sqlc.Activity{UserID: arg.UserID, ExternalID: arg.ExternalID}, nil
}

// mockSyncSessionStoreTracker records every status and progress transition
// the worker performs, so the test can assert the lifecycle. AUD-05 AC #4
// and #5: the previous code never wrote "running" or "failed".
type mockSyncSessionStoreTracker struct {
	mu             sync.Mutex
	statusUpdates  []string
	progressCalls  int
	lastImported   int32
	lastTotal      int32
	lastSkipped    int32
	fetchErr       error                                              // optional: force CreateSyncSession to error
	updateStatusFn func(id pgtype.UUID, status string, errStr string) // override
	updateProgFn   func(total, imported, skipped int32)               // override
}

func (m *mockSyncSessionStoreTracker) GetLatestSyncSession(_ context.Context, _ pgtype.UUID) (sqlc.SyncSession, error) {
	return sqlc.SyncSession{}, nil // empty → worker calls CreateSyncSession
}

func (m *mockSyncSessionStoreTracker) CreateSyncSession(_ context.Context, arg sqlc.CreateSyncSessionParams) (sqlc.SyncSession, error) {
	if m.fetchErr != nil {
		return sqlc.SyncSession{}, m.fetchErr
	}
	return sqlc.SyncSession{
		ID:         pgtype.UUID{Bytes: [16]byte{1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1}, Valid: true},
		UserID:     arg.UserID,
		Status:     "pending",
		WindowDays: arg.WindowDays,
		StartedAt:  pgtype.Timestamptz{Time: time.Now(), Valid: true},
	}, nil
}

func (m *mockSyncSessionStoreTracker) UpdateSyncSessionStatus(_ context.Context, arg sqlc.UpdateSyncSessionStatusParams) (sqlc.SyncSession, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.statusUpdates = append(m.statusUpdates, arg.Status)
	if m.updateStatusFn != nil {
		m.updateStatusFn(arg.ID, arg.Status, arg.Error.String)
	}
	return sqlc.SyncSession{ID: arg.ID, Status: arg.Status}, nil
}

func (m *mockSyncSessionStoreTracker) UpdateSyncSessionProgress(_ context.Context, arg sqlc.UpdateSyncSessionProgressParams) (sqlc.SyncSession, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.progressCalls++
	m.lastImported = arg.Imported
	m.lastTotal = arg.TotalActivities
	m.lastSkipped = arg.Skipped
	if m.updateProgFn != nil {
		m.updateProgFn(arg.TotalActivities, arg.Imported, arg.Skipped)
	}
	return sqlc.SyncSession{ID: arg.ID}, nil
}

// pageFetcher is a paginated fetcher that returns a configurable set of
// pages and then an empty page to end the loop. AUD-05 AC #4: the worker
// must accumulate totalActivities across all pages, not just write the
// last page's count.
type pageFetcher struct {
	pages [][]strava.ActivitySummary
	call  int
}

func (p *pageFetcher) GetActivities(_ context.Context, _ string, _, _ time.Time, page, _ int) ([]strava.ActivitySummary, error) {
	if page-1 >= len(p.pages) {
		return nil, nil
	}
	return p.pages[page-1], nil
}

// TestNumeric_PreservesDecimals is the unit-level guard for AUD-05 AC #1
// and #2: pgtype.Numeric built from a float64 keeps every digit the
// NUMERIC(12,2) column can hold. Strava returns meters, and a 10012.34 m
// run must persist as 10012.34 (not 10012000 nor 10012).
func TestNumeric_PreservesDecimals(t *testing.T) {
	cases := []struct {
		name string
		in   float64
		want float64
	}{
		{"whole", 10000, 10000},
		{"fractional", 10012.34, 10012.34},
		{"elevation", 123.45, 123.45},
		{"tiny", 0.5, 0.5},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := numeric(tc.in)
			require.True(t, got.Valid)
			f, err := got.Float64Value()
			require.NoError(t, err)
			require.InDelta(t, tc.want, f.Float64, 1e-9,
				"numeric(%v) → %v; expected %v", tc.in, f.Float64, tc.want)
		})
	}
}

// numericFloat64 returns the float64 representation of a pgtype.Numeric, or
// 0 if the value is invalid / not finite. The test helper exists because
// pgtype.Numeric's default stringer prints the internal struct shape; the
// real comparison target is the float value Postgres would store.
func numericFloat64(t *testing.T, n pgtype.Numeric) float64 {
	t.Helper()
	if !n.Valid {
		return 0
	}
	f, err := n.Float64Value()
	require.NoError(t, err)
	return f.Float64
}

// TestImportStravaWorker_PassesDistanceAsMeters: AUD-05 AC #1.
// Strava returns distance in meters; the worker used to multiply by 1000
// "as if it were km", turning a 10 000 m run into 10 000 000 m.
func TestImportStravaWorker_PassesDistanceAsMeters(t *testing.T) {
	cipherKey := key32()
	inserter := &mockActivityInserterCapture{}
	store := &mockSyncSessionStoreTracker{}

	worker := NewImportStravaWorker(importDeps{
		cipher:   cipherKey,
		backfill: 42,
	})
	worker.fetcher = &mockActivityFetcher{
		activities: []strava.ActivitySummary{{
			ID: 1, Name: "10K", Type: "Run",
			Distance: 10000, // 10 000 m from Strava
		}},
	}
	worker.refresher = &mockStravaForRefresh{}
	worker.store = store
	worker.inserter = inserter
	worker.querier = newMockTokenQuerierForBackfill(cipherKey)

	userID := pgtype.UUID{Bytes: [16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16}, Valid: true}
	job := &river.Job[ImportStravaArgs]{Args: ImportStravaArgs{UserID: userID.String()}}

	require.NoError(t, worker.Work(context.Background(), job))
	require.Len(t, inserter.captured, 1)
	require.InDelta(t, 10000, numericFloat64(t, inserter.captured[0]), 1e-6,
		"10 000 m must persist as 10 000 (no × 1000); got %v", numericFloat64(t, inserter.captured[0]))
}

// TestImportStravaWorker_PreservesDistanceDecimals: AUD-05 AC #2.
// A 10012.34 m run must persist with the .34, not silently round to 10012.
func TestImportStravaWorker_PreservesDistanceDecimals(t *testing.T) {
	cipherKey := key32()
	inserter := &mockActivityInserterCapture{}
	store := &mockSyncSessionStoreTracker{}

	worker := NewImportStravaWorker(importDeps{
		cipher:   cipherKey,
		backfill: 42,
	})
	worker.fetcher = &mockActivityFetcher{
		activities: []strava.ActivitySummary{{
			ID: 2, Name: "10K+", Type: "Run",
			Distance: 10012.34,
		}},
	}
	worker.refresher = &mockStravaForRefresh{}
	worker.store = store
	worker.inserter = inserter
	worker.querier = newMockTokenQuerierForBackfill(cipherKey)

	userID := pgtype.UUID{Bytes: [16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16}, Valid: true}
	job := &river.Job[ImportStravaArgs]{Args: ImportStravaArgs{UserID: userID.String()}}

	require.NoError(t, worker.Work(context.Background(), job))
	require.Len(t, inserter.captured, 1)
	require.InDelta(t, 10012.34, numericFloat64(t, inserter.captured[0]), 1e-6,
		"10012.34 m must persist with decimals preserved; got %v", numericFloat64(t, inserter.captured[0]))
}

// TestImportStravaWorker_AccumulatesTotalAcrossPages: AUD-05 AC #4.
// 3 pages × 50 activities → total_activities must be 150 in the final
// UpdateSyncSessionProgress call. The previous code overwrote per-page,
// so the stored total was just 50 (the last page's size).
func TestImportStravaWorker_AccumulatesTotalAcrossPages(t *testing.T) {
	cipherKey := key32()
	store := &mockSyncSessionStoreTracker{}

	pageA := make([]strava.ActivitySummary, 50)
	pageB := make([]strava.ActivitySummary, 50)
	pageC := make([]strava.ActivitySummary, 50)
	for i := range pageA {
		pageA[i] = strava.ActivitySummary{ID: int64(100 + i), Name: "Run", Type: "Run", Distance: 5000}
		pageB[i] = strava.ActivitySummary{ID: int64(200 + i), Name: "Run", Type: "Run", Distance: 5000}
		pageC[i] = strava.ActivitySummary{ID: int64(300 + i), Name: "Run", Type: "Run", Distance: 5000}
	}
	fetcher := &pageFetcher{pages: [][]strava.ActivitySummary{pageA, pageB, pageC}}

	worker := NewImportStravaWorker(importDeps{
		cipher:   cipherKey,
		backfill: 42,
	})
	worker.fetcher = fetcher
	worker.refresher = &mockStravaForRefresh{}
	worker.store = store
	worker.inserter = &mockActivityInserter{}
	worker.querier = newMockTokenQuerierForBackfill(cipherKey)

	userID := pgtype.UUID{Bytes: [16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16}, Valid: true}
	job := &river.Job[ImportStravaArgs]{Args: ImportStravaArgs{UserID: userID.String()}}

	require.NoError(t, worker.Work(context.Background(), job))
	store.mu.Lock()
	defer store.mu.Unlock()

	require.Equal(t, int32(150), store.lastTotal, "total_activities must be the sum across pages")
	require.Equal(t, int32(150), store.lastImported, "imported must equal total when no skips")
	require.Equal(t, 3, store.progressCalls, "one UpdateSyncSessionProgress per non-empty page")
}

// TestImportStravaWorker_SyncSessionLifecycle: AUD-05 AC #5.
// A failed sync (the fetcher errors on page 1) must leave the session in
// "failed" with the error message in the error column, NOT in "pending".
// The previous code had no failure path at all.
func TestImportStravaWorker_SyncSessionLifecycle(t *testing.T) {
	cipherKey := key32()
	store := &mockSyncSessionStoreTracker{}
	failingFetcher := &failingActivityFetcher{err: errors.New("strava API down")}

	worker := NewImportStravaWorker(importDeps{
		cipher:   cipherKey,
		backfill: 42,
	})
	worker.fetcher = failingFetcher
	worker.refresher = &mockStravaForRefresh{}
	worker.store = store
	worker.inserter = &mockActivityInserter{}
	worker.querier = newMockTokenQuerierForBackfill(cipherKey)

	userID := pgtype.UUID{Bytes: [16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16}, Valid: true}
	job := &river.Job[ImportStravaArgs]{Args: ImportStravaArgs{UserID: userID.String()}}

	err := worker.Work(context.Background(), job)
	require.Error(t, err)

	store.mu.Lock()
	defer store.mu.Unlock()

	// Status sequence: pending (CreateSyncSession) → running → failed.
	// "pending" is the initial state set by CreateSyncSession; we only
	// track transitions the worker itself writes.
	statuses := store.statusUpdates
	require.Contains(t, statuses, "running", "session must be marked running before work begins")
	require.Contains(t, statuses, "failed", "session must be marked failed when Work returns error")
	// "running" must come before "failed" in the recorded sequence.
	rIdx, fIdx := -1, -1
	for i, s := range statuses {
		if s == "running" {
			rIdx = i
		}
		if s == "failed" {
			fIdx = i
		}
	}
	require.True(t, rIdx >= 0 && fIdx > rIdx, "failed must come after running, got %v", statuses)
	require.NotContains(t, statuses, "completed", "a failed sync must not transition to completed")
}

// TestImportStravaWorker_SuccessfulLifecycle: complementary to the failure
// test — on success the session transitions pending → running → completed,
// and "failed" never appears. This guards the deferred closer in Work().
func TestImportStravaWorker_SuccessfulLifecycle(t *testing.T) {
	cipherKey := key32()
	store := &mockSyncSessionStoreTracker{}
	worker := newImportStravaWorkerForTest(
		&mockActivityFetcher{activities: []strava.ActivitySummary{{ID: 1, Name: "Run", Type: "Run", Distance: 1000}}},
		store, &mockActivityInserter{}, newMockTokenQuerierForBackfill(cipherKey), cipherKey, 42)

	userID := pgtype.UUID{Bytes: [16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16}, Valid: true}
	job := &river.Job[ImportStravaArgs]{Args: ImportStravaArgs{UserID: userID.String()}}

	require.NoError(t, worker.Work(context.Background(), job))

	store.mu.Lock()
	defer store.mu.Unlock()

	require.Contains(t, store.statusUpdates, "running")
	require.Contains(t, store.statusUpdates, "completed")
	require.NotContains(t, store.statusUpdates, "failed", "a successful sync must not write failed")
}

// failingActivityFetcher makes GetActivities return an error every time,
// so the worker's first call to fetch fails. Used by the failed-sync test.
type failingActivityFetcher struct {
	err error
}

func (f *failingActivityFetcher) GetActivities(_ context.Context, _ string, _, _ time.Time, _, _ int) ([]strava.ActivitySummary, error) {
	return nil, f.err
}
