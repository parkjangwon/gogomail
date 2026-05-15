package protocolmetrics

import (
	"fmt"
	"log/slog"
	"time"
)

// Logger provides structured logging for protocol metrics and events
type Logger struct {
	slog *slog.Logger
	level slog.Level
}

// NewLogger creates a new structured logger for protocol metrics
func NewLogger() *Logger {
	return &Logger{
		slog:  slog.Default(),
		level: slog.LevelInfo,
	}
}

// SetLevel sets the logging level
func (l *Logger) SetLevel(level slog.Level) {
	if l != nil {
		l.level = level
	}
}

// LogConnection logs a connection event
func (l *Logger) LogConnection(userID string, event string) {
	if l == nil || l.slog == nil {
		return
	}
	l.slog.Log(nil, l.level, fmt.Sprintf("connection %s", event),
		slog.String("user_id", userID),
		slog.String("event", event),
		slog.String("timestamp", time.Now().UTC().Format(time.RFC3339Nano)),
	)
}

// LogCommand logs a command execution
func (l *Logger) LogCommand(userID string, duration time.Duration, success bool) {
	if l == nil || l.slog == nil {
		return
	}
	level := l.level
	if !success {
		level = slog.LevelError
	}
	l.slog.Log(nil, level, "command_executed",
		slog.String("user_id", userID),
		slog.Int64("duration_ms", duration.Milliseconds()),
		slog.Bool("success", success),
		slog.String("timestamp", time.Now().UTC().Format(time.RFC3339Nano)),
	)
}

// LogError logs an error event
func (l *Logger) LogError(userID string, errMsg string, context map[string]interface{}) {
	if l == nil || l.slog == nil {
		return
	}
	attrs := []slog.Attr{
		slog.String("user_id", userID),
		slog.String("error", errMsg),
		slog.String("timestamp", time.Now().UTC().Format(time.RFC3339Nano)),
	}
	for k, v := range context {
		attrs = append(attrs, slog.Any(k, v))
	}
	l.slog.LogAttrs(nil, slog.LevelError, "error_event", attrs...)
}

// LogRateLimitViolation logs a rate limit violation
func (l *Logger) LogRateLimitViolation(userID string, limitType string) {
	if l == nil || l.slog == nil {
		return
	}
	l.slog.Log(nil, slog.LevelWarn, "rate_limit_exceeded",
		slog.String("user_id", userID),
		slog.String("limit_type", limitType),
		slog.String("timestamp", time.Now().UTC().Format(time.RFC3339Nano)),
	)
}

// LogDegradation logs a degradation event
func (l *Logger) LogDegradation(status string, reason string, metrics map[string]interface{}) {
	if l == nil || l.slog == nil {
		return
	}
	attrs := []slog.Attr{
		slog.String("status", status),
		slog.String("reason", reason),
		slog.String("timestamp", time.Now().UTC().Format(time.RFC3339Nano)),
	}
	for k, v := range metrics {
		attrs = append(attrs, slog.Any(k, v))
	}
	l.slog.LogAttrs(nil, slog.LevelWarn, "degradation_event", attrs...)
}

// LogMetricsSnapshot logs a metrics snapshot
func (l *Logger) LogMetricsSnapshot(snap MetricsSnapshot) {
	if l == nil || l.slog == nil {
		return
	}
	l.slog.Log(nil, slog.LevelDebug, "metrics_snapshot",
		slog.Int64("connected_users", snap.ConnectedUsers),
		slog.Int64("peak_connected_users", snap.PeakConnectedUsers),
		slog.Int64("total_connect_attempts", snap.TotalConnectAttempts),
		slog.Int64("total_disconnects", snap.TotalDisconnects),
		slog.Int64("commands_processed", snap.CommandsProcessed),
		slog.Int64("command_errors", snap.CommandErrors),
		slog.Int64("avg_command_duration_ms", snap.AverageCommandDuration.Milliseconds()),
		slog.Float64("error_rate", snap.ErrorRate),
		slog.Int64("rate_limit_exceeded", snap.RateLimitExceeded),
		slog.Int64("connection_limit_exceeded", snap.ConnectionLimitExceeded),
		slog.Int64("uptime_seconds", int64(snap.Uptime.Seconds())),
		slog.String("timestamp", time.Now().UTC().Format(time.RFC3339Nano)),
	)
}
