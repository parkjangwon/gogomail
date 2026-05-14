package smtpd

import (
	"context"
	"testing"
	"time"
)

func TestDeliveryCounterCanDeliver(t *testing.T) {
	dc := NewDeliveryCounter(2) // max 2 concurrent deliveries

	ctx := context.Background()

	// First delivery should succeed
	ok, err := dc.CanDeliver(ctx, "example.com")
	if !ok || err != nil {
		t.Errorf("first delivery failed: ok=%v err=%v", ok, err)
	}

	// Second delivery should succeed (at limit)
	ok, err = dc.CanDeliver(ctx, "example.com")
	if !ok || err != nil {
		t.Errorf("second delivery failed: ok=%v err=%v", ok, err)
	}

	// Third delivery should fail (exceeds limit)
	ok, err = dc.CanDeliver(ctx, "example.com")
	if ok {
		t.Error("third delivery should have been rejected due to concurrency limit")
	}

	// Record a success to free up a slot
	dc.RecordSuccess("example.com")

	// Now we should be able to deliver again
	ok, err = dc.CanDeliver(ctx, "example.com")
	if !ok || err != nil {
		t.Errorf("fourth delivery failed: ok=%v err=%v", ok, err)
	}
}

func TestDeliveryCounterDifferentDomains(t *testing.T) {
	dc := NewDeliveryCounter(1) // max 1 concurrent per domain

	ctx := context.Background()

	// Deliver to domain1
	ok, err := dc.CanDeliver(ctx, "domain1.com")
	if !ok || err != nil {
		t.Error("delivery to domain1 failed")
	}

	// Deliver to domain2 should succeed (different domain)
	ok, err = dc.CanDeliver(ctx, "domain2.com")
	if !ok || err != nil {
		t.Error("delivery to domain2 failed")
	}

	// Another delivery to domain1 should fail (limit exceeded)
	ok, err = dc.CanDeliver(ctx, "domain1.com")
	if ok {
		t.Error("second delivery to domain1 should have failed")
	}
}

func TestDeliveryCounterCircuitBreaker(t *testing.T) {
	dc := NewDeliveryCounter(5)
	dc.failureThreshold = 2 // open circuit after 2 failures

	ctx := context.Background()

	// Record 2 failures
	dc.RecordFailure("example.com")
	dc.RecordFailure("example.com")

	// Circuit should now be open
	ok, err := dc.CanDeliver(ctx, "example.com")
	if ok || err != ErrCircuitOpen {
		t.Errorf("circuit should be open: ok=%v err=%v", ok, err)
	}

	// Manually transition to half-open for testing
	dc.mu.Lock()
	counter := dc.domainCounters["example.com"]
	dc.mu.Unlock()

	if counter != nil {
		counter.mu.Lock()
		counter.circuitState = circuitHalfOpen
		counter.circuitOpenedAt = time.Now().Add(-60 * time.Second) // old enough to transition
		counter.mu.Unlock()
	}

	// Half-open allows one attempt
	ok, err = dc.CanDeliver(ctx, "example.com")
	if !ok {
		t.Error("half-open state should allow one delivery attempt")
	}

	// Success in half-open closes the circuit
	dc.RecordSuccess("example.com")

	ok, err = dc.CanDeliver(ctx, "example.com")
	if !ok || err != nil {
		t.Error("circuit should be closed after successful half-open delivery")
	}
}

func TestDeliveryCounterCircuitHalfOpenTimeout(t *testing.T) {
	dc := NewDeliveryCounter(5)
	dc.failureThreshold = 1
	dc.halfOpenTimeout = 10 * time.Millisecond

	ctx := context.Background()

	// Record a failure to open circuit
	dc.RecordFailure("example.com")

	// Circuit should be open
	ok, err := dc.CanDeliver(ctx, "example.com")
	if ok || err != ErrCircuitOpen {
		t.Error("circuit should be open immediately after threshold")
	}

	// Wait for half-open timeout
	time.Sleep(20 * time.Millisecond)

	// Now it should transition to half-open and allow one delivery
	ok, err = dc.CanDeliver(ctx, "example.com")
	if !ok {
		t.Error("circuit should transition to half-open and allow delivery")
	}
}

func TestDeliveryCounterGetStats(t *testing.T) {
	dc := NewDeliveryCounter(5)

	ctx := context.Background()

	// Deliver to a domain
	dc.CanDeliver(ctx, "example.com")

	stats := dc.GetStats("example.com")
	if stats["active_count"] != 1 {
		t.Errorf("expected 1 active delivery, got %v", stats["active_count"])
	}

	if stats["circuit_state"] != "closed" {
		t.Errorf("expected circuit closed, got %v", stats["circuit_state"])
	}
}

func TestDeliveryCounterGetStatsUnknownDomain(t *testing.T) {
	dc := NewDeliveryCounter(5)

	stats := dc.GetStats("unknown.com")
	if stats["active_count"] != 0 {
		t.Errorf("expected 0 active deliveries for unknown domain, got %v", stats["active_count"])
	}

	if stats["circuit_state"] != "unknown" {
		t.Errorf("expected unknown circuit state, got %v", stats["circuit_state"])
	}
}

func TestDeliveryCounterReset(t *testing.T) {
	dc := NewDeliveryCounter(5)

	ctx := context.Background()

	// Deliver and record a failure
	dc.CanDeliver(ctx, "example.com")
	dc.RecordFailure("example.com")

	// Reset
	dc.Reset()

	// Should be able to deliver again as if fresh
	ok, err := dc.CanDeliver(ctx, "example.com")
	if !ok || err != nil {
		t.Error("delivery should succeed after reset")
	}
}

func BenchmarkDeliveryCounterCanDeliver(b *testing.B) {
	dc := NewDeliveryCounter(10)
	ctx := context.Background()

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		dc.CanDeliver(ctx, "example.com")
		dc.RecordSuccess("example.com")
	}
}

func BenchmarkDeliveryCounterMultipleDomains(b *testing.B) {
	dc := NewDeliveryCounter(10)
	ctx := context.Background()
	domains := []string{"domain1.com", "domain2.com", "domain3.com", "domain4.com", "domain5.com"}

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		domain := domains[i%len(domains)]
		dc.CanDeliver(ctx, domain)
		dc.RecordSuccess(domain)
	}
}
