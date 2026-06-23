package jobs

import (
	"context"

	"github.com/riverqueue/river"
)

// NewRiverWorkers creates a river.Workers instance with all registered job handlers.
// For now, returns an empty workers instance that will be configured by the river.Client setup.
func NewRiverWorkers() *river.Workers {
	return river.NewWorkers()
}

// StubJob is a minimal job type for testing River integration.
type StubJob struct {
	Message string
}

// Kind returns the job kind identifier for StubJob.
func (j StubJob) Kind() string {
	return "stub"
}

// Work is the job handler for StubJob.
func (j StubJob) Work(ctx context.Context) error {
	// Stub job does nothing; used for testing enqueue and execution.
	return nil
}

// StubWorker implements river.Worker[StubJob] for processing stub jobs.
type StubWorker struct {
}

// Work processes a StubJob.
func (w *StubWorker) Work(ctx context.Context, job *river.Job[StubJob]) error {
	// Stub job does nothing; used for testing enqueue and execution.
	return nil
}
