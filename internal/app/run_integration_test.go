//go:build integration

package app

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river/riverdriver/riverpgxv5"
	"github.com/riverqueue/river/rivermigrate"
	"github.com/stretchr/testify/require"
)

func TestRunProductionCompositionStartsWorkersAndMountsWebhooks(t *testing.T) {
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		t.Skip("DATABASE_URL not set; skipping integration test")
	}

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	port := strconv.Itoa(listener.Addr().(*net.TCPAddr).Port)
	require.NoError(t, listener.Close())

	t.Setenv("ENV", "test")
	t.Setenv("PORT", port)
	t.Setenv("DATABASE_URL", databaseURL)
	t.Setenv("CLERK_JWKS_URL", "https://clerk.example.invalid/jwks")
	t.Setenv("STRAVA_CLIENT_ID", "production-composition-client")
	t.Setenv("STRAVA_CLIENT_SECRET", "production-composition-secret")
	t.Setenv("STRAVA_CIPHER_KEY", base64.StdEncoding.EncodeToString(make([]byte, 32)))
	t.Setenv("STRAVA_WEBHOOK_VERIFY_TOKEN", "production-composition-token")
	t.Setenv("STRAVA_REDIRECT_URL", "http://localhost/strava/callback")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	migrateRiverForAppTest(t, ctx, databaseURL)
	errCh := make(chan error, 1)
	go func() {
		errCh <- run(ctx)
	}()

	url := fmt.Sprintf("http://127.0.0.1:%s/strava/webhook?hub.mode=subscribe&hub.challenge=production-proof&hub.verify_token=production-composition-token", port)
	client := &http.Client{Timeout: 250 * time.Millisecond}
	var challenge map[string]string
	deadline := time.NewTimer(10 * time.Second)
	defer deadline.Stop()

	for challenge["hub.challenge"] != "production-proof" {
		select {
		case runErr := <-errCh:
			require.NoError(t, runErr, "production composition stopped before serving webhooks")
			t.Fatal("production composition stopped before serving webhooks")
		case <-deadline.C:
			t.Fatal("production composition did not serve the webhook challenge before the deadline")
		default:
		}

		response, requestErr := client.Get(url) //nolint:noctx // The client timeout bounds each readiness probe.
		if requestErr == nil {
			decodeErr := json.NewDecoder(response.Body).Decode(&challenge)
			closeErr := response.Body.Close()
			if response.StatusCode != http.StatusOK || decodeErr != nil || closeErr != nil {
				challenge = nil
			}
		}
		if challenge["hub.challenge"] != "production-proof" {
			time.Sleep(50 * time.Millisecond)
		}
	}

	cancel()
	select {
	case runErr := <-errCh:
		require.NoError(t, runErr)
	case <-time.After(10 * time.Second):
		t.Fatal("production composition did not shut down after cancellation")
	}
}

func migrateRiverForAppTest(t *testing.T, ctx context.Context, databaseURL string) {
	t.Helper()
	pool, err := pgxpool.New(ctx, databaseURL)
	require.NoError(t, err)
	t.Cleanup(pool.Close)
	migrator, err := rivermigrate.New(riverpgxv5.New(pool), nil)
	require.NoError(t, err)
	_, err = migrator.Migrate(ctx, rivermigrate.DirectionUp, nil)
	require.NoError(t, err)
}
