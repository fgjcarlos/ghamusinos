package strava

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

// Client is the Strava API client.
type Client struct {
	httpClient   *http.Client
	accessToken  string
	baseURL      string
	rateLimiter  *RateLimitTracker
	breaker      *CircuitBreaker
	logger       *slog.Logger
	maxRetries   int
	retryBackoff time.Duration
}

// NewClient creates a new Strava API client.
func NewClient(accessToken string, logger *slog.Logger) *Client {
	if logger == nil {
		logger = slog.New(slog.NewTextHandler(io.Discard, nil))
	}

	return &Client{
		httpClient:   &http.Client{Timeout: 30 * time.Second},
		accessToken:  accessToken,
		baseURL:      "https://www.strava.com/api/v3",
		rateLimiter:  NewRateLimitTracker(logger),
		breaker:      NewCircuitBreaker(5, 60*time.Second), // 5 failures, 60s timeout
		logger:       logger,
		maxRetries:   3,
		retryBackoff: 100 * time.Millisecond,
	}
}

// Get performs a GET request to the Strava API.
func (c *Client) Get(ctx context.Context, path string, params map[string]string) ([]byte, error) {
	return c.doRequest(ctx, "GET", path, params, nil)
}

// Post performs a POST request to the Strava API.
func (c *Client) Post(ctx context.Context, path string, params map[string]string, body interface{}) ([]byte, error) {
	var bodyBytes []byte
	var err error
	if body != nil {
		bodyBytes, err = json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("strava: failed to marshal request body: %w", err)
		}
	}
	return c.doRequest(ctx, "POST", path, params, bodyBytes)
}

// doRequest performs an HTTP request with retries and circuit breaker.
func (c *Client) doRequest(ctx context.Context, method, path string, params map[string]string, body []byte) ([]byte, error) {
	// Use circuit breaker
	var respBody []byte
	err := c.breaker.Call(func() error {
		var innerErr error
		respBody, innerErr = c.executeRequestWithRetries(ctx, method, path, params, body)
		return innerErr
	})
	return respBody, err
}

// executeRequestWithRetries performs the actual HTTP request with exponential backoff retries.
func (c *Client) executeRequestWithRetries(ctx context.Context, method, path string, params map[string]string, body []byte) ([]byte, error) {
	var lastErr error

	for attempt := 0; attempt < c.maxRetries; attempt++ {
		respBody, statusCode, err := c.executeRequest(ctx, method, path, params, body)

		// Success
		if statusCode >= 200 && statusCode < 300 {
			return respBody, nil
		}

		// Rate limit: honor Retry-After
		if statusCode == 429 {
			retryAfter := 60 // default
			// Parse Retry-After header if present
			// (would need response headers passed through, simplified for now)
			lastErr = fmt.Errorf("strava: rate limited, retry after %ds", retryAfter)
			if attempt < c.maxRetries-1 {
				time.Sleep(time.Duration(retryAfter) * time.Second)
				continue
			}
		}

		// Server error: retry
		if statusCode >= 500 || (err != nil && statusCode == 0) {
			lastErr = fmt.Errorf("strava: server error or network issue (status %d): %w", statusCode, err)
			if attempt < c.maxRetries-1 {
				backoff := c.retryBackoff * time.Duration(1<<uint(attempt)) // exponential backoff
				time.Sleep(backoff)
				continue
			}
		}

		// Client error: don't retry
		if statusCode >= 400 && statusCode < 500 {
			return nil, fmt.Errorf("strava: client error (%d): %s", statusCode, string(respBody))
		}

		// Network error
		if err != nil {
			lastErr = fmt.Errorf("strava: network error: %w", err)
			if attempt < c.maxRetries-1 {
				backoff := c.retryBackoff * time.Duration(1<<uint(attempt))
				time.Sleep(backoff)
				continue
			}
		}

		return respBody, lastErr
	}

	return nil, fmt.Errorf("strava: max retries exceeded: %w", lastErr)
}

// executeRequest performs a single HTTP request.
func (c *Client) executeRequest(ctx context.Context, method, path string, params map[string]string, body []byte) ([]byte, int, error) {
	u, err := url.Parse(c.baseURL + path)
	if err != nil {
		return nil, 0, fmt.Errorf("strava: invalid URL: %w", err)
	}

	// Add query parameters
	q := u.Query()
	for k, v := range params {
		q.Set(k, v)
	}
	u.RawQuery = q.Encode()

	// Create request
	var bodyReader io.Reader
	if body != nil {
		bodyReader = bytes.NewReader(body)
	}

	req, err := http.NewRequestWithContext(ctx, method, u.String(), bodyReader)
	if err != nil {
		return nil, 0, fmt.Errorf("strava: failed to create request: %w", err)
	}

	// Set headers
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.accessToken))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	// Execute
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, 0, fmt.Errorf("strava: request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	// Read response body
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, resp.StatusCode, fmt.Errorf("strava: failed to read response body: %w", err)
	}

	// Update rate limiter from headers
	c.rateLimiter.UpdateFromHeaders(
		resp.Header.Get("X-RateLimit-Usage"),
		resp.Header.Get("X-RateLimit-Limit"),
	)

	return respBody, resp.StatusCode, nil
}

// GetActivities fetches activities for the authenticated user.
// Supports pagination and filtering by date range.
func (c *Client) GetActivities(ctx context.Context, after, before time.Time, page int) ([]Activity, error) {
	params := map[string]string{
		"after":    strconv.FormatInt(after.Unix(), 10),
		"before":   strconv.FormatInt(before.Unix(), 10),
		"page":     strconv.Itoa(page),
		"per_page": "30",
	}

	respBody, err := c.Get(ctx, "/athlete/activities", params)
	if err != nil {
		return nil, fmt.Errorf("strava: failed to fetch activities: %w", err)
	}

	var activities []Activity
	if err := json.Unmarshal(respBody, &activities); err != nil {
		return nil, fmt.Errorf("strava: failed to parse activities: %w", err)
	}

	return activities, nil
}

// GetActivity fetches a single activity by ID.
func (c *Client) GetActivity(ctx context.Context, id int64) (*Activity, error) {
	path := fmt.Sprintf("/activities/%d", id)
	respBody, err := c.Get(ctx, path, nil)
	if err != nil {
		return nil, fmt.Errorf("strava: failed to fetch activity %d: %w", id, err)
	}

	var activity Activity
	if err := json.Unmarshal(respBody, &activity); err != nil {
		return nil, fmt.Errorf("strava: failed to parse activity: %w", err)
	}

	return &activity, nil
}

// GetStreams fetches time-series data for an activity.
func (c *Client) GetStreams(ctx context.Context, id int64, streamTypes []string) (*Streams, error) {
	// Default to common stream types if not specified
	if len(streamTypes) == 0 {
		streamTypes = []string{"time", "distance", "altitude", "heartrate", "cadence", "watts", "latlng"}
	}

	// Build query parameter
	typesStr := ""
	for i, t := range streamTypes {
		if i > 0 {
			typesStr += ","
		}
		typesStr += t
	}

	path := fmt.Sprintf("/activities/%d/streams", id)
	params := map[string]string{
		"keys":        typesStr,
		"key_by_type": "true",
	}

	respBody, err := c.Get(ctx, path, params)
	if err != nil {
		return nil, fmt.Errorf("strava: failed to fetch streams for activity %d: %w", id, err)
	}

	var streams Streams
	if err := json.Unmarshal(respBody, &streams); err != nil {
		return nil, fmt.Errorf("strava: failed to parse streams: %w", err)
	}

	return &streams, nil
}

// SubscribeWebhook creates a webhook subscription for events.
func (c *Client) SubscribeWebhook(ctx context.Context, callbackURL string) (*Subscription, error) {
	body := map[string]interface{}{
		"object_type":  "activity",
		"aspect_type":  "create,update",
		"callback_url": callbackURL,
	}

	respBody, err := c.Post(ctx, "/push_subscriptions", nil, body)
	if err != nil {
		return nil, fmt.Errorf("strava: failed to subscribe to webhook: %w", err)
	}

	var sub Subscription
	if err := json.Unmarshal(respBody, &sub); err != nil {
		return nil, fmt.Errorf("strava: failed to parse subscription response: %w", err)
	}

	return &sub, nil
}

// RateLimitUsage returns current rate limit usage.
func (c *Client) RateLimitUsage() (usage15min, usage24h int) {
	return c.rateLimiter.GetUsage()
}

// CircuitBreakerState returns the current state of the circuit breaker.
func (c *Client) CircuitBreakerState() State {
	return c.breaker.GetState()
}
