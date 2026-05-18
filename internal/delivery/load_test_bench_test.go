package delivery

import (
	"context"
	"fmt"
	"io"
	"net"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gogomail/gogomail/internal/outbound"
)

// BenchmarkBulkSendThroughput measures delivery throughput with connection pooling and pipelining
func BenchmarkBulkSendThroughput(b *testing.B) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		b.Fatalf("listen: %v", err)
	}
	defer listener.Close()

	addr := listener.Addr().(*net.TCPAddr)
	host := fmt.Sprintf("127.0.0.1:%d", addr.Port)

	// Start mock SMTP server
	successCount := atomic.Int64{}
	go mockBulkSMTPServer(listener, &successCount, 10000)

	// Setup transport with pooling
	transport := NewDirectSMTPTransport()
	transport.Hello = "test.example.com"
	transport.Resolver = mockResolver{hosts: []string{host}}
	transport.pool = NewSMTPConnectionPool(16, 30*time.Second, 5*time.Minute)
	defer transport.pool.Close()

	// Create test messages
	messages := make([]Job, b.N)
	for i := 0; i < b.N; i++ {
		messages[i] = Job{
			QueuedMessage: QueuedMessage{
				MessageID: fmt.Sprintf("msg-%d", i),
				From:      outbound.Address{Email: "sender@example.com"},
				To: []outbound.Address{
					{Email: fmt.Sprintf("recipient%d@example.com", i%10)},
				},
			},
			OpenMessage: func(context.Context) (io.ReadCloser, error) {
				return io.NopCloser(strings.NewReader("Subject: test\r\nFrom: sender@example.com\r\nTo: recipient@example.com\r\n\r\nBody")), nil
			},
		}
	}

	b.ResetTimer()
	b.ReportAllocs()

	// Measure delivery throughput
	delivered := 0
	failed := 0
	for i := 0; i < b.N; i++ {
		err := transport.Deliver(context.Background(), messages[i])
		if err == nil {
			delivered++
		} else {
			failed++
		}
	}

	b.StopTimer()
	elapsed := b.Elapsed()
	throughput := float64(delivered) / elapsed.Seconds()

	b.ReportMetric(throughput, "msg/sec")
	b.Logf("Delivered: %d, Failed: %d, Throughput: %.2f msg/sec", delivered, failed, throughput)

	// Log pool metrics
	hits, misses := transport.pool.Metrics()
	hitRate := float64(hits) / float64(hits+misses) * 100
	b.Logf("Pool metrics - Hits: %d, Misses: %d, Hit rate: %.2f%%", hits, misses, hitRate)
}

// BenchmarkBulkSendWithPipelining demonstrates pipelined RCPT performance
func BenchmarkBulkSendWithPipelining(b *testing.B) {
	tests := []struct {
		name       string
		recipients int
	}{
		{"5_recipients", 5},
		{"10_recipients", 10},
		{"25_recipients", 25},
	}

	for _, tt := range tests {
		b.Run(tt.name, func(b *testing.B) {
			listener, err := net.Listen("tcp", "127.0.0.1:0")
			if err != nil {
				b.Fatalf("listen: %v", err)
			}
			defer listener.Close()

			addr := listener.Addr().(*net.TCPAddr)
			host := fmt.Sprintf("127.0.0.1:%d", addr.Port)

			successCount := atomic.Int64{}
			go mockBulkSMTPServer(listener, &successCount, 10000)

			transport := NewDirectSMTPTransport()
			transport.Hello = "test.example.com"
			transport.Resolver = mockResolver{hosts: []string{host}}
			transport.pool = NewSMTPConnectionPool(16, 30*time.Second, 5*time.Minute)
			defer transport.pool.Close()

			b.ReportAllocs()
			b.ResetTimer()

			for i := 0; i < b.N; i++ {
				recipients := make([]outbound.Address, tt.recipients)
				for j := 0; j < tt.recipients; j++ {
					recipients[j] = outbound.Address{Email: fmt.Sprintf("user%d@example.com", j)}
				}

				job := Job{
					QueuedMessage: QueuedMessage{
						MessageID: fmt.Sprintf("msg-%d", i),
						From:      outbound.Address{Email: "sender@example.com"},
						To:        recipients,
					},
					OpenMessage: func(context.Context) (io.ReadCloser, error) {
						return io.NopCloser(strings.NewReader("Subject: test\r\nFrom: sender@example.com\r\n\r\nBody")), nil
					},
				}

				_ = transport.Deliver(context.Background(), job)
			}

			b.StopTimer()
			elapsed := b.Elapsed()
			throughput := float64(b.N*tt.recipients) / elapsed.Seconds()
			b.ReportMetric(throughput, "rcpt/sec")
			b.Logf("Recipients/sec: %.2f", throughput)
		})
	}
}

func BenchmarkDirectSMTPBatchingVsIndividual(b *testing.B) {
	recipients := benchmarkRecipients(100, 10)
	newTransport := func(transactionCounter *atomic.Int64) *DirectSMTPTransport {
		return &DirectSMTPTransport{
			Router: staticRouter{route: Route{Hosts: []string{"mx.example.test"}}},
			deliverHost: func(_ context.Context, _ Job, _ Route, _ string, recipients []outbound.Address) error {
				transactionCounter.Add(1)
				if len(recipients) == 0 {
					b.Fatal("empty recipient batch")
				}
				return nil
			},
		}
	}

	b.Run("batched_by_domain", func(b *testing.B) {
		var transactions atomic.Int64
		transport := newTransport(&transactions)
		job := Job{
			QueuedMessage: QueuedMessage{
				MessageID: "bulk-message",
				From:      outbound.Address{Email: "sender@example.com"},
				To:        recipients,
			},
		}

		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			if err := transport.Deliver(context.Background(), job); err != nil {
				b.Fatalf("Deliver returned error: %v", err)
			}
		}
		b.StopTimer()
		b.ReportMetric(float64(b.N*len(recipients))/b.Elapsed().Seconds(), "rcpt/sec")
		b.ReportMetric(float64(transactions.Load())/float64(b.N), "smtp_txn/op")
	})

	b.Run("individual_recipients", func(b *testing.B) {
		var transactions atomic.Int64
		transport := newTransport(&transactions)

		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			for _, recipient := range recipients {
				job := Job{
					QueuedMessage: QueuedMessage{
						MessageID: "single-message",
						From:      outbound.Address{Email: "sender@example.com"},
						To:        []outbound.Address{recipient},
					},
				}
				if err := transport.Deliver(context.Background(), job); err != nil {
					b.Fatalf("Deliver returned error: %v", err)
				}
			}
		}
		b.StopTimer()
		b.ReportMetric(float64(b.N*len(recipients))/b.Elapsed().Seconds(), "rcpt/sec")
		b.ReportMetric(float64(transactions.Load())/float64(b.N), "smtp_txn/op")
	})
}

func mockBulkSMTPServer(listener net.Listener, successCount *atomic.Int64, timeout int) {
	for {
		conn, err := listener.Accept()
		if err != nil {
			return
		}
		go func() {
			defer conn.Close()
			handleMockBulkSMTPConn(conn, successCount)
		}()
	}
}

func handleMockBulkSMTPConn(conn net.Conn, successCount *atomic.Int64) {
	defer conn.Close()

	conn.Write([]byte("220 test.example.com SMTP ready\r\n"))

	buf := make([]byte, 4096)
	for {
		conn.SetReadDeadline(time.Now().Add(1 * time.Second))
		n, err := conn.Read(buf)
		if err != nil {
			return
		}

		cmd := string(buf[:n])
		if len(cmd) >= 4 {
			switch cmd[:4] {
			case "HELO", "EHLO":
				conn.Write([]byte("250 OK\r\n"))
			case "MAIL":
				conn.Write([]byte("250 OK\r\n"))
			case "RCPT":
				conn.Write([]byte("250 OK\r\n"))
				successCount.Add(1)
			case "DATA":
				conn.Write([]byte("354 Start mail input\r\n"))
				// Read until \r\n.\r\n
				for {
					n, _ := conn.Read(buf)
					if n >= 5 && string(buf[n-5:n]) == "\r\n.\r\n" {
						break
					}
				}
				conn.Write([]byte("250 OK\r\n"))
				successCount.Add(1)
			case "QUIT":
				conn.Write([]byte("221 Bye\r\n"))
				return
			default:
				conn.Write([]byte("500 Syntax error\r\n"))
			}
		}
	}
}

// TestBulkSendPoolingMetrics verifies connection pool metrics during bulk send
func TestBulkSendPoolingMetrics(t *testing.T) {
	pool := NewSMTPConnectionPool(4, 30*time.Second, 5*time.Minute)
	defer pool.Close()

	// Simulate bulk send accessing the pool
	key := SMTPConnPoolKey{Host: "mail.example.com", Port: 25}

	// Multiple attempts to get should result in misses (no connection yet)
	for i := 0; i < 10; i++ {
		conn, err := pool.Get(context.Background(), key)
		if err != nil {
			t.Fatalf("Get failed: %v", err)
		}
		if conn != nil {
			t.Fatalf("expected nil for empty pool")
		}
	}

	// Check metrics
	hits, misses := pool.Metrics()
	if misses != 10 {
		t.Errorf("expected 10 misses, got %d", misses)
	}
	if hits != 0 {
		t.Errorf("expected 0 hits, got %d", hits)
	}

	t.Logf("Pool metrics - Hits: %d, Misses: %d", hits, misses)
}

type mockResolver struct {
	hosts []string
}

func (m mockResolver) LookupMX(ctx context.Context, domain string) ([]*net.MX, error) {
	return []*net.MX{{Host: m.hosts[0], Pref: 10}}, nil
}
