package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/rivertype"
	"github.com/stretchr/testify/require"

	"github.com/fgjcarlos/ghamusinos/internal/auth"
	"github.com/fgjcarlos/ghamusinos/internal/db/sqlc"
)

// dashboardMockQuerier embebe activitiesMockQuerier (que ya implementa
// todos los métodos de sqlc.Querier con stubs zero-value) y sobreescribe
// los métodos que el handler dashboard invoca.
type dashboardMockQuerier struct {
	*activitiesMockQuerier

	SumHRZonesInRangeErr      error
	SumHRZonesInRangeFunc     func(ctx context.Context, arg sqlc.SumHRZonesInRangeParams) (sqlc.SumHRZonesInRangeRow, error)
	SumActivitiesDurationFunc func(ctx context.Context, arg sqlc.SumActivitiesDurationInRangeParams) (sqlc.SumActivitiesDurationInRangeRow, error)
	FirstActivityForUserFunc  func(ctx context.Context, userID pgtype.UUID) (pgtype.Timestamptz, error)
	GetDashboardMetadataFunc  func(ctx context.Context, userID pgtype.UUID) (sqlc.DashboardMetadatum, error)
	GetLatestSyncSessionFunc  func(ctx context.Context, userID pgtype.UUID) (sqlc.SyncSession, error)
	ListTrainingLoadRangeFunc func(ctx context.Context, arg sqlc.ListTrainingLoadRangeParams) ([]sqlc.TrainingLoadDaily, error)
}

func (m *dashboardMockQuerier) SumHRZonesInRange(ctx context.Context, arg sqlc.SumHRZonesInRangeParams) (sqlc.SumHRZonesInRangeRow, error) {
	if m.SumHRZonesInRangeErr != nil {
		return sqlc.SumHRZonesInRangeRow{}, m.SumHRZonesInRangeErr
	}
	if m.SumHRZonesInRangeFunc != nil {
		return m.SumHRZonesInRangeFunc(ctx, arg)
	}
	return sqlc.SumHRZonesInRangeRow{}, nil
}

func (m *dashboardMockQuerier) SumActivitiesDurationInRange(ctx context.Context, arg sqlc.SumActivitiesDurationInRangeParams) (sqlc.SumActivitiesDurationInRangeRow, error) {
	if m.SumActivitiesDurationFunc != nil {
		return m.SumActivitiesDurationFunc(ctx, arg)
	}
	return sqlc.SumActivitiesDurationInRangeRow{}, nil
}

func (m *dashboardMockQuerier) FirstActivityForUser(ctx context.Context, userID pgtype.UUID) (pgtype.Timestamptz, error) {
	if m.FirstActivityForUserFunc != nil {
		return m.FirstActivityForUserFunc(ctx, userID)
	}
	return pgtype.Timestamptz{}, nil
}

func (m *dashboardMockQuerier) GetDashboardMetadata(ctx context.Context, userID pgtype.UUID) (sqlc.DashboardMetadatum, error) {
	if m.GetDashboardMetadataFunc != nil {
		return m.GetDashboardMetadataFunc(ctx, userID)
	}
	return sqlc.DashboardMetadatum{}, nil
}

func (m *dashboardMockQuerier) GetLatestSyncSession(ctx context.Context, userID pgtype.UUID) (sqlc.SyncSession, error) {
	if m.GetLatestSyncSessionFunc != nil {
		return m.GetLatestSyncSessionFunc(ctx, userID)
	}
	return sqlc.SyncSession{}, nil
}

func (m *dashboardMockQuerier) ListTrainingLoadRange(ctx context.Context, arg sqlc.ListTrainingLoadRangeParams) ([]sqlc.TrainingLoadDaily, error) {
	if m.ListTrainingLoadRangeFunc != nil {
		return m.ListTrainingLoadRangeFunc(ctx, arg)
	}
	return nil, nil
}

// withDevAuthRequest inyecta un usuario sintético dev@local en el
// contexto de la request (mismo truco que devUserMiddleware cuando
// AUTH_DISABLED=true).
func withDevAuthRequest(r *http.Request) *http.Request {
	dev := &auth.User{
		ID:          "00000000-0000-0000-0000-000000000001",
		ClerkUserID: "user_dev",
		Email:       "dev@local",
	}
	return r.WithContext(auth.WithAuthUser(r.Context(), dev))
}

// newDashboardMock returns a dashboardMockQuerier backed by an
// activitiesMockQuerier (which provides zero-value stubs for all
// other sqlc.Querier methods).
func newDashboardMock() *dashboardMockQuerier {
	return &dashboardMockQuerier{
		activitiesMockQuerier: newActivitiesMockQuerier(nil),
	}
}

func TestDashboardSummary_401WithoutAuth(t *testing.T) {
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/v1/dashboard/summary", nil)
	rec := httptest.NewRecorder()
	GetDashboardSummary(newDashboardMock()).ServeHTTP(rec, req)
	require.Equal(t, http.StatusUnauthorized, rec.Code, "expected 401 sin auth")
}

func TestDashboardSummary_OKShapeWithData(t *testing.T) {
	mock := newDashboardMock()
	mock.SumHRZonesInRangeFunc = func(_ context.Context, _ sqlc.SumHRZonesInRangeParams) (sqlc.SumHRZonesInRangeRow, error) {
		return sqlc.SumHRZonesInRangeRow{
			Z1Seconds:           0,
			Z2Seconds:           0,
			Z3Seconds:           3600,
			Z4Seconds:           1800,
			Z5Seconds:           0,
			ActivitiesWithZones: 2,
		}, nil
	}
	mock.SumActivitiesDurationFunc = func(_ context.Context, _ sqlc.SumActivitiesDurationInRangeParams) (sqlc.SumActivitiesDurationInRangeRow, error) {
		return sqlc.SumActivitiesDurationInRangeRow{
			ActivitiesTotal:         3,
			ElapsedSecondsTotal:     5400,
			ElapsedSecondsWithAvgHr: 5400,
			ActivitiesWithAvgHr:     3,
		}, nil
	}

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/v1/dashboard/summary", nil)
	req = withDevAuthRequest(req)
	rec := httptest.NewRecorder()
	GetDashboardSummary(mock).ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)

	var body map[string]interface{}
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&body))

	// Spec DA-001 shape:
	//   { weekly_volume_m, weekly_elevation_m, weekly_activities_count,
	//     trend_7d_pct, trend_30d_pct }
	require.Contains(t, body, "weekly_volume_m")
	require.Contains(t, body, "weekly_elevation_m")
	require.Contains(t, body, "weekly_activities_count")
	require.Contains(t, body, "trend_7d_pct")
	require.Contains(t, body, "trend_30d_pct")
}

func TestDashboardSummary_OKEmptyData(t *testing.T) {
	mock := newDashboardMock()
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/v1/dashboard/summary", nil)
	req = withDevAuthRequest(req)
	rec := httptest.NewRecorder()
	GetDashboardSummary(mock).ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)
}

func TestDashboardSummary_ParseDashboardRange(t *testing.T) {
	req := httptest.NewRequest("GET", "/x?from=2024-01-01&to=2024-01-31", nil)
	from, to, err := parseDashboardRange(req)
	require.NoError(t, err)
	require.Equal(t, 2024, from.Year())
	require.Equal(t, time.January, from.Month())
	require.Equal(t, 1, from.Day())
	require.Equal(t, 2024, to.Year())
	require.Equal(t, time.January, to.Month())
	require.Equal(t, 31, to.Day())

	// Default (sin params) → 30 días.
	req2 := httptest.NewRequest("GET", "/x", nil)
	from, to, err = parseDashboardRange(req2)
	require.NoError(t, err)
	require.Equal(t, 30, int(to.Sub(from).Hours()/24))

	// From > to → error.
	req3 := httptest.NewRequest("GET", "/x?from=2024-02-01&to=2024-01-01", nil)
	_, _, err = parseDashboardRange(req3)
	require.Error(t, err)

	// Formato inválido → error.
	req4 := httptest.NewRequest("GET", "/x?to=hola", nil)
	_, _, err = parseDashboardRange(req4)
	require.Error(t, err)
}

func TestDashboardSummary_Defaults30Days(t *testing.T) {
	mock := newDashboardMock()
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/v1/dashboard/summary", nil)
	req = withDevAuthRequest(req)
	rec := httptest.NewRecorder()
	GetDashboardSummary(mock).ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)
}
func TestDashboardLoad_401WithoutAuth(t *testing.T) {
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/v1/dashboard/load", nil)
	rec := httptest.NewRecorder()
	GetTrainingLoad(newDashboardMock()).ServeHTTP(rec, req)
	require.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestDashboardLoad_400BadRange(t *testing.T) {
	cases := []string{
		"/api/v1/dashboard/load?to=hola",
		"/api/v1/dashboard/load?from=bad",
		"/api/v1/dashboard/load?from=2024-02-01&to=2024-01-01",
	}
	for _, url := range cases {
		t.Run(url, func(t *testing.T) {
			req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, url, nil)
			req = withDevAuthRequest(req)
			rec := httptest.NewRecorder()
			GetTrainingLoad(newDashboardMock()).ServeHTTP(rec, req)
			require.Equal(t, http.StatusBadRequest, rec.Code)
		})
	}
}

func TestDashboardLoad_OKShapeWithData(t *testing.T) {
	mock := newDashboardMock()
	// Two days of training load in the window [2024-01-01, 2024-01-03].
	mock.ListTrainingLoadRangeFunc = func(_ context.Context, _ sqlc.ListTrainingLoadRangeParams) ([]sqlc.TrainingLoadDaily, error) {
		return []sqlc.TrainingLoadDaily{
			{Day: pgtype.Date{Time: mustDate(t, "2024-01-01"), Valid: true}, Tss: 100, ActivityCount: 1, DistanceM: 10000, ElevationGainM: 100},
			{Day: pgtype.Date{Time: mustDate(t, "2024-01-03"), Valid: true}, Tss: 80, ActivityCount: 0, DistanceM: 0, ElevationGainM: 0},
		}, nil
	}
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/v1/dashboard/load?from=2024-01-01&to=2024-01-03", nil)
	req = withDevAuthRequest(req)
	rec := httptest.NewRecorder()
	GetTrainingLoad(mock).ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)

	var body map[string]interface{}
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&body))
	require.Contains(t, body, "from")
	require.Contains(t, body, "to")
	require.Contains(t, body, "series")

	series, ok := body["series"].([]interface{})
	require.True(t, ok)
	// FillMissingDays debe llenar los 3 días (01, 02, 03).
	require.Equal(t, 3, len(series), "series debe tener un entry por día en [from,to]")
}

func TestDashboardLoad_OKEmptyData(t *testing.T) {
	mock := newDashboardMock()
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/v1/dashboard/load?from=2024-01-01&to=2024-01-07", nil)
	req = withDevAuthRequest(req)
	rec := httptest.NewRecorder()
	GetTrainingLoad(mock).ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)

	var body map[string]interface{}
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&body))
	series, _ := body["series"].([]interface{})
	require.Equal(t, 7, len(series), "sin datos, FillMissingDays aún debe poblar los 7 días con zeros")
}

// mustDate parses a YYYY-MM-DD string into a UTC time.Time; t.Fatal on failure.
func mustDate(t *testing.T, s string) time.Time {
	t.Helper()
	parsed, err := time.ParseInLocation("2006-01-02", s, time.UTC)
	require.NoError(t, err)
	return parsed
}

func TestDashboardHRZones_401WithoutAuth(t *testing.T) {
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/v1/dashboard/hr-zones", nil)
	rec := httptest.NewRecorder()
	GetHRZones(newDashboardMock()).ServeHTTP(rec, req)
	require.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestDashboardHRZones_OKExactWhenHrZonesPresent(t *testing.T) {
	mock := newDashboardMock()
	mock.SumHRZonesInRangeFunc = func(_ context.Context, _ sqlc.SumHRZonesInRangeParams) (sqlc.SumHRZonesInRangeRow, error) {
		return sqlc.SumHRZonesInRangeRow{
			Z1Seconds:           600,
			Z2Seconds:           1200,
			Z3Seconds:           1800,
			Z4Seconds:           300,
			Z5Seconds:           0,
			ActivitiesWithZones: 5,
		}, nil
	}
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/v1/dashboard/hr-zones?from=2024-01-01&to=2024-01-07", nil)
	req = withDevAuthRequest(req)
	rec := httptest.NewRecorder()
	GetHRZones(mock).ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)

	var body map[string]interface{}
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&body))
	require.Equal(t, false, body["degraded"])

	zones, ok := body["zones"].([]interface{})
	require.True(t, ok)
	require.Equal(t, 5, len(zones))
	// Z1=600/60=10, Z2=1200/60=20, Z3=1800/60=30, Z4=300/60=5, Z5=0
	expected := []float64{10, 20, 30, 5, 0}
	for i, z := range zones {
		m := z.(map[string]interface{})
		require.Equal(t, expected[i], m["minutes"])
	}
}

func TestDashboardHRZones_OKDegradedFallback(t *testing.T) {
	mock := newDashboardMock()
	// SumHRZonesInRange devuelve todo cero (sin streams HR ingestados).
	mock.SumHRZonesInRangeFunc = func(_ context.Context, _ sqlc.SumHRZonesInRangeParams) (sqlc.SumHRZonesInRangeRow, error) {
		return sqlc.SumHRZonesInRangeRow{ActivitiesWithZones: 0}, nil
	}
	// SumActivitiesDurationInRange devuelve 6000 segundos con avg_hr → 100 min totales → 20 min/zona.
	mock.SumActivitiesDurationFunc = func(_ context.Context, _ sqlc.SumActivitiesDurationInRangeParams) (sqlc.SumActivitiesDurationInRangeRow, error) {
		return sqlc.SumActivitiesDurationInRangeRow{
			ElapsedSecondsWithAvgHr: 6000,
		}, nil
	}
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/v1/dashboard/hr-zones?from=2024-01-01&to=2024-01-07", nil)
	req = withDevAuthRequest(req)
	rec := httptest.NewRecorder()
	GetHRZones(mock).ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)

	var body map[string]interface{}
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&body))
	require.Equal(t, true, body["degraded"])

	zones, ok := body["zones"].([]interface{})
	require.True(t, ok)
	require.Equal(t, 5, len(zones))
	for _, z := range zones {
		m := z.(map[string]interface{})
		require.Equal(t, 20.0, m["minutes"])
	}
}

func TestDashboardHRZones_400BadRange(t *testing.T) {
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/v1/dashboard/hr-zones?to=hola", nil)
	req = withDevAuthRequest(req)
	rec := httptest.NewRecorder()
	GetHRZones(newDashboardMock()).ServeHTTP(rec, req)
	require.Equal(t, http.StatusBadRequest, rec.Code)
}

// fakeRiverInserter implements jobs.RiverJobInserter for DA-004 tests.
type fakeRiverInserter struct {
	JobID     int64
	InsertErr error
	calls     int
}

func (f *fakeRiverInserter) Insert(_ context.Context, _ river.JobArgs, _ *river.InsertOpts) (*rivertype.JobInsertResult, error) {
	f.calls++
	if f.InsertErr != nil {
		return nil, f.InsertErr
	}
	return &rivertype.JobInsertResult{
		Job: &rivertype.JobRow{ID: f.JobID},
	}, nil
}

func TestDashboardRecalc_401WithoutAuth(t *testing.T) {
	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/v1/dashboard/recalc", nil)
	rec := httptest.NewRecorder()
	PostDashboardRecalc(newDashboardMock(), &fakeRiverInserter{JobID: 99}).ServeHTTP(rec, req)
	require.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestDashboardRecalc_503WithoutInserter(t *testing.T) {
	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/v1/dashboard/recalc", nil)
	req = withDevAuthRequest(req)
	rec := httptest.NewRecorder()
	PostDashboardRecalc(newDashboardMock(), nil).ServeHTTP(rec, req)
	require.Equal(t, http.StatusServiceUnavailable, rec.Code)
}

func TestDashboardRecalc_202Accepted(t *testing.T) {
	mock := newDashboardMock()
	mock.FirstActivityForUserFunc = func(_ context.Context, _ pgtype.UUID) (pgtype.Timestamptz, error) {
		return pgtype.Timestamptz{Time: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC), Valid: true}, nil
	}
	inserter := &fakeRiverInserter{JobID: 42}
	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/v1/dashboard/recalc", nil)
	req = withDevAuthRequest(req)
	rec := httptest.NewRecorder()
	PostDashboardRecalc(mock, inserter).ServeHTTP(rec, req)
	require.Equal(t, http.StatusAccepted, rec.Code)
	require.Equal(t, 1, inserter.calls)

	var body map[string]interface{}
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&body))
	require.Equal(t, "42", body["job_id"])
	require.Equal(t, "2024-01-01", body["will_recompute_from"])
	require.NotEmpty(t, body["queued_at"])
}

func TestDashboardRecalc_202WithoutFirstActivity(t *testing.T) {
	mock := newDashboardMock()
	// FirstActivityForUser returns zero/Valid=false → will_recompute_from=null.
	mock.FirstActivityForUserFunc = func(_ context.Context, _ pgtype.UUID) (pgtype.Timestamptz, error) {
		return pgtype.Timestamptz{}, nil
	}
	inserter := &fakeRiverInserter{JobID: 7}
	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/v1/dashboard/recalc", nil)
	req = withDevAuthRequest(req)
	rec := httptest.NewRecorder()
	PostDashboardRecalc(mock, inserter).ServeHTTP(rec, req)
	require.Equal(t, http.StatusAccepted, rec.Code)

	var body map[string]interface{}
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&body))
	require.Nil(t, body["will_recompute_from"])
}

func TestDashboardHRZones_401_Additional(t *testing.T) {
	t.Skip("covered by TestDashboardHRZones_401WithoutAuth")
}

// healthQuerier implements HealthQuerier for tests of /healthz extended.
type healthQuerier struct {
	*activitiesMockQuerier
	metadataFunc func(ctx context.Context, userID pgtype.UUID) (sqlc.DashboardMetadatum, error)
	syncFunc     func(ctx context.Context, userID pgtype.UUID) (sqlc.SyncSession, error)
}

func (h *healthQuerier) GetDashboardMetadata(ctx context.Context, userID pgtype.UUID) (sqlc.DashboardMetadatum, error) {
	if h.metadataFunc != nil {
		return h.metadataFunc(ctx, userID)
	}
	return sqlc.DashboardMetadatum{}, nil
}

func (h *healthQuerier) GetLatestSyncSession(ctx context.Context, userID pgtype.UUID) (sqlc.SyncSession, error) {
	if h.syncFunc != nil {
		return h.syncFunc(ctx, userID)
	}
	return sqlc.SyncSession{}, nil
}

func TestHealthExtended_InternalHeader_DBUp(t *testing.T) {
	hq := &healthQuerier{
		activitiesMockQuerier: newActivitiesMockQuerier(nil),
		metadataFunc: func(_ context.Context, _ pgtype.UUID) (sqlc.DashboardMetadatum, error) {
			return sqlc.DashboardMetadatum{
				LastRecalcAt:     pgtype.Timestamptz{Time: time.Date(2024, 1, 5, 10, 0, 0, 0, time.UTC), Valid: true},
				TrainingLoadRows: 30,
			}, nil
		},
		syncFunc: func(_ context.Context, _ pgtype.UUID) (sqlc.SyncSession, error) {
			return sqlc.SyncSession{
				FinishedAt: pgtype.Timestamptz{Time: time.Date(2024, 1, 5, 9, 30, 0, 0, time.UTC), Valid: true},
			}, nil
		},
	}
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/healthz", nil)
	req.Header.Set(InternalHealthHeader, "1")
	rec := httptest.NewRecorder()
	Health(okPinger{}, hq).ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)

	var body map[string]interface{}
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&body))
	require.Equal(t, "ok", body["status"])
	require.Equal(t, "2024-01-05T10:00:00Z", body["last_recalc_at"])
	require.Equal(t, float64(30), body["training_load_rows"])
	strava, ok := body["strava"].(map[string]interface{})
	require.True(t, ok)
	require.Equal(t, true, strava["ok"])
	require.Equal(t, "2024-01-05T09:30:00Z", strava["last_sync_at"])
	ai, ok := body["ai"].(map[string]interface{})
	require.True(t, ok)
	require.Equal(t, false, ai["enabled"])
}

func TestHealthExtended_PublicNoInternalHeader(t *testing.T) {
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()
	Health(okPinger{}, &healthQuerier{activitiesMockQuerier: newActivitiesMockQuerier(nil)}).ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)

	var body map[string]string
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&body))
	require.Equal(t, "ok", body["status"])
	require.Equal(t, 1, len(body), "public shape es {status:ok} exclusivamente")
}

func TestHealthExtended_DBDegradedReturns503(t *testing.T) {
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/healthz", nil)
	req.Header.Set(InternalHealthHeader, "1")
	rec := httptest.NewRecorder()
	Health(&fakePinger{err: errPingFailed}, nil).ServeHTTP(rec, req)
	require.Equal(t, http.StatusServiceUnavailable, rec.Code)

	var body map[string]interface{}
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&body))
	require.Equal(t, "degraded", body["status"])
	dbm, ok := body["db"].(map[string]interface{})
	require.True(t, ok)
	require.Equal(t, false, dbm["ok"])
}
