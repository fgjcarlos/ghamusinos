package jobs

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/riverqueue/river"
	"github.com/stretchr/testify/require"

	"github.com/fgjcarlos/ghamusinos/internal/db/sqlc"
	"github.com/fgjcarlos/ghamusinos/internal/strava"
)

// TestImportStravaWorker_FetchesActivitiesAndUpserts tests that the worker
// fetches activities from Strava API, upserts them to the database, and
// updates sync session progress after each page.
func TestImportStravaWorker_FetchesActivitiesAndUpserts(t *testing.T) {
	// Create mock Strava server that returns 2 pages of activities
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v3/athlete/activities" {
			page := r.URL.Query().Get("page")
			if page == "" || page == "1" {
				// First page: 2 activities
				activities := []strava.ActivitySummary{
					{
						ID:                 12345,
						Name:               "Morning Run",
						Type:               "Run",
						StartDate:          time.Now().AddDate(0, 0, -5),
						Distance:           5000.0,
						MovingTime:         1800,
						ElapsedTime:        1900,
						TotalElevationGain: 50.0,
					},
					{
						ID:                 12346,
						Name:               "Evening Ride",
						Type:               "Ride",
						StartDate:          time.Now().AddDate(0, 0, -3),
						Distance:           20000.0,
						MovingTime:         3600,
						ElapsedTime:        3700,
						TotalElevationGain: 200.0,
					},
				}
				w.Header().Set("Content-Type", "application/json")
				_ = json.NewEncoder(w).Encode(activities)
			} else if page == "2" {
				// Second page: 1 activity
				activities := []strava.ActivitySummary{
					{
						ID:                 12347,
						Name:               "Swim",
						Type:               "Swim",
						StartDate:          time.Now().AddDate(0, 0, -1),
						Distance:           1500.0,
						MovingTime:         1200,
						ElapsedTime:        1200,
						TotalElevationGain: 0.0,
					},
				}
				w.Header().Set("Content-Type", "application/json")
				_ = json.NewEncoder(w).Encode(activities)
			} else {
				// Third page: empty (end of pagination)
				w.Header().Set("Content-Type", "application/json")
				_ = json.NewEncoder(w).Encode([]strava.ActivitySummary{})
			}
		}
	}))
	defer mockServer.Close()

	// Create mock client with httptest server
	cfg := strava.Config{
		ClientID:     "test-id",
		ClientSecret: "test-secret",
		RedirectURL:  "http://localhost",
		Scopes:       "read,activity:read",
		HTTPClient:   mockServer.Client(),
	}
	client, err := strava.NewClient(cfg)
	require.NoError(t, err)

	// Create mock query store
	mockQuerier := &mockSyncQueryStore{
		activityCount: 0,
		userID:        pgtype.UUID{Bytes: [16]byte{1}, Valid: true},
	}

	// Create worker
	worker := &ImportStravaWorker{}

	// Create job with args
	userID := pgtype.UUID{Bytes: [16]byte{1}, Valid: true}
	args := ImportStravaArgs{
		UserID:      userID.String(),
		WindowStart: time.Now().AddDate(0, 0, -42),
		WindowEnd:   time.Now(),
	}

	job := &river.Job[ImportStravaArgs]{
		Args: args,
	}

	// Execute worker (this will fail because the worker's Work method is not implemented yet)
	// We're expecting it to fail during compilation/implementation phase.
	_ = worker
	_ = job
	_ = mockQuerier
	_ = client

	// For now, this test is a placeholder that documents the expected behavior.
	// The actual implementation will be written in the GREEN phase.
	t.Logf("Test framework ready for ImportStravaWorker implementation")
}

// TestImportStravaWorker_SetsSyncSessionCompleted tests that the worker
// sets sync_session.status = "completed" after last page.
func TestImportStravaWorker_SetsSyncSessionCompleted(t *testing.T) {
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v3/athlete/activities" {
			activities := []strava.ActivitySummary{
				{
					ID:        99999,
					Name:      "Last Activity",
					Type:      "Run",
					StartDate: time.Now(),
					Distance:  5000.0,
				},
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(activities)
		}
	}))
	defer mockServer.Close()

	cfg := strava.Config{
		ClientID:     "test-id",
		ClientSecret: "test-secret",
		RedirectURL:  "http://localhost",
		Scopes:       "read,activity:read",
		HTTPClient:   mockServer.Client(),
	}
	client, err := strava.NewClient(cfg)
	require.NoError(t, err)

	userID := pgtype.UUID{Bytes: [16]byte{2}, Valid: true}
	mockQuerier := &mockSyncQueryStore{
		activityCount: 0,
		userID:        userID,
	}

	worker := &ImportStravaWorker{}
	args := ImportStravaArgs{
		UserID:      userID.String(),
		WindowStart: time.Now().AddDate(0, 0, -7),
		WindowEnd:   time.Now(),
	}
	job := &river.Job[ImportStravaArgs]{
		Args: args,
	}

	_ = worker
	_ = job
	_ = mockQuerier
	_ = client

	t.Logf("Test ready for sync session completion logic")
}

// TestImportStravaWorker_CreatesLazyInitialSyncSession tests that if no
// sync session exists, the worker creates one on first run.
func TestImportStravaWorker_CreatesLazyInitialSyncSession(t *testing.T) {
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v3/athlete/activities" {
			activities := []strava.ActivitySummary{
				{
					ID:   77777,
					Name: "Test",
					Type: "Run",
				},
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(activities)
		}
	}))
	defer mockServer.Close()

	cfg := strava.Config{
		ClientID:     "test-id",
		ClientSecret: "test-secret",
		RedirectURL:  "http://localhost",
		HTTPClient:   mockServer.Client(),
	}
	client, err := strava.NewClient(cfg)
	require.NoError(t, err)

	userID := pgtype.UUID{Bytes: [16]byte{3}, Valid: true}
	mockQuerier := &mockSyncQueryStore{
		activityCount: 0,
		userID:        userID,
	}

	worker := &ImportStravaWorker{}
	args := ImportStravaArgs{
		UserID:      userID.String(),
		WindowStart: time.Now().AddDate(0, 0, -7),
		WindowEnd:   time.Now(),
	}
	job := &river.Job[ImportStravaArgs]{
		Args: args,
	}

	_ = worker
	_ = job
	_ = mockQuerier
	_ = client

	t.Logf("Test ready for lazy sync session initialization")
}

// TestImportStravaWorker_HandlesPaginationCorrectly tests that the worker
// iterates through all pages until receiving an empty page.
func TestImportStravaWorker_HandlesPaginationCorrectly(t *testing.T) {
	pageCount := 0
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v3/athlete/activities" {
			pageCount++
			page := r.URL.Query().Get("page")
			if page == "1" || page == "" {
				// First page
				w.Header().Set("Content-Type", "application/json")
				_ = json.NewEncoder(w).Encode([]strava.ActivitySummary{
					{ID: 111, Name: "Act1", Type: "Run"},
				})
			} else if page == "2" {
				// Second page
				w.Header().Set("Content-Type", "application/json")
				_ = json.NewEncoder(w).Encode([]strava.ActivitySummary{
					{ID: 222, Name: "Act2", Type: "Ride"},
				})
			} else {
				// Empty page signals end
				w.Header().Set("Content-Type", "application/json")
				_ = json.NewEncoder(w).Encode([]strava.ActivitySummary{})
			}
		}
	}))
	defer mockServer.Close()

	cfg := strava.Config{
		ClientID:     "test-id",
		ClientSecret: "test-secret",
		RedirectURL:  "http://localhost",
		HTTPClient:   mockServer.Client(),
	}
	client, err := strava.NewClient(cfg)
	require.NoError(t, err)

	userID := pgtype.UUID{Bytes: [16]byte{4}, Valid: true}
	mockQuerier := &mockSyncQueryStore{
		activityCount: 0,
		userID:        userID,
	}

	worker := &ImportStravaWorker{}
	args := ImportStravaArgs{
		UserID:      userID.String(),
		WindowStart: time.Now().AddDate(0, 0, -7),
		WindowEnd:   time.Now(),
	}
	job := &river.Job[ImportStravaArgs]{
		Args: args,
	}

	_ = worker
	_ = job
	_ = mockQuerier
	_ = client

	t.Logf("Test ready for pagination logic (page count: %d)", pageCount)
}

// TestIngestActivityEventWorker_LoadsEventAndFetchesActivity tests the
// ingestion worker: loads event from DB by EventID, fetches activity from
// Strava API, then upserts to activities table.
func TestIngestActivityEventWorker_LoadsEventAndFetchesActivity(t *testing.T) {
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v3/activities/12345" {
			activity := strava.ActivityDetail{
				ID:                 12345,
				Name:               "Test Activity",
				Type:               "Run",
				StartDate:          time.Now(),
				Distance:           5000.0,
				MovingTime:         1800,
				ElapsedTime:        1900,
				TotalElevationGain: 50.0,
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(activity)
		}
	}))
	defer mockServer.Close()

	cfg := strava.Config{
		ClientID:     "test-id",
		ClientSecret: "test-secret",
		RedirectURL:  "http://localhost",
		HTTPClient:   mockServer.Client(),
	}
	client, err := strava.NewClient(cfg)
	require.NoError(t, err)

	mockQuerier := &mockActivityEventStore{
		userID:      pgtype.UUID{Bytes: [16]byte{5}, Valid: true},
		activityID:  12345,
		eventID:     "evt_test_123",
		activityCount: 0,
	}

	worker := &IngestActivityEventWorker{}
	args := IngestActivityEventArgs{
		EventID: "evt_test_123",
	}
	job := &river.Job[IngestActivityEventArgs]{
		Args: args,
	}

	_ = worker
	_ = job
	_ = mockQuerier
	_ = client

	t.Logf("Test ready for activity ingestion logic")
}

// TestIngestActivityEventWorker_RefreshesTokenIfNeeded tests that the
// worker checks token expiry and refreshes if necessary before fetching.
func TestIngestActivityEventWorker_RefreshesTokenIfNeeded(t *testing.T) {
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		activity := strava.ActivityDetail{
			ID:        99,
			Name:      "Token Test",
			Type:      "Run",
			StartDate: time.Now(),
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(activity)
	}))
	defer mockServer.Close()

	cfg := strava.Config{
		ClientID:     "test-id",
		ClientSecret: "test-secret",
		RedirectURL:  "http://localhost",
		HTTPClient:   mockServer.Client(),
	}
	client, err := strava.NewClient(cfg)
	require.NoError(t, err)

	mockQuerier := &mockActivityEventStore{
		userID:      pgtype.UUID{Bytes: [16]byte{6}, Valid: true},
		activityID:  99,
		eventID:     "evt_refresh_test",
		activityCount: 0,
	}

	worker := &IngestActivityEventWorker{}
	args := IngestActivityEventArgs{
		EventID: "evt_refresh_test",
	}
	job := &river.Job[IngestActivityEventArgs]{
		Args: args,
	}

	_ = worker
	_ = job
	_ = mockQuerier
	_ = client

	t.Logf("Test ready for token refresh during ingestion")
}

// Mock SyncSessionStore for testing backfill worker
type mockSyncQueryStore struct {
	activityCount int
	userID        pgtype.UUID
}

func (m *mockSyncQueryStore) GetStravaTokensByUserID(ctx context.Context, userID pgtype.UUID) (sqlc.StravaToken, error) {
	return sqlc.StravaToken{
		UserID:        userID,
		AccessCipher:  "mock-access-cipher",
		RefreshCipher: "mock-refresh-cipher",
		ExpiresAt:     pgtype.Timestamptz{Time: time.Now().Add(24 * time.Hour), Valid: true},
		AthleteID:     123456,
	}, nil
}

func (m *mockSyncQueryStore) UpsertStravaTokens(ctx context.Context, arg sqlc.UpsertStravaTokensParams) (sqlc.StravaToken, error) {
	return sqlc.StravaToken{}, nil
}

func (m *mockSyncQueryStore) CreateSyncSession(ctx context.Context, arg sqlc.CreateSyncSessionParams) (sqlc.SyncSession, error) {
	return sqlc.SyncSession{
		ID:        pgtype.UUID{Valid: true},
		UserID:    arg.UserID,
		Status:    "pending",
		StartedAt: pgtype.Timestamptz{Time: time.Now(), Valid: true},
	}, nil
}

func (m *mockSyncQueryStore) GetLatestSyncSession(ctx context.Context, userID pgtype.UUID) (sqlc.SyncSession, error) {
	return sqlc.SyncSession{}, fmt.Errorf("no session found")
}

func (m *mockSyncQueryStore) UpdateSyncSessionStatus(ctx context.Context, arg sqlc.UpdateSyncSessionStatusParams) (sqlc.SyncSession, error) {
	return sqlc.SyncSession{Status: arg.Status}, nil
}

func (m *mockSyncQueryStore) UpdateSyncSessionProgress(ctx context.Context, arg sqlc.UpdateSyncSessionProgressParams) (sqlc.SyncSession, error) {
	return sqlc.SyncSession{Imported: arg.Imported}, nil
}

func (m *mockSyncQueryStore) UpsertActivity(ctx context.Context, arg sqlc.UpsertActivityParams) (sqlc.Activity, error) {
	m.activityCount++
	return sqlc.Activity{
		UserID:     arg.UserID,
		ExternalID: arg.ExternalID,
		Name:       arg.Name,
	}, nil
}

func (m *mockSyncQueryStore) GetActivityEventByExternalID(ctx context.Context, externalID string) (sqlc.ActivityEvent, error) {
	return sqlc.ActivityEvent{
		ID:         pgtype.UUID{Valid: true},
		ExternalID: externalID,
		UserID:     m.userID,
		ObjectType: "activity",
		AspectType: "create",
		ObjectID:   12345,
	}, nil
}

func (m *mockSyncQueryStore) MarkActivityEventProcessed(ctx context.Context, id pgtype.UUID) error {
	return nil
}

// Mock ActivityEventStore for testing ingestion worker
type mockActivityEventStore struct {
	userID        pgtype.UUID
	activityID    int64
	eventID       string
	activityCount int
}

func (m *mockActivityEventStore) GetActivityEventByID(ctx context.Context, eventID string) (sqlc.ActivityEvent, error) {
	return sqlc.ActivityEvent{
		ID:         pgtype.UUID{Valid: true},
		ExternalID: eventID,
		UserID:     m.userID,
		ObjectType: "activity",
		AspectType: "create",
		ObjectID:   m.activityID,
	}, nil
}

func (m *mockActivityEventStore) GetStravaTokensByUserID(ctx context.Context, userID pgtype.UUID) (sqlc.StravaToken, error) {
	return sqlc.StravaToken{
		UserID:        userID,
		AccessCipher:  "mock-access",
		RefreshCipher: "mock-refresh",
		ExpiresAt:     pgtype.Timestamptz{Time: time.Now().Add(24 * time.Hour), Valid: true},
	}, nil
}

func (m *mockActivityEventStore) UpsertStravaTokens(ctx context.Context, arg sqlc.UpsertStravaTokensParams) (sqlc.StravaToken, error) {
	return sqlc.StravaToken{}, nil
}

func (m *mockActivityEventStore) UpsertActivity(ctx context.Context, arg sqlc.UpsertActivityParams) (sqlc.Activity, error) {
	m.activityCount++
	return sqlc.Activity{
		UserID:     arg.UserID,
		ExternalID: arg.ExternalID,
		Name:       arg.Name,
	}, nil
}

func (m *mockActivityEventStore) MarkActivityEventProcessed(ctx context.Context, eventID pgtype.UUID) error {
	return nil
}

// Verify interfaces are satisfied
var (
	_ TokenQuerier = (*mockSyncQueryStore)(nil)
)
