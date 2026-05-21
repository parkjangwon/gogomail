package delivery

import (
	"fmt"
	"net"
	"net/smtp"
	"testing"
	"time"

	"github.com/gogomail/gogomail/internal/outbound"
)

// BenchmarkSMTPPipelining measures current RCPT performance without pipelining
func BenchmarkSMTPPipelining(b *testing.B) {
	tests := []struct {
		name       string
		recipients int
	}{
		{"5_recipients", 5},
		{"10_recipients", 10},
		{"50_recipients", 50},
		{"100_recipients", 100},
	}

	for _, tt := range tests {
		b.Run(tt.name, func(b *testing.B) {
			benchmarkRCPTPipelining(b, tt.recipients)
		})
	}
}

func BenchmarkPlanRecipientBatches(b *testing.B) {
	tests := []struct {
		name       string
		recipients int
		domains    int
	}{
		{name: "1k_10_domains", recipients: 1_000, domains: 10},
		{name: "10k_100_domains", recipients: 10_000, domains: 100},
		{name: "100k_1k_domains", recipients: 100_000, domains: 1_000},
	}

	for _, tt := range tests {
		recipients := benchmarkRecipients(tt.recipients, tt.domains)
		b.Run(tt.name, func(b *testing.B) {
			b.ReportAllocs()
			b.SetBytes(int64(tt.recipients))
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				batches := PlanRecipientBatches(recipients)
				if len(batches) != tt.domains {
					b.Fatalf("batches = %d, want %d", len(batches), tt.domains)
				}
			}
		})
	}
}

func BenchmarkDSNRCPTOptionsLookup(b *testing.B) {
	recipients := make([]DSNRecipientOptions, 10_000)
	for i := range recipients {
		recipients[i] = DSNRecipientOptions{
			Address:           fmt.Sprintf("user%d@example.test", i),
			Notify:            []string{"FAILURE", "DELAY"},
			OriginalRecipient: fmt.Sprintf("rfc822;user%d+40example.test", i),
		}
	}
	optionsByAddress := dsnRCPTOptionsByAddress(recipients)
	addresses := make([]string, len(recipients))
	for i := range addresses {
		addresses[i] = fmt.Sprintf("USER%d@example.test", i)
	}

	b.ReportAllocs()
	b.SetBytes(int64(len(addresses)))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		options := dsnOptionsForRecipientMap(optionsByAddress, addresses[i%len(addresses)])
		if len(options) != 2 {
			b.Fatalf("options = %+v, want DSN parameters", options)
		}
	}
}

func benchmarkRecipients(count int, domains int) []outbound.Address {
	recipients := make([]outbound.Address, count)
	for i := 0; i < count; i++ {
		recipients[i] = outbound.Address{Email: fmt.Sprintf("user%d@example-%d.test", i, i%domains)}
	}
	return recipients
}

func benchmarkRCPTPipelining(b *testing.B, numRecipients int) {
	// Mock SMTP server that simulates real server latency
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		b.Fatalf("listen: %v", err)
	}
	defer listener.Close()

	addr := listener.Addr().(*net.TCPAddr)
	host := fmt.Sprintf("127.0.0.1:%d", addr.Port)

	// Start mock SMTP server
	go mockSMTPServer(listener, numRecipients)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		conn, err := net.Dial("tcp", host)
		if err != nil {
			b.Fatalf("dial: %v", err)
		}
		client, err := smtp.NewClient(conn, "127.0.0.1")
		if err != nil {
			b.Fatalf("newclient: %v", err)
		}

		// Hello
		if err := client.Hello("test.example.com"); err != nil {
			b.Fatalf("hello: %v", err)
		}

		// Mail
		if err := client.Mail("sender@example.com"); err != nil {
			b.Fatalf("mail: %v", err)
		}

		// RCPT (non-pipelined: sequential)
		for j := 0; j < numRecipients; j++ {
			rcpt := fmt.Sprintf("rcpt%d@example.com", j)
			if err := client.Rcpt(rcpt); err != nil {
				b.Fatalf("rcpt: %v", err)
			}
		}

		// Data
		writer, err := client.Data()
		if err != nil {
			b.Fatalf("data: %v", err)
		}
		fmt.Fprintf(writer, "From: sender@example.com\r\nTo: test@example.com\r\n\r\nTest message\r\n")
		if err := writer.Close(); err != nil {
			b.Fatalf("close: %v", err)
		}

		client.Close()
		conn.Close()
	}
}

func mockSMTPServer(listener net.Listener, expectedRcpts int) {
	for {
		conn, err := listener.Accept()
		if err != nil {
			return
		}
		go handleMockSMTPConn(conn, expectedRcpts)
	}
}

func handleMockSMTPConn(conn net.Conn, expectedRcpts int) {
	defer conn.Close()

	// Send initial greeting
	conn.Write([]byte("220 test.example.com SMTP ready\r\n"))

	// Handle SMTP commands
	buf := make([]byte, 4096)
	rcptCount := 0

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
				// Simulate small delay per RCPT
				time.Sleep(1 * time.Millisecond)
				conn.Write([]byte("250 OK\r\n"))
				rcptCount++
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
				return
			case "QUIT":
				conn.Write([]byte("221 Bye\r\n"))
				return
			default:
				conn.Write([]byte("500 Syntax error\r\n"))
			}
		}
	}
}

// Test that demonstrates recipient batching for bulk operations
func TestAcceptRecipientsTypicalUsage(t *testing.T) {
	recipients := []outbound.Address{
		{Email: "user1@example.com"},
		{Email: "user2@example.com"},
		{Email: "user3@example.com"},
	}

	callCount := 0
	acceptedRecipients, failedRecipients := acceptRecipients(recipients, func(recipient outbound.Address) error {
		callCount++
		return nil
	})

	if callCount != 3 {
		t.Errorf("expected 3 calls, got %d", callCount)
	}
	if len(acceptedRecipients) != 3 {
		t.Errorf("expected 3 accepted, got %d", len(acceptedRecipients))
	}
	if len(failedRecipients) != 0 {
		t.Errorf("expected 0 failures, got %d", len(failedRecipients))
	}
}
