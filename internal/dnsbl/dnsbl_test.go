package dnsbl

import (
	"errors"
	"testing"
)

type mockResolver struct {
	records map[string][]string
	err     error
}

func (m *mockResolver) LookupHost(host string) ([]string, error) {
	if m.err != nil {
		return nil, m.err
	}
	if addrs, ok := m.records[host]; ok {
		return addrs, nil
	}
	return nil, errors.New("NXDOMAIN")
}

func TestCheckListed(t *testing.T) {
	resolver := &mockResolver{
		records: map[string][]string{
			"4.3.2.1.zen.spamhaus.org": {"127.0.0.2"},
		},
	}
	dnsbl := New("zen.spamhaus.org", resolver)

	result, err := dnsbl.Check("1.2.3.4")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Listed {
		t.Fatalf("expected listed")
	}
	if result.Code != "127.0.0.2" {
		t.Fatalf("expected code 127.0.0.2, got %s", result.Code)
	}
	if result.Zone != "zen.spamhaus.org" {
		t.Fatalf("expected zone zen.spamhaus.org, got %s", result.Zone)
	}
}

func TestCheckNotListed(t *testing.T) {
	resolver := &mockResolver{}
	dnsbl := New("zen.spamhaus.org", resolver)

	result, err := dnsbl.Check("1.2.3.4")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Listed {
		t.Fatalf("expected not listed")
	}
	if result.Code != "" {
		t.Fatalf("expected empty code, got %s", result.Code)
	}
}

func TestCheckResolverError(t *testing.T) {
	resolver := &mockResolver{err: errors.New("network error")}
	dnsbl := New("zen.spamhaus.org", resolver)

	_, err := dnsbl.Check("1.2.3.4")
	if err == nil {
		t.Fatalf("expected error")
	}
}

func TestCheckIPv6(t *testing.T) {
	resolver := &mockResolver{
		records: map[string][]string{
			"b.a.0.0.9.8.7.6.5.4.3.2.1.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.zen.spamhaus.org": {"127.0.0.3"},
		},
	}
	dnsbl := New("zen.spamhaus.org", resolver)

	result, err := dnsbl.Check("::1:2345:6789:ab")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Listed {
		t.Fatalf("expected listed")
	}
	if result.Code != "127.0.0.3" {
		t.Fatalf("expected code 127.0.0.3, got %s", result.Code)
	}
}

func TestReverseIPv4(t *testing.T) {
	tests := []struct {
		ip       string
		expected string
	}{
		{"1.2.3.4", "4.3.2.1"},
		{"10.0.0.1", "1.0.0.10"},
		{"192.168.1.1", "1.1.168.192"},
	}

	for _, tt := range tests {
		got, err := reverseIPv4(tt.ip)
		if err != nil {
			t.Fatalf("unexpected error for %s: %v", tt.ip, err)
		}
		if got != tt.expected {
			t.Fatalf("reverseIPv4(%s) = %s; want %s", tt.ip, got, tt.expected)
		}
	}
}

func TestReverseIPv6(t *testing.T) {
	tests := []struct {
		ip       string
		expected string
	}{
		{"::1", "1.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0"},
		{"2001:db8::1", "1.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.8.b.d.0.1.0.0.2"},
	}

	for _, tt := range tests {
		got, err := reverseIPv6(tt.ip)
		if err != nil {
			t.Fatalf("unexpected error for %s: %v", tt.ip, err)
		}
		if got != tt.expected {
			t.Fatalf("reverseIPv6(%s) = %s; want %s", tt.ip, got, tt.expected)
		}
	}
}

func TestNetResolver(t *testing.T) {
	var _ Resolver = NetResolver

	_, err := NetResolver.LookupHost("localhost")
	if err != nil {
		t.Logf("localhost lookup: %v", err)
	}
}

func TestCheckInvalidIP(t *testing.T) {
	resolver := &mockResolver{}
	dnsbl := New("zen.spamhaus.org", resolver)

	_, err := dnsbl.Check("not-an-ip")
	if err == nil {
		t.Fatalf("expected error for invalid IP")
	}
}

func TestCheckMultipleRecords(t *testing.T) {
	resolver := &mockResolver{
		records: map[string][]string{
			"4.3.2.1.zen.spamhaus.org": {"127.0.0.2", "127.0.0.4"},
		},
	}
	dnsbl := New("zen.spamhaus.org", resolver)

	result, err := dnsbl.Check("1.2.3.4")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Listed {
		t.Fatalf("expected listed")
	}
	// Should return the first code.
	if result.Code != "127.0.0.2" {
		t.Fatalf("expected code 127.0.0.2, got %s", result.Code)
	}
}

func TestCheckIPv4WithIPv6Input(t *testing.T) {
	resolver := &mockResolver{}
	dnsbl := New("zen.spamhaus.org", resolver)

	_, err := dnsbl.Check("::ffff:1.2.3.4")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	_ = dnsbl
}
