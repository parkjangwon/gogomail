package protocolmetrics

import (
	"net/http"
	"sync"
)

// RequestLimiter enforces request rate limits and connection limits with graceful rejection
type RequestLimiter struct {
	rateLimiter *RateLimiter
	metrics     *GatewayMetrics
	mu          sync.RWMutex
}

// NewRequestLimiter creates a new request limiter with metrics integration
func NewRequestLimiter(maxConnections int, maxRequestsPerSec float64, metrics *GatewayMetrics) *RequestLimiter {
	return &RequestLimiter{
		rateLimiter: NewRateLimiter(maxConnections, maxRequestsPerSec),
		metrics:     metrics,
	}
}

// CanAccept checks if a user can accept a new request without exceeding limits
func (rl *RequestLimiter) CanAccept(userID string) bool {
	if rl == nil || rl.rateLimiter == nil {
		return true
	}

	if !rl.rateLimiter.CanConnect(userID) {
		if rl.metrics != nil {
			rl.metrics.RecordConnectionLimitExceeded()
		}
		return false
	}
	return true
}

// ReleaseConnection marks a user connection as complete
func (rl *RequestLimiter) ReleaseConnection(userID string) {
	if rl == nil || rl.rateLimiter == nil {
		return
	}
	rl.rateLimiter.RecordDisconnection(userID)
}

// MiddlewareIMAPRateLimit wraps an IMAP ServeConn handler with rate limiting
func (rl *RequestLimiter) MiddlewareIMAPRateLimitHandler(next func(userID string) error) func(userID string) error {
	return func(userID string) error {
		if !rl.CanAccept(userID) {
			return ErrRateLimitExceeded{userID: userID}
		}
		defer rl.ReleaseConnection(userID)
		return next(userID)
	}
}

// MiddlewareHTTPRateLimitHandler wraps an HTTP handler with rate limiting
func (rl *RequestLimiter) MiddlewareHTTPRateLimitHandler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		userID := "unknown"
		if auth := r.Header.Get("X-User-ID"); auth != "" {
			userID = auth
		}

		if !rl.CanAccept(userID) {
			w.Header().Set("Retry-After", "60")
			http.Error(w, "rate limit exceeded", http.StatusTooManyRequests)
			return
		}
		defer rl.ReleaseConnection(userID)
		next.ServeHTTP(w, r)
	})
}

// ErrRateLimitExceeded is returned when rate limit is exceeded
type ErrRateLimitExceeded struct {
	userID string
}

func (e ErrRateLimitExceeded) Error() string {
	return "rate limit exceeded for user: " + e.userID
}

// GracefulDegradationConfig holds configuration for graceful degradation
type GracefulDegradationConfig struct {
	ConnectionWarningThreshold  float64 // 0.8 = warn at 80% capacity
	CommandErrorRateThreshold   float64 // 0.1 = warn at 10% error rate
	CommandLatencyThresholdMs   int64   // milliseconds
	EnableAdaptiveThrottling    bool    // slow down responses under load
	AdaptiveThrottleStartMs     int64   // start throttling at this latency
	AdaptiveThrottleMaxDelayMs  int64   // max throttle delay
}

// GracefulDegradation monitors metrics and enables adaptive throttling
type GracefulDegradation struct {
	config  GracefulDegradationConfig
	metrics *GatewayMetrics
	mu      sync.RWMutex
	status  string
}

// NewGracefulDegradation creates a new graceful degradation monitor
func NewGracefulDegradation(config GracefulDegradationConfig, metrics *GatewayMetrics) *GracefulDegradation {
	if config.ConnectionWarningThreshold <= 0 {
		config.ConnectionWarningThreshold = 0.8
	}
	if config.CommandErrorRateThreshold <= 0 {
		config.CommandErrorRateThreshold = 0.1
	}
	if config.CommandLatencyThresholdMs <= 0 {
		config.CommandLatencyThresholdMs = 1000 // 1 second
	}
	if config.AdaptiveThrottleStartMs <= 0 {
		config.AdaptiveThrottleStartMs = 500
	}
	if config.AdaptiveThrottleMaxDelayMs <= 0 {
		config.AdaptiveThrottleMaxDelayMs = 5000
	}

	return &GracefulDegradation{
		config:  config,
		metrics: metrics,
		status:  "healthy",
	}
}

// CheckStatus evaluates current system health
func (gd *GracefulDegradation) CheckStatus() string {
	if gd == nil || gd.metrics == nil {
		return "healthy"
	}

	snap := gd.metrics.Snapshot()

	// Check error rate
	if snap.ErrorRate > gd.config.CommandErrorRateThreshold {
		gd.mu.Lock()
		gd.status = "degraded"
		gd.mu.Unlock()
		return "degraded"
	}

	// Check latency
	if snap.AverageCommandDuration.Milliseconds() > gd.config.CommandLatencyThresholdMs {
		gd.mu.Lock()
		gd.status = "slow"
		gd.mu.Unlock()
		return "slow"
	}

	gd.mu.Lock()
	gd.status = "healthy"
	gd.mu.Unlock()
	return "healthy"
}

// GetStatus returns current degradation status
func (gd *GracefulDegradation) GetStatus() string {
	if gd == nil {
		return "healthy"
	}
	gd.mu.RLock()
	defer gd.mu.RUnlock()
	return gd.status
}

// GetAdaptiveThrottleDelay returns delay to apply based on current load
func (gd *GracefulDegradation) GetAdaptiveThrottleDelay() int64 {
	if gd == nil || !gd.config.EnableAdaptiveThrottling || gd.metrics == nil {
		return 0
	}

	snap := gd.metrics.Snapshot()
	latency := snap.AverageCommandDuration.Milliseconds()

	if latency < gd.config.AdaptiveThrottleStartMs {
		return 0
	}

	// Linear throttling: 0ms at start, max at 2x threshold
	ratio := float64(latency - gd.config.AdaptiveThrottleStartMs) / float64(gd.config.AdaptiveThrottleStartMs)
	if ratio > 1.0 {
		ratio = 1.0
	}

	return int64(ratio * float64(gd.config.AdaptiveThrottleMaxDelayMs))
}
