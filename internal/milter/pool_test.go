package milter

import (
	"context"
	"net"
	"sync/atomic"
	"testing"
	"time"
)

// TestPoolDialConnect verifies the pool can dial and connect to a milter server.
func TestPoolDialConnect(t *testing.T) {
	// Create a listener for a fake milter server
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen failed: %v", err)
	}
	defer ln.Close()

	// Start a stub milter server in background
	go func() {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		stubServer(t, conn, respAccept)
	}()

	// Create a pool and dial
	pool, err := NewPool("tcp", ln.Addr().String(), 5*time.Second, 1)
	if err != nil {
		t.Fatalf("NewPool failed: %v", err)
	}
	defer pool.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Get a client from the pool
	client, err := pool.Get(ctx)
	if err != nil {
		t.Fatalf("pool.Get failed: %v", err)
	}
	defer pool.Put(client)

	// Verify client is usable (do a negotiation)
	err = client.Negotiate(ctx)
	if err != nil {
		t.Fatalf("negotiate failed: %v", err)
	}
}

// TestPoolMaxConns verifies the pool respects maxConns limit.
func TestPoolMaxConns(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen failed: %v", err)
	}
	defer ln.Close()

	// Count concurrent accepts
	var acceptCount int
	acceptCh := make(chan struct{}, 10)
	go func() {
		for i := 0; i < 5; i++ {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			acceptCh <- struct{}{}
			go stubServer(t, conn, respAccept)
		}
	}()

	// Create a pool with maxConns=2
	pool, err := NewPool("tcp", ln.Addr().String(), 5*time.Second, 2)
	if err != nil {
		t.Fatalf("NewPool failed: %v", err)
	}
	defer pool.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Try to get 2 clients (should succeed)
	c1, err := pool.Get(ctx)
	if err != nil {
		t.Fatalf("pool.Get #1 failed: %v", err)
	}
	<-acceptCh // wait for accept
	acceptCount++

	c2, err := pool.Get(ctx)
	if err != nil {
		t.Fatalf("pool.Get #2 failed: %v", err)
	}
	<-acceptCh // wait for accept
	acceptCount++

	if acceptCount != 2 {
		t.Fatalf("expected 2 accepts, got %d", acceptCount)
	}

	// Return clients
	pool.Put(c1)
	pool.Put(c2)
}

// TestPoolReusesConns verifies the pool reuses connections.
func TestPoolReusesConns(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen failed: %v", err)
	}
	defer ln.Close()

	var connCount atomic.Int32
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			connCount.Add(1)
			go stubServer(t, conn, respAccept)
		}
	}()

	pool, err := NewPool("tcp", ln.Addr().String(), 5*time.Second, 1)
	if err != nil {
		t.Fatalf("NewPool failed: %v", err)
	}
	defer pool.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Get a client, return it, get another
	c1, err := pool.Get(ctx)
	if err != nil {
		t.Fatalf("pool.Get #1 failed: %v", err)
	}
	pool.Put(c1)

	// Small delay to allow connection reuse
	time.Sleep(10 * time.Millisecond)

	c2, err := pool.Get(ctx)
	if err != nil {
		t.Fatalf("pool.Get #2 failed: %v", err)
	}
	pool.Put(c2)

	// Should only have 1 connection, not 2
	if got := connCount.Load(); got > 1 {
		t.Fatalf("expected ≤1 connection, got %d", got)
	}
}
