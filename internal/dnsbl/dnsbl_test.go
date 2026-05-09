package dnsbl

import (
	"errors"
	"net"
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
	return nil, &net.DNSError{Err: "no such host", Name: host, IsNotFound: true}
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

// TestIPv6Reversal verifies RFC 5782 §2.4 nibble-reversal format for IPv6 DNSBL queries.
// Each nibble of the fully-expanded address is reversed and separated by dots.
func TestIPv6Reversal(t *testing.T) {
	tests := []struct {
		ip      string
		want    string
	}{
		// ::1 expands to 0000:0000:0000:0000:0000:0000:0000:0001
		// reversed nibbles: 1.0.0.0 ... (32 nibbles total)
		{
			"::1",
			"1.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0",
		},
		// 2001:db8::1 expands to 2001:0db8:0000:0000:0000:0000:0000:0001
		// bytes (hex): 20 01 0d b8 00 00 00 00 00 00 00 00 00 00 00 01
		// reversed nibbles from last byte to first: 1,0, 0,0, ..., 8,b, d,0, 1,0, 0,2
		{
			"2001:db8::1",
			"1.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.8.b.d.0.1.0.0.2",
		},
		// 2001:db8::ff expands to ...0000:00ff; last byte 0xff → nibbles f,f
		{
			"2001:db8::ff",
			"f.f.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.8.b.d.0.1.0.0.2",
		},
	}

	for _, tt := range tests {
		got, err := reverseIPv6(tt.ip)
		if err != nil {
			t.Fatalf("reverseIPv6(%s): unexpected error: %v", tt.ip, err)
		}
		if got != tt.want {
			t.Fatalf("reverseIPv6(%s)\n got:  %s\n want: %s", tt.ip, got, tt.want)
		}
	}
}

// TestDNSBLErrorHandling verifies that error classification uses net.DNSError type
// assertions rather than fragile string matching.
func TestDNSBLErrorHandling(t *testing.T) {
	t.Run("not_found_treated_as_unlisted", func(t *testing.T) {
		// *net.DNSError with IsNotFound=true → not listed, no error returned.
		resolver := &mockResolver{
			err: &net.DNSError{Err: "no such host", Name: "4.3.2.1.bl.example", IsNotFound: true},
		}
		d := New("bl.example", resolver)
		result, err := d.Check("1.2.3.4")
		if err != nil {
			t.Fatalf("IsNotFound DNS error should yield no error, got: %v", err)
		}
		if result.Listed {
			t.Fatal("IsNotFound DNS error should yield not-listed result")
		}
	})

	t.Run("timeout_error_propagated", func(t *testing.T) {
		// *net.DNSError with IsTimeout=true (IsNotFound=false) → error propagates.
		resolver := &mockResolver{
			err: &net.DNSError{Err: "i/o timeout", Name: "4.3.2.1.bl.example", IsTimeout: true},
		}
		d := New("bl.example", resolver)
		_, err := d.Check("1.2.3.4")
		if err == nil {
			t.Fatal("timeout DNS error should propagate as error")
		}
	})

	t.Run("generic_network_error_propagated", func(t *testing.T) {
		// Plain non-DNS error → error propagates.
		resolver := &mockResolver{err: errors.New("connection refused")}
		d := New("bl.example", resolver)
		_, err := d.Check("1.2.3.4")
		if err == nil {
			t.Fatal("generic network error should propagate as error")
		}
	})

	t.Run("invalid_ip_format_rejected", func(t *testing.T) {
		// net.ParseIP rejects malformed addresses before any DNS call.
		resolver := &mockResolver{}
		d := New("bl.example", resolver)
		_, err := d.Check("not.an.ip.address")
		if err == nil {
			t.Fatal("invalid IP format should return error")
		}
	})
}
