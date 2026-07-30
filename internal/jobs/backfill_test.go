package jobs

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/fgjcarlos/ghamusinos/internal/crypto"
	"github.com/fgjcarlos/ghamusinos/internal/db/sqlc"
	"github.com/fgjcarlos/ghamusinos/internal/strava"
)

// ====== Test Helpers ======

// createTestCipherKey creates a 32-byte key for AES-256 testing.
func createTestCipherKey() []byte {
	return []byte("0123456789abcdef0123456789abcdef")
}

// encryptTestToken encrypts a test token string using the test cipher key.
func encryptTestToken(plaintext string) string {
	encrypted, err := crypto.Encrypt([]byte(plaintext), createTestCipherKey())
	if err != nil {
		panic(err)
	}
	return encrypted
}

// ====== Mock Implementations ======

type mockActivityFetcher struct {
	getActivitiesFn func(ctx context.Context, accessToken string, after, before time.Time, page, perPage int) ([]strava.ActivitySummary, error)
	getActivityFn   func(ctx context.Context, accessToken string, id int64) (*strava.ActivityDetail, error)
}

func (m *mockActivityFetcher) GetActivities(ctx context.Context, accessToken string, after, before time.Time, page, perPage int) ([]strava.ActivitySummary, error) {
	if m.getActivitiesFn == nil {
		return nil, errors.New("mock: GetActivities not configured")
	}
	return m.getActivitiesFn(ctx, accessToken, after, before, page, perPage)
}

func (m *mockActivityFetcher) GetActivity(ctx context.Context, accessToken string, id int64) (*strava.ActivityDetail, error) {
	if m.getActivityFn == nil {
		return nil, errors.New("mock: GetActivity not configured")
	}
	return m.getActivityFn(ctx, accessToken, id)
}

type mockSyncSessionStore struct {
	getLatestSyncSessionFn     func(ctx context.Context, userID pgtype.UUID) (sqlc.SyncSession, error)
	createSyncSessionFn        func(ctx context.Context, arg sqlc.CreateSyncSessionParams) (sqlc.SyncSession, error)
	updateSyncSessionStatusFn  func(ctx context.Context, arg sqlc.UpdateSyncSessionStatusParams) (sqlc.SyncSession, error)
	updateSyncSessionProgressFn func(ctx context.Context, arg sqlc.UpdateSyncSessionProgressParams) (sqlc.SyncSession, error)
}

func (m *mockSyncSessionStore) GetLatestSyncSession(ctx context.Context, userID pgtype.UUID) (sqlc.SyncSession, error) {
	if m.getLatestSyncSessionFn == nil {
		return sqlc.SyncSession{}, errors.New("mock: GetLatestSyncSession not configured")
	}
	return m.getLatestSyncSessionFn(ctx, userID)
}

func (m *mockSyncSessionStore) CreateSyncSession(ctx context.Context, arg sqlc.CreateSyncSessionParams) (sqlc.SyncSession, error) {
	if m.createSyncSessionFn == nil {
		return sqlc.SyncSession{}, errors.New("mock: CreateSyncSession not configured")
	}
	return m.createSyncSessionFn(ctx, arg)
}

func (m *mockSyncSessionStore) UpdateSyncSessionStatus(ctx context.Context, arg sqlc.UpdateSyncSessionStatusParams) (sqlc.SyncSession, error) {
	if m.updateSyncSessionStatusFn == nil {
		return sqlc.SyncSession{}, errors.New("mock: UpdateSyncSessionStatus not configured")
	}
	return m.updateSyncSessionStatusFn(ctx, arg)
}

func (m *mockSyncSessionStore) UpdateSyncSessionProgress(ctx context.Context, arg sqlc.UpdateSyncSessionProgressParams) (sqlc.SyncSession, error) {
	if m.updateSyncSessionProgressFn == nil {
		return sqlc.SyncSession{}, errors.New("mock: UpdateSyncSessionProgress not configured")
	}
	return m.updateSyncSessionProgressFn(ctx, arg)
}

type mockActivityInserter struct {
	upsertActivityFn func(ctx context.Context, arg sqlc.UpsertActivityParams) (sqlc.Activity, error)
}

func (m *mockActivityInserter) UpsertActivity(ctx context.Context, arg sqlc.UpsertActivityParams) (sqlc.Activity, error) {
	if m.upsertActivityFn == nil {
		return sqlc.Activity{}, errors.New("mock: UpsertActivity not configured")
	}
	return m.upsertActivityFn(ctx, arg)
}

// ====== T5a.1: ImportStravaWorker fetches activities within time window ======

func TestImportStravaWorker_FetchesActivitiesInWindow(t *testing.T) {
	ctx := context.Background()
	userID := pgtype.UUID{Bytes: [16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16}, Valid: true}
	syncSessionID := pgtype.UUID{Bytes: [16]byte{2, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16}, Valid: true}
	windowStart := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	windowEnd := time.Date(2024, 2, 1, 0, 0, 0, 0, time.UTC)

	// Encrypt test tokens
	accessCipher := encryptTestToken("test_access_token")
	refreshCipher := encryptTestToken("test_refresh_token")

	// Setup: Mock activities from Strava (2 pages, 3 activities total)
	page1Activities := []strava.ActivitySummary{
		{ID: 1001, Name: "Run 1", Type: "Run", StartDate: time.Date(2024, 1, 10, 8, 0, 0, 0, time.UTC)},
		{ID: 1002, Name: "Bike 1", Type: "Ride", StartDate: time.Date(2024, 1, 15, 9, 0, 0, 0, time.UTC)},
	}

	page2Activities := []strava.ActivitySummary{
		{ID: 1003, Name: "Run 2", Type: "Run", StartDate: time.Date(2024, 1, 20, 8, 0, 0, 0, time.UTC)},
	}

	fetchCalls := 0
	fetcher := &mockActivityFetcher{
		getActivitiesFn: func(ctx context.Context, accessToken string, after, before time.Time, page, perPage int) ([]strava.ActivitySummary, error) {
			fetchCalls++
			if page == 1 {
				return page1Activities, nil
			} else if page == 2 {
				return page2Activities, nil
			}
			return []strava.ActivitySummary{}, nil
		},
	}

	// Setup: Mock sync session store
	sessionStore := &mockSyncSessionStore{
		getLatestSyncSessionFn: func(ctx context.Context, uid pgtype.UUID) (sqlc.SyncSession, error) {
			return sqlc.SyncSession{
				ID:     syncSessionID,
				UserID: uid,
				Status: "pending",
			}, nil
		},
		updateSyncSessionStatusFn: func(ctx context.Context, arg sqlc.UpdateSyncSessionStatusParams) (sqlc.SyncSession, error) {
			return sqlc.SyncSession{ID: arg.ID, Status: arg.Status}, nil
		},
		updateSyncSessionProgressFn: func(ctx context.Context, arg sqlc.UpdateSyncSessionProgressParams) (sqlc.SyncSession, error) {
			return sqlc.SyncSession{}, nil
		},
	}

	// Setup: Mock activity inserter
	upsertCount := 0
	activityStore := &mockActivityInserter{
		upsertActivityFn: func(ctx context.Context, arg sqlc.UpsertActivityParams) (sqlc.Activity, error) {
			upsertCount++
			return sqlc.Activity{ID: pgtype.UUID{Valid: true}}, nil
		},
	}

	// Setup: Mock token dependencies
	tokenQuerier := &mockTokenQuerier{
		getTokensFn: func(ctx context.Context, uid pgtype.UUID) (sqlc.StravaToken, error) {
			return sqlc.StravaToken{
				UserID:        uid,
				AccessCipher:  accessCipher,
				RefreshCipher: refreshCipher,
				ExpiresAt:     pgtype.Timestamptz{Time: time.Now().Add(1 * time.Hour), Valid: true},
			}, nil
		},
		upsertTokensFn: func(ctx context.Context, arg sqlc.UpsertStravaTokensParams) (sqlc.StravaToken, error) {
			return sqlc.StravaToken{}, nil
		},
	}

	tokenRefresher := &mockStravaClient{
		refreshFn: func(ctx context.Context, refreshToken string) (*strava.TokenSet, error) {
			return &strava.TokenSet{
				AccessToken:  "new_access_token",
				RefreshToken: "new_refresh_token",
				ExpiresAt:    time.Now().Add(6 * time.Hour),
				AthleteID:    12345,
				Scopes:       "activity:read_all",
			}, nil
		},
	}

	// Create implementation instance and test
	impl := &importStravaWorkerImpl{
		fetcher:        fetcher,
		sessionStore:   sessionStore,
		activityStore:  activityStore,
		cipherKey:      createTestCipherKey(),
		tokenQuerier:   tokenQuerier,
		tokenRefresher: tokenRefresher,
	}

	err := impl.execute(ctx, userID.String(), windowStart, windowEnd)
	if err != nil {
		t.Fatalf("execute() returned error: %v", err)
	}

	if fetchCalls < 2 {
		t.Errorf("expected at least 2 fetch calls, got %d", fetchCalls)
	}

	if upsertCount != 3 {
		t.Errorf("expected 3 upserts, got %d", upsertCount)
	}

	t.Log("T5a.1: ImportStravaWorker fetches activities in window — PASSED")
}

// ====== T5a.2: ImportStravaWorker marks sync session as completed ======

func TestImportStravaWorker_MarksSyncSessionCompleted(t *testing.T) {
	ctx := context.Background()
	userID := pgtype.UUID{Bytes: [16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16}, Valid: true}
	syncSessionID := pgtype.UUID{Bytes: [16]byte{2, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16}, Valid: true}
	windowStart := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	windowEnd := time.Date(2024, 2, 1, 0, 0, 0, 0, time.UTC)

	accessCipher := encryptTestToken("test_access_token")
	refreshCipher := encryptTestToken("test_refresh_token")

	// No activities to fetch
	fetcher := &mockActivityFetcher{
		getActivitiesFn: func(ctx context.Context, accessToken string, after, before time.Time, page, perPage int) ([]strava.ActivitySummary, error) {
			return []strava.ActivitySummary{}, nil
		},
	}

	// Track status updates
	statusUpdates := []string{}
	sessionStore := &mockSyncSessionStore{
		getLatestSyncSessionFn: func(ctx context.Context, uid pgtype.UUID) (sqlc.SyncSession, error) {
			return sqlc.SyncSession{
				ID:     syncSessionID,
				UserID: uid,
				Status: "pending",
			}, nil
		},
		updateSyncSessionStatusFn: func(ctx context.Context, arg sqlc.UpdateSyncSessionStatusParams) (sqlc.SyncSession, error) {
			statusUpdates = append(statusUpdates, arg.Status)
			return sqlc.SyncSession{ID: arg.ID, Status: arg.Status}, nil
		},
		updateSyncSessionProgressFn: func(ctx context.Context, arg sqlc.UpdateSyncSessionProgressParams) (sqlc.SyncSession, error) {
			return sqlc.SyncSession{}, nil
		},
	}

	activityStore := &mockActivityInserter{
		upsertActivityFn: func(ctx context.Context, arg sqlc.UpsertActivityParams) (sqlc.Activity, error) {
			return sqlc.Activity{}, nil
		},
	}

	tokenQuerier := &mockTokenQuerier{
		getTokensFn: func(ctx context.Context, uid pgtype.UUID) (sqlc.StravaToken, error) {
			return sqlc.StravaToken{
				UserID:        uid,
				AccessCipher:  accessCipher,
				RefreshCipher: refreshCipher,
				ExpiresAt:     pgtype.Timestamptz{Time: time.Now().Add(1 * time.Hour), Valid: true},
			}, nil
		},
		upsertTokensFn: func(ctx context.Context, arg sqlc.UpsertStravaTokensParams) (sqlc.StravaToken, error) {
			return sqlc.StravaToken{}, nil
		},
	}

	tokenRefresher := &mockStravaClient{
		refreshFn: func(ctx context.Context, refreshToken string) (*strava.TokenSet, error) {
			return &strava.TokenSet{
				AccessToken:  "new_access_token",
				RefreshToken: "new_refresh_token",
				ExpiresAt:    time.Now().Add(6 * time.Hour),
				AthleteID:    12345,
				Scopes:       "activity:read_all",
			}, nil
		},
	}

	impl := &importStravaWorkerImpl{
		fetcher:        fetcher,
		sessionStore:   sessionStore,
		activityStore:  activityStore,
		cipherKey:      createTestCipherKey(),
		tokenQuerier:   tokenQuerier,
		tokenRefresher: tokenRefresher,
	}

	err := impl.execute(ctx, userID.String(), windowStart, windowEnd)
	if err != nil {
		t.Fatalf("execute() returned error: %v", err)
	}

	if len(statusUpdates) < 2 {
		t.Errorf("expected at least 2 status updates, got %d", len(statusUpdates))
	}

	foundRunning, foundCompleted := false, false
	for _, status := range statusUpdates {
		if status == "running" {
			foundRunning = true
		}
		if status == "completed" {
			foundCompleted = true
		}
	}

	if !foundRunning {
		t.Error("expected status 'running' to be set")
	}
	if !foundCompleted {
		t.Error("expected status 'completed' to be set")
	}

	t.Log("T5a.2: ImportStravaWorker marks sync session completed — PASSED")
}

// ====== T5a.3: ImportStravaWorker deduplicates by unique constraint ======

func TestImportStravaWorker_DeduplicatesByUniqueConstraint(t *testing.T) {
	ctx := context.Background()
	userID := pgtype.UUID{Bytes: [16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16}, Valid: true}
	syncSessionID := pgtype.UUID{Bytes: [16]byte{2, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16}, Valid: true}

	accessCipher := encryptTestToken("test_access_token")
	refreshCipher := encryptTestToken("test_refresh_token")

	// Same activity appears twice
	duplicateActivity := strava.ActivitySummary{
		ID:        9999,
		Name:      "Duplicate Run",
		Type:      "Run",
		StartDate: time.Date(2024, 1, 10, 8, 0, 0, 0, time.UTC),
	}

	fetcher := &mockActivityFetcher{
		getActivitiesFn: func(ctx context.Context, accessToken string, after, before time.Time, page, perPage int) ([]strava.ActivitySummary, error) {
			if page == 1 || page == 2 {
				return []strava.ActivitySummary{duplicateActivity}, nil
			}
			return []strava.ActivitySummary{}, nil
		},
	}

	sessionStore := &mockSyncSessionStore{
		getLatestSyncSessionFn: func(ctx context.Context, uid pgtype.UUID) (sqlc.SyncSession, error) {
			return sqlc.SyncSession{ID: syncSessionID, UserID: uid, Status: "pending"}, nil
		},
		updateSyncSessionStatusFn: func(ctx context.Context, arg sqlc.UpdateSyncSessionStatusParams) (sqlc.SyncSession, error) {
			return sqlc.SyncSession{ID: arg.ID, Status: arg.Status}, nil
		},
		updateSyncSessionProgressFn: func(ctx context.Context, arg sqlc.UpdateSyncSessionProgressParams) (sqlc.SyncSession, error) {
			return sqlc.SyncSession{}, nil
		},
	}

	upsertCount := 0
	activityStore := &mockActivityInserter{
		upsertActivityFn: func(ctx context.Context, arg sqlc.UpsertActivityParams) (sqlc.Activity, error) {
			upsertCount++
			return sqlc.Activity{
				ID:             pgtype.UUID{Bytes: [16]byte{3, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16}, Valid: true},
				UserID:         arg.UserID,
				ExternalID:     arg.ExternalID,
				ExternalSource: arg.ExternalSource,
				SportType:      arg.SportType,
				Name:           arg.Name,
			}, nil
		},
	}

	tokenQuerier := &mockTokenQuerier{
		getTokensFn: func(ctx context.Context, uid pgtype.UUID) (sqlc.StravaToken, error) {
			return sqlc.StravaToken{
				UserID:        uid,
				AccessCipher:  accessCipher,
				RefreshCipher: refreshCipher,
				ExpiresAt:     pgtype.Timestamptz{Time: time.Now().Add(1 * time.Hour), Valid: true},
			}, nil
		},
		upsertTokensFn: func(ctx context.Context, arg sqlc.UpsertStravaTokensParams) (sqlc.StravaToken, error) {
			return sqlc.StravaToken{}, nil
		},
	}

	tokenRefresher := &mockStravaClient{
		refreshFn: func(ctx context.Context, refreshToken string) (*strava.TokenSet, error) {
			return &strava.TokenSet{
				AccessToken:  "new_access_token",
				RefreshToken: "new_refresh_token",
				ExpiresAt:    time.Now().Add(6 * time.Hour),
				AthleteID:    12345,
				Scopes:       "activity:read_all",
			}, nil
		},
	}

	impl := &importStravaWorkerImpl{
		fetcher:        fetcher,
		sessionStore:   sessionStore,
		activityStore:  activityStore,
		cipherKey:      createTestCipherKey(),
		tokenQuerier:   tokenQuerier,
		tokenRefresher: tokenRefresher,
	}

	err := impl.execute(ctx, userID.String(), time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC), time.Date(2024, 2, 1, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("execute() returned error: %v", err)
	}

	if upsertCount != 2 {
		t.Errorf("expected 2 upsert attempts, got %d", upsertCount)
	}

	t.Log("T5a.3: ImportStravaWorker handles duplicate deduplication — PASSED")
}

// ====== T5a.4: IngestActivityEventWorker fetches and persists single activity ======

func TestIngestActivityEventWorker_FetchesAndPersistsSingleActivity(t *testing.T) {
	ctx := context.Background()
	userID := pgtype.UUID{Bytes: [16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16}, Valid: true}
	activityID := int64(5555)

	accessCipher := encryptTestToken("test_access_token")
	refreshCipher := encryptTestToken("test_refresh_token")

	// Mock a single activity detail
	activityDetail := &strava.ActivityDetail{
		ID:        activityID,
		Name:      "Evening Run",
		Type:      "Run",
		StartDate: time.Date(2024, 1, 20, 18, 0, 0, 0, time.UTC),
	}

	fetcher := &mockActivityFetcher{
		getActivityFn: func(ctx context.Context, accessToken string, id int64) (*strava.ActivityDetail, error) {
			if id == activityID {
				return activityDetail, nil
			}
			return nil, errors.New("activity not found")
		},
	}

	upsertedActivity := (*strava.ActivityDetail)(nil)
	activityStore := &mockActivityInserter{
		upsertActivityFn: func(ctx context.Context, arg sqlc.UpsertActivityParams) (sqlc.Activity, error) {
			if arg.ExternalID == activityID {
				upsertedActivity = activityDetail
				return sqlc.Activity{
					ID:             pgtype.UUID{Valid: true},
					UserID:         arg.UserID,
					ExternalID:     arg.ExternalID,
					ExternalSource: arg.ExternalSource,
					SportType:      arg.SportType,
					Name:           arg.Name,
				}, nil
			}
			return sqlc.Activity{}, errors.New("unexpected activity ID")
		},
	}

	tokenQuerier := &mockTokenQuerier{
		getTokensFn: func(ctx context.Context, uid pgtype.UUID) (sqlc.StravaToken, error) {
			return sqlc.StravaToken{
				UserID:        uid,
				AccessCipher:  accessCipher,
				RefreshCipher: refreshCipher,
				ExpiresAt:     pgtype.Timestamptz{Time: time.Now().Add(1 * time.Hour), Valid: true},
			}, nil
		},
		upsertTokensFn: func(ctx context.Context, arg sqlc.UpsertStravaTokensParams) (sqlc.StravaToken, error) {
			return sqlc.StravaToken{}, nil
		},
	}

	tokenRefresher := &mockStravaClient{
		refreshFn: func(ctx context.Context, refreshToken string) (*strava.TokenSet, error) {
			return &strava.TokenSet{
				AccessToken:  "new_access_token",
				RefreshToken: "new_refresh_token",
				ExpiresAt:    time.Now().Add(6 * time.Hour),
				AthleteID:    12345,
				Scopes:       "activity:read_all",
			}, nil
		},
	}

	impl := &ingestActivityEventWorkerImpl{
		fetcher:        fetcher,
		activityStore:  activityStore,
		cipherKey:      createTestCipherKey(),
		tokenQuerier:   tokenQuerier,
		tokenRefresher: tokenRefresher,
	}

	err := impl.execute(ctx, userID.String(), activityID)
	if err != nil {
		t.Fatalf("execute() returned error: %v", err)
	}

	if upsertedActivity == nil {
		t.Error("expected activity to be upserted")
	} else if upsertedActivity.Name != "Evening Run" {
		t.Errorf("expected activity name 'Evening Run', got %q", upsertedActivity.Name)
	}

	t.Log("T5a.4: IngestActivityEventWorker fetches and persists single activity — PASSED")
}
