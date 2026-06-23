package strava

import (
	"testing"
)

// (Tests for circuit breaker are in ratelimit_test.go after TestRateLimitTrackerConcurrency)
// This file is for additional circuit breaker unit tests if needed.

// TestCircuitBreakerConcurrency verifies thread safety.
func TestCircuitBreakerConcurrency(t *testing.T) {
	cb := NewCircuitBreaker(100, 1e9) // High threshold to avoid opening

	done := make(chan bool)

	// Multiple goroutines calling simultaneously
	for i := 0; i < 20; i++ {
		go func() {
			_ = cb.Call(func() error { return nil })
			done <- true
		}()
	}

	for i := 0; i < 20; i++ {
		<-done
	}

	// Verify no race conditions occurred
	if cb.GetState() != StateClosed {
		t.Fatalf("state should remain closed, got %v", cb.GetState())
	}
}
