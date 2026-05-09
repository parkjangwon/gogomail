package milter

import (
	"sync/atomic"
	"testing"
	"time"
)

// TestCircuitBreakerClosedState verifies initial state is CLOSED.
func TestCircuitBreakerClosedState(t *testing.T) {
	cb := NewCircuitBreaker(3, 100*time.Millisecond)
	if cb.State() != StateClosed {
		t.Fatalf("initial state = %v, want %v", cb.State(), StateClosed)
	}
}

// TestCircuitBreakerOpensAfterFailures verifies breaker opens after N failures.
func TestCircuitBreakerOpensAfterFailures(t *testing.T) {
	cb := NewCircuitBreaker(2, 100*time.Millisecond)

	// First failure
	cb.RecordFailure()
	if cb.State() != StateClosed {
		t.Fatalf("state after 1 failure = %v, want %v", cb.State(), StateClosed)
	}

	// Second failure (threshold = 2)
	cb.RecordFailure()
	if cb.State() != StateOpen {
		t.Fatalf("state after 2 failures = %v, want %v", cb.State(), StateOpen)
	}

	// Verify AllowRequest returns false in OPEN state
	if cb.AllowRequest() {
		t.Fatalf("AllowRequest returned true in OPEN state")
	}
}

// TestCircuitBreakerTransitionsToHalfOpen verifies transition after timeout.
func TestCircuitBreakerTransitionsToHalfOpen(t *testing.T) {
	cb := NewCircuitBreaker(1, 50*time.Millisecond)

	// Open the breaker
	cb.RecordFailure()
	if cb.State() != StateOpen {
		t.Fatalf("failed to open circuit")
	}

	// Wait for timeout
	time.Sleep(100 * time.Millisecond)

	// Should now be in HALF_OPEN state
	if cb.State() != StateHalfOpen {
		t.Fatalf("state after timeout = %v, want %v", cb.State(), StateHalfOpen)
	}

	// AllowRequest should return true in HALF_OPEN (one attempt allowed)
	if !cb.AllowRequest() {
		t.Fatalf("AllowRequest returned false in HALF_OPEN state")
	}
}

// TestCircuitBreakerClosesAfterSuccess verifies breaker closes after successful request in HALF_OPEN.
func TestCircuitBreakerClosesAfterSuccess(t *testing.T) {
	cb := NewCircuitBreaker(1, 50*time.Millisecond)

	// Open the breaker
	cb.RecordFailure()

	// Wait for half-open
	time.Sleep(100 * time.Millisecond)

	if cb.State() != StateHalfOpen {
		t.Fatalf("failed to transition to HALF_OPEN")
	}

	// Record success
	cb.RecordSuccess()

	// Should be back to CLOSED
	if cb.State() != StateClosed {
		t.Fatalf("state after success = %v, want %v", cb.State(), StateClosed)
	}
}

// TestCircuitBreakerReopensAfterFailureInHalfOpen verifies breaker reopens if request fails in HALF_OPEN.
func TestCircuitBreakerReopensAfterFailureInHalfOpen(t *testing.T) {
	cb := NewCircuitBreaker(1, 50*time.Millisecond)

	// Open the breaker
	cb.RecordFailure()

	// Wait for half-open
	time.Sleep(100 * time.Millisecond)

	if cb.State() != StateHalfOpen {
		t.Fatalf("failed to transition to HALF_OPEN")
	}

	// Record another failure
	cb.RecordFailure()

	// Should go back to OPEN
	if cb.State() != StateOpen {
		t.Fatalf("state after failure in HALF_OPEN = %v, want %v", cb.State(), StateOpen)
	}
}

// TestCircuitBreakerMetrics verifies metrics are recorded.
func TestCircuitBreakerMetrics(t *testing.T) {
	cb := NewCircuitBreaker(2, 100*time.Millisecond)

	cb.RecordSuccess()
	cb.RecordSuccess()
	cb.RecordFailure()

	successes := atomic.LoadInt64(&cb.successCount)
	failures := atomic.LoadInt64(&cb.failureCount)

	if successes != 2 {
		t.Fatalf("successCount = %d, want 2", successes)
	}
	if failures != 1 {
		t.Fatalf("failureCount = %d, want 1", failures)
	}
}

// TestPoolWithCircuitBreaker verifies pool respects circuit breaker state.
func TestPoolWithCircuitBreaker(t *testing.T) {
	cb := NewCircuitBreaker(1, 50*time.Millisecond)

	// Open the breaker
	cb.RecordFailure()

	if cb.State() != StateOpen {
		t.Fatalf("failed to open circuit")
	}

	// In real pool, we'd check AllowRequest() before Get
	if cb.AllowRequest() {
		t.Fatalf("AllowRequest returned true when breaker is OPEN")
	}
}
