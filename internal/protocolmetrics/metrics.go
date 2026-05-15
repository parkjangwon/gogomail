package protocolmetrics

import (
	"sync"
	"sync/atomic"
	"time"
)

// GatewayMetrics tracks performance and usage metrics for protocol gateways (IMAP, CalDAV, CardDAV)
type GatewayMetrics struct {
	// Connection tracking
	connectedUsers          int64
	peakConnectedUsers      int64
	totalConnectAttempts    int64
	totalDisconnects        int64

	// Command/Operation tracking
	commandsProcessed       int64
	commandErrors           int64
	totalCommandDuration    int64  // nanoseconds

	// Rate limit tracking
	rateLimitExceeded       int64
	connectionLimitExceeded int64

	// Per-user tracking (map protected by mutex)
	mu                sync.RWMutex
	userConnections   map[string]int64
	userCommands      map[string]int64
	userErrors        map[string]int64

	startTime time.Time
	logger    *Logger // Optional structured logging
}

// NewGatewayMetrics creates a new metrics collector
func NewGatewayMetrics() *GatewayMetrics {
	return &GatewayMetrics{
		userConnections: make(map[string]int64),
		userCommands:    make(map[string]int64),
		userErrors:      make(map[string]int64),
		startTime:       time.Now(),
		logger:          NewLogger(),
	}
}

// SetLogger sets optional structured logger for metrics events
func (m *GatewayMetrics) SetLogger(logger *Logger) {
	if m == nil {
		return
	}
	m.logger = logger
}

// RecordConnect records a new user connection
func (m *GatewayMetrics) RecordConnect(userID string) {
	if m == nil {
		return
	}
	atomic.AddInt64(&m.totalConnectAttempts, 1)
	current := atomic.AddInt64(&m.connectedUsers, 1)

	// Update peak
	for {
		peak := atomic.LoadInt64(&m.peakConnectedUsers)
		if current <= peak || atomic.CompareAndSwapInt64(&m.peakConnectedUsers, peak, current) {
			break
		}
	}

	// Track per-user
	m.mu.Lock()
	m.userConnections[userID]++
	m.mu.Unlock()

	// Log connection event
	if m.logger != nil {
		m.logger.LogConnection(userID, "connected")
	}
}

// RecordDisconnect records a user disconnection
func (m *GatewayMetrics) RecordDisconnect() {
	if m == nil {
		return
	}
	atomic.AddInt64(&m.connectedUsers, -1)
	atomic.AddInt64(&m.totalDisconnects, 1)
}

// RecordCommand records a successfully processed command/operation
func (m *GatewayMetrics) RecordCommand(userID string, duration time.Duration) {
	if m == nil {
		return
	}
	atomic.AddInt64(&m.commandsProcessed, 1)
	atomic.AddInt64(&m.totalCommandDuration, duration.Nanoseconds())

	m.mu.Lock()
	m.userCommands[userID]++
	m.mu.Unlock()
}

// RecordError records a command/operation error
func (m *GatewayMetrics) RecordError(userID string) {
	if m == nil {
		return
	}
	atomic.AddInt64(&m.commandErrors, 1)

	m.mu.Lock()
	m.userErrors[userID]++
	m.mu.Unlock()

	// Log error event
	if m.logger != nil {
		m.logger.LogError(userID, "command_failed", nil)
	}
}

// RecordRateLimitExceeded records a rate limit violation
func (m *GatewayMetrics) RecordRateLimitExceeded() {
	if m == nil {
		return
	}
	atomic.AddInt64(&m.rateLimitExceeded, 1)

	// Log rate limit event
	if m.logger != nil {
		m.logger.LogRateLimitViolation("unknown", "rate_limit")
	}
}

// RecordConnectionLimitExceeded records a connection limit violation
func (m *GatewayMetrics) RecordConnectionLimitExceeded() {
	if m == nil {
		return
	}
	atomic.AddInt64(&m.connectionLimitExceeded, 1)
}

// Snapshot captures current metrics state
type MetricsSnapshot struct {
	ConnectedUsers          int64
	PeakConnectedUsers      int64
	TotalConnectAttempts    int64
	TotalDisconnects        int64
	CommandsProcessed       int64
	CommandErrors           int64
	AverageCommandDuration  time.Duration
	RateLimitExceeded       int64
	ConnectionLimitExceeded int64
	Uptime                  time.Duration
	ErrorRate               float64
}

// Snapshot returns current metrics snapshot
func (m *GatewayMetrics) Snapshot() MetricsSnapshot {
	if m == nil {
		return MetricsSnapshot{}
	}

	connected := atomic.LoadInt64(&m.connectedUsers)
	peak := atomic.LoadInt64(&m.peakConnectedUsers)
	connects := atomic.LoadInt64(&m.totalConnectAttempts)
	disconnects := atomic.LoadInt64(&m.totalDisconnects)
	commands := atomic.LoadInt64(&m.commandsProcessed)
	errors := atomic.LoadInt64(&m.commandErrors)
	duration := atomic.LoadInt64(&m.totalCommandDuration)
	rateLimits := atomic.LoadInt64(&m.rateLimitExceeded)
	connLimits := atomic.LoadInt64(&m.connectionLimitExceeded)

	uptime := time.Since(m.startTime)

	var avgDuration time.Duration
	if commands > 0 {
		avgDuration = time.Duration(duration / commands)
	}

	var errorRate float64
	if commands > 0 {
		errorRate = float64(errors) / float64(commands)
	}

	return MetricsSnapshot{
		ConnectedUsers:          connected,
		PeakConnectedUsers:      peak,
		TotalConnectAttempts:    connects,
		TotalDisconnects:        disconnects,
		CommandsProcessed:       commands,
		CommandErrors:           errors,
		AverageCommandDuration:  avgDuration,
		RateLimitExceeded:       rateLimits,
		ConnectionLimitExceeded: connLimits,
		Uptime:                  uptime,
		ErrorRate:               errorRate,
	}
}

// GetUserMetrics returns metrics for a specific user
func (m *GatewayMetrics) GetUserMetrics(userID string) (connections, commands, errors int64) {
	if m == nil {
		return 0, 0, 0
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	return m.userConnections[userID], m.userCommands[userID], m.userErrors[userID]
}

// RateLimiter enforces per-user rate limits
type RateLimiter struct {
	maxConnections    int
	maxRequestsPerSec float64
	mu                sync.RWMutex
	userConnections   map[string]int
}

// NewRateLimiter creates a new rate limiter
func NewRateLimiter(maxConnections int, maxRequestsPerSec float64) *RateLimiter {
	return &RateLimiter{
		maxConnections:    maxConnections,
		maxRequestsPerSec: maxRequestsPerSec,
		userConnections:   make(map[string]int),
	}
}

// CanConnect checks if user can open a new connection
func (rl *RateLimiter) CanConnect(userID string) bool {
	if rl == nil || rl.maxConnections <= 0 {
		return true
	}

	rl.mu.Lock()
	defer rl.mu.Unlock()

	count := rl.userConnections[userID]
	if count >= rl.maxConnections {
		return false
	}

	rl.userConnections[userID] = count + 1
	return true
}

// RecordDisconnection removes a connection from tracking
func (rl *RateLimiter) RecordDisconnection(userID string) {
	if rl == nil {
		return
	}

	rl.mu.Lock()
	defer rl.mu.Unlock()

	if count := rl.userConnections[userID]; count > 1 {
		rl.userConnections[userID] = count - 1
	} else {
		delete(rl.userConnections, userID)
	}
}
