package httpapi

import (
	"net/http"
	"regexp"
	"strconv"
	"time"
)

// HTTPRequestObserver receives HTTP request metrics.
type HTTPRequestObserver interface {
	ObserveHTTPRequest(method, route, status string, dur time.Duration)
}

var (
	pathUUIDRe  = regexp.MustCompile(`[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}`)
	pathDigitRe = regexp.MustCompile(`(/)\d+(/)`)
	pathTailRe  = regexp.MustCompile(`(/)\d+$`)
)

// normalizePath replaces UUIDs and numeric path segments with {id} to prevent
// unbounded Prometheus label cardinality.
func normalizePath(path string) string {
	path = pathUUIDRe.ReplaceAllString(path, "{id}")
	path = pathDigitRe.ReplaceAllString(path, "${1}{id}${2}")
	path = pathTailRe.ReplaceAllString(path, "${1}{id}")
	return path
}

// MetricsMiddleware wraps an http.Handler and records request duration.
// If obs is nil it is a no-op.
func MetricsMiddleware(obs HTTPRequestObserver, next http.Handler) http.Handler {
	if obs == nil {
		return next
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rw := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rw, r)
		obs.ObserveHTTPRequest(r.Method, normalizePath(r.URL.Path), strconv.Itoa(rw.status), time.Since(start))
	})
}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *statusRecorder) WriteHeader(code int) {
	r.status = code
	r.ResponseWriter.WriteHeader(code)
}

// Flush propagates http.Flusher to support SSE and streaming responses.
func (r *statusRecorder) Flush() {
	if f, ok := r.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}
