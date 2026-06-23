package strava

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// TestClientGetActivities verifies fetching activities.
func TestClientGetActivities(t *testing.T) {
	logger := (*slog.Logger)(nil) // use nil for no-op logger

	activities := []Activity{
		{
			ID:                123456,
			Name:              "Morning Run",
			Type:              "Run",
			StartDate:         "2025-06-24T06:00:00Z",
			DistanceMeters:    10000,
			MovingTimeSeconds: 3600,
		},
	}

	// Create mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v3/athlete/activities" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}

		if auth := r.Header.Get("Authorization"); auth != "Bearer test_token" {
			t.Fatalf("unexpected auth header: %s", auth)
		}

		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("X-RateLimit-Usage", "10,100")
		w.Header().Set("X-RateLimit-Limit", "600,30000")
		json.NewEncoder(w).Encode(activities)
	}))
	defer server.Close()

	client := NewClient("test_token", logger)
	client.baseURL = server.URL + "/api/v3"

	ctx := context.Background()
	result, err := client.GetActivities(ctx, time.Now().Add(-24*time.Hour), time.Now(), 1)

	if err != nil {
		t.Fatalf("GetActivities failed: %v", err)
	}

	if len(result) != 1 {
		t.Fatalf("expected 1 activity, got %d", len(result))
	}

	if result[0].ID != 123456 {
		t.Fatalf("activity ID mismatch: got %d, want 123456", result[0].ID)
	}
}

// TestClientGetActivity verifies fetching a single activity.
func TestClientGetActivity(t *testing.T) {
	logger := (*slog.Logger)(nil) // use nil for no-op logger

	activity := Activity{
		ID:                123456,
		Name:              "Evening Ride",
		Type:              "Ride",
		StartDate:         "2025-06-24T18:00:00Z",
		DistanceMeters:    50000,
		MovingTimeSeconds: 7200,
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v3/activities/123456" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}

		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("X-RateLimit-Usage", "11,101")
		w.Header().Set("X-RateLimit-Limit", "600,30000")
		json.NewEncoder(w).Encode(activity)
	}))
	defer server.Close()

	client := NewClient("test_token", logger)
	client.baseURL = server.URL + "/api/v3"

	ctx := context.Background()
	result, err := client.GetActivity(ctx, 123456)

	if err != nil {
		t.Fatalf("GetActivity failed: %v", err)
	}

	if result.ID != 123456 {
		t.Fatalf("activity ID mismatch: got %d, want 123456", result.ID)
	}
}

// TestClientGetStreams verifies fetching activity streams.
func TestClientGetStreams(t *testing.T) {
	logger := (*slog.Logger)(nil) // use nil for no-op logger

	streams := Streams{
		Time: &Stream{
			Data:       []interface{}{0, 1, 2, 3, 4},
			SeriesType: "distance",
			OrigType:   "integer",
		},
		Heartrate: &Stream{
			Data:       []interface{}{120, 125, 130, 135, 140},
			SeriesType: "time",
			OrigType:   "integer",
		},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v3/activities/123456/streams" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}

		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("X-RateLimit-Usage", "12,102")
		w.Header().Set("X-RateLimit-Limit", "600,30000")
		json.NewEncoder(w).Encode(streams)
	}))
	defer server.Close()

	client := NewClient("test_token", logger)
	client.baseURL = server.URL + "/api/v3"

	ctx := context.Background()
	result, err := client.GetStreams(ctx, 123456, []string{"time", "heartrate"})

	if err != nil {
		t.Fatalf("GetStreams failed: %v", err)
	}

	if result.Time == nil {
		t.Fatal("expected time stream")
	}

	if len(result.Time.Data) != 5 {
		t.Fatalf("expected 5 time points, got %d", len(result.Time.Data))
	}
}

// TestClientRequestWithContext verifies context propagation.
func TestClientRequestWithContext(t *testing.T) {
	logger := (*slog.Logger)(nil) // use nil for no-op logger

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("[]"))
	}))
	defer server.Close()

	client := NewClient("test_token", logger)
	client.baseURL = server.URL + "/api/v3"

	// Test with cancelled context
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := client.GetActivities(ctx, time.Now().Add(-24*time.Hour), time.Now(), 1)
	if err == nil {
		t.Fatal("expected error with cancelled context")
	}
}

// TestClientHTTPError verifies error responses.
func TestClientHTTPError(t *testing.T) {
	logger := (*slog.Logger)(nil) // use nil for no-op logger

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`{"message":"Unauthorized"}`))
	}))
	defer server.Close()

	client := NewClient("invalid_token", logger)
	client.baseURL = server.URL + "/api/v3"

	ctx := context.Background()
	_, err := client.GetActivities(ctx, time.Now().Add(-24*time.Hour), time.Now(), 1)

	if err == nil {
		t.Fatal("expected error for HTTP 401")
	}
}

// TestClientRateLimitTracking verifies rate limit headers are parsed.
func TestClientRateLimitTracking(t *testing.T) {
	logger := (*slog.Logger)(nil) // use nil for no-op logger

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("X-RateLimit-Usage", "500,15000")
		w.Header().Set("X-RateLimit-Limit", "600,30000")
		w.Write([]byte("[]"))
	}))
	defer server.Close()

	client := NewClient("test_token", logger)
	client.baseURL = server.URL + "/api/v3"

	ctx := context.Background()
	_, _ = client.GetActivities(ctx, time.Now().Add(-24*time.Hour), time.Now(), 1)

	usage15, usage24 := client.RateLimitUsage()
	if usage15 != 500 || usage24 != 15000 {
		t.Fatalf("rate limit usage mismatch: got (%d, %d), want (500, 15000)", usage15, usage24)
	}
}

// TestClientCircuitBreakerOpens verifies circuit breaker opens after failures.
func TestClientCircuitBreakerOpens(t *testing.T) {
	logger := (*slog.Logger)(nil) // use nil for no-op logger

	failureCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		failureCount++
		w.WriteHeader(http.StatusServiceUnavailable)
		w.Write([]byte("Service Unavailable"))
	}))
	defer server.Close()

	client := NewClient("test_token", logger)
	client.baseURL = server.URL + "/api/v3"
	// Set threshold low and long timeout to keep circuit open during test
	client.breaker = NewCircuitBreaker(1, 10*time.Second) // 1 failure, 10s timeout to stay open

	ctx := context.Background()

	// First call will exhaust retries (3) and open the circuit
	_, _ = client.GetActivities(ctx, time.Now().Add(-24*time.Hour), time.Now(), 1)

	// Verify circuit is open
	if client.CircuitBreakerState() != StateOpen {
		t.Fatal("circuit breaker should be open after failures")
	}

	// Record failure count before testing open state
	failureCountAfterOpen := failureCount

	// Next call should be rejected by the open circuit
	_, _ = client.GetActivities(ctx, time.Now().Add(-24*time.Hour), time.Now(), 1)

	// The circuit should reject the call immediately without making the request
	// But since the threshold is 1, it likely already made ~3 requests before opening
	// After opening, it should make 0 new requests (or very few if half-open probe happens)
	newFailures := failureCount - failureCountAfterOpen
	if newFailures > 2 {
		t.Fatalf("circuit breaker should reject calls when open; got %d new failures (expected ≤2)", newFailures)
	}
}
