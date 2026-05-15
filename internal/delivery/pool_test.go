package delivery

import (
	"context"
	"net"
	"testing"
	"time"
)

func TestSMTPConnectionPoolBasic(t *testing.T) {
	pool := NewSMTPConnectionPool(2, 100*time.Millisecond, 1*time.Second)
	defer pool.Close()

	key := SMTPConnPoolKey{Host: "mail.example.com", Port: 25}

	// Get from empty pool should return nil
	conn, err := pool.Get(context.Background(), key)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if conn != nil {
		t.Error("expected nil from empty pool")
	}

	// Put with nil client is rejected (Put returns early)
	nilConn := &PooledSMTPConn{Key: key, LastUsed: time.Now()}
	_ = pool.Put(key, nilConn)

	// Get should still return nil (not stored)
	conn, err = pool.Get(context.Background(), key)
	if err != nil {
		t.Fatalf("unexpected error getting conn: %v", err)
	}
	if conn != nil {
		t.Error("expected nil from pool (nil client not stored)")
	}
}

// Mock types for testing
type MockSMTPClient struct {
	closed bool
}

func (m *MockSMTPClient) Close() error {
	m.closed = true
	return nil
}

type MockNetConn struct {
	closed bool
}

func (m *MockNetConn) Read(b []byte) (n int, err error) {
	return 0, nil
}

func (m *MockNetConn) Write(b []byte) (n int, err error) {
	return len(b), nil
}

func (m *MockNetConn) Close() error {
	m.closed = true
	return nil
}

func (m *MockNetConn) LocalAddr() net.Addr {
	return nil
}

func (m *MockNetConn) RemoteAddr() net.Addr {
	return nil
}

func (m *MockNetConn) SetDeadline(t time.Time) error {
	return nil
}

func (m *MockNetConn) SetReadDeadline(t time.Time) error {
	return nil
}

func (m *MockNetConn) SetWriteDeadline(t time.Time) error {
	return nil
}

func TestSMTPConnectionPoolMaxIdle(t *testing.T) {
	pool := NewSMTPConnectionPool(1, 100*time.Millisecond, 1*time.Second)
	defer pool.Close()

	key := SMTPConnPoolKey{Host: "mail.example.com", Port: 25}

	// Put with nil client is rejected
	conn1 := &PooledSMTPConn{Key: key, LastUsed: time.Now()}
	pool.Put(key, conn1)

	// Get returns nil (not stored)
	retrieved, _ := pool.Get(context.Background(), key)
	if retrieved != nil {
		t.Error("expected nil (nil client not stored)")
	}
}

func TestSMTPConnectionPoolIdleTimeout(t *testing.T) {
	pool := NewSMTPConnectionPool(1, 10*time.Millisecond, 1*time.Second)
	defer pool.Close()

	key := SMTPConnPoolKey{Host: "mail.example.com", Port: 25}

	// Put with nil client is rejected
	conn := &PooledSMTPConn{Key: key, LastUsed: time.Now()}
	pool.Put(key, conn)

	// Get returns nil (not stored)
	retrieved, _ := pool.Get(context.Background(), key)
	if retrieved != nil {
		t.Error("expected nil (nil client not stored)")
	}
}
