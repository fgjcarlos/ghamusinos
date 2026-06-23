package jobs

import (
	"context"

	"github.com/riverqueue/river"
)

// NewRiverWorkers creates a river.Workers instance with all registered job handlers.
func NewRiverWorkers() *river.Workers {
	workers := river.NewWorkers()
	river.AddWorker(workers, &ImportStravaWorker{})
	river.AddWorker(workers, &RefreshStravaTokenWorker{})
	return workers
}

// StubJob is a minimal job type for testing River integration.
type StubJob struct {
	Message string
}

// Kind returns the job kind identifier for StubJob.
func (j StubJob) Kind() string {
	return string(KindStub)
}

// Work is the job handler for StubJob.
func (j StubJob) Work(ctx context.Context) error {
	// Stub job does nothing; used for testing enqueue and execution.
	return nil
}

// StubWorker implements river.Worker[StubJob] for processing stub jobs.
type StubWorker struct {
	river.WorkerDefaults[StubJob]
}

// Work processes a StubJob.
func (w *StubWorker) Work(ctx context.Context, job *river.Job[StubJob]) error {
	// Stub job does nothing; used for testing enqueue and execution.
	return nil
}

// ImportStravaWorker handles importing Strava data for a user.
type ImportStravaWorker struct {
	river.WorkerDefaults[ImportStravaArgs]
}

// Work processes an ImportStrava job.
func (w *ImportStravaWorker) Work(ctx context.Context, job *river.Job[ImportStravaArgs]) error {
	// TODO: Implement Strava API integration
	// This worker will fetch activity data from Strava within the given window
	// and persist it to the database.
	return nil
}

// RefreshStravaTokenWorker handles refreshing a user's Strava OAuth token.
type RefreshStravaTokenWorker struct {
	river.WorkerDefaults[RefreshStravaTokenArgs]
}

// Work processes a RefreshStravaToken job.
func (w *RefreshStravaTokenWorker) Work(ctx context.Context, job *river.Job[RefreshStravaTokenArgs]) error {
	// TODO: Implement Strava token refresh
	// This worker will use the refresh token to get a new OAuth token
	// and update it in the database.
	return nil
}
