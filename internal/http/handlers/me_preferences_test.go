// Tests for /api/v1/me/preferences handlers (issue #159).

package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/fgjcarlos/ghamusinos/internal/auth"
	"github.com/fgjcarlos/ghamusinos/internal/db/sqlc"
)

// aiConsentMigration is the migration that makes the AI opt-in explicit
// for every user (phase 1.5, T2).
const aiConsentMigration = "00014_ai_explicit_opt_in.sql"

// readRepoFile reads a file relative to this package directory. It is
// used to assert the on-disk contract of the consent migration and of
// the sqlc schema, which are the storage half of the same consent
// guarantee the handlers expose.
func readRepoFile(t *testing.T, parts ...string) string {
	t.Helper()
	path := filepath.Join(append([]string{"..", ".."}, parts...)...)
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(b)
}

// gooseSections splits a goose migration into its Up and Down halves.
// A migration without a Down section yields an empty Down string.
func gooseSections(t *testing.T, migration string) (up string, down string) {
	t.Helper()
	idx := strings.Index(migration, "-- +goose Down")
	if idx < 0 {
		return migration, ""
	}
	return migration[:idx], migration[idx:]
}

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
//
// The struct also exposes a configurable hook for
// DeleteStravaTokensByUserID so the strava disconnect tests in
// strava_connection_test.go can drive the same mock without
// copying the entire interface. Defaults to zero-value behaviour
// when nil.
type preferencesMockQuerier struct {
	t                  *testing.T
	deleteStravaTokens func(userID pgtype.UUID) error
	// getPreferences overrides the stored preferences returned by
	// GetUserPreferencesByID. Defaults to the zero value.
	getPreferences func(userID pgtype.UUID) sqlc.GetUserPreferencesByIDRow
	// updatePreferences captures the parameters the handler sends to
	// UpdateUserPreferences. Defaults to a no-op.
	updatePreferences func(params sqlc.UpdateUserPreferencesParams)
}

func (m *preferencesMockQuerier) GetActivityStreams(_ context.Context, _ pgtype.UUID) ([]sqlc.ActivityStream, error) {
	return nil, nil
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
	if m.deleteStravaTokens != nil {
		return m.deleteStravaTokens(userID)
	}
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
	if m.getPreferences != nil {
		return m.getPreferences(id), nil
	}
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
	if m.updatePreferences != nil {
		m.updatePreferences(arg)
	}
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
func (m *preferencesMockQuerier) CreateGPXMuro(ctx context.Context, arg sqlc.CreateGPXMuroParams) (sqlc.GpxMuro, error) {
	return sqlc.GpxMuro{}, nil
}
func (m *preferencesMockQuerier) CreateGPXRecoveryZone(ctx context.Context, arg sqlc.CreateGPXRecoveryZoneParams) (sqlc.GpxRecoveryZone, error) {
	return sqlc.GpxRecoveryZone{}, nil
}
func (m *preferencesMockQuerier) UpsertGPXKmVertical(ctx context.Context, arg sqlc.UpsertGPXKmVerticalParams) (sqlc.GpxKmVertical, error) {
	return sqlc.GpxKmVertical{}, nil
}
func (m *preferencesMockQuerier) GetGPXKmVerticalByTrack(ctx context.Context, trackID pgtype.UUID) (sqlc.GpxKmVertical, error) {
	return sqlc.GpxKmVertical{}, nil
}
func (m *preferencesMockQuerier) ListGPXMurosByTrack(ctx context.Context, trackID pgtype.UUID) ([]sqlc.GpxMuro, error) {
	return nil, nil
}
func (m *preferencesMockQuerier) ListGPXRecoveryZonesByTrack(ctx context.Context, trackID pgtype.UUID) ([]sqlc.GpxRecoveryZone, error) {
	return nil, nil
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

func (m *preferencesMockQuerier) SumHRZonesInRange(ctx context.Context, arg sqlc.SumHRZonesInRangeParams) (sqlc.SumHRZonesInRangeRow, error) {
	return sqlc.SumHRZonesInRangeRow{}, nil
}
func (m *preferencesMockQuerier) SumActivitiesDurationInRange(ctx context.Context, arg sqlc.SumActivitiesDurationInRangeParams) (sqlc.SumActivitiesDurationInRangeRow, error) {
	return sqlc.SumActivitiesDurationInRangeRow{}, nil
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

func (m *preferencesMockQuerier) FirstActivityForUser(ctx context.Context, userID pgtype.UUID) (pgtype.Timestamptz, error) {
	return pgtype.Timestamptz{}, nil
}

func (m *preferencesMockQuerier) GetDashboardMetadata(ctx context.Context, userID pgtype.UUID) (sqlc.DashboardMetadatum, error) {
	return sqlc.DashboardMetadatum{}, nil
}

func (m *preferencesMockQuerier) ListActivitiesInRange(ctx context.Context, arg sqlc.ListActivitiesInRangeParams) ([]sqlc.ListActivitiesInRangeRow, error) {
	return []sqlc.ListActivitiesInRangeRow{}, nil
}

// Stub añadido con la query de drift (DA-007); estos tests no la usan.
func (m *preferencesMockQuerier) ListHeartRateStreamsInRange(ctx context.Context, arg sqlc.ListHeartRateStreamsInRangeParams) ([]sqlc.ListHeartRateStreamsInRangeRow, error) {
	return []sqlc.ListHeartRateStreamsInRangeRow{}, nil
}

func (m *preferencesMockQuerier) ListTrainingLoadFromFirstActivity(ctx context.Context, userID pgtype.UUID) ([]sqlc.TrainingLoadDaily, error) {
	return []sqlc.TrainingLoadDaily{}, nil
}

func (m *preferencesMockQuerier) ListTrainingLoadRange(ctx context.Context, arg sqlc.ListTrainingLoadRangeParams) ([]sqlc.TrainingLoadDaily, error) {
	return []sqlc.TrainingLoadDaily{}, nil
}

func (m *preferencesMockQuerier) ListUserIDsForTrainingLoadRecalc(ctx context.Context) ([]pgtype.UUID, error) {
	return nil, nil
}

func (m *preferencesMockQuerier) UpsertDashboardMetadata(ctx context.Context, arg sqlc.UpsertDashboardMetadataParams) (sqlc.DashboardMetadatum, error) {
	return sqlc.DashboardMetadatum{}, nil
}

func (m *preferencesMockQuerier) UpsertTrainingLoadDaily(ctx context.Context, arg sqlc.UpsertTrainingLoadDailyParams) (sqlc.TrainingLoadDaily, error) {
	return sqlc.TrainingLoadDaily{}, nil
}

func (m *preferencesMockQuerier) GetUserByID(ctx context.Context, id pgtype.UUID) (sqlc.User, error) {
	return sqlc.User{}, nil
}

// ---------------------------------------------------------------------------
// AI consent contract (phase 1.5, T2 — S3/S5)
//
// The consent flag (users.ai_enabled) is the only switch that allows the
// product to send training data to an external AI provider. The rules
// below pin the whole contract:
//
//   - the stored default is false (nobody is opted in by accident),
//   - an omitted ai_enabled in a PATCH body is stored as false
//     (omission is never consent),
//   - an explicit ai_enabled=true is the consent act,
//   - the rest of the profile is stored unchanged.
// ---------------------------------------------------------------------------

// TestPatchPreferences_ExplicitTrueStoresConsent proves that an explicit
// ai_enabled=true is stored as true: that PATCH is the consent act.
func TestPatchPreferences_ExplicitTrueStoresConsent(t *testing.T) {
	var captured sqlc.UpdateUserPreferencesParams
	m := &preferencesMockQuerier{
		t: t,
		updatePreferences: func(p sqlc.UpdateUserPreferencesParams) {
			captured = p
		},
	}
	h := PatchPreferences(m)
	req := newPatchReq(`{"hr_max":180,"timezone":"Europe/Madrid","ai_enabled":true}`)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d (body=%s)", w.Code, w.Body.String())
	}
	if !captured.AiEnabled {
		t.Error("expected ai_enabled=true to be stored as the explicit consent")
	}
}

// TestPatchPreferences_OmittedConsentIsStoredAsFalse proves that leaving
// ai_enabled out of the body never grants consent: the omission
// semantics of the existing PATCH contract stay "false".
func TestPatchPreferences_OmittedConsentIsStoredAsFalse(t *testing.T) {
	var captured sqlc.UpdateUserPreferencesParams
	m := &preferencesMockQuerier{
		t: t,
		updatePreferences: func(p sqlc.UpdateUserPreferencesParams) {
			captured = p
		},
	}
	h := PatchPreferences(m)
	req := newPatchReq(`{"hr_max":180,"timezone":"Europe/Madrid"}`)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d (body=%s)", w.Code, w.Body.String())
	}
	if captured.AiEnabled {
		t.Error("expected an omitted ai_enabled to be stored as false, got true")
	}
}

// TestPatchPreferences_ExplicitFalseRevokesConsent proves the user can
// always take the consent back with an explicit false.
func TestPatchPreferences_ExplicitFalseRevokesConsent(t *testing.T) {
	var captured sqlc.UpdateUserPreferencesParams
	m := &preferencesMockQuerier{
		t: t,
		updatePreferences: func(p sqlc.UpdateUserPreferencesParams) {
			captured = p
		},
	}
	h := PatchPreferences(m)
	req := newPatchReq(`{"timezone":"UTC","ai_enabled":false}`)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d (body=%s)", w.Code, w.Body.String())
	}
	if captured.AiEnabled {
		t.Error("expected ai_enabled=false to be stored as false, got true")
	}
}

// TestPatchPreferences_PreservesUnrelatedPreferences proves that saving
// the consent does not alter the rest of the profile: the same
// normalization/validation of #159 is reused for every field.
func TestPatchPreferences_PreservesUnrelatedPreferences(t *testing.T) {
	var captured sqlc.UpdateUserPreferencesParams
	m := &preferencesMockQuerier{
		t: t,
		updatePreferences: func(p sqlc.UpdateUserPreferencesParams) {
			captured = p
		},
	}
	h := PatchPreferences(m)
	req := newPatchReq(`{"hr_max":180,"lthr":150,"ftp":280,"level":"advanced","timezone":"Europe/Madrid","ai_enabled":true}`)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d (body=%s)", w.Code, w.Body.String())
	}
	if captured.HrMax.Int16 != 180 {
		t.Errorf("hr_max = %d, wanted 180", captured.HrMax.Int16)
	}
	if captured.Lthr.Int16 != 150 {
		t.Errorf("lthr = %d, wanted 150", captured.Lthr.Int16)
	}
	if captured.Ftp.Int16 != 280 {
		t.Errorf("ftp = %d, wanted 280", captured.Ftp.Int16)
	}
	if !captured.Level.Valid || captured.Level.String != "advanced" {
		t.Errorf("level = %+v, wanted advanced", captured.Level)
	}
	if captured.Timezone != "Europe/Madrid" {
		t.Errorf("timezone = %q, wanted Europe/Madrid", captured.Timezone)
	}
}

// TestGetPreferences_ReportsStoredConsent proves the GET contract
// exposes the stored consent flag verbatim, so the SPA can render the
// unchecked box and the transfer notice for a user that never accepted.
func TestGetPreferences_ReportsStoredConsent(t *testing.T) {
	m := &preferencesMockQuerier{
		t: t,
		getPreferences: func(pgtype.UUID) sqlc.GetUserPreferencesByIDRow {
			return sqlc.GetUserPreferencesByIDRow{
				Timezone:  "UTC",
				AiEnabled: false,
			}
		},
	}
	h := GetPreferences(m)
	req := httptest.NewRequestWithContext(
		auth.WithAuthUser(context.Background(), &auth.User{ID: "00000000-0000-0000-0000-000000000001"}),
		http.MethodGet, "/api/v1/me/preferences", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var resp map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("expected JSON body: %v", err)
	}
	if got, ok := resp["ai_enabled"].(bool); !ok || got {
		t.Errorf("expected ai_enabled=false in %q", w.Body.String())
	}
}

// TestMigration00014_UpResetsConsentAndDefaultsToFalse pins the storage
// half of the consent contract: the column default becomes false (fresh
// users are never opted in) and every previously stored true is reset
// (the old default-true value cannot prove consent).
func TestMigration00014_UpResetsConsentAndDefaultsToFalse(t *testing.T) {
	migration := readRepoFile(t, "db", "migrations", aiConsentMigration)
	up, _ := gooseSections(t, migration)

	if !strings.Contains(up, "SET DEFAULT false") {
		t.Errorf("expected the migration to set the ai_enabled default to false, got:\n%s", up)
	}
	if !strings.Contains(up, "ALTER COLUMN ai_enabled") {
		t.Errorf("expected the migration to alter the ai_enabled column, got:\n%s", up)
	}
	if !strings.Contains(up, "UPDATE users") || !strings.Contains(up, "SET ai_enabled = false") {
		t.Errorf("expected the migration to reset existing ai_enabled=true rows to false, got:\n%s", up)
	}
}

// TestMigration00014_DownNeverConsents proves the rollback path cannot
// grant consent to anybody: the Down section neither flips a stored
// value to true nor restores a default that would opt in new users.
func TestMigration00014_DownNeverConsents(t *testing.T) {
	migration := readRepoFile(t, "db", "migrations", aiConsentMigration)
	_, down := gooseSections(t, migration)
	if down == "" {
		t.Fatal("expected the migration to declare a Down section")
	}
	if strings.Contains(down, "SET DEFAULT true") {
		t.Errorf("the Down section must not restore a default that consents new users:\n%s", down)
	}
	if strings.Contains(down, "ai_enabled = true") {
		t.Errorf("the Down section must never store ai_enabled=true:\n%s", down)
	}
	if strings.Contains(down, "UPDATE users") {
		t.Errorf("the Down section must not mutate stored consent values:\n%s", down)
	}
}

// TestSchemaDeclaresConsentDefaultFalse proves the sqlc schema (the
// source of truth for the generated queries) agrees with the migration:
// a row inserted without ai_enabled is not opted in.
func TestSchemaDeclaresConsentDefaultFalse(t *testing.T) {
	schema := readRepoFile(t, "db", "schema.sql")
	for _, line := range strings.Split(schema, "\n") {
		if !strings.Contains(line, "ai_enabled") {
			continue
		}
		if strings.Contains(line, "DEFAULT true") {
			t.Errorf("schema.sql must declare ai_enabled with DEFAULT false, got: %s", strings.TrimSpace(line))
		}
		if strings.Contains(line, "DEFAULT false") {
			return
		}
	}
	t.Error("schema.sql does not declare ai_enabled with DEFAULT false")
}
