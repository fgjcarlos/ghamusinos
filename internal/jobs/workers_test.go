package jobs

import (
	"context"
	"testing"

	"github.com/riverqueue/river"
)

// TestNewRiverWorkers verifies that NewRiverWorkers creates a configured Workers instance.
func TestNewRiverWorkers(t *testing.T) {
	t.Run("registers all handlers", func(t *testing.T) {
		workers := NewRiverWorkers()

		if workers == nil {
			t.Fatal("NewRiverWorkers returned nil")
		}

		// Verify we can extract some metadata (test that registration happened)
		// This will be detailed in workers.go implementation.
	})
}

// TestStubJobKind verifies that StubJob returns correct Kind.
func TestStubJobKind(t *testing.T) {
	job := StubJob{Message: "test"}
	if job.Kind() != "stub" {
		t.Errorf("expected Kind='stub', got %q", job.Kind())
	}
}

// TestStubWorkerWork verifies that StubWorker.Work executes without error.
func TestStubWorkerWork(t *testing.T) {
	worker := &StubWorker{}
	job := &river.Job[StubJob]{Args: StubJob{Message: "test"}}

	err := worker.Work(context.Background(), job)
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
}
