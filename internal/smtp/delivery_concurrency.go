package smtpd

import (
	"context"
	"errors"
	"sync"
	"time"
)

// DeliveryCounter tracks concurrent deliveries per domain with circuit breaker capability.
type DeliveryCounter struct {
	mu              sync.RWMutex
	domainCounters  map[string]*domainCounter
	maxConcurrency  int           // max concurrent deliveries per domain (default 10)
	failureThreshold int          // failures before circuit opens (default 5)
	failureWindow   time.Duration // window for counting failures (default 1 minute)
	halfOpenTimeout time.Duration // time circuit stays half-open before trying again (default 30s)
}

type domainCounter struct {
	mu               sync.Mutex
	activeCount      int
	circuitState     circuitState
	lastFailureTime  time.Time
	consecutiveFailures int
	circuitOpenedAt  time.Time
}

type circuitState string

const (
	circuitClosed circuitState = "closed"
	circuitOpen   circuitState = "open"
	circuitHalfOpen circuitState = "half-open"
)

// NewDeliveryCounter creates a new delivery concurrency counter.
func NewDeliveryCounter(maxConcurrency int) *DeliveryCounter {
	if maxConcurrency <= 0 {
		maxConcurrency = 10 // default
	}
	return &DeliveryCounter{
		domainCounters:   make(map[string]*domainCounter),
		maxConcurrency:   maxConcurrency,
		failureThreshold: 5,
		failureWindow:    1 * time.Minute,
		halfOpenTimeout:  30 * time.Second,
	}
}

// CanDeliver checks if a delivery to a domain can proceed.
// Returns true if within concurrency limit and circuit is not open.
func (dc *DeliveryCounter) CanDeliver(ctx context.Context, domain string) (bool, error) {
	dc.mu.Lock()
	counter := dc.domainCounters[domain]
	if counter == nil {
		counter = &domainCounter{circuitState: circuitClosed}
		dc.domainCounters[domain] = counter
	}
	dc.mu.Unlock()

	counter.mu.Lock()
	defer counter.mu.Unlock()

	// Check circuit breaker state
	if counter.circuitState == circuitOpen {
		// Check if we should transition to half-open
		if time.Since(counter.circuitOpenedAt) > dc.halfOpenTimeout {
			counter.circuitState = circuitHalfOpen
			counter.consecutiveFailures = 0
		} else {
			return false, ErrCircuitOpen
		}
	}

	// Check concurrency limit
	if counter.activeCount >= dc.maxConcurrency {
		return false, nil
	}

	// Half-open state: allow one attempt
	if counter.circuitState == circuitHalfOpen {
		counter.activeCount++
		return true, nil
	}

	// Closed state: normal operation
	counter.activeCount++
	return true, nil
}

// RecordSuccess marks a successful delivery and decrements active count.
func (dc *DeliveryCounter) RecordSuccess(domain string) {
	dc.mu.Lock()
	counter := dc.domainCounters[domain]
	if counter == nil {
		counter = &domainCounter{circuitState: circuitClosed}
		dc.domainCounters[domain] = counter
	}
	dc.mu.Unlock()

	counter.mu.Lock()
	defer counter.mu.Unlock()

	if counter.activeCount > 0 {
		counter.activeCount--
	}

	// Reset failures on success
	if counter.circuitState == circuitHalfOpen {
		// Successful half-open delivery closes the circuit
		counter.circuitState = circuitClosed
		counter.consecutiveFailures = 0
	} else if counter.circuitState == circuitClosed {
		counter.consecutiveFailures = 0
	}
}

// RecordFailure marks a failed delivery and updates circuit breaker state.
func (dc *DeliveryCounter) RecordFailure(domain string) {
	dc.mu.Lock()
	counter := dc.domainCounters[domain]
	if counter == nil {
		counter = &domainCounter{circuitState: circuitClosed}
		dc.domainCounters[domain] = counter
	}
	dc.mu.Unlock()

	counter.mu.Lock()
	defer counter.mu.Unlock()

	if counter.activeCount > 0 {
		counter.activeCount--
	}

	counter.lastFailureTime = time.Now()
	counter.consecutiveFailures++

	// Check if we should open the circuit
	if counter.consecutiveFailures >= dc.failureThreshold {
		counter.circuitState = circuitOpen
		counter.circuitOpenedAt = time.Now()
	}
}

// GetStats returns statistics for a domain's delivery state.
func (dc *DeliveryCounter) GetStats(domain string) map[string]interface{} {
	dc.mu.RLock()
	counter := dc.domainCounters[domain]
	dc.mu.RUnlock()

	if counter == nil {
		return map[string]interface{}{
			"domain":        domain,
			"active_count":  0,
			"circuit_state": "unknown",
		}
	}

	counter.mu.Lock()
	defer counter.mu.Unlock()

	return map[string]interface{}{
		"domain":                domain,
		"active_count":          counter.activeCount,
		"max_concurrency":       dc.maxConcurrency,
		"circuit_state":         string(counter.circuitState),
		"consecutive_failures":  counter.consecutiveFailures,
		"last_failure_time":     counter.lastFailureTime.Format(time.RFC3339),
		"circuit_opened_at":     counter.circuitOpenedAt.Format(time.RFC3339),
	}
}

// Reset clears all counters (useful for testing).
func (dc *DeliveryCounter) Reset() {
	dc.mu.Lock()
	defer dc.mu.Unlock()
	dc.domainCounters = make(map[string]*domainCounter)
}

var (
	ErrCircuitOpen = errors.New("delivery circuit open")
)
