package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/rivertype"
	"github.com/stretchr/testify/require"

	"github.com/fgjcarlos/ghamusinos/internal/auth"
	"github.com/fgjcarlos/ghamusinos/internal/db/sqlc"
	"github.com/fgjcarlos/ghamusinos/internal/jobs"
)

type fakeActivityStreamsStore struct {
	activity    sqlc.Activity
	activityErr error
	streams     []sqlc.ActivityStream
	streamsErr  error
}

func (f *fakeActivityStreamsStore) GetActivityByExternalID(context.Context, sqlc.GetActivityByExternalIDParams) (sqlc.Activity, error) {
	if f.activityErr != nil {
		return sqlc.Activity{}, f.activityErr
	}
	return f.activity, nil
}

func (f *fakeActivityStreamsStore) GetActivityStreams(context.Context, pgtype.UUID) ([]sqlc.ActivityStream, error) {
	if f.streamsErr != nil {
		return nil, f.streamsErr
	}
	return f.streams, nil
}

type fakeStreamsInserter struct {
	calls  int
	args   river.JobArgs
	opts   *river.InsertOpts
	result *rivertype.JobInsertResult
	err    error
}

func (f *fakeStreamsInserter) Insert(_ context.Context, args river.JobArgs, opts *river.InsertOpts) (*rivertype.JobInsertResult, error) {
	f.calls++
	f.args = args
	f.opts = opts
	if f.err != nil {
		return nil, f.err
	}
	return f.result, nil
}

func streamsRequest(t *testing.T, method string, user *auth.User, idStr string) *http.Request {
	t.Helper()
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", idStr)
	ctx := context.WithValue(context.Background(), chi.RouteCtxKey, rctx)
	if user != nil {
		ctx = auth.WithAuthUser(ctx, user)
	}
	return httptest.NewRequestWithContext(ctx, method, "/api/v1/activities/"+idStr+"/streams", nil)
}

func streamsTestUser() *auth.User {
	return &auth.User{ID: "00000000-0000-0000-0000-000000000001"}
}

func streamsTestActivity() sqlc.Activity {
	return sqlc.Activity{
		ID:             pgtype.UUID{Bytes: [16]byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 2}, Valid: true},
		UserID:         pgtype.UUID{Bytes: [16]byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1}, Valid: true},
		ExternalSource: "strava",
		ExternalID:     123,
		StartedAt:      pgtype.Timestamptz{Time: time.Date(2026, 5, 1, 8, 0, 0, 0, time.UTC), Valid: true},
	}
}

func TestGetActivityStreams_ReturnsEveryStoredStream(t *testing.T) {
	activity := streamsTestActivity()
	store := &fakeActivityStreamsStore{
		activity: activity,
		streams: []sqlc.ActivityStream{
			{ActivityID: activity.ID, StreamType: "time", Data: []byte(`[0,1,2,3]`)},
			{ActivityID: activity.ID, StreamType: "latlng", Data: []byte(`[[42.1,-8.1],[42.2,-8.2]]`)},
			{ActivityID: activity.ID, StreamType: "altitude", Data: []byte(`[100,110]`)},
		},
	}

	rec := httptest.NewRecorder()
	GetActivityStreams(store).ServeHTTP(rec, streamsRequest(t, http.MethodGet, streamsTestUser(), "123"))

	require.Equal(t, http.StatusOK, rec.Code)
	var body activityStreamsResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	require.Equal(t, "00000000-0000-0000-0000-000000000002", body.ActivityID)
	require.Equal(t, activity.StartedAt.Time.UTC(), body.StartedAt)
	require.JSONEq(t, `[0,1,2,3]`, string(body.Streams["time"]))
	require.JSONEq(t, `[[42.1,-8.1],[42.2,-8.2]]`, string(body.Streams["latlng"]))
}

func TestGetActivityStreams_MissingTime_Returns409(t *testing.T) {
	activity := streamsTestActivity()
	store := &fakeActivityStreamsStore{
		activity: activity,
		streams: []sqlc.ActivityStream{
			{ActivityID: activity.ID, StreamType: "latlng", Data: []byte(`[[42.1,-8.1]]`)},
		},
	}

	rec := httptest.NewRecorder()
	GetActivityStreams(store).ServeHTTP(rec, streamsRequest(t, http.MethodGet, streamsTestUser(), "123"))

	require.Equal(t, http.StatusConflict, rec.Code)
	var problem ProblemDetail
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &problem))
	require.Equal(t, "Streams Unavailable", problem.Title)
	require.Contains(t, problem.Detail, "time")
}

func TestGetActivityStreams_EmptyStreamIsTreatedAsMissing(t *testing.T) {
	activity := streamsTestActivity()
	store := &fakeActivityStreamsStore{
		activity: activity,
		streams: []sqlc.ActivityStream{
			{ActivityID: activity.ID, StreamType: "latlng", Data: []byte(`[]`)},
			{ActivityID: activity.ID, StreamType: "time", Data: []byte(`[0,1]`)},
		},
	}

	rec := httptest.NewRecorder()
	GetActivityStreams(store).ServeHTTP(rec, streamsRequest(t, http.MethodGet, streamsTestUser(), "123"))

	require.Equal(t, http.StatusConflict, rec.Code)
}

func TestGetActivityStreams_UnknownActivity_Returns404(t *testing.T) {
	store := &fakeActivityStreamsStore{activityErr: pgx.ErrNoRows}

	rec := httptest.NewRecorder()
	GetActivityStreams(store).ServeHTTP(rec, streamsRequest(t, http.MethodGet, streamsTestUser(), "123"))

	require.Equal(t, http.StatusNotFound, rec.Code)
}

func TestGetActivityStreams_Unauthenticated_Returns401(t *testing.T) {
	rec := httptest.NewRecorder()
	GetActivityStreams(&fakeActivityStreamsStore{}).ServeHTTP(rec, streamsRequest(t, http.MethodGet, nil, "123"))

	require.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestGetActivityStreams_InvalidID_Returns400(t *testing.T) {
	rec := httptest.NewRecorder()
	GetActivityStreams(&fakeActivityStreamsStore{}).ServeHTTP(rec, streamsRequest(t, http.MethodGet, streamsTestUser(), "not-a-number"))

	require.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestPostActivityStreamsRefresh_EnqueuesImport(t *testing.T) {
	inserter := &fakeStreamsInserter{result: &rivertype.JobInsertResult{Job: &rivertype.JobRow{ID: 42}}}
	store := &fakeActivityStreamsStore{activity: streamsTestActivity()}

	rec := httptest.NewRecorder()
	PostActivityStreamsRefresh(store, inserter).ServeHTTP(rec, streamsRequest(t, http.MethodPost, streamsTestUser(), "123"))

	require.Equal(t, http.StatusAccepted, rec.Code)
	require.Equal(t, 1, inserter.calls)
	args, ok := inserter.args.(jobs.ImportStravaStreamsArgs)
	require.True(t, ok, "expected ImportStravaStreamsArgs, got %T", inserter.args)
	require.Equal(t, streamsTestUser().ID, args.UserID)
	require.EqualValues(t, 123, args.StravaActivityID)
	require.NotNil(t, inserter.opts)
	require.True(t, inserter.opts.UniqueOpts.ByArgs)

	var body map[string]interface{}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	require.Equal(t, "queued", body["status"])
	require.Equal(t, "42", body["job_id"])
}

func TestPostActivityStreamsRefresh_QueueUnavailable_Returns503(t *testing.T) {
	store := &fakeActivityStreamsStore{activity: streamsTestActivity()}

	rec := httptest.NewRecorder()
	PostActivityStreamsRefresh(store, nil).ServeHTTP(rec, streamsRequest(t, http.MethodPost, streamsTestUser(), "123"))

	require.Equal(t, http.StatusServiceUnavailable, rec.Code)
}

func TestPostActivityStreamsRefresh_EnqueueFailure_Returns503(t *testing.T) {
	store := &fakeActivityStreamsStore{activity: streamsTestActivity()}
	inserter := &fakeStreamsInserter{err: context.DeadlineExceeded}

	rec := httptest.NewRecorder()
	PostActivityStreamsRefresh(store, inserter).ServeHTTP(rec, streamsRequest(t, http.MethodPost, streamsTestUser(), "123"))

	require.Equal(t, http.StatusServiceUnavailable, rec.Code)
}

func TestPostActivityStreamsRefresh_UnknownActivity_Returns404(t *testing.T) {
	inserter := &fakeStreamsInserter{}
	store := &fakeActivityStreamsStore{activityErr: pgx.ErrNoRows}

	rec := httptest.NewRecorder()
	PostActivityStreamsRefresh(store, inserter).ServeHTTP(rec, streamsRequest(t, http.MethodPost, streamsTestUser(), "123"))

	require.Equal(t, http.StatusNotFound, rec.Code)
	require.Zero(t, inserter.calls)
}
