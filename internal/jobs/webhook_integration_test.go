//go:build integration

package jobs

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"

	"github.com/fgjcarlos/ghamusinos/internal/config"
	appcrypto "github.com/fgjcarlos/ghamusinos/internal/crypto"
	"github.com/fgjcarlos/ghamusinos/internal/db/sqlc"
	"github.com/fgjcarlos/ghamusinos/internal/db/status"
	"github.com/fgjcarlos/ghamusinos/internal/strava"
)

// This is the authoritative sample from Strava's Webhook Events API docs.
const documentedWebhookEvent = `{
    "aspect_type": "update",
    "event_time": 1516126040,
    "object_id": 1360128428,
    "object_type": "activity",
    "owner_id": 134815,
    "subscription_id": 120475,
    "updates": {"title": "Messy"}
}`

func TestWebhookRuntimeComposition_ProcessesDocumentedEventEndToEnd(t *testing.T) {
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		t.Skip("DATABASE_URL not set; skipping integration test")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	pool, err := pgxpool.New(ctx, databaseURL)
	require.NoError(t, err)
	t.Cleanup(pool.Close)
	migrateRiver(t, ctx, pool)

	queries := sqlc.New(pool)
	clerkID := fmt.Sprintf("issue165-e2e-%d", time.Now().UnixNano())
	user, err := queries.CreateUser(ctx, sqlc.CreateUserParams{
		ClerkUserID:  clerkID,
		Email:        clerkID + "@example.invalid",
		DisplayName:  pgtype.Text{String: "Issue 165 E2E", Valid: true},
		InviteStatus: status.InviteStatusActive,
	})
	require.NoError(t, err)
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cleanupCancel()
		_, _ = pool.Exec(cleanupCtx, "DELETE FROM activity_events WHERE object_id = $1 AND aspect_type = $2 AND event_time = to_timestamp($3)", int64(1360128428), "update", int64(1516126040))
		_, _ = pool.Exec(cleanupCtx, "DELETE FROM users WHERE id = $1", user.ID)
	})

	cipherKey := make([]byte, 32)
	for i := range cipherKey {
		cipherKey[i] = byte(i)
	}
	accessCipher, err := appcrypto.Encrypt([]byte("runtime-access-token"), cipherKey)
	require.NoError(t, err)
	refreshCipher, err := appcrypto.Encrypt([]byte("runtime-refresh-token"), cipherKey)
	require.NoError(t, err)
	_, err = queries.UpsertStravaTokens(ctx, sqlc.UpsertStravaTokensParams{
		UserID:        user.ID,
		AccessCipher:  accessCipher,
		RefreshCipher: refreshCipher,
		ExpiresAt:     pgtype.Timestamptz{Time: time.Now().Add(time.Hour), Valid: true},
		AthleteID:     134815,
		Scopes:        "activity:read",
	})
	require.NoError(t, err)

	var fetches atomic.Int32
	stravaClient, err := strava.NewClient(strava.Config{
		ClientID:     "runtime-client",
		ClientSecret: "runtime-secret",
		HTTPClient: &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			if req.URL.Path != "/api/v3/activities/1360128428" {
				return nil, fmt.Errorf("unexpected Strava path %q", req.URL.Path)
			}
			if got := req.Header.Get("Authorization"); got != "Bearer runtime-access-token" {
				return nil, fmt.Errorf("unexpected Authorization header %q", got)
			}
			fetches.Add(1)
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body: io.NopCloser(strings.NewReader(`{
					"id":1360128428,
					"name":"Messy",
					"type":"Run",
					"start_date":"2018-01-16T16:47:20Z",
					"distance":12345.6,
					"moving_time":3600,
					"elapsed_time":3700,
					"total_elevation_gain":456.7
				}`)),
				Request: req,
			}, nil
		})},
	})
	require.NoError(t, err)

	client, err := NewClient(ctx, pool, Deps{
		Pool:      pool,
		Config:    &config.Config{Strava: &config.StravaConfig{BackfillDays: 42}},
		Strava:    stravaClient,
		CipherKey: cipherKey,
	}, true)
	require.NoError(t, err)
	require.NoError(t, client.Start(ctx))
	t.Cleanup(func() {
		stopCtx, stopCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer stopCancel()
		_ = client.Stop(stopCtx)
	})

	handler := strava.WebhookHandler(NewActivityEventStoreAdapter(queries, client))
	req := httptest.NewRequestWithContext(ctx, http.MethodPost, "/strava/webhook", strings.NewReader(documentedWebhookEvent))
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, req)
	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())

	require.Eventually(t, func() bool {
		var processed bool
		err := pool.QueryRow(ctx, `SELECT processed_at IS NOT NULL
			FROM activity_events
			WHERE object_id = $1 AND aspect_type = $2 AND event_time = to_timestamp($3)`,
			int64(1360128428), "update", int64(1516126040)).Scan(&processed)
		if err != nil || !processed {
			return false
		}

		var name string
		err = pool.QueryRow(ctx, `SELECT name FROM activities
			WHERE user_id = $1 AND external_source = 'strava' AND external_id = $2`,
			user.ID, int64(1360128428)).Scan(&name)
		return err == nil && name == "Messy"
	}, 10*time.Second, 100*time.Millisecond)
	require.Equal(t, int32(1), fetches.Load())
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}
