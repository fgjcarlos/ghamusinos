package strava

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stretchr/testify/require"

	"github.com/fgjcarlos/ghamusinos/internal/db/sqlc"
)

// This is Strava's documented Webhook Events API sample payload. Issue #165
// still requires a captured production event before it can be closed.
const documentedWebhookPayload = `{
    "aspect_type": "update",
    "event_time": 1516126040,
    "object_id": 1360128428,
    "object_type": "activity",
    "owner_id": 134815,
    "subscription_id": 120475,
    "updates": {"title": "Messy"}
}`

func TestWebhookHandler_PersistsDocumentedEventWithoutSignature(t *testing.T) {
	store := newKnownAthleteStore()
	handler := WebhookHandler(store)
	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/strava/webhook", bytes.NewBufferString(documentedWebhookPayload))
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, req)

	require.Equal(t, http.StatusOK, recorder.Code)
	require.True(t, store.enqueued)
	require.True(t, store.jobEnqueued)
	require.Equal(t, int64(1360128428), store.params.ObjectID)
	require.Equal(t, "update", store.params.AspectType)
	require.Equal(t, int64(134815), store.params.OwnerID.Int64)
	require.Equal(t, int64(120475), store.params.SubscriptionID.Int64)
	require.Equal(t, time.Unix(1516126040, 0).UTC(), store.params.EventTime.Time)
	require.JSONEq(t, documentedWebhookPayload, string(store.params.RawPayload))
}

func TestWebhookHandler_UnknownAthleteAcknowledgesWithoutPersisting(t *testing.T) {
	store := &mockActivityEventStore{}
	handler := WebhookHandler(store)
	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/strava/webhook", bytes.NewBufferString(documentedWebhookPayload))
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, req)

	require.Equal(t, http.StatusOK, recorder.Code)
	require.False(t, store.enqueued)
	require.False(t, store.jobEnqueued)
}

func TestWebhookHandler_RejectsMalformedPayloads(t *testing.T) {
	tests := []struct {
		name string
		body string
	}{
		{name: "invalid JSON", body: `{`},
		{name: "missing event time", body: `{"object_type":"activity","object_id":1,"aspect_type":"create","owner_id":2,"subscription_id":3}`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := WebhookHandler(newKnownAthleteStore())
			req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/strava/webhook", bytes.NewBufferString(tt.body))
			recorder := httptest.NewRecorder()

			handler.ServeHTTP(recorder, req)

			require.Equal(t, http.StatusBadRequest, recorder.Code)
		})
	}
}

func TestWebhookHandler_StoreFailuresReturnServerError(t *testing.T) {
	tests := []struct {
		name  string
		store *mockActivityEventStore
	}{
		{name: "athlete lookup", store: &mockActivityEventStore{resolveErr: errors.New("lookup failed")}},
		{name: "event persistence", store: &mockActivityEventStore{userID: testUserID(), enqueueErr: errors.New("insert failed")}},
		{name: "job enqueue", store: &mockActivityEventStore{userID: testUserID(), jobErr: errors.New("queue failed")}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := WebhookHandler(tt.store)
			req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/strava/webhook", bytes.NewBufferString(documentedWebhookPayload))
			recorder := httptest.NewRecorder()

			handler.ServeHTTP(recorder, req)

			require.Equal(t, http.StatusInternalServerError, recorder.Code)
		})
	}
}

func TestWebhookChallengeHandler(t *testing.T) {
	tests := []struct {
		name       string
		query      string
		wantStatus int
		wantBody   string
	}{
		{
			name:       "valid token returns JSON challenge",
			query:      "?hub.mode=subscribe&hub.challenge=test-challenge&hub.verify_token=test-token",
			wantStatus: http.StatusOK,
			wantBody:   `{"hub.challenge":"test-challenge"}`,
		},
		{
			name:       "invalid token is forbidden",
			query:      "?hub.mode=subscribe&hub.challenge=test-challenge&hub.verify_token=wrong",
			wantStatus: http.StatusForbidden,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := WebhookChallengeHandler("test-token")
			req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/strava/webhook"+tt.query, nil)
			recorder := httptest.NewRecorder()

			handler.ServeHTTP(recorder, req)

			require.Equal(t, tt.wantStatus, recorder.Code)
			if tt.wantBody != "" {
				require.Equal(t, "application/json", recorder.Header().Get("Content-Type"))
				require.JSONEq(t, tt.wantBody, recorder.Body.String())
			}
		})
	}
}

type mockActivityEventStore struct {
	userID      pgtype.UUID
	resolveErr  error
	enqueueErr  error
	jobErr      error
	params      sqlc.EnqueueActivityEventParams
	enqueued    bool
	jobEnqueued bool
}

func newKnownAthleteStore() *mockActivityEventStore {
	return &mockActivityEventStore{userID: testUserID()}
}

func testUserID() pgtype.UUID {
	return pgtype.UUID{Bytes: [16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16}, Valid: true}
}

func (m *mockActivityEventStore) GetUserIDByAthleteID(context.Context, int64) (pgtype.UUID, error) {
	return m.userID, m.resolveErr
}

func (m *mockActivityEventStore) EnqueueActivityEvent(_ context.Context, arg sqlc.EnqueueActivityEventParams) (sqlc.ActivityEvent, error) {
	m.enqueued = true
	m.params = arg
	return sqlc.ActivityEvent{ID: testUserID()}, m.enqueueErr
}

func (m *mockActivityEventStore) EnqueueActivityEventJob(context.Context, string) error {
	m.jobEnqueued = true
	return m.jobErr
}
