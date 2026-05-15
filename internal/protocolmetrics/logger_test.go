package protocolmetrics

import (
	"log/slog"
	"testing"
	"time"
)

// TestLoggerCreation tests logger initialization
func TestLoggerCreation(t *testing.T) {
	l := NewLogger()
	if l == nil {
		t.Error("expected logger to be created")
	}
	if l.slog == nil {
		t.Error("expected slog to be initialized")
	}
}

// TestLoggerSetLevel tests log level configuration
func TestLoggerSetLevel(t *testing.T) {
	l := NewLogger()
	l.SetLevel(slog.LevelError)
	if l.level != slog.LevelError {
		t.Errorf("expected level to be Error, got %v", l.level)
	}
}

// TestLoggerLogConnection tests connection logging
func TestLoggerLogConnection(t *testing.T) {
	l := NewLogger()

	// Should not panic
	l.LogConnection("user1", "connected")
	l.LogConnection("user1", "authenticated")
	l.LogConnection("user1", "disconnected")
}

// TestLoggerLogCommand tests command logging
func TestLoggerLogCommand(t *testing.T) {
	l := NewLogger()

	// Should not panic
	l.LogCommand("user1", 100*time.Millisecond, true)
	l.LogCommand("user1", 50*time.Millisecond, false) // Error log
}

// TestLoggerLogError tests error logging
func TestLoggerLogError(t *testing.T) {
	l := NewLogger()
	ctx := map[string]interface{}{
		"command": "SELECT",
		"mailbox": "INBOX",
	}

	// Should not panic
	l.LogError("user1", "connection timeout", ctx)
	l.LogError("user1", "auth failed", nil) // nil context
}

// TestLoggerLogRateLimitViolation tests rate limit logging
func TestLoggerLogRateLimitViolation(t *testing.T) {
	l := NewLogger()

	// Should not panic
	l.LogRateLimitViolation("user1", "connection_limit")
	l.LogRateLimitViolation("user2", "request_rate")
}

// TestLoggerLogDegradation tests degradation logging
func TestLoggerLogDegradation(t *testing.T) {
	l := NewLogger()
	metrics := map[string]interface{}{
		"error_rate": 0.15,
		"avg_latency_ms": 500,
	}

	// Should not panic
	l.LogDegradation("degraded", "error rate exceeded threshold", metrics)
	l.LogDegradation("slow", "latency above threshold", nil)
}

// TestLoggerLogMetricsSnapshot tests metrics snapshot logging
func TestLoggerLogMetricsSnapshot(t *testing.T) {
	l := NewLogger()
	snap := MetricsSnapshot{
		ConnectedUsers:     5,
		PeakConnectedUsers: 10,
		CommandsProcessed:  1000,
		CommandErrors:      10,
		Uptime:             1 * time.Hour,
		ErrorRate:          0.01,
	}

	// Should not panic
	l.LogMetricsSnapshot(snap)
}

// TestLoggerNilSafety tests nil safety
func TestLoggerNilSafety(t *testing.T) {
	var l *Logger

	// Should not panic
	l.LogConnection("user1", "connected")
	l.LogCommand("user1", 100*time.Millisecond, true)
	l.LogError("user1", "error", nil)
	l.LogRateLimitViolation("user1", "type")
	l.LogDegradation("status", "reason", nil)
	l.LogMetricsSnapshot(MetricsSnapshot{})
}

// TestLoggerConcurrency tests concurrent logging
func TestLoggerConcurrency(t *testing.T) {
	l := NewLogger()
	done := make(chan bool, 10)

	for i := 0; i < 10; i++ {
		go func(id int) {
			l.LogConnection("user", "connected")
			l.LogCommand("user", 100*time.Millisecond, true)
			l.LogError("user", "error", nil)
			done <- true
		}(i)
	}

	for i := 0; i < 10; i++ {
		<-done
	}
}
