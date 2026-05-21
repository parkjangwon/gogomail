package httpapi

import (
	"net/http"
	"strconv"
	"time"
)

// HTTPRequestObserver receives HTTP request metrics.
type HTTPRequestObserver interface {
	ObserveHTTPRequest(method, route, status string, dur time.Duration)
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
		obs.ObserveHTTPRequest(r.Method, r.URL.Path, strconv.Itoa(rw.status), time.Since(start))
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
