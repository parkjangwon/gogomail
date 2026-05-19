package delivery

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gogomail/gogomail/internal/outbound"
)

// mockTransport is a Transport that returns a pre-configured error or nil.
type mockTransport struct {
	mu      sync.Mutex
	callErr error
	calls   int
}

func (m *mockTransport) Deliver(_ context.Context, _ Job) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.calls++
	return m.callErr
}

func buildTestJob(domain string) Job {
	return Job{
		QueuedMessage: QueuedMessage{
			Event:     "mail.queued",
			MessageID: "cbtest-msg-id",
			To:        []outbound.Address{{Email: "user@" + domain}},
		},
	}
}

func TestCircuitBreakerTransport_OpenOnFailures(t *testing.T) {
	mock := &mockTransport{callErr: errors.New("connection refused")}
	cb := NewCircuitBreakerTransport(mock, 10)
	cb.FailureThreshold = 3
	cb.HalfOpenTimeout = 100 * time.Millisecond

	ctx := context.Background()
	job := buildTestJob("example.com")

	// First 3 failures should trip the breaker
	for i := 0; i < 3; i++ {
		err := cb.Deliver(ctx, job)
		if err == nil {
			t.Fatalf("expected error on attempt %d, got nil", i)
		}
	}

	// Next call should return ErrCircuitOpen
	err := cb.Deliver(ctx, job)
	if !errors.Is(err, ErrCircuitOpen) {
		t.Errorf("expected ErrCircuitOpen, got %v", err)
	}
}

func TestCircuitBreakerTransport_HalfOpen(t *testing.T) {
	mock := &mockTransport{callErr: errors.New("unavailable")}
	cb := NewCircuitBreakerTransport(mock, 10)
	cb.FailureThreshold = 2
	cb.HalfOpenTimeout = 100 * time.Millisecond

	ctx := context.Background()
	job := buildTestJob("halfopen.example.com")

	// Trip the breaker
	for i := 0; i < 2; i++ {
		cb.Deliver(ctx, job)
	}

	// Verify open
	if err := cb.Deliver(ctx, job); !errors.Is(err, ErrCircuitOpen) {
		t.Fatalf("expected ErrCircuitOpen after tripping, got %v", err)
	}

	// Wait for half-open timeout
	time.Sleep(150 * time.Millisecond)

	// Allow the probe — set mock to succeed
	mock.mu.Lock()
	mock.callErr = nil
	mock.mu.Unlock()

	// The probe call should succeed and close the breaker
	if err := cb.Deliver(ctx, job); err != nil {
		t.Errorf("expected success in half-open state, got %v", err)
	}

	// Subsequent calls should also succeed (breaker closed)
	if err := cb.Deliver(ctx, job); err != nil {
		t.Errorf("expected success after breaker reset, got %v", err)
	}
}

func TestCircuitBreakerTransport_ConcurrencyLimit(t *testing.T) {
	// Use a transport that blocks until released
	gate := make(chan struct{})
	blocker := &blockingTransport{gate: gate}

	cb := NewCircuitBreakerTransport(blocker, 2) // max 2 concurrent
	cb.FailureThreshold = 100

	ctx := context.Background()
	job := buildTestJob("conc.example.com")

	var wg sync.WaitGroup
	var limitErrCount int64

	// Launch 5 goroutines; only 2 should get through, 3 should be rejected
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			err := cb.Deliver(ctx, job)
			if errors.Is(err, ErrConcurrencyLimitExceeded) {
				atomic.AddInt64(&limitErrCount, 1)
			}
		}()
	}

	// Give goroutines time to enter Deliver
	time.Sleep(50 * time.Millisecond)

	// Release all blocked goroutines
	close(gate)
	wg.Wait()

	if atomic.LoadInt64(&limitErrCount) == 0 {
		t.Error("expected at least one ErrConcurrencyLimitExceeded with limit=2 and 5 goroutines")
	}
}

// blockingTransport blocks until its gate channel is closed.
type blockingTransport struct {
	gate chan struct{}
}

func (b *blockingTransport) Deliver(ctx context.Context, _ Job) error {
	select {
	case <-b.gate:
	case <-ctx.Done():
	}
	return nil
}
