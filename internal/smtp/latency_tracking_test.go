package smtpd

import (
	"context"
	"testing"
	"time"
)

func TestMessageTracingPhaseLatency(t *testing.T) {
	trace := &MessageTracing{
		MessageID:   "test-123",
		EnvelopFrom: "sender@example.com",
		Recipients:  []string{"user@example.com"},
		ReceivedAt:  time.Now(),
	}

	// Record phases
	trace.RecordPhaseLatency("received", 10*time.Millisecond)
	trace.RecordPhaseLatency("parsed", 20*time.Millisecond)
	trace.RecordPhaseLatency("stored", 30*time.Millisecond)

	if len(trace.Phases) != 3 {
		t.Errorf("expected 3 phases, got %d", len(trace.Phases))
	}

	if trace.Phases[0].StageName != "received" {
		t.Errorf("expected first phase to be 'received', got %s", trace.Phases[0].StageName)
	}

	trace.RecordCompletion()

	expectedTotal := 60 * time.Millisecond
	if trace.TotalLatency != expectedTotal {
		t.Errorf("expected total latency %v, got %v", expectedTotal, trace.TotalLatency)
	}
}

func TestLatencyTrackerStoreTrace(t *testing.T) {
	tracker := NewLatencyTracker(100)

	// Create and store traces
	for i := 0; i < 10; i++ {
		trace := &MessageTracing{
			MessageID:   "msg-" + string(rune(i)),
			EnvelopFrom: "sender@example.com",
			Recipients:  []string{"user@example.com"},
			ReceivedAt:  time.Now(),
		}
		trace.RecordPhaseLatency("phase1", 10*time.Millisecond)
		trace.RecordCompletion()
		tracker.StoreTrace(trace)
	}

	stats := tracker.GetStats()
	if stats["measurements"] != 10 {
		t.Errorf("expected 10 measurements, got %v", stats["measurements"])
	}
}

func TestLatencyTrackerPercentiles(t *testing.T) {
	tracker := NewLatencyTracker(100)

	// Create traces with varying latencies
	latencies := []time.Duration{
		10 * time.Millisecond,
		20 * time.Millisecond,
		30 * time.Millisecond,
		40 * time.Millisecond,
		50 * time.Millisecond,
		60 * time.Millisecond,
		70 * time.Millisecond,
		80 * time.Millisecond,
		90 * time.Millisecond,
		100 * time.Millisecond,
	}

	for i, latency := range latencies {
		trace := &MessageTracing{
			MessageID:   "msg-" + string(rune(i)),
			EnvelopFrom: "sender@example.com",
			Recipients:  []string{"user@example.com"},
			ReceivedAt:  time.Now(),
		}
		trace.RecordPhaseLatency("phase1", latency)
		trace.RecordCompletion()
		tracker.StoreTrace(trace)
	}

	stats := tracker.GetStats()
	if stats["p50"] == nil || stats["p95"] == nil || stats["p99"] == nil {
		t.Error("expected percentiles to be calculated")
	}
}

func TestLatencyTrackerSlidingWindow(t *testing.T) {
	tracker := NewLatencyTracker(5) // max 5 measurements

	// Add 10 traces
	for i := 0; i < 10; i++ {
		trace := &MessageTracing{
			MessageID:   "msg-" + string(rune(i)),
			EnvelopFrom: "sender@example.com",
			Recipients:  []string{"user@example.com"},
			ReceivedAt:  time.Now(),
		}
		trace.RecordPhaseLatency("phase1", time.Duration(i*10)*time.Millisecond)
		trace.RecordCompletion()
		tracker.StoreTrace(trace)
	}

	stats := tracker.GetStats()
	if stats["measurements"] != 5 {
		t.Errorf("expected 5 measurements in sliding window, got %v", stats["measurements"])
	}
}

func TestLatencyTrackerPhaseStats(t *testing.T) {
	tracker := NewLatencyTracker(100)

	trace := &MessageTracing{
		MessageID:   "test-1",
		EnvelopFrom: "sender@example.com",
		Recipients:  []string{"user@example.com"},
		ReceivedAt:  time.Now(),
	}
	trace.RecordPhaseLatency("received", 10*time.Millisecond)
	trace.RecordPhaseLatency("parsed", 20*time.Millisecond)
	trace.RecordPhaseLatency("stored", 15*time.Millisecond)
	trace.RecordCompletion()

	tracker.StoreTrace(trace)

	phaseStats := tracker.GetPhaseStats()

	if len(phaseStats) != 3 {
		t.Errorf("expected 3 phases, got %d", len(phaseStats))
	}

	if phaseStats["received"]["count"] != 1 {
		t.Error("expected 1 'received' phase")
	}

	if phaseStats["parsed"]["count"] != 1 {
		t.Error("expected 1 'parsed' phase")
	}
}

func TestLatencyTrackerEmptyStats(t *testing.T) {
	tracker := NewLatencyTracker(100)

	stats := tracker.GetStats()
	if stats["measurements"] != 0 {
		t.Errorf("expected 0 measurements, got %v", stats["measurements"])
	}

	if stats["p50"] != nil || stats["p95"] != nil || stats["p99"] != nil {
		t.Error("expected nil percentiles for empty tracker")
	}
}

func TestTracingContext(t *testing.T) {
	trace := &MessageTracing{
		MessageID:   "test-1",
		EnvelopFrom: "sender@example.com",
	}

	ctx := context.Background()
	ctx = WithTracing(ctx, trace)

	retrieved := TracingFromContext(ctx)
	if retrieved == nil {
		t.Fatal("expected to retrieve tracing from context")
	}

	if retrieved.MessageID != "test-1" {
		t.Errorf("expected MessageID test-1, got %s", retrieved.MessageID)
	}
}

func TestTracingContextNotPresent(t *testing.T) {
	ctx := context.Background()
	retrieved := TracingFromContext(ctx)

	if retrieved != nil {
		t.Error("expected nil when tracing not in context")
	}
}

func TestLatencyTrackerReset(t *testing.T) {
	tracker := NewLatencyTracker(100)

	// Add some traces
	for i := 0; i < 5; i++ {
		trace := &MessageTracing{
			MessageID:   "msg-" + string(rune(i)),
			EnvelopFrom: "sender@example.com",
		}
		trace.RecordPhaseLatency("phase1", 10*time.Millisecond)
		trace.RecordCompletion()
		tracker.StoreTrace(trace)
	}

	stats := tracker.GetStats()
	if stats["measurements"] != 5 {
		t.Errorf("expected 5 measurements before reset, got %v", stats["measurements"])
	}

	// Reset
	tracker.Reset()

	stats = tracker.GetStats()
	if stats["measurements"] != 0 {
		t.Errorf("expected 0 measurements after reset, got %v", stats["measurements"])
	}
}

func BenchmarkMessageTracingPhaseRecording(b *testing.B) {
	trace := &MessageTracing{
		MessageID:   "test-123",
		EnvelopFrom: "sender@example.com",
		Recipients:  []string{"user@example.com"},
		ReceivedAt:  time.Now(),
	}

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		trace.RecordPhaseLatency("phase", 10*time.Millisecond)
	}
}

func BenchmarkLatencyTrackerStoreTrace(b *testing.B) {
	tracker := NewLatencyTracker(1000)

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		trace := &MessageTracing{
			MessageID:   "msg-" + string(rune(i)),
			EnvelopFrom: "sender@example.com",
		}
		trace.RecordPhaseLatency("phase1", 10*time.Millisecond)
		trace.RecordCompletion()
		tracker.StoreTrace(trace)
	}
}

func BenchmarkTracingWithContext(b *testing.B) {
	trace := &MessageTracing{
		MessageID:   "test-123",
		EnvelopFrom: "sender@example.com",
	}

	ctx := context.Background()

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		ctx = WithTracing(ctx, trace)
		_ = TracingFromContext(ctx)
	}
}
