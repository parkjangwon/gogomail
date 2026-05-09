package milter

import (
	"context"
	"net"
	"testing"
	"time"
)

// TestPoolRespectsCircuitBreakerState verifies Get() fails when circuit is open.
func TestPoolRespectsCircuitBreakerState(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen failed: %v", err)
	}
	defer ln.Close()

	go func() {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		stubServer(t, conn, respAccept)
	}()

	// Create a pool with circuit breaker
	pool, err := NewPoolWithCircuitBreaker("tcp", ln.Addr().String(), 5*time.Second, 1, 1, 50*time.Millisecond)
	if err != nil {
		t.Fatalf("NewPoolWithCircuitBreaker failed: %v", err)
	}
	defer pool.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Force circuit open by recording failures
	pool.breaker.RecordFailure()
	if pool.breaker.State() != StateOpen {
		t.Fatalf("failed to open circuit")
	}

	// Get() should fail immediately (circuit open)
	_, err = pool.Get(ctx)
	if err == nil {
		t.Fatalf("Get() should fail when circuit is open")
	}
}

// TestPoolHealthCheckTransitionsHalfOpen verifies health check leads to HALF_OPEN.
func TestPoolHealthCheckTransitionsHalfOpen(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen failed: %v", err)
	}
	defer ln.Close()

	// Server that rejects first connection, succeeds on rest
	firstConn := true
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			if firstConn {
				firstConn = false
				conn.Close()
			} else {
				go stubServer(t, conn, respAccept)
			}
		}
	}()

	// Create pool with short health check interval
	pool, err := NewPoolWithCircuitBreaker("tcp", ln.Addr().String(), 5*time.Second, 1, 1, 50*time.Millisecond)
	if err != nil {
		t.Fatalf("NewPoolWithCircuitBreaker failed: %v", err)
	}
	defer pool.Close()

	// Force circuit open
	pool.breaker.RecordFailure()

	// Wait for health check to run and transition to HALF_OPEN
	// First health check at ~50ms triggers OPEN→HALF_OPEN and fails (connection closed)
	// Circuit should be HALF_OPEN
	time.Sleep(150 * time.Millisecond)

	if pool.breaker.State() != StateHalfOpen {
		t.Fatalf("health check should transition circuit to HALF_OPEN, got %v", pool.breaker.State())
	}
}

// TestPoolHealthCheckClosesCircuit verifies successful health check closes circuit.
func TestPoolHealthCheckClosesCircuit(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen failed: %v", err)
	}
	defer ln.Close()

	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			go stubServer(t, conn, respAccept)
		}
	}()

	pool, err := NewPoolWithCircuitBreaker("tcp", ln.Addr().String(), 5*time.Second, 1, 1, 50*time.Millisecond)
	if err != nil {
		t.Fatalf("NewPoolWithCircuitBreaker failed: %v", err)
	}
	defer pool.Close()

	// Open circuit
	pool.breaker.RecordFailure()

	// Wait for health check to run, transition to HALF_OPEN, and then succeed in closing
	// Health check interval is 50ms, so multiple checks should definitely occur in 500ms
	time.Sleep(500 * time.Millisecond)

	// Circuit should be back to CLOSED after successful health check
	if pool.breaker.State() != StateClosed {
		t.Fatalf("health check should close circuit after success, got %v", pool.breaker.State())
	}
}

// TestPoolHealthCheckMetrics verifies health check increments metrics.
func TestPoolHealthCheckMetrics(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen failed: %v", err)
	}
	defer ln.Close()

	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			go stubServer(t, conn, respAccept)
		}
	}()

	pool, err := NewPoolWithCircuitBreaker("tcp", ln.Addr().String(), 5*time.Second, 1, 1, 50*time.Millisecond)
	if err != nil {
		t.Fatalf("NewPoolWithCircuitBreaker failed: %v", err)
	}
	defer pool.Close()

	// Wait for at least one health check
	time.Sleep(150 * time.Millisecond)

	success, failure := pool.breaker.Metrics()
	if success == 0 {
		t.Fatalf("expected success metrics > 0, got %d", success)
	}
	if failure != 0 {
		t.Fatalf("expected no failures during health checks, got %d", failure)
	}
}
