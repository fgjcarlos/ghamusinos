package handlers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/fgjcarlos/ghamusinos/internal/auth"
	"github.com/fgjcarlos/ghamusinos/internal/gpx"
	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stretchr/testify/require"
)

type detailGPXStore struct {
	detail  *gpx.StoredTrackDetail
	err     error
	userID  pgtype.UUID
	trackID pgtype.UUID
}

func (s *detailGPXStore) GetDetail(_ context.Context, userID, trackID pgtype.UUID) (*gpx.StoredTrackDetail, error) {
	s.userID, s.trackID = userID, trackID
	return s.detail, s.err
}

func getGPXRequest(authenticated bool) *http.Request {
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/v1/gpx/00000000-0000-0000-0000-000000000002", nil)
	routeCtx := chi.NewRouteContext()
	routeCtx.URLParams.Add("id", "00000000-0000-0000-0000-000000000002")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, routeCtx))
	if authenticated {
		req = req.WithContext(auth.WithAuthUser(req.Context(), &auth.User{ID: "00000000-0000-0000-0000-000000000001"}))
	}
	return req
}

func TestGetGPXRejectsUnauthenticatedRequest(t *testing.T) {
	recorder := httptest.NewRecorder()
	GetGPX(&detailGPXStore{}).ServeHTTP(recorder, getGPXRequest(false))

	require.Equal(t, http.StatusUnauthorized, recorder.Code)
	require.Contains(t, recorder.Body.String(), "not authenticated")
}

func TestGetGPXReturnsNotFoundForMissingOrUnownedTrack(t *testing.T) {
	recorder := httptest.NewRecorder()
	GetGPX(&detailGPXStore{err: pgx.ErrNoRows}).ServeHTTP(recorder, getGPXRequest(true))

	require.Equal(t, http.StatusNotFound, recorder.Code)
	require.Contains(t, recorder.Body.String(), "GPX track not found")
}

func TestGetGPXReturnsOwnedTrackDetail(t *testing.T) {
	id := pgtype.UUID{Bytes: [16]byte{15: 2}, Valid: true}
	detail := &gpx.StoredTrackDetail{
		Track:     gpx.StoredTrack{Track: gpx.Track{ID: id, Name: "Owned"}},
		Climbs:    []gpx.Climb{{StartIdx: 1, EndIdx: 2, IsKingClimb: true}},
		RiskZones: []gpx.RiskZone{{RiskType: "steep", Severity: "high"}},
	}
	store := &detailGPXStore{detail: detail}
	recorder := httptest.NewRecorder()
	GetGPX(store).ServeHTTP(recorder, getGPXRequest(true))

	require.Equal(t, http.StatusOK, recorder.Code)
	require.Equal(t, "application/json", recorder.Header().Get("Content-Type"))
	require.Contains(t, recorder.Body.String(), "Owned")
	require.Contains(t, recorder.Body.String(), `"is_king_climb":true`)
	require.Contains(t, recorder.Body.String(), `"risk_type":"steep"`)
	require.True(t, store.userID.Valid)
	require.Equal(t, id, store.trackID)
}
