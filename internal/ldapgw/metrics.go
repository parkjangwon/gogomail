package ldapgw

import "context"

type MetricResult string

const (
	MetricAccepted MetricResult = "accepted"
	MetricRejected MetricResult = "rejected"
)

type MetricEvent struct {
	Operation  string
	Result     MetricResult
	ResultCode int
	RemoteAddr string
	Entries    int
	Error      string
}

type Metrics interface {
	ObserveLDAP(ctx context.Context, event MetricEvent)
}

type noopMetrics struct{}

func (noopMetrics) ObserveLDAP(context.Context, MetricEvent) {}

func metricsOrDefault(metrics Metrics) Metrics {
	if metrics != nil {
		return metrics
	}
	return noopMetrics{}
}
