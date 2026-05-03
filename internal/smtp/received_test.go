package smtpd

import (
	"strings"
	"testing"
	"time"
)

func TestBuildReceivedHeader(t *testing.T) {
	t.Parallel()

	header := BuildReceivedHeader("203.0.113.10:25", "mx.example.com", "abc123", time.Date(2026, 5, 3, 9, 0, 0, 0, time.UTC))

	if !strings.HasPrefix(header, "Received: from 203.0.113.10 by mx.example.com with ESMTP id abc123; ") {
		t.Fatalf("header = %q", header)
	}
	if !strings.HasSuffix(header, "\r\n") {
		t.Fatalf("header does not end with CRLF: %q", header)
	}
	if strings.Contains(header[:len(header)-2], "\n") {
		t.Fatalf("header contains embedded newline: %q", header)
	}
}

func TestBuildReceivedHeaderWithProtocol(t *testing.T) {
	t.Parallel()

	header := BuildReceivedHeaderWithProtocol("client.example.com", "submit.example.com", "ESMTPA", "id-1", time.Date(2026, 5, 3, 9, 0, 0, 0, time.UTC))

	if !strings.Contains(header, " with ESMTPA id id-1; ") {
		t.Fatalf("header = %q", header)
	}
}
