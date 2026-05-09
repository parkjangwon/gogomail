package milter

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

// Pool manages a pool of milter client connections.
// It dials on demand up to maxConns, maintains idle connections,
// and discards broken connections.
type Pool struct {
	network   string
	address   string
	timeout   time.Duration
	maxConns  int
	activeNum int32 // atomic count of active connections

	idleMu    sync.Mutex
	idleConns []*Client // FIFO queue of idle clients

	// circuit breaker (optional)
	breaker     *CircuitBreaker
	healthTick  *time.Ticker
	healthStopCh chan struct{}

	closeMu  sync.Mutex
	closed   bool
	closeCh  chan struct{} // signals shutdown
	stopOnce sync.Once     // ensures shutdown runs once
}

// NewPool creates a new milter connection pool.
// maxConns limits the number of concurrent connections.
func NewPool(network, address string, timeout time.Duration, maxConns int) (*Pool, error) {
	if maxConns <= 0 {
		return nil, fmt.Errorf("milter: maxConns must be > 0")
	}
	return &Pool{
		network:   network,
		address:   address,
		timeout:   timeout,
		maxConns:  maxConns,
		idleConns: make([]*Client, 0, maxConns),
		closeCh:   make(chan struct{}),
	}, nil
}

// NewPoolWithCircuitBreaker creates a pool with circuit breaker and health check.
// failureThreshold: failures before opening circuit.
// resetTimeout: time to wait before HALF_OPEN attempt.
// healthInterval: health check frequency.
func NewPoolWithCircuitBreaker(network, address string, timeout time.Duration, maxConns int, failureThreshold int64, resetTimeout time.Duration) (*Pool, error) {
	pool, err := NewPool(network, address, timeout, maxConns)
	if err != nil {
		return nil, err
	}

	pool.breaker = NewCircuitBreaker(failureThreshold, resetTimeout)
	pool.healthTick = time.NewTicker(resetTimeout)
	pool.healthStopCh = make(chan struct{})

	// Start health check goroutine
	go pool.healthCheckLoop()

	return pool, nil
}

// healthCheckLoop periodically checks milter server health.
func (p *Pool) healthCheckLoop() {
	for {
		select {
		case <-p.healthTick.C:
			p.healthCheck()
		case <-p.healthStopCh:
			p.healthTick.Stop()
			return
		case <-p.closeCh:
			p.healthTick.Stop()
			return
		}
	}
}

// healthCheck attempts to dial and ping the milter server.
func (p *Pool) healthCheck() {
	if p.breaker == nil {
		return
	}

	// Check state first to trigger OPEN→HALF_OPEN transitions
	state := p.breaker.State()
	if state == StateOpen {
		// Circuit is still open, don't attempt yet
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), p.timeout)
	defer cancel()

	client, err := Dial(ctx, p.network, p.address, p.timeout)
	if err != nil {
		p.breaker.RecordFailure()
		return
	}
	defer client.Close()

	// Perform negotiation as health check
	if err := client.Negotiate(ctx); err != nil {
		p.breaker.RecordFailure()
		return
	}

	p.breaker.RecordSuccess()
}

// Get returns a milter client. If circuit breaker is open, returns error immediately.
// If no idle client is available, dials a new one (up to maxConns). Blocks if the pool is at capacity.
func (p *Pool) Get(ctx context.Context) (*Client, error) {
	// Check circuit breaker first
	if p.breaker != nil && !p.breaker.AllowRequest() {
		return nil, fmt.Errorf("milter: circuit breaker is open")
	}
	// Try to get an idle client first
	p.idleMu.Lock()
	if len(p.idleConns) > 0 {
		client := p.idleConns[len(p.idleConns)-1]
		p.idleConns = p.idleConns[:len(p.idleConns)-1]
		p.idleMu.Unlock()
		return client, nil
	}
	p.idleMu.Unlock()

	// Check if we can create a new connection
	for {
		current := atomic.LoadInt32(&p.activeNum)
		if current >= int32(p.maxConns) {
			// At capacity, wait for an idle connection
			select {
			case <-ctx.Done():
				return nil, fmt.Errorf("milter: context done waiting for available connection: %w", ctx.Err())
			case <-p.closeCh:
				return nil, fmt.Errorf("milter: pool closed")
			case <-time.After(100 * time.Millisecond):
				// Retry getting an idle client
				p.idleMu.Lock()
				if len(p.idleConns) > 0 {
					client := p.idleConns[len(p.idleConns)-1]
					p.idleConns = p.idleConns[:len(p.idleConns)-1]
					p.idleMu.Unlock()
					return client, nil
				}
				p.idleMu.Unlock()
			}
			continue
		}

		// Try to increment active count
		if atomic.CompareAndSwapInt32(&p.activeNum, current, current+1) {
			// Successfully reserved a slot, dial a new connection
			client, err := Dial(ctx, p.network, p.address, p.timeout)
			if err != nil {
				atomic.AddInt32(&p.activeNum, -1)
				return nil, fmt.Errorf("milter dial: %w", err)
			}
			return client, nil
		}
		// Retry CompareAndSwap
	}
}

// Put returns a client to the pool for reuse.
func (p *Pool) Put(client *Client) {
	if client == nil {
		return
	}

	p.closeMu.Lock()
	if p.closed {
		p.closeMu.Unlock()
		_ = client.Close()
		atomic.AddInt32(&p.activeNum, -1)
		return
	}
	p.closeMu.Unlock()

	// Return to idle pool
	p.idleMu.Lock()
	p.idleConns = append(p.idleConns, client)
	p.idleMu.Unlock()
}

// Close closes all idle connections and signals the pool is closed.
// In-flight Get calls will eventually fail. Active clients should be
// returned with Put before Close is called.
func (p *Pool) Close() error {
	p.closeMu.Lock()
	if p.closed {
		p.closeMu.Unlock()
		return nil
	}
	p.closed = true
	p.closeMu.Unlock()

	// Stop health check if running
	if p.healthStopCh != nil {
		select {
		case <-p.healthStopCh:
		default:
			close(p.healthStopCh)
		}
	}

	// Signal shutdown
	p.stopOnce.Do(func() {
		close(p.closeCh)
	})

	// Close all idle connections
	p.idleMu.Lock()
	var lastErr error
	for _, client := range p.idleConns {
		if err := client.Close(); err != nil {
			lastErr = err
		}
	}
	p.idleConns = p.idleConns[:0]
	p.idleMu.Unlock()

	return lastErr
}
