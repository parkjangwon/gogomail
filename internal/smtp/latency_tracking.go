package smtpd

import (
	"context"
	"sync"
	"time"
)

// PhaseLatency tracks the duration of each SMTP processing phase.
type PhaseLatency struct {
	StageName string        // e.g., "received", "spooled", "parsed", "stored"
	StartedAt time.Time     // when this phase started
	Duration  time.Duration // how long this phase took
}

// MessageTracing tracks the complete lifecycle and latency of a message through SMTP.
type MessageTracing struct {
	MessageID string           // unique message identifier
	EnvelopFrom string          // sender address
	Recipients []string         // recipient addresses
	ReceivedAt time.Time        // when message was received
	Phases    []PhaseLatency    // latency of each processing phase
	TotalLatency time.Duration  // total time from receipt to storage
	TraceID   string            // optional distributed trace ID
}

// LatencyTracker accumulates latency statistics for observability.
type LatencyTracker struct {
	mu              sync.RWMutex
	measurements    []*MessageTracing
	maxMeasurements int // keep sliding window of last N messages
	percentiles     map[int]time.Duration // p50, p95, p99
}

// NewLatencyTracker creates a new latency tracker.
func NewLatencyTracker(windowSize int) *LatencyTracker {
	if windowSize <= 0 {
		windowSize = 1000 // default
	}
	return &LatencyTracker{
		measurements:    make([]*MessageTracing, 0, windowSize),
		maxMeasurements: windowSize,
		percentiles:     make(map[int]time.Duration),
	}
}

// StartTracingMessage begins tracking a message's latency.
func (lt *LatencyTracker) StartTracingMessage(messageID string, envelopeFrom string, recipients []string) *MessageTracing {
	return &MessageTracing{
		MessageID:  messageID,
		EnvelopFrom: envelopeFrom,
		Recipients: recipients,
		ReceivedAt: time.Now(),
		Phases:    make([]PhaseLatency, 0),
	}
}

// RecordPhaseLatency records the duration of a processing phase.
func (trace *MessageTracing) RecordPhaseLatency(stageName string, duration time.Duration) {
	trace.Phases = append(trace.Phases, PhaseLatency{
		StageName: stageName,
		Duration:  duration,
	})
}

// RecordCompletion finalizes the trace and calculates total latency.
func (trace *MessageTracing) RecordCompletion() {
	var total time.Duration
	for _, phase := range trace.Phases {
		total += phase.Duration
	}
	trace.TotalLatency = total
}

// StoreTrace adds a completed trace to the tracker.
func (lt *LatencyTracker) StoreTrace(trace *MessageTracing) {
	lt.mu.Lock()
	defer lt.mu.Unlock()

	lt.measurements = append(lt.measurements, trace)

	// Maintain sliding window
	if len(lt.measurements) > lt.maxMeasurements {
		lt.measurements = lt.measurements[len(lt.measurements)-lt.maxMeasurements:]
	}

	// Recalculate percentiles
	lt.calculatePercentiles()
}

// calculatePercentiles computes p50, p95, p99 latencies.
func (lt *LatencyTracker) calculatePercentiles() {
	if len(lt.measurements) == 0 {
		return
	}

	// Simple percentile calculation from current window
	latencies := make([]time.Duration, 0, len(lt.measurements))
	for _, trace := range lt.measurements {
		latencies = append(latencies, trace.TotalLatency)
	}

	// Bubble sort for small windows
	for i := 0; i < len(latencies); i++ {
		for j := i + 1; j < len(latencies); j++ {
			if latencies[j] < latencies[i] {
				latencies[i], latencies[j] = latencies[j], latencies[i]
			}
		}
	}

	// Calculate percentiles
	lt.percentiles[50] = latencies[len(latencies)/2]
	lt.percentiles[95] = latencies[int(float64(len(latencies))*0.95)]
	lt.percentiles[99] = latencies[int(float64(len(latencies))*0.99)]
}

// GetStats returns current latency statistics.
func (lt *LatencyTracker) GetStats() map[string]interface{} {
	lt.mu.RLock()
	defer lt.mu.RUnlock()

	if len(lt.measurements) == 0 {
		return map[string]interface{}{
			"measurements": 0,
			"p50":          nil,
			"p95":          nil,
			"p99":          nil,
		}
	}

	// Calculate average
	var avgLatency time.Duration
	for _, trace := range lt.measurements {
		avgLatency += trace.TotalLatency
	}
	avgLatency /= time.Duration(len(lt.measurements))

	return map[string]interface{}{
		"measurements": len(lt.measurements),
		"avg_latency":  avgLatency.String(),
		"p50":          lt.percentiles[50].String(),
		"p95":          lt.percentiles[95].String(),
		"p99":          lt.percentiles[99].String(),
	}
}

// GetPhaseStats returns statistics by processing phase.
func (lt *LatencyTracker) GetPhaseStats() map[string]map[string]interface{} {
	lt.mu.RLock()
	defer lt.mu.RUnlock()

	phaseStats := make(map[string]map[string]interface{})

	phaseCount := make(map[string]int)
	phaseTotal := make(map[string]time.Duration)
	phaseMax := make(map[string]time.Duration)

	for _, trace := range lt.measurements {
		for _, phase := range trace.Phases {
			phaseCount[phase.StageName]++
			phaseTotal[phase.StageName] += phase.Duration

			if phase.Duration > phaseMax[phase.StageName] {
				phaseMax[phase.StageName] = phase.Duration
			}
		}
	}

	for stageName, count := range phaseCount {
		avg := phaseTotal[stageName] / time.Duration(count)
		phaseStats[stageName] = map[string]interface{}{
			"count":     count,
			"avg":       avg.String(),
			"max":       phaseMax[stageName].String(),
			"total":     phaseTotal[stageName].String(),
		}
	}

	return phaseStats
}

// Reset clears all measurements (useful for testing/restart).
func (lt *LatencyTracker) Reset() {
	lt.mu.Lock()
	defer lt.mu.Unlock()
	lt.measurements = lt.measurements[:0]
	lt.percentiles = make(map[int]time.Duration)
}

// ContextKey is used to store values in context.
type ContextKey string

const (
	// TracingContextKey stores the current MessageTracing in context.
	TracingContextKey ContextKey = "message_tracing"
)

// WithTracing adds a tracing context to support phase latency recording.
func WithTracing(ctx context.Context, trace *MessageTracing) context.Context {
	return context.WithValue(ctx, TracingContextKey, trace)
}

// TracingFromContext retrieves the MessageTracing from context.
func TracingFromContext(ctx context.Context) *MessageTracing {
	trace, ok := ctx.Value(TracingContextKey).(*MessageTracing)
	if !ok {
		return nil
	}
	return trace
}
