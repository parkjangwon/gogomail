package delivery

import (
	"context"
	"net"
	"net/smtp"
	"sync"
	"time"
)

// SMTPConnPoolKey uniquely identifies a pool (host, TLS mode, auth)
// Note: must be comparable for use as map key
type SMTPConnPoolKey struct {
	Host        string
	Port        int
	ImplicitTLS bool
	AuthUser    string
}

// Equal compares two pool keys
func (k SMTPConnPoolKey) Equal(other SMTPConnPoolKey) bool {
	return k.Host == other.Host && k.Port == other.Port &&
		k.ImplicitTLS == other.ImplicitTLS && k.AuthUser == other.AuthUser
}

// PooledSMTPConn wraps an SMTP client with pool metadata
type PooledSMTPConn struct {
	Client   *smtp.Client
	Conn     net.Conn
	LastUsed time.Time
	Key      SMTPConnPoolKey
}

// SMTPConnectionPool manages reusable SMTP connections per route/host
type SMTPConnectionPool struct {
	mu          sync.RWMutex
	conns       map[SMTPConnPoolKey]*PooledSMTPConn
	maxIdle     int
	idleTimeout time.Duration
	maxAge      time.Duration
}

// NewSMTPConnectionPool creates a new connection pool
func NewSMTPConnectionPool(maxIdle int, idleTimeout, maxAge time.Duration) *SMTPConnectionPool {
	if maxIdle <= 0 {
		maxIdle = 4 // default: 4 conns per host
	}
	if idleTimeout <= 0 {
		idleTimeout = 30 * time.Second
	}
	if maxAge <= 0 {
		maxAge = 5 * time.Minute
	}
	return &SMTPConnectionPool{
		conns:       make(map[SMTPConnPoolKey]*PooledSMTPConn),
		maxIdle:     maxIdle,
		idleTimeout: idleTimeout,
		maxAge:      maxAge,
	}
}

// Get returns a connection from the pool or creates a new one
func (p *SMTPConnectionPool) Get(ctx context.Context, key SMTPConnPoolKey) (*PooledSMTPConn, error) {
	p.mu.Lock()
	if conn, ok := p.conns[key]; ok {
		if p.isValidConn(conn) {
			delete(p.conns, key)
			p.mu.Unlock()
			return conn, nil
		}
		if conn.Conn != nil {
			_ = conn.Conn.Close()
		}
		if conn.Client != nil {
			_ = conn.Client.Close()
		}
		delete(p.conns, key)
	}
	p.mu.Unlock()
	return nil, nil // caller creates new connection
}

// Put returns a connection to the pool
func (p *SMTPConnectionPool) Put(key SMTPConnPoolKey, conn *PooledSMTPConn) error {
	if conn == nil || conn.Client == nil {
		return nil
	}
	p.mu.Lock()
	defer p.mu.Unlock()

	// Check current pool size
	count := 0
	for k := range p.conns {
		if k == key {
			count++
		}
	}
	if count >= p.maxIdle {
		_ = conn.Conn.Close()
		_ = conn.Client.Close()
		return nil
	}

	conn.LastUsed = time.Now()
	p.conns[key] = conn
	return nil
}

// Close closes all idle connections
func (p *SMTPConnectionPool) Close() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	for key, conn := range p.conns {
		_ = conn.Client.Close()
		_ = conn.Conn.Close()
		delete(p.conns, key)
	}
	return nil
}

// isValidConn checks if connection is still usable
func (p *SMTPConnectionPool) isValidConn(conn *PooledSMTPConn) bool {
	if conn == nil || conn.Client == nil || conn.Conn == nil {
		return false
	}
	now := time.Now()
	if now.Sub(conn.LastUsed) > p.idleTimeout {
		return false
	}
	if now.Sub(conn.LastUsed) > p.maxAge {
		return false
	}
	return true
}

// PruneStale removes expired connections
func (p *SMTPConnectionPool) PruneStale() {
	p.mu.Lock()
	defer p.mu.Unlock()
	now := time.Now()
	for key, conn := range p.conns {
		if now.Sub(conn.LastUsed) > p.idleTimeout || now.Sub(conn.LastUsed) > p.maxAge {
			_ = conn.Client.Close()
			_ = conn.Conn.Close()
			delete(p.conns, key)
		}
	}
}
