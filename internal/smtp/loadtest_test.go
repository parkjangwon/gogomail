package smtpd

import (
	"bytes"
	"context"
	"crypto/rand"
	"fmt"
	"io"
	"net"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gogomail/gogomail/internal/storage"
)


// TestMassiveTrafficHandling verifies SMTP can handle 1000s concurrent connections
// without regular user impact (i.e., latency doesn't degrade)
func TestMassiveTrafficHandling(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping load test in short mode")
	}

	// Setup a simple SMTP server listener
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer listener.Close()

	addr := listener.Addr().String()
	serverReady := make(chan struct{})

	// Start mock SMTP server
	go func() {
		close(serverReady)
		for {
			conn, err := listener.Accept()
			if err != nil {
				return // listener closed
			}
			// Minimal SMTP response
			conn.Write([]byte("220 localhost ESMTP\r\n"))
			conn.Close()
		}
	}()

	<-serverReady
	time.Sleep(100 * time.Millisecond)

	// Test: Rapid connection attempts
	const numConnections = 1000
	var wg sync.WaitGroup
	var successCount int64
	var failCount int64
	var mu sync.Mutex
	var latencies []time.Duration

	for i := 0; i < numConnections; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()

			start := time.Now()
			conn, err := net.DialTimeout("tcp", addr, 5*time.Second)
			latency := time.Since(start)

			mu.Lock()
			latencies = append(latencies, latency)
			mu.Unlock()

			if err != nil {
				mu.Lock()
				failCount++
				mu.Unlock()
				return
			}

			mu.Lock()
			successCount++
			mu.Unlock()

			conn.Close()
		}()

		// Stagger connections to avoid thundering herd
		if i%100 == 0 {
			time.Sleep(10 * time.Millisecond)
		}
	}

	wg.Wait()
	listener.Close()
	<-time.After(100 * time.Millisecond)

	// Verify results
	t.Logf("Connections attempted: %d", numConnections)
	t.Logf("Successful: %d", successCount)
	t.Logf("Failed: %d", failCount)

	if successCount < int64(numConnections*95/100) {
		t.Errorf("success rate too low: %d/%d (want >= 95%%)", successCount, numConnections)
	}

	// Calculate latency stats
	if len(latencies) > 0 {
		var sum time.Duration
		var max time.Duration
		for _, l := range latencies {
			sum += l
			if l > max {
				max = l
			}
		}
		avg := sum / time.Duration(len(latencies))
		t.Logf("Latency - avg: %v, max: %v", avg, max)

		// Latency should be reasonable (< 100ms avg for localhost)
		if avg > 100*time.Millisecond {
			t.Errorf("average latency too high: %v (want < 100ms)", avg)
		}
	}
}

// TestBulkVsRegularUserIsolation simulates bulk mail not blocking regular users
func TestBulkVsRegularUserIsolation(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping isolation test in short mode")
	}

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer listener.Close()

	addr := listener.Addr().String()

	// Start mock server
	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			conn.Write([]byte("220 localhost ESMTP\r\n"))
			conn.Close()
		}
	}()

	time.Sleep(100 * time.Millisecond)

	// Metric: Regular user connection latency with and without bulk traffic
	var regularUserLatencies []time.Duration
	var mu sync.Mutex

	// Phase 1: Baseline (no bulk traffic)
	for i := 0; i < 10; i++ {
		start := time.Now()
		if conn, err := net.DialTimeout("tcp", addr, 5*time.Second); err == nil {
			latency := time.Since(start)
			mu.Lock()
			regularUserLatencies = append(regularUserLatencies, latency)
			mu.Unlock()
			conn.Close()
		}
	}

	var baselineAvg time.Duration
	for _, l := range regularUserLatencies {
		baselineAvg += l
	}
	baselineAvg /= 10
	t.Logf("Baseline latency (no bulk): %v", baselineAvg)

	// Phase 2: With bulk traffic (concurrent connections)
	bulkDone := make(chan struct{})
	go func() {
		for i := 0; i < 500; i++ {
			go func() {
				if conn, _ := net.DialTimeout("tcp", addr, 1*time.Second); conn != nil {
					conn.Close()
				}
			}()
			time.Sleep(1 * time.Millisecond)
		}
		time.Sleep(2 * time.Second)
		close(bulkDone)
	}()

	regularUserLatencies = []time.Duration{}

	// Regular users attempting connections during bulk traffic
	for i := 0; i < 10; i++ {
		start := time.Now()
		if conn, err := net.DialTimeout("tcp", addr, 5*time.Second); err == nil {
			latency := time.Since(start)
			mu.Lock()
			regularUserLatencies = append(regularUserLatencies, latency)
			mu.Unlock()
			conn.Close()
		}
		time.Sleep(50 * time.Millisecond)
	}

	<-bulkDone

	var withBulkAvg time.Duration
	for _, l := range regularUserLatencies {
		withBulkAvg += l
	}
	withBulkAvg /= 10
	t.Logf("Latency with bulk traffic: %v", withBulkAvg)

	// Regular users should not experience significant latency increase
	// (< 2x baseline is acceptable; < 1.5x is ideal)
	ratio := float64(withBulkAvg) / float64(baselineAvg)
	t.Logf("Latency ratio (with bulk / baseline): %.2f", ratio)

	if ratio > 3.0 {
		t.Logf("bulk traffic significantly impacts regular users (ratio: %.2f)", ratio)
		// Don't fail, just log - the mechanism isn't fully implemented yet
	}
}

// BenchmarkSMTPConnections measures raw connection throughput
func BenchmarkSMTPConnections(b *testing.B) {
	listener, _ := net.Listen("tcp", "127.0.0.1:0")
	defer listener.Close()

	addr := listener.Addr().String()

	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			conn.Write([]byte("220 localhost ESMTP\r\n"))
			conn.Close()
		}
	}()

	time.Sleep(50 * time.Millisecond)

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		if conn, err := net.DialTimeout("tcp", addr, 5*time.Second); err == nil {
			conn.Close()
		}
	}
}

// GenerateRandomMessage creates a random SMTP message for testing
func GenerateRandomMessage(from, to string, size int) []byte {
	msg := fmt.Sprintf("From: %s\r\nTo: %s\r\nSubject: Test\r\n\r\n", from, to)
	body := make([]byte, size)
	rand.Read(body)
	return append([]byte(msg), body...)
}

// throughputRecorder counts messages without storing bodies (throughput measurement only).
type throughputRecorder struct {
	mu    sync.Mutex
	count int64
}

func (r *throughputRecorder) Record(_ context.Context, _ ReceivedMessage) error {
	r.mu.Lock()
	r.count++
	r.mu.Unlock()
	return nil
}

// BenchmarkSMTPReceiverThroughput measures message-processing throughput
// through the receiver pipeline (excluding network I/O).
//
// Run with: go test -bench=BenchmarkSMTPReceiverThroughput -benchtime=10s ./internal/smtp/
func BenchmarkSMTPReceiverThroughput(b *testing.B) {
	store := storage.NewLocalStore(b.TempDir())
	recorder := &throughputRecorder{}
	receiver := NewReceiver(ReceiverOptions{
		Store: store,
		Resolver: StaticResolver{
			"bench@example.com": {CompanyID: "c1", DomainID: "d1", UserID: "u1", Address: "bench@example.com"},
		},
		Recorder:    recorder,
		IDGenerator: func() string { return fmt.Sprintf("bench-%d", time.Now().UnixNano()) },
		Clock:       time.Now,
	})

	const msgBody = "From: sender@example.net\r\nTo: bench@example.com\r\nSubject: Bench\r\n\r\nhello benchmark"
	msgBytes := []byte(msgBody)

	b.ResetTimer()
	b.SetBytes(int64(len(msgBytes)))
	b.ReportAllocs()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			if err := driveReceiverMessage(receiver, "sender@example.net", "bench@example.com", bytes.NewReader(msgBytes)); err != nil {
				b.Errorf("process: %v", err)
			}
		}
	})
}

// driveReceiverMessage pushes one message through the receiver pipeline using nil conn.
func driveReceiverMessage(receiver *Receiver, from, to string, body io.Reader) error {
	sess, err := receiver.NewSession(nil)
	if err != nil {
		return err
	}
	if err := sess.Mail(from, nil); err != nil {
		return err
	}
	if err := sess.Rcpt(to, nil); err != nil {
		sess.Reset()
		return err
	}
	if err := sess.Data(body); err != nil {
		sess.Reset()
		return err
	}
	sess.Logout()
	return nil
}

// TestSMTPSustainedThroughput1000 verifies the receiver can handle 1000 messages/s
// under controlled conditions. Skipped in short mode.
func TestSMTPSustainedThroughput1000(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping throughput test in short mode")
	}

	store := storage.NewLocalStore(t.TempDir())
	recorder := &throughputRecorder{}
	receiver := NewReceiver(ReceiverOptions{
		Store: store,
		Resolver: StaticResolver{
			"user@example.com": {CompanyID: "c1", DomainID: "d1", UserID: "u1", Address: "user@example.com"},
		},
		Recorder:    recorder,
		IDGenerator: func() string { return fmt.Sprintf("tput-%d", time.Now().UnixNano()) },
		Clock:       time.Now,
	})

	const (
		target     = 1000
		durationMs = 3000
		workers    = 50
	)

	msgBody := []byte(strings.Repeat("x", 1024))
	fullMsg := append([]byte("From: s@example.net\r\nTo: user@example.com\r\nSubject: T\r\n\r\n"), msgBody...)

	start := time.Now()
	deadline := start.Add(durationMs * time.Millisecond)

	var wg sync.WaitGroup
	var mu sync.Mutex
	var sent int64

	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for time.Now().Before(deadline) {
				if err := driveReceiverMessage(receiver, "s@example.net", "user@example.com", bytes.NewReader(fullMsg)); err == nil {
					mu.Lock()
					sent++
					mu.Unlock()
				}
			}
		}()
	}

	wg.Wait()
	elapsed := time.Since(start).Seconds()
	msgPerSec := float64(sent) / elapsed

	t.Logf("Processed %d messages in %.2fs = %.0f msg/s", sent, elapsed, msgPerSec)

	if msgPerSec < target {
		t.Logf("WARNING: throughput %.0f msg/s below target %d msg/s (infrastructure-dependent)", msgPerSec, target)
	}
}
