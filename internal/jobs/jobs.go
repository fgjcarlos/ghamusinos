package jobs

import (
	"time"

	"github.com/riverqueue/river"
)

// Job kind constants for River job types
type Kind string

const (
	KindImportStrava       Kind = "import_strava"
	KindRefreshStravaToken Kind = "refresh_strava_token"
	KindStub               Kind = "stub"
)

// ImportStravaArgs contains arguments for importing Strava data for a user
type ImportStravaArgs struct {
	UserID      string
	StravaID    int64
	WindowStart time.Time
	WindowEnd   time.Time
}

// Kind returns the job kind for ImportStravaArgs
func (a ImportStravaArgs) Kind() string {
	return string(KindImportStrava)
}

// RefreshStravaTokenArgs contains arguments for refreshing a user's Strava token
type RefreshStravaTokenArgs struct {
	UserID string
}

// Kind returns the job kind for RefreshStravaTokenArgs
func (a RefreshStravaTokenArgs) Kind() string {
	return string(KindRefreshStravaToken)
}

// RegisterHandlers creates and returns a river.Workers instance.
// Job registration happens in workers.go via the Workers type.
func RegisterHandlers() *river.Workers {
	return river.NewWorkers()
}
