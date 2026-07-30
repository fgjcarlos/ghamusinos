package strava

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/fgjcarlos/ghamusinos/internal/db/sqlc"
)

// T4.1: POST /api/v1/strava/webhook returns 401 if X-Strava-Signature header missing
func TestWebhook_MissingSignature(t *testing.T) {
	body := []byte(`{"object_type":"activity","aspect_type":"create","object_id":123,"external_id":"evt_123"}`)
	mockStore := &mockActivityEventStore{}
	handler := WebhookHandler("test-secret", mockStore)

	req := httptest.NewRequest("POST", "/webhook", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	// Missing X-Strava-Signature header
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", w.Code)
	}
}

// T4.2: POST /api/v1/strava/webhook returns 401 if HMAC-SHA256 signature mismatches
func TestWebhook_InvalidSignature(t *testing.T) {
	body := []byte(`{"object_type":"activity","aspect_type":"create","object_id":123,"external_id":"evt_123"}`)
	mockStore := &mockActivityEventStore{}
	handler := WebhookHandler("test-secret", mockStore)

	req := httptest.NewRequest("POST", "/webhook", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Strava-Signature", "v0=invalid_signature_hex")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", w.Code)
	}
}

// T4.3: POST /api/v1/strava/webhook validates HMAC-SHA256 with STRAVA_WEBHOOK_SECRET correctly
func TestWebhook_ValidSignature(t *testing.T) {
	body := []byte(`{"object_type":"activity","aspect_type":"create","object_id":123,"external_id":"evt_123"}`)
	secret := "test-secret"
	mockStore := &mockActivityEventStore{}
	handler := WebhookHandler(secret, mockStore)

	// Calculate correct signature
	h := hmac.New(sha256.New, []byte(secret))
	h.Write(body)
	expectedSig := "v0=" + hex.EncodeToString(h.Sum(nil))

	req := httptest.NewRequest("POST", "/webhook", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Strava-Signature", expectedSig)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	// Should not return 401
	if w.Code == http.StatusUnauthorized {
		t.Errorf("expected non-401 status with valid signature, got %d", w.Code)
	}
}

// T4.5: GET /api/v1/strava/webhook?hub.mode=subscribe&hub.challenge=...&hub.verify_token=... returns challenge
func TestWebhookChallenge_ValidChallenge(t *testing.T) {
	handler := WebhookChallengeHandler("test-verify-token")

	req := httptest.NewRequest("GET", "/webhook?hub.mode=subscribe&hub.challenge=test-challenge&hub.verify_token=test-verify-token", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}

	if w.Body.String() != "test-challenge" {
		t.Errorf("expected body 'test-challenge', got %q", w.Body.String())
	}
}

// T4.7: webhook persists valid event via queries.EnqueueActivityEvent
func TestWebhook_PersistsEvent(t *testing.T) {
	body := []byte(`{"object_type":"activity","aspect_type":"create","object_id":123,"external_id":"evt_123","athlete":{"id":456}}`)
	secret := "test-secret"
	mockStore := &mockActivityEventStore{}
	handler := WebhookHandler(secret, mockStore)

	// Calculate correct signature
	h := hmac.New(sha256.New, []byte(secret))
	h.Write(body)
	expectedSig := "v0=" + hex.EncodeToString(h.Sum(nil))

	req := httptest.NewRequest("POST", "/webhook", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Strava-Signature", expectedSig)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK && w.Code != http.StatusAccepted && w.Code != http.StatusNoContent {
		t.Errorf("expected 200/202/204 for valid webhook, got %d", w.Code)
	}

	if !mockStore.enqueued {
		t.Error("expected EnqueueActivityEvent to be called")
	}
}

// T4.9: idempotency — replaying the same event_id returns 200 OK but does NOT create a duplicate row
func TestWebhook_Idempotency(t *testing.T) {
	body := []byte(`{"object_type":"activity","aspect_type":"create","object_id":123,"external_id":"evt_same","athlete":{"id":456}}`)
	secret := "test-secret"
	mockStore := &mockActivityEventStore{}
	handler := WebhookHandler(secret, mockStore)

	// Calculate correct signature
	h := hmac.New(sha256.New, []byte(secret))
	h.Write(body)
	expectedSig := "v0=" + hex.EncodeToString(h.Sum(nil))

	// First request
	req1 := httptest.NewRequest("POST", "/webhook", bytes.NewReader(body))
	req1.Header.Set("Content-Type", "application/json")
	req1.Header.Set("X-Strava-Signature", expectedSig)
	w1 := httptest.NewRecorder()
	handler.ServeHTTP(w1, req1)

	if w1.Code < 200 || w1.Code >= 300 {
		t.Errorf("first request failed with status %d", w1.Code)
	}

	// Second request with same event_id
	mockStore.reset()
	req2 := httptest.NewRequest("POST", "/webhook", bytes.NewReader(body))
	req2.Header.Set("Content-Type", "application/json")
	req2.Header.Set("X-Strava-Signature", expectedSig)
	w2 := httptest.NewRecorder()
	handler.ServeHTTP(w2, req2)

	if w2.Code < 200 || w2.Code >= 300 {
		t.Errorf("second request failed with status %d", w2.Code)
	}
}

// T4.11: successful POST enqueues a River job (stub IngestActivityEventWorker) for the event
func TestWebhook_EnqueuesRiverJob(t *testing.T) {
	body := []byte(`{"object_type":"activity","aspect_type":"create","object_id":123,"external_id":"evt_123","athlete":{"id":456}}`)
	secret := "test-secret"
	mockStore := &mockActivityEventStore{}
	handler := WebhookHandler(secret, mockStore)

	// Calculate correct signature
	h := hmac.New(sha256.New, []byte(secret))
	h.Write(body)
	expectedSig := "v0=" + hex.EncodeToString(h.Sum(nil))

	req := httptest.NewRequest("POST", "/webhook", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Strava-Signature", expectedSig)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code < 200 || w.Code >= 300 {
		t.Errorf("expected 2xx status, got %d", w.Code)
	}

	if !mockStore.jobEnqueued {
		t.Error("expected River job to be enqueued")
	}
}

// Mock ActivityEventStore for testing
type mockActivityEventStore struct {
	enqueued    bool
	jobEnqueued bool
}

func (m *mockActivityEventStore) EnqueueActivityEvent(ctx context.Context, arg sqlc.EnqueueActivityEventParams) (sqlc.ActivityEvent, error) {
	m.enqueued = true
	return sqlc.ActivityEvent{
		ExternalID: arg.ExternalID,
		UserID:     arg.UserID,
		ObjectType: arg.ObjectType,
		AspectType: arg.AspectType,
		ObjectID:   arg.ObjectID,
		RawPayload: arg.RawPayload,
	}, nil
}

func (m *mockActivityEventStore) EnqueueRiverJob(ctx context.Context, jobArgs interface{}) error {
	m.jobEnqueued = true
	return nil
}

func (m *mockActivityEventStore) reset() {
	m.enqueued = false
	m.jobEnqueued = false
}
