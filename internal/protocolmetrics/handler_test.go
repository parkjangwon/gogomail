package protocolmetrics

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// TestMetricsHandlerServeMetrics tests Prometheus metrics output
func TestMetricsHandlerServeMetrics(t *testing.T) {
	m := NewGatewayMetrics()
	m.RecordConnect("user1")
	m.RecordCommand("user1", 100*time.Millisecond)

	handler := NewMetricsHandler(m)
	req := httptest.NewRequest("GET", "/metrics", nil)
	w := httptest.NewRecorder()

	handler.ServeMetrics(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	body, _ := io.ReadAll(w.Body)
	content := string(body)

	if !strings.Contains(content, "protocol_connected_users 1") {
		t.Errorf("expected connected_users metric in output")
	}
	if !strings.Contains(content, "protocol_commands_processed 1") {
		t.Errorf("expected commands_processed metric in output")
	}
	if !strings.Contains(content, "# TYPE protocol_connected_users gauge") {
		t.Errorf("expected TYPE directive for connected_users")
	}
}

// TestMetricsHandlerContentType tests Prometheus content type
func TestMetricsHandlerContentType(t *testing.T) {
	m := NewGatewayMetrics()
	handler := NewMetricsHandler(m)
	req := httptest.NewRequest("GET", "/metrics", nil)
	w := httptest.NewRecorder()

	handler.ServeMetrics(w, req)

	contentType := w.Header().Get("Content-Type")
	if contentType != "text/plain; version=0.0.4" {
		t.Errorf("expected Prometheus content-type, got %s", contentType)
	}
}

// TestMetricsHandlerMethodNotAllowed tests method validation
func TestMetricsHandlerMethodNotAllowed(t *testing.T) {
	m := NewGatewayMetrics()
	handler := NewMetricsHandler(m)

	for _, method := range []string{"POST", "PUT", "DELETE"} {
		req := httptest.NewRequest(method, "/metrics", nil)
		w := httptest.NewRecorder()
		handler.ServeMetrics(w, req)

		if w.Code != http.StatusMethodNotAllowed {
			t.Errorf("expected 405 for %s, got %d", method, w.Code)
		}
	}
}

// TestMetricsHandlerNilMetrics tests nil safety
func TestMetricsHandlerNilMetrics(t *testing.T) {
	handler := NewMetricsHandler(nil)
	req := httptest.NewRequest("GET", "/metrics", nil)
	w := httptest.NewRecorder()

	handler.ServeMetrics(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("expected 503 for nil metrics, got %d", w.Code)
	}
}

// TestMetricsHandlerServeHealth tests health endpoint
func TestMetricsHandlerServeHealth(t *testing.T) {
	handler := NewMetricsHandler(NewGatewayMetrics())
	req := httptest.NewRequest("GET", "/health", nil)
	w := httptest.NewRecorder()

	handler.ServeHealth(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	body, _ := io.ReadAll(w.Body)
	if !strings.Contains(string(body), "healthy") {
		t.Errorf("expected 'healthy' in health response")
	}
}

// TestMetricsHandlerServeReadiness tests readiness endpoint
func TestMetricsHandlerServeReadiness(t *testing.T) {
	m := NewGatewayMetrics()
	m.RecordConnect("user1")

	handler := NewMetricsHandler(m)
	req := httptest.NewRequest("GET", "/readiness", nil)
	w := httptest.NewRecorder()

	handler.ServeReadiness(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	body, _ := io.ReadAll(w.Body)
	if !strings.Contains(string(body), "ready") {
		t.Errorf("expected 'ready' in readiness response")
	}
}

// TestMetricsHandlerReadinessNotReady tests readiness when metrics unavailable
func TestMetricsHandlerReadinessNotReady(t *testing.T) {
	handler := NewMetricsHandler(nil)
	req := httptest.NewRequest("GET", "/readiness", nil)
	w := httptest.NewRecorder()

	handler.ServeReadiness(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("expected status 503 when not ready, got %d", w.Code)
	}
}

// TestMetricsHandlerPrometheusFormat tests complete Prometheus format
func TestMetricsHandlerPrometheusFormat(t *testing.T) {
	m := NewGatewayMetrics()
	m.RecordConnect("user1")
	m.RecordConnect("user2")
	m.RecordCommand("user1", 50*time.Millisecond)
	m.RecordCommand("user1", 150*time.Millisecond)
	m.RecordError("user1")

	handler := NewMetricsHandler(m)
	req := httptest.NewRequest("GET", "/metrics", nil)
	w := httptest.NewRecorder()

	handler.ServeMetrics(w, req)

	body, _ := io.ReadAll(w.Body)
	content := string(body)

	// Check for key Prometheus markers
	requiredMetrics := []string{
		"protocol_connected_users",
		"protocol_peak_connected_users",
		"protocol_total_connect_attempts",
		"protocol_commands_processed",
		"protocol_command_errors",
		"protocol_error_rate",
		"protocol_uptime_seconds",
	}

	for _, metric := range requiredMetrics {
		if !strings.Contains(content, metric) {
			t.Errorf("expected metric %s in output", metric)
		}
	}

	// Verify values
	if !strings.Contains(content, "protocol_connected_users 2") {
		t.Errorf("expected 2 connected users")
	}
	if !strings.Contains(content, "protocol_command_errors 1") {
		t.Errorf("expected 1 command error")
	}
}

// BenchmarkMetricsHandlerServeMetrics measures metrics export performance
func BenchmarkMetricsHandlerServeMetrics(b *testing.B) {
	m := NewGatewayMetrics()
	for i := 0; i < 1000; i++ {
		m.RecordCommand("user1", 100*time.Millisecond)
	}

	handler := NewMetricsHandler(m)

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		req := httptest.NewRequest("GET", "/metrics", nil)
		w := httptest.NewRecorder()
		handler.ServeMetrics(w, req)
	}
}
