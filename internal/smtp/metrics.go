package smtpd

import "context"

type MetricResult string

const (
	MetricAccepted MetricResult = "accepted"
	MetricRejected MetricResult = "rejected"
)

type MetricEvent struct {
	Stage        Stage
	Result       MetricResult
	RemoteAddr   string
	EnvelopeFrom string
	Recipient    string
	Recipients   []string
	Size         int64
	Error        string
}

type Metrics interface {
	ObserveSMTP(ctx context.Context, event MetricEvent)
	ObserveRFCNonCompliance(compliance RFCCompliance)
}

type noopMetrics struct{}

func (noopMetrics) ObserveSMTP(context.Context, MetricEvent)     {}
func (noopMetrics) ObserveRFCNonCompliance(RFCCompliance)        {}

func metricsOrDefault(metrics Metrics) Metrics {
	if metrics != nil {
		return metrics
	}
	return noopMetrics{}
}
