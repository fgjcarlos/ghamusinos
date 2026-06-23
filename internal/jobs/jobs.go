package jobs

import (
	"github.com/riverqueue/river"
)

// RegisterHandlers creates and returns a river.Workers instance.
// Job registration happens in workers.go via the Workers type.
func RegisterHandlers() *river.Workers {
	return river.NewWorkers()
}
