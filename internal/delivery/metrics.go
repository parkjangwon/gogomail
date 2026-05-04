package delivery

import "context"

type MetricStage string

const (
	MetricQueuedDecoded      MetricStage = "queued_decoded"
	MetricTransportDelivered MetricStage = "transport_delivered"
	MetricTransportFailed    MetricStage = "transport_failed"
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
