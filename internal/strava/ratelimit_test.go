package strava

import (
	"fmt"
	"testing"
	"time"
)

// TestRateLimitTrackerUpdateFromHeaders verifies parsing of rate limit headers.
func TestRateLimitTrackerUpdateFromHeaders(t *testing.T) {
	tests := []struct {
		name        string
		usage       string
		limit       string
		expectedU15 int
		expectedU24 int
		expectedL15 int
		expectedL24 int
	}{
		{
			name:        "Normal usage",
			usage:       "160,1000",
			limit:       "200,2000",
			expectedU15: 160,
			expectedU24: 1000,
			expectedL15: 200,
			expectedL24: 2000,
		},
		{
			name:        "Zero usage",
			usage:       "0,0",
			limit:       "600,30000",
			expectedU15: 0,
			expectedU24: 0,
			expectedL15: 600,
			expectedL24: 30000,
		},
		{
			name:        "Whitespace handling",
			usage:       " 100 , 500 ",
			limit:       " 600 , 2000 ",
			expectedU15: 100,
			expectedU24: 500,
			expectedL15: 600,
			expectedL24: 2000,
		},
		{
			name:        "Empty strings",
			usage:       "",
			limit:       "",
			expectedU15: 0, // unchanged from default
			expectedU24: 0,
			expectedL15: 0,
			expectedL24: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rtLocal := NewRateLimitTracker(nil) // Fresh tracker for each test
			rtLocal.UpdateFromHeaders(tt.usage, tt.limit)

			u15, u24 := rtLocal.GetUsage()
			if u15 != tt.expectedU15 || u24 != tt.expectedU24 {
				t.Fatalf("usage: got (%d, %d), want (%d, %d)", u15, u24, tt.expectedU15, tt.expectedU24)
			}

			l15, l24 := rtLocal.GetLimits()
			if l15 != tt.expectedL15 || l24 != tt.expectedL24 {
				t.Fatalf("limits: got (%d, %d), want (%d, %d)", l15, l24, tt.expectedL15, tt.expectedL24)
			}
		})
	}
}

// TestRateLimitTrackerWarningLogged verifies warning is logged at 80% usage.
func TestRateLimitTrackerWarningLogged(t *testing.T) {
	// This test would need a custom logger to verify the warning was logged.
	// For now, just verify the tracker updates without panicking.
	rt := NewRateLimitTracker(nil)

	// 80% of 200 = 160
	rt.UpdateFromHeaders("160,1000", "200,2000")

	u15, _ := rt.GetUsage()
	if u15 != 160 {
		t.Fatalf("expected usage 160, got %d", u15)
	}
}

// TestRateLimitTrackerConcurrency verifies thread safety.
func TestRateLimitTrackerConcurrency(t *testing.T) {
	rt := NewRateLimitTracker(nil)

	done := make(chan bool)

	// Multiple writers
	for i := 0; i < 10; i++ {
		go func() {
			rt.UpdateFromHeaders("100,500", "200,2000")
			done <- true
		}()
	}

	// Multiple readers
	for i := 0; i < 10; i++ {
		go func() {
			_, _ = rt.GetUsage()
			_, _ = rt.GetLimits()
			done <- true
		}()
	}

	for i := 0; i < 20; i++ {
		<-done
	}
}

// TestCircuitBreakerStateClosed verifies initial state is closed.
func TestCircuitBreakerStateClosed(t *testing.T) {
	cb := NewCircuitBreaker(3, 1e9) // Long timeout to prevent state transitions
	if cb.GetState() != StateClosed {
		t.Fatalf("initial state should be closed, got %v", cb.GetState())
	}
}

// TestCircuitBreakerSuccessfulCall verifies state remains closed on success.
func TestCircuitBreakerSuccessfulCall(t *testing.T) {
	cb := NewCircuitBreaker(3, 1e9)

	err := cb.Call(func() error { return nil })
	if err != nil {
		t.Fatalf("successful call should not return error, got %v", err)
	}
	if cb.GetState() != StateClosed {
		t.Fatalf("state should remain closed after success, got %v", cb.GetState())
	}
}

// TestCircuitBreakerFailureThreshold verifies transition to open after N failures.
func TestCircuitBreakerFailureThreshold(t *testing.T) {
	cb := NewCircuitBreaker(3, 1e9)

	// 2 failures: should still be closed
	for i := 0; i < 2; i++ {
		_ = cb.Call(func() error { return fmt.Errorf("error") })
	}
	if cb.GetState() != StateClosed {
		t.Fatalf("after 2 failures, state should still be closed, got %v", cb.GetState())
	}

	// 3rd failure: should open
	_ = cb.Call(func() error { return fmt.Errorf("error") })
	if cb.GetState() != StateOpen {
		t.Fatalf("after 3 failures, state should be open, got %v", cb.GetState())
	}
}

// TestCircuitBreakerOpenRejectsImmediately verifies open state rejects requests.
func TestCircuitBreakerOpenRejectsImmediately(t *testing.T) {
	cb := NewCircuitBreaker(1, 1e9)

	// Open the circuit
	_ = cb.Call(func() error { return fmt.Errorf("error") })
	if cb.GetState() != StateOpen {
		t.Fatalf("circuit should be open")
	}

	// Next call should fail immediately without calling the function
	called := false
	err := cb.Call(func() error {
		called = true
		return nil
	})

	if called {
		t.Fatal("function should not be called when circuit is open")
	}
	if err == nil {
		t.Fatal("should return an error when circuit is open")
	}
}

// TestCircuitBreakerHalfOpenTransition verifies timeout-based transition to half-open.
func TestCircuitBreakerHalfOpenTransition(t *testing.T) {
	cb := NewCircuitBreaker(1, 100*time.Millisecond)

	// Open the circuit
	_ = cb.Call(func() error { return fmt.Errorf("error") })
	if cb.GetState() != StateOpen {
		t.Fatalf("circuit should be open")
	}

	// Wait for timeout
	time.Sleep(150 * time.Millisecond)

	// Next call should transition to half-open and attempt
	called := false
	err := cb.Call(func() error {
		called = true
		return nil
	})

	if !called {
		t.Fatal("function should be called in half-open state")
	}
	if err != nil {
		t.Fatalf("half-open probe with success should not return error, got %v", err)
	}
	if cb.GetState() != StateClosed {
		t.Fatalf("state should close after successful probe, got %v", cb.GetState())
	}
}

// TestCircuitBreakerReset verifies reset functionality.
func TestCircuitBreakerReset(t *testing.T) {
	cb := NewCircuitBreaker(1, 1e9)

	// Open the circuit
	_ = cb.Call(func() error { return fmt.Errorf("error") })
	if cb.GetState() != StateOpen {
		t.Fatalf("circuit should be open")
	}

	// Reset
	cb.Reset()
	if cb.GetState() != StateClosed {
		t.Fatalf("state should be closed after reset, got %v", cb.GetState())
	}
}
