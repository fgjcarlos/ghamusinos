package jobs

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/riverqueue/river"
	"github.com/stretchr/testify/require"

	"github.com/fgjcarlos/ghamusinos/internal/crypto"
	"github.com/fgjcarlos/ghamusinos/internal/db/sqlc"
	"github.com/fgjcarlos/ghamusinos/internal/strava"
)

// TestImportStravaStreamsWorker_FetchesAndUpsertsStreams tests that the worker
// fetches streams from the Strava API and upserts them to the database.
func TestImportStravaStreamsWorker_FetchesAndUpsertsStreams(t *testing.T) {
	ctx := context.Background()

	// Setup cipher key
	cipherKey := make([]byte, 32)
	for i := range cipherKey {
		cipherKey[i] = byte(i)
	}

	// Setup: mock Strava streams
	mockStreams := []strava.StreamFrame{
		{
			Type:       "heartrate",
			Data:       []interface{}{120.0, 125.0, 130.0},
			SeriesType: "distance",
		},
		{
			Type:       "watts",
			Data:       []interface{}{150.0, 155.0, 160.0},
			SeriesType: "distance",
		},
	}

	// Mock fetcher
	mockFetcher := &testStreamFetcher{
		streams: mockStreams,
	}

	// Mock inserter
	mockInserter := &testStreamInserter{}

	// Mock activity locator
	activityID := pgtype.UUID{Bytes: [16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16}, Valid: true}
	mockLocator := &testActivityLocator{
		activity: sqlc.Activity{ID: activityID},
	}

	// Mock HR zone store
	mockZoneStore := &testHRZoneStore{
		userHRMax: pgtype.Int2{Valid: false},
	}

	// Mock token querier with encrypted tokens
	accessToken := "test_access_token"
	refreshToken := "test_refresh_token"
	encAccessCipher, _ := crypto.Encrypt([]byte(accessToken), cipherKey)
	encRefreshCipher, _ := crypto.Encrypt([]byte(refreshToken), cipherKey)

	mockQuerier := &testMockTokenQuerier{
		token: sqlc.StravaToken{
			AccessCipher:  encAccessCipher,
			RefreshCipher: encRefreshCipher,
			ExpiresAt:     pgtype.Timestamptz{Time: time.Now().Add(1 * time.Hour), Valid: true},
			AthleteID:     12345,
			Scopes:        "read,read_all",
			CreatedAt:     pgtype.Timestamptz{Time: time.Now(), Valid: true},
			UpdatedAt:     pgtype.Timestamptz{Time: time.Now(), Valid: true},
		},
	}

	// Mock token refresher
	mockRefresher := &testMockTokenRefresher{}

	worker := &ImportStravaStreamsWorker{
		fetcher:   mockFetcher,
		inserter:  mockInserter,
		locator:   mockLocator,
		querier:   mockQuerier,
		refresher: mockRefresher,
		zoneStore: mockZoneStore,
		cipherKey: cipherKey,
	}

	args := ImportStravaStreamsArgs{
		UserID:             "550e8400-e29b-41d4-a716-446655440000",
		ActivityExternalID: "12345678",
		StravaActivityID:   12345678,
	}

	job := &river.Job[ImportStravaStreamsArgs]{Args: args}
	err := worker.Work(ctx, job)
	require.NoError(t, err)

	// Verify UpsertActivityStream was called for each stream
	require.Equal(t, 2, mockInserter.callCount)
	require.Equal(t, "watts", mockInserter.lastStreamType)
}

// TestImportStravaStreamsWorker_CalculatesHRZones_WhenUserHRMaxSet tests HR zone calculation.
func TestImportStravaStreamsWorker_CalculatesHRZones_WhenUserHRMaxSet(t *testing.T) {
	ctx := context.Background()

	// Setup cipher key
	cipherKey := make([]byte, 32)
	for i := range cipherKey {
		cipherKey[i] = byte(i)
	}

	// Setup: mock data with HR stream
	mockStreams := []strava.StreamFrame{
		{
			Type:       "heartrate",
			Data:       []interface{}{100.0, 120.0, 140.0, 155.0, 165.0}, // z1, z2, z3, z4, z5 at hrMax=180
			SeriesType: "time",
		},
	}

	mockFetcher := &testStreamFetcher{
		streams: mockStreams,
	}
	mockInserter := &testStreamInserter{}

	activityID := pgtype.UUID{Bytes: [16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16}, Valid: true}
	mockLocator := &testActivityLocator{
		activity: sqlc.Activity{ID: activityID},
	}

	// hr_max = 180
	mockZoneStore := &testHRZoneStore{
		userHRMax: pgtype.Int2{Int16: 180, Valid: true},
	}

	// Mock token querier with encrypted tokens
	accessToken := "test_access_token"
	refreshToken := "test_refresh_token"
	encAccessCipher, _ := crypto.Encrypt([]byte(accessToken), cipherKey)
	encRefreshCipher, _ := crypto.Encrypt([]byte(refreshToken), cipherKey)

	mockQuerier := &testMockTokenQuerier{
		token: sqlc.StravaToken{
			AccessCipher:  encAccessCipher,
			RefreshCipher: encRefreshCipher,
			ExpiresAt:     pgtype.Timestamptz{Time: time.Now().Add(1 * time.Hour), Valid: true},
			AthleteID:     12345,
			Scopes:        "read,read_all",
			CreatedAt:     pgtype.Timestamptz{Time: time.Now(), Valid: true},
			UpdatedAt:     pgtype.Timestamptz{Time: time.Now(), Valid: true},
		},
	}

	mockRefresher := &testMockTokenRefresher{}

	worker := &ImportStravaStreamsWorker{
		fetcher:   mockFetcher,
		inserter:  mockInserter,
		locator:   mockLocator,
		querier:   mockQuerier,
		refresher: mockRefresher,
		zoneStore: mockZoneStore,
		cipherKey: cipherKey,
	}

	args := ImportStravaStreamsArgs{
		UserID:             "550e8400-e29b-41d4-a716-446655440000",
		ActivityExternalID: "12345678",
		StravaActivityID:   12345678,
	}

	job := &river.Job[ImportStravaStreamsArgs]{Args: args}
	err := worker.Work(ctx, job)
	require.NoError(t, err)

	// Verify UpsertHRZones was called
	require.True(t, mockZoneStore.upsertCalled)
	// 100 < 108 (60% of 180) → z1
	// 120 is in [108,126) → z2
	// 140 is in [126,144) → z3
	// 155 is in [144,162) → z4
	// 165 >= 162 → z5
	require.Equal(t, int32(1), mockZoneStore.lastZ1)
	require.Equal(t, int32(1), mockZoneStore.lastZ2)
	require.Equal(t, int32(1), mockZoneStore.lastZ3)
	require.Equal(t, int32(1), mockZoneStore.lastZ4)
	require.Equal(t, int32(1), mockZoneStore.lastZ5)
}

// TestImportStravaStreamsWorker_SkipsHRZones_WhenUserHRMaxNil tests that HR zones are skipped when hr_max is NULL.
func TestImportStravaStreamsWorker_SkipsHRZones_WhenUserHRMaxNil(t *testing.T) {
	ctx := context.Background()

	// Setup cipher key
	cipherKey := make([]byte, 32)
	for i := range cipherKey {
		cipherKey[i] = byte(i)
	}

	mockStreams := []strava.StreamFrame{
		{
			Type:       "heartrate",
			Data:       []interface{}{120.0, 125.0, 130.0},
			SeriesType: "time",
		},
	}

	mockFetcher := &testStreamFetcher{
		streams: mockStreams,
	}
	mockInserter := &testStreamInserter{}

	activityID := pgtype.UUID{Bytes: [16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16}, Valid: true}
	mockLocator := &testActivityLocator{
		activity: sqlc.Activity{ID: activityID},
	}

	// hr_max = NULL
	mockZoneStore := &testHRZoneStore{
		userHRMax: pgtype.Int2{Valid: false},
	}

	// Mock token querier with encrypted tokens
	accessToken := "test_access_token"
	refreshToken := "test_refresh_token"
	encAccessCipher, _ := crypto.Encrypt([]byte(accessToken), cipherKey)
	encRefreshCipher, _ := crypto.Encrypt([]byte(refreshToken), cipherKey)

	mockQuerier := &testMockTokenQuerier{
		token: sqlc.StravaToken{
			AccessCipher:  encAccessCipher,
			RefreshCipher: encRefreshCipher,
			ExpiresAt:     pgtype.Timestamptz{Time: time.Now().Add(1 * time.Hour), Valid: true},
			AthleteID:     12345,
			Scopes:        "read,read_all",
			CreatedAt:     pgtype.Timestamptz{Time: time.Now(), Valid: true},
			UpdatedAt:     pgtype.Timestamptz{Time: time.Now(), Valid: true},
		},
	}

	mockRefresher := &testMockTokenRefresher{}

	worker := &ImportStravaStreamsWorker{
		fetcher:   mockFetcher,
		inserter:  mockInserter,
		locator:   mockLocator,
		querier:   mockQuerier,
		refresher: mockRefresher,
		zoneStore: mockZoneStore,
		cipherKey: cipherKey,
	}

	args := ImportStravaStreamsArgs{
		UserID:             "550e8400-e29b-41d4-a716-446655440000",
		ActivityExternalID: "12345678",
		StravaActivityID:   12345678,
	}

	job := &river.Job[ImportStravaStreamsArgs]{Args: args}
	err := worker.Work(ctx, job)
	require.NoError(t, err)

	// Verify UpsertHRZones was NOT called
	require.False(t, mockZoneStore.upsertCalled)
}

// TestCalcHRZones_Pure_Function tests the pure calcHRZones function.
func TestCalcHRZones_Pure_Function(t *testing.T) {
	tests := []struct {
		name     string
		hrStream []float64
		hrMax    int
		expected HRZoneSeconds
	}{
		{
			name:     "empty stream returns zero value",
			hrStream: []float64{},
			hrMax:    180,
			expected: HRZoneSeconds{},
		},
		{
			name:     "hrMax=0 returns zero value",
			hrStream: []float64{120.0, 130.0},
			hrMax:    0,
			expected: HRZoneSeconds{},
		},
		{
			name:     "hrMax<0 returns zero value",
			hrStream: []float64{120.0, 130.0},
			hrMax:    -180,
			expected: HRZoneSeconds{},
		},
		{
			name:     "all samples in z1 (< 60%)",
			hrStream: []float64{100.0, 105.0, 107.0}, // all < 108 (60% of 180)
			hrMax:    180,
			expected: HRZoneSeconds{Z1: 3},
		},
		{
			name:     "all samples in z5 (>= 90%)",
			hrStream: []float64{162.0, 165.0, 170.0}, // all >= 162 (90% of 180)
			hrMax:    180,
			expected: HRZoneSeconds{Z5: 3},
		},
		{
			name:     "mixed zones at 180 bpm",
			hrStream: []float64{100.0, 120.0, 140.0, 155.0, 165.0}, // z1, z2, z3, z4, z5
			hrMax:    180,
			expected: HRZoneSeconds{Z1: 1, Z2: 1, Z3: 1, Z4: 1, Z5: 1},
		},
		{
			name:     "edge case hrMax=120",
			hrStream: []float64{60.0, 72.0, 84.0, 96.0, 108.0}, // z1, z2, z3, z4, z5
			hrMax:    120,
			expected: HRZoneSeconds{Z1: 1, Z2: 1, Z3: 1, Z4: 1, Z5: 1},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := calcHRZones(tt.hrStream, tt.hrMax)
			require.Equal(t, tt.expected, result)
		})
	}
}

// ========== Test Helpers ==========

type testStreamFetcher struct {
	streams []strava.StreamFrame
}

func (m *testStreamFetcher) GetStreams(ctx context.Context, accessToken string, activityID int64, types []string) ([]strava.StreamFrame, error) {
	return m.streams, nil
}

type testStreamInserter struct {
	callCount      int
	lastStreamType string
}

func (m *testStreamInserter) UpsertActivityStream(ctx context.Context, arg sqlc.UpsertActivityStreamParams) (sqlc.ActivityStream, error) {
	m.callCount++
	m.lastStreamType = arg.StreamType
	return sqlc.ActivityStream{}, nil
}

type testActivityLocator struct {
	activity sqlc.Activity
}

func (m *testActivityLocator) GetActivityByExternalID(ctx context.Context, arg sqlc.GetActivityByExternalIDParams) (sqlc.Activity, error) {
	return m.activity, nil
}

type testMockTokenQuerier struct {
	token sqlc.StravaToken
}

func (m *testMockTokenQuerier) GetStravaTokensByUserID(ctx context.Context, userID pgtype.UUID) (sqlc.StravaToken, error) {
	return m.token, nil
}

func (m *testMockTokenQuerier) UpsertStravaTokens(ctx context.Context, arg sqlc.UpsertStravaTokensParams) (sqlc.StravaToken, error) {
	return sqlc.StravaToken{}, nil
}

type testMockTokenRefresher struct{}

func (m *testMockTokenRefresher) Refresh(ctx context.Context, refreshToken string) (*strava.TokenSet, error) {
	return &strava.TokenSet{}, nil
}

type testHRZoneStore struct {
	userHRMax    pgtype.Int2
	upsertCalled bool
	lastZ1       int32
	lastZ2       int32
	lastZ3       int32
	lastZ4       int32
	lastZ5       int32
}

func (m *testHRZoneStore) GetUserHRMaxByID(ctx context.Context, userID pgtype.UUID) (pgtype.Int2, error) {
	return m.userHRMax, nil
}

func (m *testHRZoneStore) UpsertHRZones(ctx context.Context, arg sqlc.UpsertHRZonesParams) (sqlc.HrZone, error) {
	m.upsertCalled = true
	m.lastZ1 = arg.Z1Seconds
	m.lastZ2 = arg.Z2Seconds
	m.lastZ3 = arg.Z3Seconds
	m.lastZ4 = arg.Z4Seconds
	m.lastZ5 = arg.Z5Seconds
	return sqlc.HrZone{}, nil
}
