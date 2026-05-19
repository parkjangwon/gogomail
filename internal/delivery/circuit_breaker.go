package delivery

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"
)

// ErrCircuitOpen is returned when the circuit breaker is open for a domain.
var ErrCircuitOpen = errors.New("circuit breaker open: domain temporarily unavailable")

// ErrConcurrencyLimitExceeded is returned when the per-domain concurrency cap is hit.
var ErrConcurrencyLimitExceeded = errors.New("delivery concurrency limit exceeded for domain")

const (
	circuitClosed   = "closed"
	circuitOpen     = "open"
	circuitHalfOpen = "half-open"
)

// domainCircuit tracks per-domain circuit-breaker state.
type domainCircuit struct {
	mu               sync.Mutex
	activeCount      int
	state            string
	consecutiveFails int
	openedAt         time.Time
}

// CircuitBreakerTransport wraps a Transport with per-domain circuit breaking
// and concurrency limiting.
type CircuitBreakerTransport struct {
	next Transport

	mu      sync.RWMutex
	domains map[string]*domainCircuit

	// MaxConcurrency is the maximum simultaneous deliveries per domain.
	MaxConcurrency int
	// FailureThreshold is the consecutive failure count that trips the breaker.
	FailureThreshold int
	// HalfOpenTimeout is how long to wait in the open state before allowing a probe.
	HalfOpenTimeout time.Duration
}

// NewCircuitBreakerTransport creates a new CircuitBreakerTransport wrapping next.
func NewCircuitBreakerTransport(next Transport, maxConcurrency int) *CircuitBreakerTransport {
	if maxConcurrency <= 0 {
		maxConcurrency = 10
	}
	return &CircuitBreakerTransport{
		next:             next,
		domains:          make(map[string]*domainCircuit),
		MaxConcurrency:   maxConcurrency,
		FailureThreshold: 5,
		HalfOpenTimeout:  30 * time.Second,
	}
}

func (t *CircuitBreakerTransport) domain(job Job) string {
	recipients := job.Recipients()
	if len(recipients) == 0 {
		return ""
	}
	email := recipients[0].Email
	if idx := strings.LastIndex(email, "@"); idx >= 0 {
		return strings.ToLower(email[idx+1:])
	}
	return strings.ToLower(email)
}

func (t *CircuitBreakerTransport) getOrCreateCircuit(domain string) *domainCircuit {
	t.mu.RLock()
	c, ok := t.domains[domain]
	t.mu.RUnlock()
	if ok {
		return c
	}

	t.mu.Lock()
	defer t.mu.Unlock()
	if c, ok = t.domains[domain]; ok {
		return c
	}
	c = &domainCircuit{state: circuitClosed}
	t.domains[domain] = c
	return c
}

// Deliver attempts delivery, respecting the circuit breaker and concurrency limit.
func (t *CircuitBreakerTransport) Deliver(ctx context.Context, job Job) error {
	domain := t.domain(job)
	circuit := t.getOrCreateCircuit(domain)

	circuit.mu.Lock()

	// Check and possibly transition open → half-open
	if circuit.state == circuitOpen {
		if time.Since(circuit.openedAt) >= t.HalfOpenTimeout {
			circuit.state = circuitHalfOpen
		} else {
			circuit.mu.Unlock()
			return fmt.Errorf("%w for %s", ErrCircuitOpen, domain)
		}
	}

	// Reject if half-open already has an active probe
	if circuit.state == circuitHalfOpen && circuit.activeCount > 0 {
		circuit.mu.Unlock()
		return fmt.Errorf("%w for %s (half-open probe in progress)", ErrCircuitOpen, domain)
	}

	// Enforce concurrency limit (only for closed state)
	if circuit.state == circuitClosed && t.MaxConcurrency > 0 && circuit.activeCount >= t.MaxConcurrency {
		circuit.mu.Unlock()
		return fmt.Errorf("%w for %s (limit %d)", ErrConcurrencyLimitExceeded, domain, t.MaxConcurrency)
	}

	circuit.activeCount++
	circuit.mu.Unlock()

	err := t.next.Deliver(ctx, job)

	circuit.mu.Lock()
	circuit.activeCount--
	if err != nil {
		circuit.consecutiveFails++
		if circuit.state != circuitOpen && circuit.consecutiveFails >= t.FailureThreshold {
			circuit.state = circuitOpen
			circuit.openedAt = time.Now()
		}
	} else {
		// Success
		circuit.consecutiveFails = 0
		circuit.state = circuitClosed
	}
	circuit.mu.Unlock()

	return err
}
