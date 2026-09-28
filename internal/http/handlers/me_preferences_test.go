// Tests for /api/v1/me/preferences handlers (issue #159).

package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/fgjcarlos/ghamusinos/internal/auth"
	"github.com/fgjcarlos/ghamusinos/internal/db/sqlc"
)

// newPatchReq builds a PATCH /api/v1/me/preferences request with a
// minimal user context so the handler reaches validation.
func newPatchReq(body string) *http.Request {
	req := httptest.NewRequestWithContext(
		auth.WithAuthUser(context.Background(), &auth.User{ID: "00000000-0000-0000-0000-000000000001"}),
		http.MethodPatch, "/api/v1/me/preferences", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	return req
}

// preferencesMockQuerier satisfies sqlc.Querier with zero-value
// returns. The methods our handlers actually touch —
// GetUserPreferencesByID and UpdateUserPreferences — are no-ops;
// every other method is included only to satisfy the interface.
type preferencesMockQuerier struct {
	t *testing.T
}

func (m *preferencesMockQuerier) CreateInvite(ctx context.Context, arg sqlc.CreateInviteParams) (sqlc.Invite, error) {
	return sqlc.Invite{}, nil
}
func (m *preferencesMockQuerier) CreateSyncSession(ctx context.Context, arg sqlc.CreateSyncSessionParams) (sqlc.SyncSession, error) {
	return sqlc.SyncSession{}, nil
}
func (m *preferencesMockQuerier) CreateUser(ctx context.Context, arg sqlc.CreateUserParams) (sqlc.User, error) {
	return sqlc.User{}, nil
}
func (m *preferencesMockQuerier) DeleteStravaTokensByUserID(ctx context.Context, userID pgtype.UUID) error {
	return nil
}
func (m *preferencesMockQuerier) EnqueueActivityEvent(ctx context.Context, arg sqlc.EnqueueActivityEventParams) (sqlc.ActivityEvent, error) {
	return sqlc.ActivityEvent{}, nil
}
func (m *preferencesMockQuerier) GetActiveInviteByEmail(ctx context.Context, email string) (sqlc.GetActiveInviteByEmailRow, error) {
	return sqlc.GetActiveInviteByEmailRow{}, nil
}
func (m *preferencesMockQuerier) GetActivityByExternalID(ctx context.Context, arg sqlc.GetActivityByExternalIDParams) (sqlc.Activity, error) {
	return sqlc.Activity{}, nil
}
func (m *preferencesMockQuerier) GetActivityEventByID(ctx context.Context, id pgtype.UUID) (sqlc.ActivityEvent, error) {
	return sqlc.ActivityEvent{}, nil
}
func (m *preferencesMockQuerier) GetLatestSyncSession(ctx context.Context, userID pgtype.UUID) (sqlc.SyncSession, error) {
	return sqlc.SyncSession{}, nil
}
func (m *preferencesMockQuerier) GetUserByClerkID(ctx context.Context, clerkUserID string) (sqlc.User, error) {
	return sqlc.User{}, nil
}
func (m *preferencesMockQuerier) GetUserIDByAthleteID(ctx context.Context, athleteID int64) (pgtype.UUID, error) {
	return pgtype.UUID{}, nil
}
func (m *preferencesMockQuerier) GetUserHRMaxByID(ctx context.Context, userID pgtype.UUID) (pgtype.Int2, error) {
	return pgtype.Int2{}, nil
}
func (m *preferencesMockQuerier) GetHRZonesByActivity(ctx context.Context, activityID pgtype.UUID) (sqlc.HrZone, error) {
	return sqlc.HrZone{}, nil
}
func (m *preferencesMockQuerier) GetUserPreferencesByID(ctx context.Context, id pgtype.UUID) (sqlc.GetUserPreferencesByIDRow, error) {
	return sqlc.GetUserPreferencesByIDRow{}, nil
}
func (m *preferencesMockQuerier) GetInviteByTokenHash(ctx context.Context, tokenHash string) (sqlc.Invite, error) {
	return sqlc.Invite{}, nil
}
func (m *preferencesMockQuerier) GetStravaTokensByUserID(ctx context.Context, userID pgtype.UUID) (sqlc.StravaToken, error) {
	return sqlc.StravaToken{}, nil
}
func (m *preferencesMockQuerier) ListActivitiesByUser(ctx context.Context, arg sqlc.ListActivitiesByUserParams) ([]sqlc.ListActivitiesByUserRow, error) {
	return nil, nil
}
func (m *preferencesMockQuerier) ListPendingActivityEvents(ctx context.Context, limit int32) ([]sqlc.ActivityEvent, error) {
	return nil, nil
}
func (m *preferencesMockQuerier) ListSyncSessionsByUser(ctx context.Context, arg sqlc.ListSyncSessionsByUserParams) ([]sqlc.SyncSession, error) {
	return nil, nil
}
func (m *preferencesMockQuerier) MarkActivityEventProcessed(ctx context.Context, id pgtype.UUID) error {
	return nil
}
func (m *preferencesMockQuerier) MarkInviteAccepted(ctx context.Context, id pgtype.UUID) error {
	return nil
}
func (m *preferencesMockQuerier) UpdateSyncSessionProgress(ctx context.Context, arg sqlc.UpdateSyncSessionProgressParams) (sqlc.SyncSession, error) {
	return sqlc.SyncSession{}, nil
}
func (m *preferencesMockQuerier) UpdateSyncSessionStatus(ctx context.Context, arg sqlc.UpdateSyncSessionStatusParams) (sqlc.SyncSession, error) {
	return sqlc.SyncSession{}, nil
}
func (m *preferencesMockQuerier) UpdateUserInviteStatus(ctx context.Context, arg sqlc.UpdateUserInviteStatusParams) (sqlc.User, error) {
	return sqlc.User{}, nil
}
func (m *preferencesMockQuerier) UpdateUserProfile(ctx context.Context, arg sqlc.UpdateUserProfileParams) (sqlc.User, error) {
	return sqlc.User{}, nil
}
func (m *preferencesMockQuerier) UpdateUserPreferences(ctx context.Context, arg sqlc.UpdateUserPreferencesParams) (sqlc.User, error) {
	return sqlc.User{}, nil
}
func (m *preferencesMockQuerier) UpsertActivity(ctx context.Context, arg sqlc.UpsertActivityParams) (sqlc.Activity, error) {
	return sqlc.Activity{}, nil
}
func (m *preferencesMockQuerier) UpsertActivityStream(ctx context.Context, arg sqlc.UpsertActivityStreamParams) (sqlc.ActivityStream, error) {
	return sqlc.ActivityStream{}, nil
}
func (m *preferencesMockQuerier) UpsertHRZones(ctx context.Context, arg sqlc.UpsertHRZonesParams) (sqlc.HrZone, error) {
	return sqlc.HrZone{}, nil
}
func (m *preferencesMockQuerier) UpsertStravaTokens(ctx context.Context, arg sqlc.UpsertStravaTokensParams) (sqlc.StravaToken, error) {
	return sqlc.StravaToken{}, nil
}

// GPX methods introduced alongside the route detail page (#157).
func (m *preferencesMockQuerier) CreateGPXClimb(ctx context.Context, arg sqlc.CreateGPXClimbParams) (sqlc.GpxClimb, error) {
	return sqlc.GpxClimb{}, nil
}
func (m *preferencesMockQuerier) CreateGPXRiskZone(ctx context.Context, arg sqlc.CreateGPXRiskZoneParams) (sqlc.GpxRiskZone, error) {
	return sqlc.GpxRiskZone{}, nil
}
func (m *preferencesMockQuerier) CreateGPXTrack(ctx context.Context, arg sqlc.CreateGPXTrackParams) (sqlc.GpxTrack, error) {
	return sqlc.GpxTrack{}, nil
}
func (m *preferencesMockQuerier) DeleteGPXTrack(ctx context.Context, arg sqlc.DeleteGPXTrackParams) error {
	return nil
}
func (m *preferencesMockQuerier) GetGPXTrackByHash(ctx context.Context, arg sqlc.GetGPXTrackByHashParams) (sqlc.GpxTrack, error) {
	return sqlc.GpxTrack{}, nil
}
func (m *preferencesMockQuerier) GetGPXTrackByID(ctx context.Context, arg sqlc.GetGPXTrackByIDParams) (sqlc.GpxTrack, error) {
	return sqlc.GpxTrack{}, nil
}
func (m *preferencesMockQuerier) ListGPXClimbsByTrack(ctx context.Context, trackID pgtype.UUID) ([]sqlc.GpxClimb, error) {
	return nil, nil
}
func (m *preferencesMockQuerier) ListGPXRiskZonesByTrack(ctx context.Context, trackID pgtype.UUID) ([]sqlc.GpxRiskZone, error) {
	return nil, nil
}
func (m *preferencesMockQuerier) ListGPXTracksByUser(ctx context.Context, arg sqlc.ListGPXTracksByUserParams) ([]sqlc.ListGPXTracksByUserRow, error) {
	return nil, nil
}

// ----- Tests -----

func TestGetPreferences_Unauthorized(t *testing.T) {
	h := GetPreferences(&preferencesMockQuerier{t: t})
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/v1/me/preferences", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", w.Code)
	}
}

func TestPatchPreferences_Unauthorized(t *testing.T) {
	h := PatchPreferences(&preferencesMockQuerier{t: t})
	body := bytes.NewBufferString(`{"hr_max":180}`)
	req := httptest.NewRequestWithContext(context.Background(), http.MethodPatch, "/api/v1/me/preferences", body)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", w.Code)
	}
}

func TestPatchPreferences_InvalidJSON(t *testing.T) {
	h := PatchPreferences(&preferencesMockQuerier{t: t})
	body := strings.NewReader("not json")
	req := httptest.NewRequestWithContext(auth.WithAuthUser(context.Background(), &auth.User{ID: "00000000-0000-0000-0000-000000000001"}),
		http.MethodPatch, "/api/v1/me/preferences", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestPatchPreferences_RejectsOutOfRange(t *testing.T) {
	cases := []struct {
		name  string
		body  string
		field string
		want  string
	}{
		{"hr_max too low", `{"hr_max":0}`, "hr_max", "hr_max must be between 1 and 260 (or null)"},
		{"hr_max too high", `{"hr_max":261}`, "hr_max", "hr_max must be between 1 and 260 (or null)"},
		{"lthr too low", `{"lthr":0}`, "lthr", "lthr must be between 1 and 260 (or null)"},
		{"ftp too low", `{"ftp":0}`, "ftp", "ftp must be between 1 and 2000 (or null)"},
		{"ftp too high", `{"ftp":2001}`, "ftp", "ftp must be between 1 and 2000 (or null)"},
		{"invalid level", `{"level":"pro"}`, "level", "level must be one of beginner | intermediate | advanced (or null)"},
		{"empty timezone", `{"timezone":""}`, "timezone", "timezone is required and must be a non-empty IANA name"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			h := PatchPreferences(&preferencesMockQuerier{t: t})
			req := newPatchReq(c.body)
			w := httptest.NewRecorder()
			h.ServeHTTP(w, req)
			if w.Code != http.StatusUnprocessableEntity {
				t.Errorf("expected 422 for %s, got %d", c.name, w.Code)
			}
			var problem ProblemDetail
			if err := json.Unmarshal(w.Body.Bytes(), &problem); err != nil {
				t.Fatalf("expected ProblemDetail JSON, got %q (err=%v)", w.Body.String(), err)
			}
			if problem.Status != http.StatusUnprocessableEntity {
				t.Errorf("expected problem.Status=422, got %d", problem.Status)
			}
			got, ok := problem.Errors[c.field]
			if !ok {
				t.Errorf("expected errors[%q] in problem body, got %q", c.field, w.Body.String())
			}
			if got != c.want {
				t.Errorf("expected errors[%q]=%q, got %q", c.field, c.want, got)
			}
		})
	}
}

// TestPatchPreferences_ReportsMultipleFieldErrors verifies that a
// single PATCH body with two invalid fields produces both entries
// in the `errors` extension member. This is the contract the
// /perfil SPA depends on to render per-field 422 messages.
//
// The fixture omits `timezone`, which is required; we therefore
// assert that the response contains at least the two named errors,
// not that the map is exactly two entries long.
func TestPatchPreferences_ReportsMultipleFieldErrors(t *testing.T) {
	h := PatchPreferences(&preferencesMockQuerier{t: t})
	body := `{"hr_max":999,"level":"pro"}`
	req := newPatchReq(body)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422, got %d (body=%s)", w.Code, w.Body.String())
	}
	var problem ProblemDetail
	if err := json.Unmarshal(w.Body.Bytes(), &problem); err != nil {
		t.Fatalf("expected ProblemDetail JSON: %v", err)
	}
	if len(problem.Errors) < 2 {
		t.Errorf("expected at least 2 entries in errors, got %d (%v)", len(problem.Errors), problem.Errors)
	}
	if _, ok := problem.Errors["hr_max"]; !ok {
		t.Errorf("expected errors[hr_max], got %v", problem.Errors)
	}
	if _, ok := problem.Errors["level"]; !ok {
		t.Errorf("expected errors[level], got %v", problem.Errors)
	}
	// The flat Detail still carries the stable summary.
	if problem.Detail == "" {
		t.Errorf("expected non-empty Detail for legacy consumers, got %q", problem.Detail)
	}
}

func TestPatchPreferences_AcceptsValid(t *testing.T) {
	cases := []string{
		`{"hr_max":180,"lthr":150,"ftp":280,"level":"advanced","timezone":"Europe/Madrid","ai_enabled":true}`,
		`{"hr_max":null,"lthr":null,"timezone":"America/New_York"}`,
		`{"hr_max":null,"lthr":null,"ftp":null,"level":null,"timezone":"UTC"}`,
	}
	for _, body := range cases {
		t.Run(body, func(t *testing.T) {
			h := PatchPreferences(&preferencesMockQuerier{t: t})
			req := newPatchReq(body)
			w := httptest.NewRecorder()
			h.ServeHTTP(w, req)
			if w.Code != http.StatusOK {
				t.Errorf("expected 200 for valid body, got %d (body=%s)", w.Code, w.Body.String())
			}
		})
	}
}

func TestPatchPreferences_InvalidTimezone(t *testing.T) {
	h := PatchPreferences(&preferencesMockQuerier{t: t})
	req := newPatchReq(`{"timezone":"Not/A/Real/Zone"}`)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusUnprocessableEntity {
		t.Errorf("expected 422 for invalid timezone, got %d", w.Code)
	}
	var problem ProblemDetail
	if err := json.Unmarshal(w.Body.Bytes(), &problem); err != nil {
		t.Fatalf("expected ProblemDetail JSON: %v", err)
	}
	got, ok := problem.Errors["timezone"]
	if !ok || !strings.Contains(got, "Not/A/Real/Zone") {
		t.Errorf("expected errors[timezone] to contain %q, got %q", "Not/A/Real/Zone", got)
	}
}

func TestPatchPreferences_WarnsWhenLthrExceedsHrMax(t *testing.T) {
	h := PatchPreferences(&preferencesMockQuerier{t: t})
	req := newPatchReq(`{"hr_max":180,"lthr":200,"timezone":"UTC"}`)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("expected 200 (warning is non-blocking), got %d", w.Code)
	}
	var resp map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	if _, ok := resp["warning"]; !ok {
		t.Errorf("expected warning field in response when lthr > hr_max, got %q", w.Body.String())
	}
}

func TestGetPreferences_ReturnsCurrentValues(t *testing.T) {
	h := GetPreferences(&preferencesMockQuerier{t: t})
	req := httptest.NewRequestWithContext(
		auth.WithAuthUser(context.Background(), &auth.User{ID: "00000000-0000-0000-0000-000000000001"}),
		http.MethodGet, "/api/v1/me/preferences", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
	var resp map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	if resp["hr_max"] != nil || resp["timezone"] != "" {
		t.Errorf("expected default empty prefs, got %q", w.Body.String())
	}
}
