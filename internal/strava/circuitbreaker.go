package strava

import (
	"fmt"
	"sync"
	"time"
)

// State represents the circuit breaker state.
type State int

const (
	StateClosed State = iota
	StateOpen
	StateHalfOpen
)

func (s State) String() string {
	switch s {
	case StateClosed:
		return "closed"
	case StateOpen:
		return "open"
	case StateHalfOpen:
		return "half-open"
	default:
		return "unknown"
	}
}

// CircuitBreaker implements a 3-state circuit breaker pattern.
// Closed: requests proceed normally
// Open: requests fail immediately (after N consecutive failures)
// Half-Open: one probe request allowed; on success, transitions to Closed
type CircuitBreaker struct {
	mu sync.RWMutex

	state            State
	failureCount     int
	lastFailureTime  time.Time
	lastTransition   time.Time
	failureThreshold int
	timeout          time.Duration
}

// NewCircuitBreaker creates a new circuit breaker.
// failureThreshold: number of consecutive errors before opening
// timeout: duration to wait in open state before trying half-open
func NewCircuitBreaker(failureThreshold int, timeout time.Duration) *CircuitBreaker {
	return &CircuitBreaker{
		state:            StateClosed,
		failureThreshold: failureThreshold,
		timeout:          timeout,
		lastTransition:   time.Now(),
	}
}

// Call executes the given function and updates the circuit breaker state.
func (cb *CircuitBreaker) Call(fn func() error) error {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	state := cb.state
	now := time.Now()

	switch state {
	case StateClosed:
		// Attempt the call
		err := fn()
		if err != nil {
			cb.failureCount++
			cb.lastFailureTime = now
			if cb.failureCount >= cb.failureThreshold {
				cb.state = StateOpen
				cb.lastTransition = now
			}
			return err
		}
		// Success, reset
		cb.failureCount = 0
		return nil

	case StateOpen:
		// Check if timeout has elapsed
		if now.Sub(cb.lastTransition) >= cb.timeout {
			cb.state = StateHalfOpen
			cb.lastTransition = now
			// Fall through to attempt half-open probe
		} else {
			return fmt.Errorf("circuit breaker open (will retry in %v)", cb.timeout-now.Sub(cb.lastTransition))
		}
		fallthrough

	case StateHalfOpen:
		// Attempt the call (probe)
		err := fn()
		if err != nil {
			// Probe failed, reopen
			cb.failureCount = 1
			cb.lastFailureTime = now
			cb.state = StateOpen
			cb.lastTransition = now
			return fmt.Errorf("circuit breaker probe failed: %w", err)
		}
		// Probe succeeded, close circuit
		cb.failureCount = 0
		cb.state = StateClosed
		cb.lastTransition = now
		return nil
	}

	return fmt.Errorf("unknown circuit breaker state: %v", state)
}

// State returns the current state of the circuit breaker.
func (cb *CircuitBreaker) GetState() State {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	return cb.state
}

// Reset resets the circuit breaker to closed state.
func (cb *CircuitBreaker) Reset() {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	cb.state = StateClosed
	cb.failureCount = 0
	cb.lastTransition = time.Now()
}
