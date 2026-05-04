package dnscheck

import (
	"context"
	"net"
	"strings"
)

type DNSResolver interface {
	LookupMX(ctx context.Context, name string) ([]*net.MX, error)
	LookupTXT(ctx context.Context, name string) ([]string, error)
}

type NetResolver struct {
	Resolver *net.Resolver
}

func (r NetResolver) resolver() *net.Resolver {
	if r.Resolver != nil {
		return r.Resolver
	}
	return net.DefaultResolver
}

func (r NetResolver) LookupMX(ctx context.Context, name string) ([]*net.MX, error) {
	return r.resolver().LookupMX(ctx, name)
}

func (r NetResolver) LookupTXT(ctx context.Context, name string) ([]string, error) {
	return r.resolver().LookupTXT(ctx, name)
}

type Status string

const (
	StatusOK       Status = "ok"
	StatusMissing  Status = "missing"
	StatusMismatch Status = "mismatch"
	StatusError    Status = "error"
)

type RecordCheck struct {
	Name     string   `json:"name"`
	Host     string   `json:"host"`
	Status   Status   `json:"status"`
	Expected string   `json:"expected,omitempty"`
	Found    []string `json:"found,omitempty"`
	Error    string   `json:"error,omitempty"`
}

type DKIMExpectation struct {
	Selector     string
	PublicKeyDNS string
}

type DomainReport struct {
	Domain string        `json:"domain"`
	MX     RecordCheck   `json:"mx"`
	SPF    RecordCheck   `json:"spf"`
	DMARC  RecordCheck   `json:"dmarc"`
	DKIM   []RecordCheck `json:"dkim"`
}

func (r DomainReport) SummaryStatus() Status {
	status := StatusOK
	for _, check := range append([]RecordCheck{r.MX, r.SPF, r.DMARC}, r.DKIM...) {
		switch check.Status {
		case StatusError:
			return StatusError
		case StatusMismatch:
			status = StatusMismatch
		case StatusMissing:
			if status == StatusOK {
				status = StatusMissing
			}
		}
	}
	return status
}

type Verifier struct {
	DNS DNSResolver
}

func (v Verifier) VerifyDomain(ctx context.Context, domain string, dkim []DKIMExpectation) DomainReport {
	dns := v.DNS
	if dns == nil {
		dns = NetResolver{}
	}
	domain = normalizeDomain(domain)
	report := DomainReport{
		Domain: domain,
		MX:     v.checkMX(ctx, dns, domain),
		SPF:    v.checkTXTPrefix(ctx, dns, domain, "spf", "v=spf1"),
		DMARC:  v.checkTXTPrefix(ctx, dns, "_dmarc."+domain, "dmarc", "v=DMARC1"),
		DKIM:   make([]RecordCheck, 0, len(dkim)),
	}
	for _, key := range dkim {
		selector := strings.TrimSpace(key.Selector)
		host := selector + "._domainkey." + domain
		check := RecordCheck{
			Name:     "dkim",
			Host:     host,
			Expected: strings.TrimSpace(key.PublicKeyDNS),
		}
		if selector == "" {
			check.Status = StatusError
			check.Error = "selector is required"
			report.DKIM = append(report.DKIM, check)
			continue
		}
		report.DKIM = append(report.DKIM, v.checkExpectedTXT(ctx, dns, check))
	}
	return report
}

func (v Verifier) checkMX(ctx context.Context, dns DNSResolver, domain string) RecordCheck {
	check := RecordCheck{Name: "mx", Host: domain}
	mxs, err := dns.LookupMX(ctx, domain)
	if err != nil {
		check.Status = StatusError
		check.Error = err.Error()
		return check
	}
	if len(mxs) == 0 {
		check.Status = StatusMissing
		return check
	}
	check.Status = StatusOK
	for _, mx := range mxs {
		host := strings.TrimSuffix(strings.TrimSpace(mx.Host), ".")
		if host != "" {
			check.Found = append(check.Found, host)
		}
	}
	return check
}

func (v Verifier) checkTXTPrefix(ctx context.Context, dns DNSResolver, host string, name string, prefix string) RecordCheck {
	check := RecordCheck{Name: name, Host: host, Expected: prefix}
	txts, err := dns.LookupTXT(ctx, host)
	if err != nil {
		check.Status = StatusError
		check.Error = err.Error()
		return check
	}
	prefixLower := strings.ToLower(prefix)
	for _, txt := range txts {
		txt = strings.TrimSpace(txt)
		if strings.HasPrefix(strings.ToLower(txt), prefixLower) {
			check.Status = StatusOK
			check.Found = []string{txt}
			return check
		}
	}
	check.Found = append([]string(nil), txts...)
	check.Status = StatusMissing
	return check
}

func (v Verifier) checkExpectedTXT(ctx context.Context, dns DNSResolver, check RecordCheck) RecordCheck {
	txts, err := dns.LookupTXT(ctx, check.Host)
	if err != nil {
		check.Status = StatusError
		check.Error = err.Error()
		return check
	}
	check.Found = append([]string(nil), txts...)
	expected := canonicalTXT(check.Expected)
	if expected == "" {
		check.Status = StatusMissing
		check.Error = "expected TXT record is empty"
		return check
	}
	for _, txt := range txts {
		if canonicalTXT(txt) == expected {
			check.Status = StatusOK
			return check
		}
	}
	if len(txts) == 0 {
		check.Status = StatusMissing
		return check
	}
	check.Status = StatusMismatch
	return check
}

func normalizeDomain(domain string) string {
	return strings.ToLower(strings.TrimSuffix(strings.TrimSpace(domain), "."))
}

func canonicalTXT(value string) string {
	return strings.Join(strings.Fields(strings.TrimSpace(value)), "")
}
