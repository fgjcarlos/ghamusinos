package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
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