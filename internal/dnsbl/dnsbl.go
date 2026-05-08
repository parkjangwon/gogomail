package dnsbl

import (
	"fmt"
	"net"
	"strings"
)

// Resolver abstracts DNS lookups for testability.
type Resolver interface {
	LookupHost(host string) ([]string, error)
}

// netResolver wraps net.LookupHost to satisfy Resolver.
type netResolver struct{}

func (netResolver) LookupHost(host string) ([]string, error) {
	return net.LookupHost(host)
}

// NetResolver is the default DNS resolver.
var NetResolver Resolver = netResolver{}

// Result is the outcome of a DNSBL check.
type Result struct {
	Listed bool
	Code   string
	Zone   string
}

// DNSBL performs DNS-based blacklist lookups.
type DNSBL struct {
	zone     string
	resolver Resolver
}

// New creates a DNSBL checker for the given zone.
func New(zone string, resolver Resolver) *DNSBL {
	return &DNSBL{
		zone:     zone,
		resolver: resolver,
	}
}

// Check queries the DNSBL for the given IP address.
func (d *DNSBL) Check(ip string) (Result, error) {
	parsed := net.ParseIP(ip)
	if parsed == nil {
		return Result{}, fmt.Errorf("invalid IP address: %s", ip)
	}

	var reversed string
	if parsed.To4() != nil {
		reversed = reverseIPv4Str(parsed.To4())
	} else {
		reversed = reverseIPv6Str(parsed)
	}

	query := fmt.Sprintf("%s.%s", reversed, d.zone)
	addrs, err := d.resolver.LookupHost(query)
	if err != nil {
		if isNotFound(err) {
			return Result{Listed: false, Zone: d.zone}, nil
		}
		return Result{}, fmt.Errorf("dnsbl lookup %s: %w", query, err)
	}

	if len(addrs) == 0 {
		return Result{Listed: false, Zone: d.zone}, nil
	}

	return Result{Listed: true, Code: addrs[0], Zone: d.zone}, nil
}

func reverseIPv4Str(ip net.IP) string {
	return fmt.Sprintf("%d.%d.%d.%d", ip[3], ip[2], ip[1], ip[0])
}

func reverseIPv6Str(ip net.IP) string {
	ip = ip.To16()
	var b strings.Builder
	for i := len(ip) - 1; i >= 0; i-- {
		if i < len(ip)-1 {
			b.WriteByte('.')
		}
		fmt.Fprintf(&b, "%x.%x", ip[i]&0x0f, ip[i]>>4)
	}
	return b.String()
}

func isNotFound(err error) bool {
	if err == nil {
		return false
	}
	s := err.Error()
	return strings.Contains(s, "NXDOMAIN") || strings.Contains(s, "no such host") || strings.Contains(s, "not found")
}

func reverseIPv4(ip string) (string, error) {
	parsed := net.ParseIP(ip)
	if parsed == nil || parsed.To4() == nil {
		return "", fmt.Errorf("invalid IPv4 address: %s", ip)
	}
	return reverseIPv4Str(parsed.To4()), nil
}

func reverseIPv6(ip string) (string, error) {
	parsed := net.ParseIP(ip)
	if parsed == nil || parsed.To4() != nil {
		return "", fmt.Errorf("invalid IPv6 address: %s", ip)
	}
	return reverseIPv6Str(parsed), nil
}
