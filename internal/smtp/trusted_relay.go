package smtpd

import (
	"context"
	"fmt"
	"net/netip"
	"strings"
)

type StaticTrustedRelays struct {
	prefixes []netip.Prefix
}

func NewStaticTrustedRelays(values []string) (StaticTrustedRelays, error) {
	prefixes := make([]netip.Prefix, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		prefix, err := parseTrustedRelayPrefix(value)
		if err != nil {
			return StaticTrustedRelays{}, err
		}
		prefixes = append(prefixes, prefix)
	}
	return StaticTrustedRelays{prefixes: prefixes}, nil
}

func (r StaticTrustedRelays) AllowRelay(_ context.Context, remoteAddr string) (bool, error) {
	if len(r.prefixes) == 0 {
		return true, nil
	}
	addr, err := parseRelayRemoteAddr(remoteAddr)
	if err != nil {
		return false, nil
	}
	for _, prefix := range r.prefixes {
		if prefix.Contains(addr) {
			return true, nil
		}
	}
	return false, nil
}

func parseRelayRemoteAddr(remoteAddr string) (netip.Addr, error) {
	remoteAddr = strings.TrimSpace(remoteAddr)
	if addr, err := netip.ParseAddr(remoteAddr); err == nil {
		return addr.Unmap(), nil
	}
	addrPort, err := netip.ParseAddrPort(remoteAddr)
	if err != nil {
		return netip.Addr{}, err
	}
	return addrPort.Addr().Unmap(), nil
}

func parseTrustedRelayPrefix(value string) (netip.Prefix, error) {
	if prefix, err := netip.ParsePrefix(value); err == nil {
		return prefix.Masked(), nil
	}
	addr, err := netip.ParseAddr(value)
	if err != nil {
		return netip.Prefix{}, fmt.Errorf("invalid trusted relay %q: %w", value, err)
	}
	if addr.Is4() {
		return netip.PrefixFrom(addr, 32), nil
	}
	return netip.PrefixFrom(addr, 128), nil
}
