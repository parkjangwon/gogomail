package httpapi

import "context"

// WebDAVMetricMethod identifies the WebDAV HTTP method.
type WebDAVMetricMethod string

const (
	WebDAVMethodPropfind WebDAVMetricMethod = "PROPFIND"
	WebDAVMethodMkcol    WebDAVMetricMethod = "MKCOL"
	WebDAVMethodGet      WebDAVMetricMethod = "GET"
	WebDAVMethodPut      WebDAVMetricMethod = "PUT"
	WebDAVMethodDelete   WebDAVMetricMethod = "DELETE"
	WebDAVMethodMove     WebDAVMetricMethod = "MOVE"
	WebDAVMethodCopy     WebDAVMetricMethod = "COPY"
	WebDAVMethodProppatch WebDAVMetricMethod = "PROPPATCH"
	WebDAVMethodLock     WebDAVMetricMethod = "LOCK"
	WebDAVMethodUnlock   WebDAVMetricMethod = "UNLOCK"
	WebDAVMethodOptions  WebDAVMetricMethod = "OPTIONS"
)

// WebDAVMetricResult describes the outcome of a WebDAV operation.
type WebDAVMetricResult string

const (
	WebDAVResultOK       WebDAVMetricResult = "ok"
	WebDAVResultError    WebDAVMetricResult = "error"
	WebDAVResultRejected WebDAVMetricResult = "rejected"
)

// WebDAVMetricEvent records a single WebDAV method execution.
type WebDAVMetricEvent struct {
	Method  WebDAVMetricMethod
	Result  WebDAVMetricResult
	UserID  string
	Path    string
	Error   string
}

// WebDAVMetrics observes WebDAV method executions.
type WebDAVMetrics interface {
	ObserveWebDAV(ctx context.Context, event WebDAVMetricEvent)
}

type noopWebDAVMetrics struct{}

func (noopWebDAVMetrics) ObserveWebDAV(context.Context, WebDAVMetricEvent) {}

func webdavMetricsOrDefault(metrics WebDAVMetrics) WebDAVMetrics {
	if metrics != nil {
		return metrics
	}
	return noopWebDAVMetrics{}
}