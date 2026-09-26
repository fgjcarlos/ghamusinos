package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/fgjcarlos/ghamusinos/internal/auth"
	"github.com/fgjcarlos/ghamusinos/internal/db/sqlc"
	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stretchr/testify/require"
)

// reqWithActivityID builds a GET /api/v1/activities/{idStr} request with
// both the auth user and the chi RouteCtx so chi.URLParam(r, "id")
// inside the handler returns idStr. Replaces the previous pattern that
// relied on r.URL.Path parsing — GetActivity now uses chi.URLParam.
func reqWithActivityID(t *testing.T, user *auth.User, idStr string) *http.Request {
	t.Helper()
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", idStr)
	ctx := auth.WithAuthUser(context.WithValue(context.Background(), chi.RouteCtxKey, rctx), user)
	return httptest.NewRequestWithContext(ctx, "GET", "/api/v1/activities/"+idStr, nil)
}

// T1.1: GET /api/v1/activities returns user's activities sorted by started_at DESC, paginated (limit/offset)
func TestListActivities_ReturnsActivitiesSortedAndPaginated(t *testing.T) {
	mockQ := newActivitiesMockQuerier(t)

	// Setup: mock returns 2 activities for user 1, sorted DESC
	// UUID must match: 00000000-0000-0000-0000-000000000001
	user1ID := pgtype.UUID{Bytes: [16]byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1}, Valid: true}
	activity1 := sqlc.Activity{
		ID:             pgtype.UUID{Bytes: [16]byte{2}, Valid: true},
		UserID:         user1ID,
		ExternalSource: "strava",
		ExternalID:     99,
		Name:           "Morning Run",
		SportType:      "Run",
		StartedAt:      pgtype.Timestamptz{Time: time.Date(2025, 1, 15, 8, 0, 0, 0, time.UTC), Valid: true},
		ElapsedSeconds: 3600,
		MovingSeconds:  3500,
		DistanceMeters: pgtype.Numeric{Valid: true},
		ElevationGainM: pgtype.Numeric{Valid: true},
		AvgHr:          pgtype.Int2{Int16: 140, Valid: true},
		MaxHr:          pgtype.Int2{Int16: 155, Valid: true},
		AvgPower:       pgtype.Int2{Int16: 0, Valid: false},
		RawPayload:     []byte("{}"),
		CreatedAt:      pgtype.Timestamptz{Time: time.Now(), Valid: true},
		UpdatedAt:      pgtype.Timestamptz{Time: time.Now(), Valid: true},
	}
	activity2 := sqlc.Activity{
		ID:             pgtype.UUID{Bytes: [16]byte{3}, Valid: true},
		UserID:         user1ID,
		ExternalSource: "strava",
		ExternalID:     98,
		Name:           "Evening Bike",
		SportType:      "Ride",
		StartedAt:      pgtype.Timestamptz{Time: time.Date(2025, 1, 10, 18, 0, 0, 0, time.UTC), Valid: true},
		ElapsedSeconds: 5400,
		MovingSeconds:  5000,
		DistanceMeters: pgtype.Numeric{Valid: true},
		ElevationGainM: pgtype.Numeric{Valid: true},
		AvgHr:          pgtype.Int2{Int16: 130, Valid: true},
		MaxHr:          pgtype.Int2{Int16: 160, Valid: true},
		AvgPower:       pgtype.Int2{Int16: 200, Valid: true},
		RawPayload:     []byte("{}"),
		CreatedAt:      pgtype.Timestamptz{Time: time.Now(), Valid: true},
		UpdatedAt:      pgtype.Timestamptz{Time: time.Now(), Valid: true},
	}

	mockQ.ListActivitiesByUserFunc = func(ctx context.Context, arg sqlc.ListActivitiesByUserParams) ([]sqlc.ListActivitiesByUserRow, error) {
		// Verificamos los params que el handler pasa tras el cambio a
		// limit+1 (issue #172, M6): el handler pide limit+1 para detectar
		// has_next sin segunda query.
		if arg.Limit != 21 || arg.Offset != 0 {
			t.Errorf("expected limit=21 (limit+1), offset=0; got limit=%d, offset=%d", arg.Limit, arg.Offset)
		}
		return []sqlc.ListActivitiesByUserRow{
			activityToRow(activity1, 42),
			activityToRow(activity2, 42),
		}, nil
	}

	handler := ListActivities(mockQ)
	req := httptest.NewRequestWithContext(
		auth.WithAuthUser(context.Background(), &auth.User{
			ID:          "00000000-0000-0000-0000-000000000001",
			ClerkUserID: "clerk_123",
		}),
		"GET",
		"/api/v1/activities?page=1&limit=20",
		nil,
	)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
		t.Logf("body: %s", w.Body.String())
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	// Check response structure
	data, ok := resp["data"].([]interface{})
	if !ok {
		t.Fatalf("expected data field as array, got %v", resp["data"])
	}

	if len(data) != 2 {
		t.Errorf("expected 2 activities, got %d", len(data))
	}

	// Check sorting: most recent first (activity1 is 2025-01-15, activity2 is 2025-01-10)
	dataMap := data[0].(map[string]interface{})
	if dataMap["name"] != "Morning Run" {
		t.Errorf("expected first activity to be 'Morning Run', got %v", dataMap["name"])
	}

	// Check pagination response fields
	if resp["page"] != float64(1) {
		t.Errorf("expected page=1, got %v", resp["page"])
	}
	if resp["total"] == nil {
		t.Errorf("expected total field, got nil")
	}
	if resp["has_next"] == nil {
		t.Errorf("expected has_next field, got nil")
	}
}

// T1.2: GET /api/v1/activities returns HTTP 401 without valid Clerk JWT
func TestListActivities_UnauthorizedWithoutJWT(t *testing.T) {
	mockQ := newActivitiesMockQuerier(t)
	handler := ListActivities(mockQ)
	req := httptest.NewRequestWithContext(context.Background(), "GET", "/api/v1/activities", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", w.Code)
	}

	// Check RFC 9457 format
	contentType := w.Header().Get("Content-Type")
	if contentType != "application/problem+json" {
		t.Errorf("expected Content-Type 'application/problem+json', got %q", contentType)
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	if resp["status"] != float64(401) {
		t.Errorf("expected status=401, got %v", resp["status"])
	}
}

// T1.3: GET /api/v1/activities?limit=20&page=2 returns correct slice (page math)
func TestListActivities_PaginationPage2(t *testing.T) {
	mockQ := newActivitiesMockQuerier(t)

	// Setup: mock verifies correct offset calculation (page 2, limit 20 = offset 20)
	// UUID must match: 00000000-0000-0000-0000-000000000001
	user1ID := pgtype.UUID{Bytes: [16]byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1}, Valid: true}
	activity := sqlc.Activity{
		ID:             pgtype.UUID{Bytes: [16]byte{10}, Valid: true},
		UserID:         user1ID,
		ExternalSource: "strava",
		ExternalID:     100,
		Name:           "Page 2 Activity",
		SportType:      "Run",
		StartedAt:      pgtype.Timestamptz{Time: time.Now(), Valid: true},
		ElapsedSeconds: 3600,
		MovingSeconds:  3500,
		RawPayload:     []byte("{}"),
		CreatedAt:      pgtype.Timestamptz{Time: time.Now(), Valid: true},
		UpdatedAt:      pgtype.Timestamptz{Time: time.Now(), Valid: true},
	}

	mockQ.ListActivitiesByUserFunc = func(ctx context.Context, arg sqlc.ListActivitiesByUserParams) ([]sqlc.ListActivitiesByUserRow, error) {
		// Page 2, limit 20 → offset=20; el handler sigue pidiendo limit+1=21.
		if arg.Limit != 21 || arg.Offset != 20 {
			t.Errorf("expected limit=21, offset=20 for page 2; got limit=%d, offset=%d", arg.Limit, arg.Offset)
		}
		return []sqlc.ListActivitiesByUserRow{activityToRow(activity, 7)}, nil
	}

	handler := ListActivities(mockQ)
	req := httptest.NewRequestWithContext(
		auth.WithAuthUser(context.Background(), &auth.User{
			ID:          "00000000-0000-0000-0000-000000000001",
			ClerkUserID: "clerk_123",
		}),
		"GET",
		"/api/v1/activities?page=2&limit=20",
		nil,
	)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

// T1.4: GET /api/v1/activities/{id} returns HTTP 200 with activity owned by requesting user
func TestGetActivity_ReturnsActivityOwnedByUser(t *testing.T) {
	mockQ := newActivitiesMockQuerier(t)
	// UUID must match: 00000000-0000-0000-0000-000000000001
	user1ID := pgtype.UUID{Bytes: [16]byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1}, Valid: true}

	activity := sqlc.Activity{
		ID:             pgtype.UUID{Bytes: [16]byte{5}, Valid: true},
		UserID:         user1ID,
		ExternalSource: "strava",
		ExternalID:     123,
		Name:           "My Activity",
		SportType:      "Run",
		StartedAt:      pgtype.Timestamptz{Time: time.Date(2025, 1, 15, 8, 0, 0, 0, time.UTC), Valid: true},
		ElapsedSeconds: 3600,
		MovingSeconds:  3500,
		DistanceMeters: pgtype.Numeric{Valid: true},
		ElevationGainM: pgtype.Numeric{Valid: true},
		AvgHr:          pgtype.Int2{Int16: 140, Valid: true},
		MaxHr:          pgtype.Int2{Int16: 155, Valid: true},
		AvgPower:       pgtype.Int2{Int16: 0, Valid: false},
		RawPayload:     []byte(`{"id":123,"name":"My Activity"}`),
		CreatedAt:      pgtype.Timestamptz{Time: time.Now(), Valid: true},
		UpdatedAt:      pgtype.Timestamptz{Time: time.Now(), Valid: true},
	}

	mockQ.GetActivityByExternalIDFunc = func(ctx context.Context, arg sqlc.GetActivityByExternalIDParams) (sqlc.Activity, error) {
		return activity, nil
	}

	handler := GetActivity(mockQ)
	req := reqWithActivityID(t, &auth.User{
		ID:          "00000000-0000-0000-0000-000000000001",
		ClerkUserID: "clerk_123",
	}, "123")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp sqlc.Activity
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	if resp.Name != "My Activity" {
		t.Errorf("expected name='My Activity', got %v", resp.Name)
	}
}

// T1.5: GET /api/v1/activities/{id} returns RFC 9457 HTTP 404 if activity not found
func TestGetActivity_NotFoundWhenActivityDoesNotExist(t *testing.T) {
	mockQ := newActivitiesMockQuerier(t)

	mockQ.GetActivityByExternalIDFunc = func(ctx context.Context, arg sqlc.GetActivityByExternalIDParams) (sqlc.Activity, error) {
		return sqlc.Activity{}, pgx.ErrNoRows
	}

	handler := GetActivity(mockQ)
	req := reqWithActivityID(t, &auth.User{
		ID:          "00000000-0000-0000-0000-000000000001",
		ClerkUserID: "clerk_123",
	}, "123")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}

	contentType := w.Header().Get("Content-Type")
	if contentType != "application/problem+json" {
		t.Errorf("expected Content-Type 'application/problem+json', got %q", contentType)
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	if resp["status"] != float64(404) {
		t.Errorf("expected status=404, got %v", resp["status"])
	}
}

// T1.6: GET /api/v1/activities/{id} returns RFC 9457 HTTP 404 if activity belongs to another user
func TestGetActivity_ForbiddenWhenActivityBelongsToAnotherUser(t *testing.T) {
	mockQ := newActivitiesMockQuerier(t)
	otherUserID := pgtype.UUID{Bytes: [16]byte{99}, Valid: true}

	activity := sqlc.Activity{
		ID:        pgtype.UUID{Bytes: [16]byte{5}, Valid: true},
		UserID:    otherUserID, // Different user!
		Name:      "Other User's Activity",
		SportType: "Run",
	}

	mockQ.GetActivityByExternalIDFunc = func(ctx context.Context, arg sqlc.GetActivityByExternalIDParams) (sqlc.Activity, error) {
		return activity, nil
	}

	handler := GetActivity(mockQ)
	req := reqWithActivityID(t, &auth.User{
		ID:          "user-123",
		ClerkUserID: "clerk_123",
	}, "123")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404 for different user's activity, got %d", w.Code)
	}
}

// T1.7: GET /api/v1/sync/status returns running session for authenticated user (requires GetLatestSyncSession query)
func TestSyncStatus_ReturnsRunningSyncSession(t *testing.T) {
	mockQ := newActivitiesMockQuerier(t)
	// UUID must match: 00000000-0000-0000-0000-000000000001
	user1ID := pgtype.UUID{Bytes: [16]byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1}, Valid: true}

	syncSession := sqlc.SyncSession{
		ID:              pgtype.UUID{Bytes: [16]byte{6}, Valid: true},
		UserID:          user1ID,
		Status:          "running",
		WindowDays:      42,
		TotalActivities: 100,
		Imported:        45,
		Skipped:         5,
		Error:           pgtype.Text{String: "", Valid: false},
		StartedAt:       pgtype.Timestamptz{Time: time.Now(), Valid: true},
		FinishedAt:      pgtype.Timestamptz{Time: time.Time{}, Valid: false},
	}

	mockQ.GetLatestSyncSessionFunc = func(ctx context.Context, userID pgtype.UUID) (sqlc.SyncSession, error) {
		return syncSession, nil
	}

	handler := SyncStatus(mockQ)
	req := httptest.NewRequestWithContext(
		auth.WithAuthUser(context.Background(), &auth.User{
			ID:          "user-123",
			ClerkUserID: "clerk_123",
		}),
		"GET",
		"/api/v1/sync/status",
		nil,
	)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp sqlc.SyncSession
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	if resp.Status != "running" {
		t.Errorf("expected status='running', got %v", resp.Status)
	}
	if resp.Imported != 45 {
		t.Errorf("expected imported=45, got %d", resp.Imported)
	}
}

// T1.8: GET /api/v1/sync/status returns RFC 9457 HTTP 404 if no session exists for user
func TestSyncStatus_NotFoundWhenNoSessionExists(t *testing.T) {
	mockQ := newActivitiesMockQuerier(t)

	mockQ.GetLatestSyncSessionFunc = func(ctx context.Context, userID pgtype.UUID) (sqlc.SyncSession, error) {
		return sqlc.SyncSession{}, pgx.ErrNoRows
	}

	handler := SyncStatus(mockQ)
	req := httptest.NewRequestWithContext(
		auth.WithAuthUser(context.Background(), &auth.User{
			ID:          "00000000-0000-0000-0000-000000000001",
			ClerkUserID: "clerk_123",
		}),
		"GET",
		"/api/v1/sync/status",
		nil,
	)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}

	contentType := w.Header().Get("Content-Type")
	if contentType != "application/problem+json" {
		t.Errorf("expected Content-Type 'application/problem+json', got %q", contentType)
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	if resp["status"] != float64(404) {
		t.Errorf("expected status=404, got %v", resp["status"])
	}
}

// Mock Querier for activities tests
type activitiesMockQuerier struct {
	t *testing.T

	ListActivitiesByUserFunc    func(context.Context, sqlc.ListActivitiesByUserParams) ([]sqlc.ListActivitiesByUserRow, error)
	GetActivityByExternalIDFunc func(context.Context, sqlc.GetActivityByExternalIDParams) (sqlc.Activity, error)
	GetLatestSyncSessionFunc    func(context.Context, pgtype.UUID) (sqlc.SyncSession, error)
}

func newActivitiesMockQuerier(t *testing.T) *activitiesMockQuerier {
	return &activitiesMockQuerier{t: t}
}

// Implement sqlc.Querier interface - stub all methods except activities ones

func (m *activitiesMockQuerier) ListActivitiesByUser(ctx context.Context, arg sqlc.ListActivitiesByUserParams) ([]sqlc.ListActivitiesByUserRow, error) {
	if m.ListActivitiesByUserFunc != nil {
		return m.ListActivitiesByUserFunc(ctx, arg)
	}
	return nil, nil
}

func (m *activitiesMockQuerier) GetActivityByExternalID(ctx context.Context, arg sqlc.GetActivityByExternalIDParams) (sqlc.Activity, error) {
	if m.GetActivityByExternalIDFunc != nil {
		return m.GetActivityByExternalIDFunc(ctx, arg)
	}
	return sqlc.Activity{}, nil
}

func (m *activitiesMockQuerier) GetActivityEventByID(ctx context.Context, id pgtype.UUID) (sqlc.ActivityEvent, error) {
	return sqlc.ActivityEvent{}, nil
}

func (m *activitiesMockQuerier) GetUserIDByAthleteID(ctx context.Context, athleteID int64) (pgtype.UUID, error) {
	return pgtype.UUID{}, nil
}

func (m *activitiesMockQuerier) GetLatestSyncSession(ctx context.Context, userID pgtype.UUID) (sqlc.SyncSession, error) {
	if m.GetLatestSyncSessionFunc != nil {
		return m.GetLatestSyncSessionFunc(ctx, userID)
	}
	return sqlc.SyncSession{}, nil
}

// Stub all other querier methods
func (m *activitiesMockQuerier) CreateInvite(ctx context.Context, arg sqlc.CreateInviteParams) (sqlc.Invite, error) {
	return sqlc.Invite{}, nil
}
func (m *activitiesMockQuerier) CreateSyncSession(ctx context.Context, arg sqlc.CreateSyncSessionParams) (sqlc.SyncSession, error) {
	return sqlc.SyncSession{}, nil
}
func (m *activitiesMockQuerier) CreateUser(ctx context.Context, arg sqlc.CreateUserParams) (sqlc.User, error) {
	return sqlc.User{}, nil
}
func (m *activitiesMockQuerier) DeleteStravaTokensByUserID(ctx context.Context, userID pgtype.UUID) error {
	return nil
}
func (m *activitiesMockQuerier) EnqueueActivityEvent(ctx context.Context, arg sqlc.EnqueueActivityEventParams) (sqlc.ActivityEvent, error) {
	return sqlc.ActivityEvent{}, nil
}
func (m *activitiesMockQuerier) GetActiveInviteByEmail(ctx context.Context, email string) (sqlc.GetActiveInviteByEmailRow, error) {
	return sqlc.GetActiveInviteByEmailRow{}, nil
}
func (m *activitiesMockQuerier) GetInviteByTokenHash(ctx context.Context, tokenHash string) (sqlc.Invite, error) {
	return sqlc.Invite{}, nil
}
func (m *activitiesMockQuerier) GetStravaTokensByUserID(ctx context.Context, userID pgtype.UUID) (sqlc.StravaToken, error) {
	return sqlc.StravaToken{}, nil
}
func (m *activitiesMockQuerier) GetUserByClerkID(ctx context.Context, clerkUserID string) (sqlc.User, error) {
	return sqlc.User{}, nil
}
func (m *activitiesMockQuerier) ListPendingActivityEvents(ctx context.Context, limit int32) ([]sqlc.ActivityEvent, error) {
	return nil, nil
}
func (m *activitiesMockQuerier) ListSyncSessionsByUser(ctx context.Context, arg sqlc.ListSyncSessionsByUserParams) ([]sqlc.SyncSession, error) {
	return nil, nil
}
func (m *activitiesMockQuerier) MarkActivityEventProcessed(ctx context.Context, id pgtype.UUID) error {
	return nil
}
func (m *activitiesMockQuerier) MarkInviteAccepted(ctx context.Context, id pgtype.UUID) error {
	return nil
}
func (m *activitiesMockQuerier) UpdateSyncSessionProgress(ctx context.Context, arg sqlc.UpdateSyncSessionProgressParams) (sqlc.SyncSession, error) {
	return sqlc.SyncSession{}, nil
}
func (m *activitiesMockQuerier) UpdateSyncSessionStatus(ctx context.Context, arg sqlc.UpdateSyncSessionStatusParams) (sqlc.SyncSession, error) {
	return sqlc.SyncSession{}, nil
}
func (m *activitiesMockQuerier) UpdateUserPreferences(ctx context.Context, arg sqlc.UpdateUserPreferencesParams) (sqlc.User, error) {
	return sqlc.User{}, nil
}
func (m *activitiesMockQuerier) UpdateUserProfile(ctx context.Context, arg sqlc.UpdateUserProfileParams) (sqlc.User, error) {
	return sqlc.User{}, nil
}
func (m *activitiesMockQuerier) UpdateUserInviteStatus(ctx context.Context, arg sqlc.UpdateUserInviteStatusParams) (sqlc.User, error) {
	return sqlc.User{}, nil
}
func (m *activitiesMockQuerier) UpsertActivity(ctx context.Context, arg sqlc.UpsertActivityParams) (sqlc.Activity, error) {
	return sqlc.Activity{}, nil
}
func (m *activitiesMockQuerier) UpsertActivityStream(ctx context.Context, arg sqlc.UpsertActivityStreamParams) (sqlc.ActivityStream, error) {
	return sqlc.ActivityStream{}, nil
}
func (m *activitiesMockQuerier) UpsertStravaTokens(ctx context.Context, arg sqlc.UpsertStravaTokensParams) (sqlc.StravaToken, error) {
	return sqlc.StravaToken{}, nil
}
func (m *activitiesMockQuerier) GetHRZonesByActivity(ctx context.Context, activityID pgtype.UUID) (sqlc.HrZone, error) {
	return sqlc.HrZone{}, nil
}
func (m *activitiesMockQuerier) GetUserHRMaxByID(ctx context.Context, userID pgtype.UUID) (pgtype.Int2, error) {
	return pgtype.Int2{}, nil
}
func (m *activitiesMockQuerier) UpsertHRZones(ctx context.Context, arg sqlc.UpsertHRZonesParams) (sqlc.HrZone, error) {
	return sqlc.HrZone{}, nil
}

// Stubs gpx añadidos tras el regen de SQLC (issue #172, M6).
// Estos tests no ejercitan el GPX lab, pero el mock debe implementar
// la interfaz completa para que el compilador no proteste.
func (m *activitiesMockQuerier) CreateGPXClimb(ctx context.Context, arg sqlc.CreateGPXClimbParams) (sqlc.GpxClimb, error) {
	return sqlc.GpxClimb{}, nil
}
func (m *activitiesMockQuerier) CreateGPXRiskZone(ctx context.Context, arg sqlc.CreateGPXRiskZoneParams) (sqlc.GpxRiskZone, error) {
	return sqlc.GpxRiskZone{}, nil
}
func (m *activitiesMockQuerier) CreateGPXTrack(ctx context.Context, arg sqlc.CreateGPXTrackParams) (sqlc.GpxTrack, error) {
	return sqlc.GpxTrack{}, nil
}
func (m *activitiesMockQuerier) DeleteGPXTrack(ctx context.Context, arg sqlc.DeleteGPXTrackParams) error {
	return nil
}
func (m *activitiesMockQuerier) GetGPXTrackByHash(ctx context.Context, arg sqlc.GetGPXTrackByHashParams) (sqlc.GpxTrack, error) {
	return sqlc.GpxTrack{}, nil
}
func (m *activitiesMockQuerier) GetGPXTrackByID(ctx context.Context, arg sqlc.GetGPXTrackByIDParams) (sqlc.GpxTrack, error) {
	return sqlc.GpxTrack{}, nil
}
func (m *activitiesMockQuerier) ListGPXClimbsByTrack(ctx context.Context, trackID pgtype.UUID) ([]sqlc.GpxClimb, error) {
	return nil, nil
}
func (m *activitiesMockQuerier) ListGPXRiskZonesByTrack(ctx context.Context, trackID pgtype.UUID) ([]sqlc.GpxRiskZone, error) {
	return nil, nil
}
func (m *activitiesMockQuerier) ListGPXTracksByUser(ctx context.Context, arg sqlc.ListGPXTracksByUserParams) ([]sqlc.ListGPXTracksByUserRow, error) {
	return nil, nil
}

// activityToRow aplana una sqlc.Activity a sqlc.ListActivitiesByUserRow
// (issue #172, M6: la query ahora trae también total_count del window
// function, y el handler lo lee de la primera fila).
func activityToRow(a sqlc.Activity, total int64) sqlc.ListActivitiesByUserRow {
	return sqlc.ListActivitiesByUserRow{
		ID:             a.ID,
		UserID:         a.UserID,
		ExternalSource: a.ExternalSource,
		ExternalID:     a.ExternalID,
		Name:           a.Name,
		SportType:      a.SportType,
		StartedAt:      a.StartedAt,
		ElapsedSeconds: a.ElapsedSeconds,
		MovingSeconds:  a.MovingSeconds,
		DistanceMeters: a.DistanceMeters,
		ElevationGainM: a.ElevationGainM,
		AvgHr:          a.AvgHr,
		MaxHr:          a.MaxHr,
		AvgPower:       a.AvgPower,
		RawPayload:     a.RawPayload,
		CreatedAt:      a.CreatedAt,
		UpdatedAt:      a.UpdatedAt,
		TotalCount:     total,
	}
}

// makeActivityRows construye N filas con TotalCount=total. Útil para
// tests que verifican que `total` se sirve del window function y no de
// offset+len (issue #172, M6).
func makeActivityRows(n int, total int64) []sqlc.ListActivitiesByUserRow {
	rows := make([]sqlc.ListActivitiesByUserRow, n)
	for i := 0; i < n; i++ {
		rows[i] = sqlc.ListActivitiesByUserRow{
			ID:             pgtype.UUID{Bytes: [16]byte{byte(i)}, Valid: true},
			UserID:         pgtype.UUID{Valid: true},
			ExternalSource: "strava",
			ExternalID:     int64(i),
			Name:           "Activity",
			StartedAt:      pgtype.Timestamptz{Time: time.Now(), Valid: true},
			TotalCount:     total,
		}
	}
	return rows
}

// TestListActivities_TotalComesFromCountOver verifica que el `total`
// del JSON sale del window function (COUNT(*) OVER()) de la primera
// fila, no de `offset + len + (1 si has_next)`. Antes mentía: con 45
// filas, página 1 reportaba 21. issue #172, M6.
func TestListActivities_TotalComesFromCountOver(t *testing.T) {
	mockQ := newActivitiesMockQuerier(t)
	// 45 filas totales, página 1 devuelve 21 (limit + sentinela).
	mockQ.ListActivitiesByUserFunc = func(ctx context.Context, arg sqlc.ListActivitiesByUserParams) ([]sqlc.ListActivitiesByUserRow, error) {
		return makeActivityRows(21, 45), nil
	}
	handler := ListActivities(mockQ)
	req := httptest.NewRequestWithContext(
		auth.WithAuthUser(context.Background(), &auth.User{ID: "00000000-0000-0000-0000-000000000001"}),
		"GET", "/api/v1/activities?page=1&limit=20", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	var resp map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	require.Equal(t, float64(45), resp["total"], "total debe ser el conteo real, no offset+len+1")
}

// TestListActivities_TotalIsSameAcrossPages verifica que página 3
// también reporta 45, no offset+len.
func TestListActivities_TotalIsSameAcrossPages(t *testing.T) {
	mockQ := newActivitiesMockQuerier(t)
	mockQ.ListActivitiesByUserFunc = func(ctx context.Context, arg sqlc.ListActivitiesByUserParams) ([]sqlc.ListActivitiesByUserRow, error) {
		// Página 3 con limit=20 → offset=40, devuelvo 5 (las últimas 5).
		return makeActivityRows(5, 45), nil
	}
	handler := ListActivities(mockQ)
	req := httptest.NewRequestWithContext(
		auth.WithAuthUser(context.Background(), &auth.User{ID: "00000000-0000-0000-0000-000000000001"}),
		"GET", "/api/v1/activities?page=3&limit=20", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	var resp map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	require.Equal(t, float64(45), resp["total"])
}

// TestListActivities_HasNextCorrectOnExactLastPage verifica el caso
// que el audit detectó: 45 filas, limit=15, página 3 (offset=30) → la
// última página cabe justa (15 filas), no hay más. Antes el handler
// decía has_next=true porque len == limit. issue #172, M6.
func TestListActivities_HasNextCorrectOnExactLastPage(t *testing.T) {
	mockQ := newActivitiesMockQuerier(t)
	mockQ.ListActivitiesByUserFunc = func(ctx context.Context, arg sqlc.ListActivitiesByUserParams) ([]sqlc.ListActivitiesByUserRow, error) {
		// Pedimos limit+1=16 para detectar centinela; devolvemos solo
		// 15 (no hay centinela → última página exacta).
		return makeActivityRows(15, 45), nil
	}
	handler := ListActivities(mockQ)
	req := httptest.NewRequestWithContext(
		auth.WithAuthUser(context.Background(), &auth.User{ID: "00000000-0000-0000-0000-000000000001"}),
		"GET", "/api/v1/activities?page=3&limit=15", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	var resp map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	require.Equal(t, false, resp["has_next"], "última página exacta debe decir has_next=false")
	require.Equal(t, float64(45), resp["total"])
}
