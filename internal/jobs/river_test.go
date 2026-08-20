package jobs

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/fgjcarlos/ghamusinos/internal/config"
)

// TestNewClient verifies that NewRiverWorkers creates a configured workers instance.
// AUD-04: NewRiverWorkers takes a Deps bundle and a registerStravaWorkers flag.
func TestNewClient(t *testing.T) {
	t.Run("returns river.Workers instance", func(t *testing.T) {
		// registerStravaWorkers=false because we have no Strava client in tests;
		// this is the "no Strava configured" branch. Pool and Config must still
		// be non-nil because they are always required.
		workers, err := NewRiverWorkers(Deps{
			Pool:   &pgxpool.Pool{},
			Config: &config.Config{},
		}, false)
		if err != nil {
			t.Fatalf("NewRiverWorkers returned error: %v", err)
		}
		if workers == nil {
			t.Fatal("NewRiverWorkers returned nil")
		}
	})
}

// TestClientStubJobEnqueue verifies that a StubJob can be enqueued.
func TestClientStubJobEnqueue(t *testing.T) {
	t.Run("enqueues stub job without error", func(t *testing.T) {
		ctx := context.Background()
		stubJob := StubJob{Message: "test"}

		// Verify job kind is set correctly
		if stubJob.Kind() != string(KindStub) {
			t.Errorf("expected job kind %q, got %q", KindStub, stubJob.Kind())
		}

		// Verify work method can be called and returns no error
		err := stubJob.Work(ctx)
		if err != nil {
			t.Errorf("StubJob.Work() returned error: %v", err)
		}
	})
}

// AUD-04 removed ErrTokenRefresherNotConfigured and made worker construction
// require explicit deps. The "Work() on an empty worker returns the config
// error" tests that used to live here were asserting a behavior that AUD-04
// makes unreachable: a worker with zero deps cannot be built in production.
// The validation moved to NewRiverWorkers; see backfill_test.go for the
// TestNewRiverWorkers_Requires* tests that now cover the contract.
