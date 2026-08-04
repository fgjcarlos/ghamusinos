package handlers

import (
	"bytes"
	"context"
	"errors"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/fgjcarlos/ghamusinos/internal/auth"
	"github.com/fgjcarlos/ghamusinos/internal/gpx"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stretchr/testify/require"
)

type uploadGPXStore struct {
	existing      *gpx.StoredTrack
	detail        *gpx.StoredTrackDetail
	findErr       error
	createErr     error
	createdTrack  *gpx.Track
	createdClimbs []gpx.Climb
	createdRisks  []gpx.RiskZone
}

func (s *uploadGPXStore) FindByHash(_ context.Context, _ pgtype.UUID, _ string) (*gpx.StoredTrack, error) {
	return s.existing, s.findErr
}

func (s *uploadGPXStore) CreateDetail(_ context.Context, track *gpx.Track, _ *gpx.Analysis, climbs []gpx.Climb, risks []gpx.RiskZone, _ *gpx.Climb) (*gpx.StoredTrackDetail, error) {
	s.createdTrack, s.createdClimbs, s.createdRisks = track, climbs, risks
	return s.detail, s.createErr
}

func uploadHandler(store UploadGPXStore) http.Handler {
	return UploadGPX(store, gpx.Parser{}, gpx.Validator{}, gpx.Analyzer{}, gpx.ClimbService{}, gpx.RiskService{}, gpx.TrackTypeService{}, gpx.Hasher{})
}

func multipartGPXRequest(t *testing.T, content []byte, authenticated bool) *http.Request {
	t.Helper()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("file", "route.gpx")
	require.NoError(t, err)
	_, err = io.Copy(part, bytes.NewReader(content))
	require.NoError(t, err)
	require.NoError(t, writer.Close())
	req := httptest.NewRequest(http.MethodPost, "/api/v1/gpx/upload", &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	if authenticated {
		req = req.WithContext(auth.WithAuthUser(req.Context(), &auth.User{ID: "00000000-0000-0000-0000-000000000001"}))
	}
	return req
}

func validUploadGPX(withElevation bool) []byte {
	ele := "<ele>100</ele>"
	if !withElevation {
		ele = ""
	}
	doc := `<?xml version="1.0"?><gpx version="1.1" creator="test" xmlns="http://www.topografix.com/GPX/1/1"><trk><name>Upload trail</name><trkseg>` +
		`<trkpt lat="40.0000" lon="-3.0000">` + ele + `</trkpt>` +
		`<trkpt lat="40.0010" lon="-3.0000">` + ele + `</trkpt>` +
		`<trkpt lat="40.0020" lon="-3.0000">` + ele + `</trkpt>` +
		`</trkseg></trk></gpx>` + strings.Repeat("<!-- padding -->", 80)
	return []byte(doc)
}

func TestUploadGPXRejectsUnauthenticatedRequest(t *testing.T) {
	recorder := httptest.NewRecorder()
	uploadHandler(&uploadGPXStore{}).ServeHTTP(recorder, multipartGPXRequest(t, validUploadGPX(true), false))

	require.Equal(t, http.StatusUnauthorized, recorder.Code)
	require.Equal(t, "application/problem+json", recorder.Header().Get("Content-Type"))
	require.Contains(t, recorder.Body.String(), "not authenticated")
}

func TestUploadGPXRejectsFileOverTenMegabytes(t *testing.T) {
	recorder := httptest.NewRecorder()
	content := bytes.Repeat([]byte("x"), 10*1024*1024+1)
	uploadHandler(&uploadGPXStore{}).ServeHTTP(recorder, multipartGPXRequest(t, content, true))

	require.Equal(t, http.StatusRequestEntityTooLarge, recorder.Code)
	require.Contains(t, recorder.Body.String(), "file exceeds 10 MB limit")
}

func TestUploadGPXRejectsFileBelowOneKilobyte(t *testing.T) {
	recorder := httptest.NewRecorder()
	uploadHandler(&uploadGPXStore{}).ServeHTTP(recorder, multipartGPXRequest(t, []byte(`<gpx version="1.1"></gpx>`), true))

	require.Equal(t, http.StatusUnprocessableEntity, recorder.Code)
	require.Contains(t, recorder.Body.String(), "file must be at least 1 KB")
}

func TestUploadGPXRejectsInvalidGPX(t *testing.T) {
	recorder := httptest.NewRecorder()
	invalid := bytes.Repeat([]byte("not-gpx"), 200)
	uploadHandler(&uploadGPXStore{}).ServeHTTP(recorder, multipartGPXRequest(t, invalid, true))

	require.Equal(t, http.StatusUnprocessableEntity, recorder.Code)
	require.Contains(t, recorder.Body.String(), "invalid GPX")
}

func TestUploadGPXReturnsConflictForDuplicate(t *testing.T) {
	id := pgtype.UUID{Bytes: [16]byte{9}, Valid: true}
	store := &uploadGPXStore{existing: &gpx.StoredTrack{Track: gpx.Track{ID: id}}}
	recorder := httptest.NewRecorder()
	uploadHandler(store).ServeHTTP(recorder, multipartGPXRequest(t, validUploadGPX(true), true))

	require.Equal(t, http.StatusConflict, recorder.Code)
	require.Contains(t, recorder.Body.String(), "duplicate track")
	require.Contains(t, recorder.Body.String(), id.String())
	require.Nil(t, store.createdTrack)
}

func TestUploadGPXReturnsCreatedFullAnalysis(t *testing.T) {
	id := pgtype.UUID{Bytes: [16]byte{7}, Valid: true}
	detail := &gpx.StoredTrackDetail{Track: gpx.StoredTrack{Track: gpx.Track{ID: id, Name: "Upload trail"}}, Climbs: []gpx.Climb{}, RiskZones: []gpx.RiskZone{}}
	store := &uploadGPXStore{detail: detail, findErr: pgx.ErrNoRows}
	recorder := httptest.NewRecorder()
	uploadHandler(store).ServeHTTP(recorder, multipartGPXRequest(t, validUploadGPX(true), true))

	require.Equal(t, http.StatusCreated, recorder.Code)
	require.Equal(t, "application/json", recorder.Header().Get("Content-Type"))
	require.Contains(t, recorder.Body.String(), id.String())
	require.Contains(t, recorder.Body.String(), `"risk_zones":[]`)
	require.Contains(t, recorder.Body.String(), `"muros":[]`)
	require.Contains(t, recorder.Body.String(), `"recovery_zones":[]`)
	require.Contains(t, recorder.Body.String(), `"km_vertical":null`)
	require.NotNil(t, store.createdTrack)
	require.Equal(t, "Upload trail", store.createdTrack.Name)
	require.Equal(t, int64(len(validUploadGPX(true))), store.createdTrack.FileSizeBytes)
}

func TestUploadGPXAcceptsMissingElevation(t *testing.T) {
	store := &uploadGPXStore{detail: &gpx.StoredTrackDetail{}, findErr: pgx.ErrNoRows}
	recorder := httptest.NewRecorder()
	uploadHandler(store).ServeHTTP(recorder, multipartGPXRequest(t, validUploadGPX(false), true))

	require.Equal(t, http.StatusCreated, recorder.Code)
	require.NotNil(t, store.createdTrack)
	require.Nil(t, store.createdTrack.Points[0].Ele)
	require.Empty(t, store.createdClimbs)
	require.Empty(t, store.createdRisks)
}

func TestUploadGPXReturnsInternalErrorWhenStoreFails(t *testing.T) {
	store := &uploadGPXStore{findErr: pgx.ErrNoRows, createErr: errors.New("db unavailable")}
	recorder := httptest.NewRecorder()
	uploadHandler(store).ServeHTTP(recorder, multipartGPXRequest(t, validUploadGPX(true), true))

	require.Equal(t, http.StatusInternalServerError, recorder.Code)
	require.Contains(t, recorder.Body.String(), "failed to persist GPX")
}
