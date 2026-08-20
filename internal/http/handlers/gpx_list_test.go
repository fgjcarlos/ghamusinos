package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/fgjcarlos/ghamusinos/internal/auth"
	"github.com/fgjcarlos/ghamusinos/internal/gpx"
	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stretchr/testify/require"
)

// listGPXStore es un stub que implementa List + Delete (necesario
// para DeleteGPX que reusa la misma interfaz).
type listGPXStore struct {
	list           *gpx.PaginatedTracks
	listErr        error
	deleteErr      error
	deletedUserID  pgtype.UUID
	deletedTrackID pgtype.UUID
}

func (s *listGPXStore) List(_ context.Context, _ pgtype.UUID, _ gpx.ListParams) (*gpx.PaginatedTracks, error) {
	return s.list, s.listErr
}

func (s *listGPXStore) Delete(_ context.Context, userID, trackID pgtype.UUID) error {
	s.deletedUserID = userID
	s.deletedTrackID = trackID
	return s.deleteErr
}

func listGPXRequest(authenticated bool, query string) *http.Request {
	url := "/api/v1/gpx/"
	if query != "" {
		url += "?" + query
	}
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, url, http.NoBody)
	routeCtx := chi.NewRouteContext()
	routeCtx.URLParams.Add("id", "")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, routeCtx))
	if authenticated {
		req = req.WithContext(auth.WithAuthUser(req.Context(), &auth.User{ID: "00000000-0000-0000-0000-000000000001"}))
	}
	return req
}

func TestListGPXRejectsUnauthenticatedRequest(t *testing.T) {
	rec := httptest.NewRecorder()
	ListGPX(&listGPXStore{}).ServeHTTP(rec, listGPXRequest(false, ""))
	require.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestListGPXValidatesLimit(t *testing.T) {
	cases := []struct {
		name       string
		query      string
		wantStatus int
		wantSub    string
	}{
		{"limit non-numeric", "limit=abc", http.StatusBadRequest, "limit"},
		{"limit zero", "limit=0", http.StatusBadRequest, "limit"},
		{"limit too big", "limit=200", http.StatusBadRequest, "limit"},
		{"limit negative", "limit=-5", http.StatusBadRequest, "limit"},
		{"offset non-numeric", "offset=xyz", http.StatusBadRequest, "offset"},
		{"offset negative", "offset=-1", http.StatusBadRequest, "offset"},
		{"sort invalid", "sort=difficulty_ASC", http.StatusBadRequest, "sort"},
		{"defaults ok", "", http.StatusOK, ""},
		{"explicit valid", "limit=5&offset=10", http.StatusOK, ""},
		{"sort default ok", "sort=created_at%20DESC", http.StatusOK, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			store := &listGPXStore{list: &gpx.PaginatedTracks{Data: []gpx.StoredTrack{}}}
			rec := httptest.NewRecorder()
			ListGPX(store).ServeHTTP(rec, listGPXRequest(true, tc.query))
			require.Equal(t, tc.wantStatus, rec.Code)
			if tc.wantSub != "" {
				require.Contains(t, strings.ToLower(rec.Body.String()), tc.wantSub)
			}
		})
	}
}

func TestListGPXReturnsStoreResults(t *testing.T) {
	store := &listGPXStore{list: &gpx.PaginatedTracks{
		Data:    []gpx.StoredTrack{},
		Total:   0,
		Limit:   20,
		Offset:  0,
		HasNext: false,
	}}
	rec := httptest.NewRecorder()
	ListGPX(store).ServeHTTP(rec, listGPXRequest(true, "limit=10"))
	require.Equal(t, http.StatusOK, rec.Code)
	require.Contains(t, rec.Header().Get("Content-Type"), "application/json")

	var body gpx.PaginatedTracks
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&body))
	require.Equal(t, int32(10), body.Limit)
}

func TestListGPXReturnsInternalErrorOnStoreFailure(t *testing.T) {
	store := &listGPXStore{listErr: errors.New("db down")}
	rec := httptest.NewRecorder()
	ListGPX(store).ServeHTTP(rec, listGPXRequest(true, ""))
	require.Equal(t, http.StatusInternalServerError, rec.Code)
}

func TestDeleteGPXRejectsUnauthenticatedRequest(t *testing.T) {
	rec := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(context.Background(), http.MethodDelete, "/api/v1/gpx/00000000-0000-0000-0000-000000000002", nil)
	routeCtx := chi.NewRouteContext()
	routeCtx.URLParams.Add("id", "00000000-0000-0000-0000-000000000002")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, routeCtx))
	DeleteGPX(&listGPXStore{}).ServeHTTP(rec, req)
	require.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestDeleteGPXInvalidUUID(t *testing.T) {
	rec := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(context.Background(), http.MethodDelete, "/api/v1/gpx/not-a-uuid", nil)
	routeCtx := chi.NewRouteContext()
	routeCtx.URLParams.Add("id", "not-a-uuid")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, routeCtx))
	req = req.WithContext(auth.WithAuthUser(req.Context(), &auth.User{ID: "00000000-0000-0000-0000-000000000001"}))
	DeleteGPX(&listGPXStore{}).ServeHTTP(rec, req)
	require.Equal(t, http.StatusNotFound, rec.Code)
}

func TestDeleteGPXNotFound(t *testing.T) {
	store := &listGPXStore{deleteErr: pgx.ErrNoRows}
	rec := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(context.Background(), http.MethodDelete, "/api/v1/gpx/00000000-0000-0000-0000-000000000002", nil)
	routeCtx := chi.NewRouteContext()
	routeCtx.URLParams.Add("id", "00000000-0000-0000-0000-000000000002")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, routeCtx))
	req = req.WithContext(auth.WithAuthUser(req.Context(), &auth.User{ID: "00000000-0000-0000-0000-000000000001"}))
	DeleteGPX(store).ServeHTTP(rec, req)
	require.Equal(t, http.StatusNotFound, rec.Code)
}

func TestDeleteGPXNoContent(t *testing.T) {
	store := &listGPXStore{}
	rec := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(context.Background(), http.MethodDelete, "/api/v1/gpx/00000000-0000-0000-0000-000000000002", nil)
	routeCtx := chi.NewRouteContext()
	routeCtx.URLParams.Add("id", "00000000-0000-0000-0000-000000000002")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, routeCtx))
	req = req.WithContext(auth.WithAuthUser(req.Context(), &auth.User{ID: "00000000-0000-0000-0000-000000000001"}))
	DeleteGPX(store).ServeHTTP(rec, req)
	require.Equal(t, http.StatusNoContent, rec.Code)
	require.True(t, store.deletedUserID.Valid)
	require.True(t, store.deletedTrackID.Valid)
}

// listOnlyStore implementa GPXListStore (sólo List, no Delete).
// Sirve para verificar que DeleteGPX hace panic cuando el store
// no implementa el método de delete.
type listOnlyStore struct{}

func (listOnlyStore) List(_ context.Context, _ pgtype.UUID, _ gpx.ListParams) (*gpx.PaginatedTracks, error) {
	return nil, nil
}

func TestDeleteGPXPanicsIfStoreLacksDelete(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic when store doesn't implement Delete")
		}
	}()
	DeleteGPX(listOnlyStore{})
}
