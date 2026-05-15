package protocolmetrics

import (
	"fmt"
	"net/http"
	"time"
)

// MetricsHandler provides HTTP endpoints for metrics and health
type MetricsHandler struct {
	metrics *GatewayMetrics
}

// NewMetricsHandler creates a new metrics HTTP handler
func NewMetricsHandler(metrics *GatewayMetrics) *MetricsHandler {
	return &MetricsHandler{metrics: metrics}
}

// ServeMetrics handles /metrics endpoint (Prometheus format)
func (mh *MetricsHandler) ServeMetrics(w http.ResponseWriter, r *http.Request) {
	if mh == nil || mh.metrics == nil {
		http.Error(w, "metrics not available", http.StatusServiceUnavailable)
		return
	}

	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	snap := mh.metrics.Snapshot()

	w.Header().Set("Content-Type", "text/plain; version=0.0.4")
	fmt.Fprintf(w, "# HELP protocol_connected_users Current number of connected users\n")
	fmt.Fprintf(w, "# TYPE protocol_connected_users gauge\n")
	fmt.Fprintf(w, "protocol_connected_users %d\n\n", snap.ConnectedUsers)

	fmt.Fprintf(w, "# HELP protocol_peak_connected_users Peak number of connected users\n")
	fmt.Fprintf(w, "# TYPE protocol_peak_connected_users gauge\n")
	fmt.Fprintf(w, "protocol_peak_connected_users %d\n\n", snap.PeakConnectedUsers)

	fmt.Fprintf(w, "# HELP protocol_total_connect_attempts Total connection attempts\n")
	fmt.Fprintf(w, "# TYPE protocol_total_connect_attempts counter\n")
	fmt.Fprintf(w, "protocol_total_connect_attempts %d\n\n", snap.TotalConnectAttempts)

	fmt.Fprintf(w, "# HELP protocol_total_disconnects Total disconnections\n")
	fmt.Fprintf(w, "# TYPE protocol_total_disconnects counter\n")
	fmt.Fprintf(w, "protocol_total_disconnects %d\n\n", snap.TotalDisconnects)

	fmt.Fprintf(w, "# HELP protocol_commands_processed Total commands processed\n")
	fmt.Fprintf(w, "# TYPE protocol_commands_processed counter\n")
	fmt.Fprintf(w, "protocol_commands_processed %d\n\n", snap.CommandsProcessed)

	fmt.Fprintf(w, "# HELP protocol_command_errors Total command errors\n")
	fmt.Fprintf(w, "# TYPE protocol_command_errors counter\n")
	fmt.Fprintf(w, "protocol_command_errors %d\n\n", snap.CommandErrors)

	fmt.Fprintf(w, "# HELP protocol_average_command_duration_ms Average command duration in milliseconds\n")
	fmt.Fprintf(w, "# TYPE protocol_average_command_duration_ms gauge\n")
	fmt.Fprintf(w, "protocol_average_command_duration_ms %d\n\n", snap.AverageCommandDuration.Milliseconds())

	fmt.Fprintf(w, "# HELP protocol_error_rate Command error rate (0-1)\n")
	fmt.Fprintf(w, "# TYPE protocol_error_rate gauge\n")
	fmt.Fprintf(w, "protocol_error_rate %.4f\n\n", snap.ErrorRate)

	fmt.Fprintf(w, "# HELP protocol_rate_limit_exceeded Rate limit violations\n")
	fmt.Fprintf(w, "# TYPE protocol_rate_limit_exceeded counter\n")
	fmt.Fprintf(w, "protocol_rate_limit_exceeded %d\n\n", snap.RateLimitExceeded)

	fmt.Fprintf(w, "# HELP protocol_connection_limit_exceeded Connection limit violations\n")
	fmt.Fprintf(w, "# TYPE protocol_connection_limit_exceeded counter\n")
	fmt.Fprintf(w, "protocol_connection_limit_exceeded %d\n\n", snap.ConnectionLimitExceeded)

	fmt.Fprintf(w, "# HELP protocol_uptime_seconds Uptime in seconds\n")
	fmt.Fprintf(w, "# TYPE protocol_uptime_seconds gauge\n")
	fmt.Fprintf(w, "protocol_uptime_seconds %g\n\n", snap.Uptime.Seconds())
}

// ServeHealth handles /health endpoint (liveness + readiness)
func (mh *MetricsHandler) ServeHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, `{"status":"healthy","timestamp":"%s"}`, time.Now().UTC().Format(time.RFC3339Nano))
}

// ServeReadiness handles /readiness endpoint
func (mh *MetricsHandler) ServeReadiness(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Readiness check: ensure we have metrics and can snapshot
	if mh == nil || mh.metrics == nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusServiceUnavailable)
		fmt.Fprintf(w, `{"status":"not_ready","reason":"metrics_unavailable"}`)
		return
	}

	snap := mh.metrics.Snapshot()
	if snap.ConnectedUsers < 0 {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusServiceUnavailable)
		fmt.Fprintf(w, `{"status":"not_ready","reason":"invalid_state"}`)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, `{"status":"ready","uptime_seconds":%g}`, snap.Uptime.Seconds())
}
