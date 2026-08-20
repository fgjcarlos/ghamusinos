package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/fgjcarlos/ghamusinos/internal/auth"
	"github.com/fgjcarlos/ghamusinos/internal/gpx"
	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stretchr/testify/require"
)

// compareGPXStore es un stub para CompareGPX. Permite programar el
// resultado por UUID de track para cubrir el caso de "uno de los IDs
// no existe".
type compareGPXStore struct {
	details map[string]*gpx.StoredTrackDetail
	err     error
}

func (s *compareGPXStore) GetDetail(_ context.Context, _ pgtype.UUID, trackID pgtype.UUID) (*gpx.StoredTrackDetail, error) {
	if s.err != nil {
		return nil, s.err
	}
	if d, ok := s.details[trackID.String()]; ok {
		return d, nil
	}
	return nil, errors.New("not found")
}

func compareGPXRequest(authenticated bool, body string) *http.Request {
	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/v1/gpx/compare", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	routeCtx := chi.NewRouteContext()
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, routeCtx))
	if authenticated {
		req = req.WithContext(auth.WithAuthUser(req.Context(), &auth.User{ID: "00000000-0000-0000-0000-000000000001"}))
	}
	return req
}

func TestCompareGPXRejectsUnauthenticatedRequest(t *testing.T) {
	rec := httptest.NewRecorder()
	CompareGPX(&compareGPXStore{}).ServeHTTP(rec, compareGPXRequest(false, `{"ids":["x"]}`))
	require.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestCompareGPXValidatesBody(t *testing.T) {
	cases := []struct {
		name string
		body string
	}{
		{"empty ids", `{"ids":[]}`},
		{"too many ids", `{"ids":["a","b","c","d"]}`},
		{"invalid uuid", `{"ids":["not-a-uuid"]}`},
		{"malformed json", `{not json`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			CompareGPX(&compareGPXStore{}).ServeHTTP(rec, compareGPXRequest(true, tc.body))
			require.Equal(t, http.StatusBadRequest, rec.Code)
		})
	}
}

func TestCompareGPXReturnsNotFoundForUnknownTrack(t *testing.T) {
	rec := httptest.NewRecorder()
	CompareGPX(&compareGPXStore{}).ServeHTTP(rec, compareGPXRequest(true, `{"ids":["00000000-0000-0000-0000-000000000001"]}`))
	require.Equal(t, http.StatusNotFound, rec.Code)
}

func TestCompareGPXWithTwoTracksComputesDiff(t *testing.T) {
	uuid1 := "00000000-0000-0000-0000-000000000001"
	uuid2 := "00000000-0000-0000-0000-000000000002"
	pguuid1 := pgUUIDFromString(t, uuid1)
	pguuid2 := pgUUIDFromString(t, uuid2)
	store := &compareGPXStore{
		details: map[string]*gpx.StoredTrackDetail{
			uuid1: {
				Track: gpx.StoredTrack{
					Track:    gpx.Track{ID: pguuid1},
					Analysis: gpx.Analysis{DistanceM: 10000, DPlusM: 500, DifficultyScore: 60, ITRAPoints: 8},
				},
			},
			uuid2: {
				Track: gpx.StoredTrack{
					Track:    gpx.Track{ID: pguuid2},
					Analysis: gpx.Analysis{DistanceM: 20000, DPlusM: 1200, DifficultyScore: 85, ITRAPoints: 14},
				},
			},
		},
	}
	rec := httptest.NewRecorder()
	body := `{"ids":["00000000-0000-0000-0000-000000000001","00000000-0000-0000-0000-000000000002"]}`
	CompareGPX(store).ServeHTTP(rec, compareGPXRequest(true, body))
	require.Equal(t, http.StatusOK, rec.Code)
	require.Contains(t, rec.Header().Get("Content-Type"), "application/json")

	var resp struct {
		Tracks []*gpx.StoredTrackDetail `json:"tracks"`
		Diff   map[string]compareMetric `json:"diff"`
	}
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&resp))
	require.Len(t, resp.Tracks, 2)

	// DistanceM: track 2 (20000) > track 1 (10000), best=1
	require.Equal(t, []float64{10000, 20000}, resp.Diff["distance_m"].Values)
	require.Equal(t, 1, resp.Diff["distance_m"].BestTrack)
	// DifficultyScore: track 2 (85) > track 1 (60), best=1
	require.Equal(t, 1, resp.Diff["difficulty_score"].BestTrack)
}

// pgUUIDFromString helper para construir pgtype.UUID desde un string
// en tests. Reutilizable en este paquete.
func pgUUIDFromString(t *testing.T, s string) pgtype.UUID {
	t.Helper()
	var u pgtype.UUID
	if err := u.Scan(s); err != nil {
		t.Fatalf("pgtype.UUID.Scan(%q): %v", s, err)
	}
	return u
}

func TestComputeDiffEmptyTracks(t *testing.T) {
	diff := computeDiff(nil)
	require.Empty(t, diff)
}
