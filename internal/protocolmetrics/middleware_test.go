package protocolmetrics

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// TestRequestLimiterCanAccept tests request acceptance
func TestRequestLimiterCanAccept(t *testing.T) {
	limiter := NewRequestLimiter(2, 10.0, NewGatewayMetrics())

	if !limiter.CanAccept("user1") {
		t.Error("expected first request to succeed")
	}
	if !limiter.CanAccept("user1") {
		t.Error("expected second request to succeed")
	}
	if limiter.CanAccept("user1") {
		t.Error("expected third request to fail (limit is 2)")
	}
}

// TestRequestLimiterRecordsMetrics tests metrics recording
func TestRequestLimiterRecordsMetrics(t *testing.T) {
	m := NewGatewayMetrics()
	limiter := NewRequestLimiter(1, 10.0, m)

	if !limiter.CanAccept("user1") {
		t.Error("expected first request to succeed")
	}
	if limiter.CanAccept("user1") {
		t.Error("expected second request to fail")
	}

	snap := m.Snapshot()
	if snap.ConnectionLimitExceeded != 1 {
		t.Errorf("expected 1 connection limit exceeded, got %d", snap.ConnectionLimitExceeded)
	}
}

// TestRequestLimiterReleaseConnection tests connection release
func TestRequestLimiterReleaseConnection(t *testing.T) {
	limiter := NewRequestLimiter(1, 10.0, NewGatewayMetrics())

	if !limiter.CanAccept("user1") {
		t.Error("expected first request to succeed")
	}
	if limiter.CanAccept("user1") {
		t.Error("expected second request to fail before release")
	}

	limiter.ReleaseConnection("user1")

	if !limiter.CanAccept("user1") {
		t.Error("expected request to succeed after release")
	}
}

// TestRequestLimiterNilSafety tests nil pointer handling
func TestRequestLimiterNilSafety(t *testing.T) {
	var limiter *RequestLimiter

	// Should not panic
	if !limiter.CanAccept("user1") {
		t.Error("nil limiter should accept requests")
	}
	limiter.ReleaseConnection("user1")
}

// TestMiddlewareHTTPRateLimitHandler tests HTTP middleware with unlimited connections
func TestMiddlewareHTTPRateLimitHandler(t *testing.T) {
	limitless := NewRequestLimiter(0, 10.0, NewGatewayMetrics()) // 0 = unlimited
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	})
	middleware := limitless.MiddlewareHTTPRateLimitHandler(handler)

	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()
	middleware.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

// TestMiddlewareHTTPRateLimitHandlerExceeded tests middleware rejection
func TestMiddlewareHTTPRateLimitHandlerExceeded(t *testing.T) {
	m := NewGatewayMetrics()
	limited := NewRequestLimiter(1, 10.0, m)

	// Exhaust the limit manually
	if !limited.CanAccept("user1") {
		t.Error("expected first accept to succeed")
	}

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	middleware := limited.MiddlewareHTTPRateLimitHandler(handler)

	// Second request with same user should be rejected
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("X-User-ID", "user1")
	w := httptest.NewRecorder()
	middleware.ServeHTTP(w, req)

	// Should be rejected (limit is 1, user1 already has 1 from manual CanAccept)
	if w.Code != http.StatusTooManyRequests {
		t.Errorf("expected 429, got %d", w.Code)
	}

	// Check Retry-After header
	if w.Header().Get("Retry-After") == "" {
		t.Error("expected Retry-After header")
	}

	// Verify metrics
	snap := m.Snapshot()
	if snap.ConnectionLimitExceeded != 1 {
		t.Errorf("expected 1 connection limit exceeded, got %d", snap.ConnectionLimitExceeded)
	}
}

// TestGracefulDegradationCheckStatus tests status checking
func TestGracefulDegradationCheckStatus(t *testing.T) {
	config := GracefulDegradationConfig{
		CommandErrorRateThreshold: 0.1,
		CommandLatencyThresholdMs: 100,
	}
	m := NewGatewayMetrics()
	gd := NewGracefulDegradation(config, m)

	// Initial status should be healthy
	if status := gd.CheckStatus(); status != "healthy" {
		t.Errorf("expected healthy, got %s", status)
	}

	// Add error to trigger degraded status
	m.RecordCommand("user1", 10*time.Millisecond)
	m.RecordError("user1")
	m.RecordCommand("user1", 10*time.Millisecond)
	m.RecordError("user1")
	m.RecordCommand("user1", 10*time.Millisecond)

	// Error rate is now 2/3 = 0.67 > 0.1 threshold
	if status := gd.CheckStatus(); status != "degraded" {
		t.Errorf("expected degraded, got %s", status)
	}
}

// TestGracefulDegradationLatencyStatus tests latency-based degradation
func TestGracefulDegradationLatencyStatus(t *testing.T) {
	config := GracefulDegradationConfig{
		CommandErrorRateThreshold: 0.1,
		CommandLatencyThresholdMs: 100,
	}
	m := NewGatewayMetrics()
	gd := NewGracefulDegradation(config, m)

	// Add slow commands
	m.RecordCommand("user1", 200*time.Millisecond)
	m.RecordCommand("user1", 200*time.Millisecond)
	m.RecordCommand("user1", 200*time.Millisecond)

	// Avg latency is 200ms > 100ms threshold
	if status := gd.CheckStatus(); status != "slow" {
		t.Errorf("expected slow, got %s", status)
	}
}

// TestGracefulDegradationGetStatus tests status getter
func TestGracefulDegradationGetStatus(t *testing.T) {
	m := NewGatewayMetrics()
	gd := NewGracefulDegradation(GracefulDegradationConfig{}, m)

	status := gd.GetStatus()
	if status != "healthy" {
		t.Errorf("expected healthy, got %s", status)
	}
}

// TestGracefulDegradationAdaptiveThrottle tests adaptive throttling
func TestGracefulDegradationAdaptiveThrottle(t *testing.T) {
	config := GracefulDegradationConfig{
		EnableAdaptiveThrottling:  true,
		AdaptiveThrottleStartMs:   100,
		AdaptiveThrottleMaxDelayMs: 1000,
	}
	m := NewGatewayMetrics()
	gd := NewGracefulDegradation(config, m)

	// No throttle when latency is low
	if delay := gd.GetAdaptiveThrottleDelay(); delay != 0 {
		t.Errorf("expected 0 delay for low latency, got %d", delay)
	}

	// Add moderate latency
	m.RecordCommand("user1", 150*time.Millisecond) // 150ms > 100ms start threshold

	delay := gd.GetAdaptiveThrottleDelay()
	if delay < 1 || delay > 1000 {
		t.Errorf("expected delay between 1-1000ms, got %d", delay)
	}

	// Add high latency
	m.RecordCommand("user1", 300*time.Millisecond) // avg now ~225ms
	delay = gd.GetAdaptiveThrottleDelay()
	if delay > 1000 {
		t.Errorf("expected delay capped at 1000ms, got %d", delay)
	}
}

// TestGracefulDegradationNilSafety tests nil safety
func TestGracefulDegradationNilSafety(t *testing.T) {
	var gd *GracefulDegradation

	// Should not panic
	status := gd.CheckStatus()
	if status != "healthy" {
		t.Errorf("expected healthy from nil GracefulDegradation, got %s", status)
	}

	status = gd.GetStatus()
	if status != "healthy" {
		t.Errorf("expected healthy from nil GracefulDegradation, got %s", status)
	}

	delay := gd.GetAdaptiveThrottleDelay()
	if delay != 0 {
		t.Errorf("expected 0 delay from nil GracefulDegradation, got %d", delay)
	}
}

// BenchmarkRequestLimiterCanAccept measures rate limiter performance
func BenchmarkRequestLimiterCanAccept(b *testing.B) {
	limiter := NewRequestLimiter(1000, 10000.0, NewGatewayMetrics())

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		limiter.CanAccept("user1")
	}
}

// BenchmarkGracefulDegradationCheckStatus measures degradation check performance
func BenchmarkGracefulDegradationCheckStatus(b *testing.B) {
	m := NewGatewayMetrics()
	for i := 0; i < 100; i++ {
		m.RecordCommand("user1", 100*time.Millisecond)
	}

	gd := NewGracefulDegradation(GracefulDegradationConfig{}, m)

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		gd.CheckStatus()
	}
}
