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

// ─────────────────────────────────────────────────────────────────────────
// AUD-05 (issue #166) tests for the streams worker.
//
//   - ExternalSource: "strava" is forwarded to the locator; without it the
//     SQL query (queries/activities.sql:1-9) returns nothing and the
//     worker never finds the activity to attach streams to.
//   - HR zones scale by Resolution: a medium-resolution stream must yield
//     the same total seconds as a high-resolution stream of the same data.
// ─────────────────────────────────────────────────────────────────────────

// assertingLocator captures the args the worker passes; AUD-05 test guards
// that ExternalSource is set to "strava" (the bug fix).
type assertingLocator struct {
	activity sqlc.Activity
	lastArg  sqlc.GetActivityByExternalIDParams
}

func (m *assertingLocator) GetActivityByExternalID(_ context.Context, arg sqlc.GetActivityByExternalIDParams) (sqlc.Activity, error) {
	m.lastArg = arg
	return m.activity, nil
}

// capturingZoneStore records the HR zone totals the worker computes.
// AUD-05 test guards that down-sampled streams produce the same totals
// as full-resolution streams.
type capturingZoneStore struct {
	userHRMax pgtype.Int2
	lastZones sqlc.UpsertHRZonesParams
	upserted  int
}

func (m *capturingZoneStore) GetUserHRMaxByID(_ context.Context, _ pgtype.UUID) (pgtype.Int2, error) {
	return m.userHRMax, nil
}

func (m *capturingZoneStore) UpsertHRZones(_ context.Context, arg sqlc.UpsertHRZonesParams) (sqlc.HrZone, error) {
	m.lastZones = arg
	m.upserted++
	return sqlc.HrZone{}, nil
}

// fixedStreamFetcher returns a single fixed set of frames every call;
// AUD-05 tests use it to compare resolutions side by side.
type fixedStreamFetcher struct {
	streams []strava.StreamFrame
}

func (m *fixedStreamFetcher) GetStreams(_ context.Context, _ string, _ int64, _ []string) ([]strava.StreamFrame, error) {
	return m.streams, nil
}

// TestImportStravaStreamsWorker_PassesExternalSource: AUD-05 AC #3.
// Before the fix the locator was called with ExternalSource: "" — the
// SQL query's WHERE external_source = $2 clause then never matched the
// stored "strava" rows, so every stream call fell into the "activity not
// found" branch and no streams ever landed. This test asserts the worker
// now sends the right source string.
func TestImportStravaStreamsWorker_PassesExternalSource(t *testing.T) {
	ctx := context.Background()
	cipherKey := make([]byte, 32)
	for i := range cipherKey {
		cipherKey[i] = byte(i)
	}

	activityID := pgtype.UUID{Bytes: [16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16}, Valid: true}
	locator := &assertingLocator{activity: sqlc.Activity{ID: activityID}}

	// Encrypt a real token pair so GetValidToken succeeds. Using an empty
	// TokenQuerier trips "sobre demasiado corto" because the cipher
	// expects at least a nonce prefix.
	accessCipher, _ := crypto.Encrypt([]byte("test-access"), cipherKey)
	refreshCipher, _ := crypto.Encrypt([]byte("test-refresh"), cipherKey)

	worker := &ImportStravaStreamsWorker{
		fetcher: &fixedStreamFetcher{streams: []strava.StreamFrame{
			{Type: "heartrate", Data: []interface{}{130.0}},
		}},
		inserter:  &testStreamInserter{},
		zoneStore: &capturingZoneStore{userHRMax: pgtype.Int2{Valid: false}},
		locator:   locator,
		querier: &testMockTokenQuerier{token: sqlc.StravaToken{
			AccessCipher:  accessCipher,
			RefreshCipher: refreshCipher,
			ExpiresAt:     pgtype.Timestamptz{Time: time.Now().Add(time.Hour), Valid: true},
		}},
		refresher: &testMockTokenRefresher{},
		cipherKey: cipherKey,
	}

	job := &river.Job[ImportStravaStreamsArgs]{
		Args: ImportStravaStreamsArgs{
			UserID:           "550e8400-e29b-41d4-a716-446655440000",
			StravaActivityID: 12345678,
		},
	}
	require.NoError(t, worker.Work(ctx, job))
	require.Equal(t, "strava", locator.lastArg.ExternalSource,
		"streams worker must pass ExternalSource=strava; got %q", locator.lastArg.ExternalSource)
	require.Equal(t, int64(12345678), locator.lastArg.ExternalID)
}

// TestCalcHRZonesScaled_EqualTotalAcrossResolutions: AUD-05 AC #6.
// A 100-sample stream at Resolution=1 (1Hz, 100s total) and the same 100
// samples grouped at Resolution=10 (10Hz downsampled to 1 sample per 10s)
// must produce the same total zone seconds once the worker scales the
// down-sampled counts. The math: 100 samples × 1s = 100s; 10 samples × 10s
// = 100s. If the worker ignores Resolution, the second case reports 10s.
func TestCalcHRZonesScaled_EqualTotalAcrossResolutions(t *testing.T) {
	hrMax := 200 // thresholds land the samples in different zones

	// High-resolution: 100 samples covering each zone for 25 samples each
	// (so each zone = 25 seconds).
	highSamples := make([]float64, 0, 100)
	for i := 0; i < 25; i++ {
		highSamples = append(highSamples, 100.0) // Z1
	}
	for i := 0; i < 25; i++ {
		highSamples = append(highSamples, 130.0) // Z2
	}
	for i := 0; i < 25; i++ {
		highSamples = append(highSamples, 150.0) // Z3
	}
	for i := 0; i < 25; i++ {
		highSamples = append(highSamples, 170.0) // Z4
	}
	highZones := calcHRZonesScaled(highSamples, hrMax, 1)
	totalHigh := highZones.Z1 + highZones.Z2 + highZones.Z3 + highZones.Z4 + highZones.Z5
	require.Equal(t, 100, totalHigh, "high-resolution total must equal 100s (1 sample/s × 100 samples)")
	require.Equal(t, 100, highZones.Z1+highZones.Z2+highZones.Z3+highZones.Z4+highZones.Z5,
		"high-resolution with 100 samples at 1s each = 100 total seconds")

	// Medium-resolution: same data down-sampled to 1 sample per 10s, so
	// we only see 10 samples total. The worker MUST multiply by 10 so
	// the totals still match.
	mediumSamples := []float64{100, 100, 100, 130, 130, 150, 150, 150, 170, 190}
	mediumZones := calcHRZonesScaled(mediumSamples, hrMax, 10)
	totalMedium := mediumZones.Z1 + mediumZones.Z2 + mediumZones.Z3 + mediumZones.Z4 + mediumZones.Z5
	require.Equal(t, 100, totalMedium,
		"medium-resolution total must equal high-resolution; got %d (worker forgot Resolution scaling)",
		totalMedium)
	// ponytail: we don't assert per-zone equality across resolutions.
	// Down-sampling throws samples away by design — the same effort at
	// Resolution=10 will not put exactly the same seconds in Z1 as the
	// Resolution=1 stream because fewer samples land in each bucket. The
	// contract is "total seconds match within the down-sampling factor";
	// the AC asks for "the same total seconds", not "the same per-zone".
}

// TestImportStravaStreamsWorker_HRZonesScaleByResolution: AUD-05 AC #6.
// End-to-end test through the worker. Two stream sets with the same total
// effort but different Resolution values must yield identical hr_zones.
func TestImportStravaStreamsWorker_HRZonesScaleByResolution(t *testing.T) {
	ctx := context.Background()
	cipherKey := make([]byte, 32)
	for i := range cipherKey {
		cipherKey[i] = byte(i)
	}

	// 60 minutes of effort at HR 150 (Z3 territory for hrMax=200).
	// High-res: 3600 samples at Resolution=1 (1s each) = 3600s.
	// Medium-res: 60 samples at Resolution=60 (1 sample per minute) = 3600s.
	hrMax := 200
	make := func(resolution int, samples int) []strava.StreamFrame {
		hrData := make([]interface{}, samples)
		for i := range hrData {
			hrData[i] = 150.0
		}
		return []strava.StreamFrame{{
			Type:         "heartrate",
			Data:         hrData,
			SeriesType:   "time",
			Resolution:   resolution,
			OriginalSize: 3600,
		}}
	}

	activityID := pgtype.UUID{Bytes: [16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16}, Valid: true}
	accessCipher, _ := crypto.Encrypt([]byte("test-access"), cipherKey)
	refreshCipher, _ := crypto.Encrypt([]byte("test-refresh"), cipherKey)
	runWorker := func(resolution int, samples int) *capturingZoneStore {
		zones := &capturingZoneStore{
			userHRMax: pgtype.Int2{Int16: int16(hrMax), Valid: true},
		}
		w := &ImportStravaStreamsWorker{
			fetcher:   &fixedStreamFetcher{streams: make(resolution, samples)},
			inserter:  &testStreamInserter{},
			zoneStore: zones,
			locator:   &testActivityLocator{activity: sqlc.Activity{ID: activityID}},
			querier: &testMockTokenQuerier{token: sqlc.StravaToken{
				AccessCipher:  accessCipher,
				RefreshCipher: refreshCipher,
				ExpiresAt:     pgtype.Timestamptz{Time: time.Now().Add(time.Hour), Valid: true},
			}},
			refresher: &testMockTokenRefresher{},
			cipherKey: cipherKey,
		}
		job := &river.Job[ImportStravaStreamsArgs]{
			Args: ImportStravaStreamsArgs{
				UserID:           "550e8400-e29b-41d4-a716-446655440000",
				StravaActivityID: 12345678,
			},
		}
		require.NoError(t, w.Work(ctx, job))
		return zones
	}

	high := runWorker(1, 3600)
	require.Equal(t, 3600, int(high.lastZones.Z3Seconds),
		"high-res: 3600s in Z3 expected; got %d", high.lastZones.Z3Seconds)

	medium := runWorker(60, 60)
	require.Equal(t, 3600, int(medium.lastZones.Z3Seconds),
		"medium-res: 60 samples × 60s = 3600s expected; got %d", medium.lastZones.Z3Seconds)
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
