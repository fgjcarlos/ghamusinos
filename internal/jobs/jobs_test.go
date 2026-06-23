package jobs

import (
	"testing"
)

// TestRegisterHandlers verifies that RegisterHandlers returns a populated Workers instance.
func TestRegisterHandlers(t *testing.T) {
	t.Run("returns workers instance", func(t *testing.T) {
		workers := RegisterHandlers()

		if workers == nil {
			t.Fatal("RegisterHandlers returned nil")
		}

		// Verify that workers is a valid *river.Workers (no panic, no nil)
		// The actual handler registration is tested in workers_test.go
	})
}
