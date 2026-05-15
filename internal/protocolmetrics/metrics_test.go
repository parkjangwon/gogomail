package protocolmetrics

import (
	"testing"
	"time"
)

// TestGatewayMetricsConnect tests connection tracking
func TestGatewayMetricsConnect(t *testing.T) {
	m := NewGatewayMetrics()

	m.RecordConnect("user1")
	m.RecordConnect("user1")
	m.RecordConnect("user2")

	snap := m.Snapshot()
	if snap.ConnectedUsers != 3 {
		t.Errorf("expected 3 connected users, got %d", snap.ConnectedUsers)
	}
	if snap.TotalConnectAttempts != 3 {
		t.Errorf("expected 3 connect attempts, got %d", snap.TotalConnectAttempts)
	}
	if snap.PeakConnectedUsers != 3 {
		t.Errorf("expected peak 3, got %d", snap.PeakConnectedUsers)
	}
}

// TestGatewayMetricsDisconnect tests disconnection tracking
func TestGatewayMetricsDisconnect(t *testing.T) {
	m := NewGatewayMetrics()

	m.RecordConnect("user1")
	m.RecordConnect("user2")
	m.RecordDisconnect()
	m.RecordDisconnect()

	snap := m.Snapshot()
	if snap.ConnectedUsers != 0 {
		t.Errorf("expected 0 connected users, got %d", snap.ConnectedUsers)
	}
	if snap.TotalDisconnects != 2 {
		t.Errorf("expected 2 disconnects, got %d", snap.TotalDisconnects)
	}
}

// TestGatewayMetricsCommand tests command processing
func TestGatewayMetricsCommand(t *testing.T) {
	m := NewGatewayMetrics()

	m.RecordCommand("user1", 100*time.Millisecond)
	m.RecordCommand("user1", 200*time.Millisecond)
	m.RecordCommand("user2", 50*time.Millisecond)

	snap := m.Snapshot()
	if snap.CommandsProcessed != 3 {
		t.Errorf("expected 3 commands, got %d", snap.CommandsProcessed)
	}

	expected := 350 * time.Millisecond / 3
	if snap.AverageCommandDuration != expected {
		t.Errorf("expected avg duration %v, got %v", expected, snap.AverageCommandDuration)
	}
}

// TestGatewayMetricsErrors tests error tracking
func TestGatewayMetricsErrors(t *testing.T) {
	m := NewGatewayMetrics()

	m.RecordCommand("user1", 100*time.Millisecond)
	m.RecordError("user1")
	m.RecordCommand("user1", 200*time.Millisecond)

	snap := m.Snapshot()
	if snap.CommandsProcessed != 2 {
		t.Errorf("expected 2 commands, got %d", snap.CommandsProcessed)
	}
	if snap.CommandErrors != 1 {
		t.Errorf("expected 1 error, got %d", snap.CommandErrors)
	}

	expectedErrorRate := 1.0 / 2.0
	if snap.ErrorRate != expectedErrorRate {
		t.Errorf("expected error rate %.2f, got %.2f", expectedErrorRate, snap.ErrorRate)
	}
}

// TestGatewayMetricsUserMetrics tests per-user metrics
func TestGatewayMetricsUserMetrics(t *testing.T) {
	m := NewGatewayMetrics()

	m.RecordConnect("user1")
	m.RecordConnect("user1")
	m.RecordCommand("user1", 100*time.Millisecond)
	m.RecordError("user1")

	conns, cmds, errs := m.GetUserMetrics("user1")
	if conns != 2 {
		t.Errorf("expected 2 connections, got %d", conns)
	}
	if cmds != 1 {
		t.Errorf("expected 1 command, got %d", cmds)
	}
	if errs != 1 {
		t.Errorf("expected 1 error, got %d", errs)
	}

	// Unknown user should return 0s
	conns, cmds, errs = m.GetUserMetrics("unknown")
	if conns != 0 || cmds != 0 || errs != 0 {
		t.Errorf("expected all zeros for unknown user, got %d/%d/%d", conns, cmds, errs)
	}
}

// TestGatewayMetricsNil tests nil safety
func TestGatewayMetricsNil(t *testing.T) {
	var m *GatewayMetrics

	// Should not panic
	m.RecordConnect("user1")
	m.RecordDisconnect()
	m.RecordCommand("user1", 100*time.Millisecond)
	m.RecordError("user1")
	m.RecordRateLimitExceeded()
	m.RecordConnectionLimitExceeded()

	snap := m.Snapshot()
	if snap.ConnectedUsers != 0 {
		t.Errorf("expected zero snapshot, got %+v", snap)
	}
}

// TestRateLimiterCanConnect tests connection limiting
func TestRateLimiterCanConnect(t *testing.T) {
	rl := NewRateLimiter(2, 10.0)

	if !rl.CanConnect("user1") {
		t.Error("expected first connection to succeed")
	}
	if !rl.CanConnect("user1") {
		t.Error("expected second connection to succeed")
	}
	if rl.CanConnect("user1") {
		t.Error("expected third connection to fail (limit is 2)")
	}

	// Different user should have independent limit
	if !rl.CanConnect("user2") {
		t.Error("expected user2 to have independent limit")
	}
}

// TestRateLimiterDisconnection tests disconnection tracking
func TestRateLimiterDisconnection(t *testing.T) {
	rl := NewRateLimiter(1, 10.0)

	if !rl.CanConnect("user1") {
		t.Error("expected first connection to succeed")
	}
	if rl.CanConnect("user1") {
		t.Error("expected second connection to fail (limit is 1)")
	}

	rl.RecordDisconnection("user1")

	if !rl.CanConnect("user1") {
		t.Error("expected connection to succeed after disconnection")
	}
}

// TestRateLimiterNoLimit tests unlimited mode
func TestRateLimiterNoLimit(t *testing.T) {
	rl := NewRateLimiter(0, 0.0)

	// Unlimited connections
	for i := 0; i < 100; i++ {
		if !rl.CanConnect("user1") {
			t.Errorf("expected connection %d to succeed with unlimited limit", i+1)
		}
	}
}

// BenchmarkGatewayMetricsRecordCommand measures command recording performance
func BenchmarkGatewayMetricsRecordCommand(b *testing.B) {
	m := NewGatewayMetrics()

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		m.RecordCommand("user1", 100*time.Millisecond)
	}
}

// BenchmarkGatewayMetricsSnapshot measures snapshot performance
func BenchmarkGatewayMetricsSnapshot(b *testing.B) {
	m := NewGatewayMetrics()
	for i := 0; i < 1000; i++ {
		m.RecordCommand("user1", 100*time.Millisecond)
	}

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_ = m.Snapshot()
	}
}

// BenchmarkRateLimiterCanConnect measures connection check performance
func BenchmarkRateLimiterCanConnect(b *testing.B) {
	rl := NewRateLimiter(100, 10.0)

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_ = rl.CanConnect("user1")
	}
}
