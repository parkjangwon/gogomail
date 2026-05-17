package webhook

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"strings"
	"time"
)

const (
	DefaultOutboundTimeout      = 10 * time.Second
	DefaultOutboundMaxRedirects = 3
)

type OutboundURLGuardOptions struct {
	AllowPrivateNetwork bool
	LookupIPAddr        func(context.Context, string) ([]net.IPAddr, error)
}

func ValidateOutboundHTTPURL(ctx context.Context, raw string, opts OutboundURLGuardOptions) (*url.URL, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, fmt.Errorf("url is required")
	}
	if strings.ContainsAny(raw, "\r\n") {
		return nil, fmt.Errorf("url cannot contain line breaks")
	}
	parsed, err := url.Parse(raw)
	if err != nil {
		return nil, fmt.Errorf("url must be valid: %w", err)
	}
	if (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Hostname() == "" {
		return nil, fmt.Errorf("url must be an http or https URL")
	}
	if opts.AllowPrivateNetwork {
		return parsed, nil
	}
	if err := rejectPrivateHost(ctx, parsed.Hostname(), opts.LookupIPAddr); err != nil {
		return nil, err
	}
	return parsed, nil
}

func GuardedHTTPClient(base *http.Client, opts OutboundURLGuardOptions) *http.Client {
	var client http.Client
	if base != nil {
		client = *base
	} else {
		client = *http.DefaultClient
	}
	if client.Timeout == 0 {
		client.Timeout = DefaultOutboundTimeout
	}
	previousRedirect := client.CheckRedirect
	client.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		if len(via) >= DefaultOutboundMaxRedirects {
			return fmt.Errorf("too many redirects")
		}
		if _, err := ValidateOutboundHTTPURL(req.Context(), req.URL.String(), opts); err != nil {
			return err
		}
		if previousRedirect != nil {
			return previousRedirect(req, via)
		}
		return nil
	}
	return &client
}

func rejectPrivateHost(ctx context.Context, host string, lookup func(context.Context, string) ([]net.IPAddr, error)) error {
	host = strings.TrimSpace(strings.TrimSuffix(host, "."))
	if host == "" {
		return fmt.Errorf("url host is required")
	}
	if strings.EqualFold(host, "localhost") {
		return fmt.Errorf("url host resolves to a private address")
	}
	if addr, err := netip.ParseAddr(host); err == nil {
		if isPrivateOutboundAddr(addr) {
			return fmt.Errorf("url host resolves to a private address")
		}
		return nil
	}
	if lookup == nil {
		lookup = net.DefaultResolver.LookupIPAddr
	}
	lookupCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	addrs, err := lookup(lookupCtx, host)
	if err != nil {
		return fmt.Errorf("resolve url host: %w", err)
	}
	if len(addrs) == 0 {
		return fmt.Errorf("url host did not resolve")
	}
	for _, ip := range addrs {
		addr, ok := netip.AddrFromSlice(ip.IP)
		if !ok || isPrivateOutboundAddr(addr.Unmap()) {
			return fmt.Errorf("url host resolves to a private address")
		}
	}
	return nil
}

func isPrivateOutboundAddr(addr netip.Addr) bool {
	if !addr.IsValid() {
		return true
	}
	return addr.IsLoopback() ||
		addr.IsPrivate() ||
		addr.IsLinkLocalUnicast() ||
		addr.IsLinkLocalMulticast() ||
		addr.IsMulticast() ||
		addr.IsUnspecified() ||
		addr.IsInterfaceLocalMulticast() ||
		addr.Is4In6() ||
		addr == netip.MustParseAddr("169.254.169.254")
}
