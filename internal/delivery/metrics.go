package delivery

import (
	"context"
	"sync"
	"sync/atomic"
	"time"
)

type MetricStage string

const (
	MetricQueuedDecoded      MetricStage = "queued_decoded"
	MetricTransportDelivered MetricStage = "transport_delivered"
	MetricTransportFailed    MetricStage = "transport_failed"
	MetricDomainBackoff      MetricStage = "domain_backoff"
	MetricThrottled          MetricStage = "throttled"
	MetricRetryScheduled     MetricStage = "retry_scheduled"
	MetricRetryExhausted     MetricStage = "retry_exhausted"
)

type MetricResult string

const (
	MetricOK       MetricResult = "ok"
	MetricFailed   MetricResult = "failed"
	MetricBounced  MetricResult = "bounced"
	MetricDeferred MetricResult = "deferred"
)

type MetricEvent struct {
	Stage          MetricStage
	Result         MetricResult
	MessageID      string
	RFCMessageID   string
	DomainID       string
	Farm           string
	RoutePool      string
	RecipientCount int
	Error          string
}

type Metrics interface {
	ObserveDelivery(ctx context.Context, event MetricEvent)
}

type noopMetrics struct{}

func (noopMetrics) ObserveDelivery(context.Context, MetricEvent) {}

func metricsOrDefault(metrics Metrics) Metrics {
	if metrics != nil {
		return metrics
	}
	return noopMetrics{}
}

type PerformanceMetrics struct {
	poolHits            int64
	poolMisses          int64
	deliverySuccesses   int64
	deliveryFailures    int64
	recipientsProcessed int64
	totalSMTPTime       int64
	eventCounts         map[MetricStage]int64
	mu                  sync.RWMutex
	startTime           time.Time
}

func NewPerformanceMetrics() *PerformanceMetrics {
	return &PerformanceMetrics{
		eventCounts: make(map[MetricStage]int64),
		startTime:   time.Now(),
	}
}

func (m *PerformanceMetrics) RecordPoolHit() {
	if m != nil {
		atomic.AddInt64(&m.poolHits, 1)
	}
}

func (m *PerformanceMetrics) RecordPoolMiss() {
	if m != nil {
		atomic.AddInt64(&m.poolMisses, 1)
	}
}

func (m *PerformanceMetrics) RecordDeliverySuccess(recipients int) {
	if m != nil {
		atomic.AddInt64(&m.deliverySuccesses, 1)
		atomic.AddInt64(&m.recipientsProcessed, int64(recipients))
	}
}

func (m *PerformanceMetrics) RecordDeliveryFailure(recipients int) {
	if m != nil {
		atomic.AddInt64(&m.deliveryFailures, 1)
		atomic.AddInt64(&m.recipientsProcessed, int64(recipients))
	}
}

func (m *PerformanceMetrics) RecordSMTPTime(d time.Duration) {
	if m != nil {
		atomic.AddInt64(&m.totalSMTPTime, d.Nanoseconds())
	}
}

func (m *PerformanceMetrics) RecordEvent(stage MetricStage) {
	if m != nil {
		m.mu.Lock()
		m.eventCounts[stage]++
		m.mu.Unlock()
	}
}

func (m *PerformanceMetrics) ObserveDelivery(ctx context.Context, event MetricEvent) {
	if m == nil {
		return
	}
	m.RecordEvent(event.Stage)
}

type MetricSnapshot struct {
	PoolHits            int64
	PoolMisses          int64
	PoolHitRate         float64
	DeliverySuccesses   int64
	DeliveryFailures    int64
	DeliverySuccessRate float64
	RecipientsProcessed int64
	AverageSMTPTime     time.Duration
	Throughput          float64
	Elapsed             time.Duration
}

func (m *PerformanceMetrics) Snapshot() MetricSnapshot {
	if m == nil {
		return MetricSnapshot{}
	}

	hits := atomic.LoadInt64(&m.poolHits)
	misses := atomic.LoadInt64(&m.poolMisses)
	successes := atomic.LoadInt64(&m.deliverySuccesses)
	failures := atomic.LoadInt64(&m.deliveryFailures)
	recipients := atomic.LoadInt64(&m.recipientsProcessed)
	smtpTime := atomic.LoadInt64(&m.totalSMTPTime)

	elapsed := time.Since(m.startTime)
	totalDeliveries := successes + failures

	hitRate := 0.0
	if hits+misses > 0 {
		hitRate = float64(hits) / float64(hits+misses)
	}

	successRate := 0.0
	if totalDeliveries > 0 {
		successRate = float64(successes) / float64(totalDeliveries)
	}

	var avgSMTPTime time.Duration
	if successes > 0 {
		avgSMTPTime = time.Duration(smtpTime / successes)
	}

	throughput := 0.0
	if elapsed.Seconds() > 0 {
		throughput = float64(successes) / elapsed.Seconds()
	}

	return MetricSnapshot{
		PoolHits:            hits,
		PoolMisses:          misses,
		PoolHitRate:         hitRate,
		DeliverySuccesses:   successes,
		DeliveryFailures:    failures,
		DeliverySuccessRate: successRate,
		RecipientsProcessed: recipients,
		AverageSMTPTime:     avgSMTPTime,
		Throughput:          throughput,
		Elapsed:             elapsed,
	}
}
