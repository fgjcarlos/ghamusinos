package jobs

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/rivertype"
	"github.com/stretchr/testify/require"

	"github.com/fgjcarlos/ghamusinos/internal/config"
	"github.com/fgjcarlos/ghamusinos/internal/db/sqlc"
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

func TestActivityEventStoreAdapter_DelegatesPersistenceAndEnqueuesTypedJob(t *testing.T) {
	queries := &fakeActivityEventQueries{userID: pgtype.UUID{Bytes: [16]byte{1}, Valid: true}}
	inserter := &fakeRiverJobInserter{}
	adapter := NewActivityEventStoreAdapter(queries, inserter)
	ctx := context.Background()

	userID, err := adapter.GetUserIDByAthleteID(ctx, 134815)
	require.NoError(t, err)
	require.Equal(t, queries.userID, userID)

	wantParams := sqlc.EnqueueActivityEventParams{ObjectID: 1360128428}
	_, err = adapter.EnqueueActivityEvent(ctx, wantParams)
	require.NoError(t, err)
	require.Equal(t, wantParams, queries.params)

	require.NoError(t, adapter.EnqueueActivityEventJob(ctx, "01020304-0506-0708-090a-0b0c0d0e0f10"))
	jobArgs, ok := inserter.args.(*IngestActivityEventArgs)
	require.True(t, ok)
	require.Equal(t, "01020304-0506-0708-090a-0b0c0d0e0f10", jobArgs.EventID)
}

func TestActivityEventStoreAdapter_RejectsMissingRiverClient(t *testing.T) {
	adapter := NewActivityEventStoreAdapter(&fakeActivityEventQueries{}, nil)
	require.ErrorContains(t, adapter.EnqueueActivityEventJob(context.Background(), "event-id"), "River client is nil")
}

type fakeActivityEventQueries struct {
	userID pgtype.UUID
	params sqlc.EnqueueActivityEventParams
}

func (f *fakeActivityEventQueries) GetUserIDByAthleteID(context.Context, int64) (pgtype.UUID, error) {
	return f.userID, nil
}

func (f *fakeActivityEventQueries) EnqueueActivityEvent(_ context.Context, params sqlc.EnqueueActivityEventParams) (sqlc.ActivityEvent, error) {
	f.params = params
	return sqlc.ActivityEvent{}, nil
}

type fakeRiverJobInserter struct {
	args river.JobArgs
}

func (f *fakeRiverJobInserter) Insert(_ context.Context, args river.JobArgs, _ *river.InsertOpts) (*rivertype.JobInsertResult, error) {
	f.args = args
	return &rivertype.JobInsertResult{}, nil
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
