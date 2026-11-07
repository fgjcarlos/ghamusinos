package handlers

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/fgjcarlos/ghamusinos/internal/auth"
	"github.com/fgjcarlos/ghamusinos/internal/gpx"
	"github.com/fgjcarlos/ghamusinos/internal/terrain"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stretchr/testify/require"
)

const terrainTrackID = "00000000-0000-0000-0000-000000000002"
const terrainUserID = "00000000-0000-0000-0000-000000000001"

type terrainFakeGPXStore struct {
	mu     sync.Mutex
	detail *gpx.StoredTrackDetail
	err    error
	calls  int
	userID pgtype.UUID
	track  pgtype.UUID
}

func (s *terrainFakeGPXStore) GetDetail(_ context.Context, userID, trackID pgtype.UUID, _ int) (*gpx.StoredTrackDetail, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.calls++
	s.userID, s.track = userID, trackID
	return s.detail, s.err
}

func (s *terrainFakeGPXStore) callCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.calls
}

type terrainFakeFetcher struct {
	mu         sync.Mutex
	ways       []terrain.Way
	err        error
	calls      int
	lastPoints []terrain.LatLon
}

func (f *terrainFakeFetcher) WaysNear(_ context.Context, points []terrain.LatLon) ([]terrain.Way, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls++
	f.lastPoints = points
	return f.ways, f.err
}

func (f *terrainFakeFetcher) callCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.calls
}

func terrainRequest(authenticated bool, id string) *http.Request {
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/v1/gpx/"+id+"/terrain", nil)
	routeCtx := chi.NewRouteContext()
	routeCtx.URLParams.Add("id", id)
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, routeCtx))
	if authenticated {
		req = req.WithContext(auth.WithAuthUser(req.Context(), &auth.User{ID: terrainUserID}))
	}
	return req
}

// terrainStoredDetail builds a ~450 m eastward track owned by the test user.
func terrainStoredDetail(t *testing.T) (*gpx.StoredTrackDetail, []gpx.Point) {
	t.Helper()
	id, err := uuid.Parse(terrainTrackID)
	require.NoError(t, err)
	const metersPerDegree = 6371000 * 3.141592653589793 / 180
	const trackLenM = 450
	const stepM = 5
	points := make([]gpx.Point, 0, trackLenM/stepM+1)
	for d := 0.0; d <= trackLenM; d += stepM {
		points = append(points, gpx.Point{Lat: 0, Lon: d / metersPerDegree})
	}
	return &gpx.StoredTrackDetail{
		Track: gpx.StoredTrack{Track: gpx.Track{ID: pgtype.UUID{Bytes: id, Valid: true}, Points: points}},
	}, points
}

func terrainCaminoWay() terrain.Way {
	return terrain.Way{
		ID:       42,
		Name:     "Camino",
		Highway:  "secondary",
		Surface:  "asphalt",
		Geometry: []terrain.LatLon{{Lat: 0, Lon: 0}, {Lat: 0, Lon: 0.0041}},
	}
}

func TestGetGPXTerrainRejectsUnauthenticatedRequest(t *testing.T) {
	store := &terrainFakeGPXStore{}
	fetcher := &terrainFakeFetcher{}
	recorder := httptest.NewRecorder()

	GetGPXTerrain(store, fetcher).ServeHTTP(recorder, terrainRequest(false, terrainTrackID))

	require.Equal(t, http.StatusUnauthorized, recorder.Code)
	require.Contains(t, recorder.Body.String(), "not authenticated")
	require.Zero(t, fetcher.callCount())
}

func TestGetGPXTerrainRejectsInvalidTrackID(t *testing.T) {
	store := &terrainFakeGPXStore{}
	fetcher := &terrainFakeFetcher{}
	recorder := httptest.NewRecorder()

	GetGPXTerrain(store, fetcher).ServeHTTP(recorder, terrainRequest(true, "not-a-uuid"))

	require.Equal(t, http.StatusNotFound, recorder.Code)
	require.Contains(t, recorder.Body.String(), "GPX track not found")
	require.Zero(t, fetcher.callCount())
}

func TestGetGPXTerrainReturnsNotFoundForUnknownTrack(t *testing.T) {
	store := &terrainFakeGPXStore{err: pgx.ErrNoRows}
	fetcher := &terrainFakeFetcher{}
	recorder := httptest.NewRecorder()

	GetGPXTerrain(store, fetcher).ServeHTTP(recorder, terrainRequest(true, terrainTrackID))

	require.Equal(t, http.StatusNotFound, recorder.Code)
	require.Contains(t, recorder.Body.String(), "GPX track not found")
	require.Zero(t, fetcher.callCount())
}

func TestGetGPXTerrainReturnsMappedSegments(t *testing.T) {
	detail, points := terrainStoredDetail(t)
	store := &terrainFakeGPXStore{detail: detail}
	fetcher := &terrainFakeFetcher{ways: []terrain.Way{terrainCaminoWay()}}
	recorder := httptest.NewRecorder()

	GetGPXTerrain(store, fetcher).ServeHTTP(recorder, terrainRequest(true, terrainTrackID))

	require.Equal(t, http.StatusOK, recorder.Code)
	require.Equal(t, "application/json", recorder.Header().Get("Content-Type"))
	require.Contains(t, recorder.Body.String(),
		`{"track_id":"`+terrainTrackID+`","available":true,"segments":[{"start_km":0,"end_km":0.45,"surface":"paved","technicality":1,"name":"Camino"}]}`)
	require.Len(t, fetcher.lastPoints, len(points))
	require.InDelta(t, points[1].Lon, fetcher.lastPoints[1].Lon, 1e-12)
	require.Equal(t, 1, store.callCount())
	require.True(t, store.userID.Valid)
	require.True(t, store.track.Valid)
}

func TestGetGPXTerrainReturnsAvailableFalseWithoutWays(t *testing.T) {
	detail, _ := terrainStoredDetail(t)
	store := &terrainFakeGPXStore{detail: detail}
	fetcher := &terrainFakeFetcher{}
	recorder := httptest.NewRecorder()

	GetGPXTerrain(store, fetcher).ServeHTTP(recorder, terrainRequest(true, terrainTrackID))

	require.Equal(t, http.StatusOK, recorder.Code)
	require.Contains(t, recorder.Body.String(),
		`{"track_id":"`+terrainTrackID+`","available":false,"segments":[]}`)
}

func TestGetGPXTerrainReturnsServiceUnavailableWhenFetcherFails(t *testing.T) {
	detail, _ := terrainStoredDetail(t)
	store := &terrainFakeGPXStore{detail: detail}
	fetcher := &terrainFakeFetcher{err: errors.New("overpass timeout")}
	recorder := httptest.NewRecorder()

	GetGPXTerrain(store, fetcher).ServeHTTP(recorder, terrainRequest(true, terrainTrackID))

	require.Equal(t, http.StatusServiceUnavailable, recorder.Code)
	require.Equal(t, "application/problem+json", recorder.Header().Get("Content-Type"))
	require.Contains(t, recorder.Body.String(), "Service Unavailable")
}

func TestGetGPXTerrainCachesRepeatedRequests(t *testing.T) {
	detail, _ := terrainStoredDetail(t)
	store := &terrainFakeGPXStore{detail: detail}
	fetcher := &terrainFakeFetcher{ways: []terrain.Way{terrainCaminoWay()}}
	handler := GetGPXTerrain(store, fetcher)

	first := httptest.NewRecorder()
	handler.ServeHTTP(first, terrainRequest(true, terrainTrackID))
	second := httptest.NewRecorder()
	handler.ServeHTTP(second, terrainRequest(true, terrainTrackID))

	require.Equal(t, http.StatusOK, first.Code)
	require.Equal(t, http.StatusOK, second.Code)
	require.Equal(t, first.Body.String(), second.Body.String())
	require.Equal(t, 1, fetcher.callCount())
}

func TestGetGPXTerrainCachesEmptyResult(t *testing.T) {
	detail, _ := terrainStoredDetail(t)
	store := &terrainFakeGPXStore{detail: detail}
	fetcher := &terrainFakeFetcher{}
	handler := GetGPXTerrain(store, fetcher)

	for i := 0; i < 2; i++ {
		recorder := httptest.NewRecorder()
		handler.ServeHTTP(recorder, terrainRequest(true, terrainTrackID))
		require.Equal(t, http.StatusOK, recorder.Code)
	}

	require.Equal(t, 1, fetcher.callCount())
}
