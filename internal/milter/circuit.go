package milter

import (
	"sync"
	"sync/atomic"
	"time"
)

// State represents the circuit breaker state.
type State int

const (
	StateClosed State = iota
	StateOpen
	StateHalfOpen
)

// String returns the string representation of the state.
func (s State) String() string {
	switch s {
	case StateClosed:
		return "CLOSED"
	case StateOpen:
		return "OPEN"
	case StateHalfOpen:
		return "HALF_OPEN"
	default:
		return "UNKNOWN"
	}
}

// CircuitBreaker implements a simple circuit breaker pattern.
// It tracks failures and prevents requests when the error rate is too high.
type CircuitBreaker struct {
	failureThreshold int64
	resetTimeout     time.Duration

	mu              sync.Mutex
	state           State
	consecutiveFail int64
	lastFailTime    time.Time

	// Metrics (atomic)
	successCount int64
	failureCount int64
}

// NewCircuitBreaker creates a new circuit breaker.
// failureThreshold: number of consecutive failures before opening the circuit.
// resetTimeout: duration to wait before attempting recovery (HALF_OPEN state).
func NewCircuitBreaker(failureThreshold int64, resetTimeout time.Duration) *CircuitBreaker {
	if failureThreshold <= 0 {
		failureThreshold = 1
	}
	return &CircuitBreaker{
		failureThreshold: failureThreshold,
		resetTimeout:     resetTimeout,
		state:            StateClosed,
	}
}

// State returns the current circuit breaker state.
func (cb *CircuitBreaker) State() State {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	// Transition from OPEN to HALF_OPEN if reset timeout has passed
	if cb.state == StateOpen {
		if time.Since(cb.lastFailTime) >= cb.resetTimeout {
			cb.state = StateHalfOpen
			cb.consecutiveFail = 0
		}
	}

	return cb.state
}

// AllowRequest returns true if a request is allowed.
// In CLOSED state: always allow.
// In OPEN state: never allow (fail fast).
// In HALF_OPEN state: allow one attempt.
func (cb *CircuitBreaker) AllowRequest() bool {
	state := cb.State() // This also handles OPEN → HALF_OPEN transition

	switch state {
	case StateClosed:
		return true
	case StateHalfOpen:
		// Allow the request attempt
		return true
	case StateOpen:
		return false
	default:
		return false
	}
}

// RecordSuccess records a successful request.
// If in HALF_OPEN state, transitions back to CLOSED.
func (cb *CircuitBreaker) RecordSuccess() {
	atomic.AddInt64(&cb.successCount, 1)

	cb.mu.Lock()
	defer cb.mu.Unlock()

	if cb.state == StateHalfOpen {
		// Recovery successful
		cb.state = StateClosed
		cb.consecutiveFail = 0
	} else if cb.state == StateClosed {
		// Reset consecutive failures on success
		cb.consecutiveFail = 0
	}
}

// RecordFailure records a failed request.
// If consecutive failures reach the threshold, opens the circuit.
// If in HALF_OPEN state, reopens the circuit immediately.
func (cb *CircuitBreaker) RecordFailure() {
	atomic.AddInt64(&cb.failureCount, 1)

	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.lastFailTime = time.Now()

	if cb.state == StateHalfOpen {
		// Recovery attempt failed, reopen circuit
		cb.state = StateOpen
		cb.consecutiveFail = 1
		return
	}

	if cb.state == StateClosed {
		cb.consecutiveFail++
		if cb.consecutiveFail >= cb.failureThreshold {
			cb.state = StateOpen
		}
	}
}

// Reset resets the circuit breaker to CLOSED state.
// Useful for testing or manual recovery.
func (cb *CircuitBreaker) Reset() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.state = StateClosed
	cb.consecutiveFail = 0
}

// Metrics returns the current success and failure counts.
func (cb *CircuitBreaker) Metrics() (success, failure int64) {
	return atomic.LoadInt64(&cb.successCount), atomic.LoadInt64(&cb.failureCount)
}
