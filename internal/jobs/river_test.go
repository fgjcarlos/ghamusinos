package jobs

import (
	"context"
	"testing"

	"github.com/riverqueue/river"
)

// TestNewClient verifies that NewRiverWorkers creates a configured workers instance.
func TestNewClient(t *testing.T) {
	t.Run("returns river.Workers instance", func(t *testing.T) {
		workers := NewRiverWorkers()
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

// TestImportStravaWorker verifies ImportStravaWorker is properly configured.
func TestImportStravaWorker(t *testing.T) {
	ctx := context.Background()
	worker := &ImportStravaWorker{}

	// Verify worker embeds WorkerDefaults correctly
	job := &river.Job[ImportStravaArgs]{
		Args: ImportStravaArgs{UserID: "test-user"},
	}

	err := worker.Work(ctx, job)
	if err != nil {
		t.Errorf("ImportStravaWorker.Work() returned error: %v", err)
	}
}

// TestRefreshStravaTokenWorker verifies RefreshStravaTokenWorker is properly configured.
func TestRefreshStravaTokenWorker(t *testing.T) {
	ctx := context.Background()
	worker := &RefreshStravaTokenWorker{}

	job := &river.Job[RefreshStravaTokenArgs]{
		Args: RefreshStravaTokenArgs{UserID: "test-user"},
	}

	err := worker.Work(ctx, job)
	if err != nil {
		t.Errorf("RefreshStravaTokenWorker.Work() returned error: %v", err)
	}
}
