package httpapi

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"time"
)

// DNSResolver is the interface for SRV record lookups (matches *net.Resolver).
type DNSResolver interface {
	LookupSRV(ctx context.Context, service, proto, name string) (string, []*net.SRV, error)
}

// SRVResult holds one DNS SRV record response.
type SRVResult struct {
	Host     string `json:"host"`
	Port     uint16 `json:"port"`
	Priority uint16 `json:"priority"`
	Weight   uint16 `json:"weight"`
}

// AutodiscoveryReport is the result of a CalDAV/CardDAV SRV autodiscovery check.
type AutodiscoveryReport struct {
	Domain       string      `json:"domain"`
	CalDAVSRV    []SRVResult `json:"caldav_srv"`
	CardDAVSRV   []SRVResult `json:"carddav_srv"`
	CalDAVFound  bool        `json:"caldav_srv_found"`
	CardDAVFound bool        `json:"carddav_srv_found"`
	CheckedAt    time.Time   `json:"checked_at"`
}

// DNSDiscoveryChecker performs RFC 6764 §3 SRV autodiscovery probes.
type DNSDiscoveryChecker struct {
	Resolver DNSResolver
}

// NewDNSDiscoveryChecker returns a checker using the default system resolver.
func NewDNSDiscoveryChecker() *DNSDiscoveryChecker {
	return &DNSDiscoveryChecker{Resolver: net.DefaultResolver}
}

// LookupCalDAVSRV queries _caldavs._tcp.{domain} DNS SRV records (RFC 6764 §3).
func (c *DNSDiscoveryChecker) LookupCalDAVSRV(ctx context.Context, domain string) ([]SRVResult, error) {
	return c.lookupSRV(ctx, "caldavs", domain)
}

// LookupCardDAVSRV queries _carddavs._tcp.{domain} DNS SRV records (RFC 6764 §3).
func (c *DNSDiscoveryChecker) LookupCardDAVSRV(ctx context.Context, domain string) ([]SRVResult, error) {
	return c.lookupSRV(ctx, "carddavs", domain)
}

func (c *DNSDiscoveryChecker) lookupSRV(ctx context.Context, service, domain string) ([]SRVResult, error) {
	_, addrs, err := c.Resolver.LookupSRV(ctx, service, "tcp", domain)
	if err != nil {
		return nil, fmt.Errorf("_%s._tcp.%s: %w", service, domain, err)
	}
	results := make([]SRVResult, len(addrs))
	for i, a := range addrs {
		results[i] = SRVResult{
			Host:     a.Target,
			Port:     a.Port,
			Priority: a.Priority,
			Weight:   a.Weight,
		}
	}
	return results, nil
}

// CheckAutodiscovery probes _caldavs._tcp and _carddavs._tcp for the given domain.
func (c *DNSDiscoveryChecker) CheckAutodiscovery(ctx context.Context, domain string) AutodiscoveryReport {
	rep := AutodiscoveryReport{
		Domain:    domain,
		CheckedAt: time.Now().UTC(),
	}
	if caldav, err := c.LookupCalDAVSRV(ctx, domain); err == nil {
		rep.CalDAVSRV = caldav
		rep.CalDAVFound = len(caldav) > 0
	}
	if carddav, err := c.LookupCardDAVSRV(ctx, domain); err == nil {
		rep.CardDAVSRV = carddav
		rep.CardDAVFound = len(carddav) > 0
	}
	return rep
}

// RegisterAutodiscoveryRoutes mounts GET /admin/v1/autodiscovery/check on mux.
func RegisterAutodiscoveryRoutes(mux *http.ServeMux, adminToken string, checker *DNSDiscoveryChecker) {
	mux.HandleFunc("GET /admin/v1/autodiscovery/check", adminAuth(adminToken, func(w http.ResponseWriter, r *http.Request) {
		domain := r.URL.Query().Get("domain")
		if domain == "" {
			writeError(w, http.StatusBadRequest, "domain parameter required")
			return
		}
		report := checker.CheckAutodiscovery(r.Context(), domain)
		writeJSON(w, http.StatusOK, report)
	}))
}
