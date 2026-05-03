package mailauth

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/netip"
	"strings"

	"github.com/emersion/go-msgauth/dkim"
	"github.com/emersion/go-msgauth/dmarc"

	smtpd "github.com/gogomail/gogomail/internal/smtp"
)

type DNSResolver interface {
	LookupTXT(ctx context.Context, name string) ([]string, error)
	LookupIP(ctx context.Context, host string) ([]netip.Addr, error)
	LookupMX(ctx context.Context, name string) ([]*net.MX, error)
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

func (r NetResolver) LookupTXT(ctx context.Context, name string) ([]string, error) {
	return r.resolver().LookupTXT(ctx, name)
}

func (r NetResolver) LookupIP(ctx context.Context, host string) ([]netip.Addr, error) {
	ips, err := r.resolver().LookupIPAddr(ctx, host)
	if err != nil {
		return nil, err
	}
	out := make([]netip.Addr, 0, len(ips))
	for _, ip := range ips {
		addr, ok := netip.AddrFromSlice(ip.IP)
		if ok {
			out = append(out, addr.Unmap())
		}
	}
	return out, nil
}

func (r NetResolver) LookupMX(ctx context.Context, name string) ([]*net.MX, error) {
	return r.resolver().LookupMX(ctx, name)
}

type Verifier struct {
	DNS                  DNSResolver
	AuthservID           string
	MaxDKIMVerifications int
	MaxSPFRedirects      int
}

func (v Verifier) VerifyAuthentication(ctx context.Context, req smtpd.AuthenticationRequest) (smtpd.AuthenticationResults, error) {
	dns := v.DNS
	if dns == nil {
		dns = NetResolver{}
	}
	spf := v.verifySPF(ctx, dns, req)
	dkimResult := v.verifyDKIM(ctx, dns, req.RawMessage)
	return smtpd.AuthenticationResults{
		SPF:   spf,
		DKIM:  dkimResult,
		DMARC: v.evaluateDMARC(ctx, dns, req, spf, dkimResult),
	}, nil
}

func (v Verifier) verifySPF(ctx context.Context, dns DNSResolver, req smtpd.AuthenticationRequest) smtpd.AuthCheckResult {
	ip, ok := parseRemoteIP(req.RemoteAddr)
	if !ok {
		return smtpd.AuthCheckResult{Result: smtpd.AuthResultNone, Reason: "remote address has no parseable IP"}
	}
	domain := domainFromAddress(req.EnvelopeFrom)
	if domain == "" {
		domain = "postmaster"
	}
	result, reason := v.evalSPFDomain(ctx, dns, ip, domain, 0)
	return smtpd.AuthCheckResult{Result: result, Reason: reason, Domain: domain, Identifier: req.EnvelopeFrom}
}

func (v Verifier) evalSPFDomain(ctx context.Context, dns DNSResolver, ip netip.Addr, domain string, depth int) (smtpd.AuthResult, string) {
	maxDepth := v.MaxSPFRedirects
	if maxDepth <= 0 {
		maxDepth = 10
	}
	if depth > maxDepth {
		return smtpd.AuthResultPermanent, "spf include/redirect depth exceeded"
	}
	txts, err := dns.LookupTXT(ctx, domain)
	if err != nil {
		return dnsAuthResult(err), "spf txt lookup failed: " + err.Error()
	}
	var records []string
	for _, txt := range txts {
		if strings.HasPrefix(strings.ToLower(strings.TrimSpace(txt)), "v=spf1") {
			records = append(records, strings.TrimSpace(txt))
		}
	}
	if len(records) == 0 {
		return smtpd.AuthResultNone, "no spf record"
	}
	if len(records) > 1 {
		return smtpd.AuthResultPermanent, "multiple spf records"
	}
	return v.evalSPFRecord(ctx, dns, ip, domain, records[0], depth)
}

func (v Verifier) evalSPFRecord(ctx context.Context, dns DNSResolver, ip netip.Addr, domain, record string, depth int) (smtpd.AuthResult, string) {
	terms := strings.Fields(record)
	redirect := ""
	for _, term := range terms[1:] {
		if strings.HasPrefix(term, "redirect=") {
			redirect = strings.TrimPrefix(term, "redirect=")
			continue
		}
		qualifier, mechanism := splitSPFTerm(term)
		name, value, cidr, err := parseMechanism(mechanism)
		if err != nil {
			return smtpd.AuthResultPermanent, err.Error()
		}
		switch name {
		case "all":
			return spfQualifierResult(qualifier), "spf all matched"
		case "ip4", "ip6":
			if matchIPMechanism(ip, value, cidr) {
				return spfQualifierResult(qualifier), "spf " + name + " matched"
			}
		case "include":
			child, _ := v.evalSPFDomain(ctx, dns, ip, value, depth+1)
			if child == smtpd.AuthResultPass {
				return spfQualifierResult(qualifier), "spf include matched " + value
			}
			if child == smtpd.AuthResultTemporary || child == smtpd.AuthResultPermanent {
				return child, "spf include failed " + value
			}
		case "a":
			host := value
			if host == "" {
				host = domain
			}
			if matched, err := matchA(ctx, dns, ip, host, cidr); err != nil {
				return dnsAuthResult(err), "spf a lookup failed: " + err.Error()
			} else if matched {
				return spfQualifierResult(qualifier), "spf a matched " + host
			}
		case "mx":
			host := value
			if host == "" {
				host = domain
			}
			if matched, err := matchMX(ctx, dns, ip, host, cidr); err != nil {
				return dnsAuthResult(err), "spf mx lookup failed: " + err.Error()
			} else if matched {
				return spfQualifierResult(qualifier), "spf mx matched " + host
			}
		default:
			if strings.ContainsAny(name, "=:") {
				continue
			}
			return smtpd.AuthResultPermanent, "unsupported spf mechanism: " + name
		}
	}
	if redirect != "" {
		return v.evalSPFDomain(ctx, dns, ip, redirect, depth+1)
	}
	return smtpd.AuthResultNeutral, "no spf mechanism matched"
}

func (v Verifier) verifyDKIM(ctx context.Context, dns DNSResolver, raw io.Reader) smtpd.AuthCheckResult {
	if raw == nil {
		return smtpd.AuthCheckResult{Result: smtpd.AuthResultNone, Reason: "raw message unavailable"}
	}
	verifications, err := dkim.VerifyWithOptions(bufio.NewReader(raw), &dkim.VerifyOptions{
		LookupTXT:        func(domain string) ([]string, error) { return dns.LookupTXT(ctx, domain) },
		MaxVerifications: v.MaxDKIMVerifications,
	})
	if err != nil && !errors.Is(err, dkim.ErrTooManySignatures) {
		if dkim.IsTempFail(err) {
			return smtpd.AuthCheckResult{Result: smtpd.AuthResultTemporary, Reason: err.Error()}
		}
		if dkim.IsPermFail(err) {
			return smtpd.AuthCheckResult{Result: smtpd.AuthResultPermanent, Reason: err.Error()}
		}
		return smtpd.AuthCheckResult{Result: smtpd.AuthResultFail, Reason: err.Error()}
	}
	if len(verifications) == 0 {
		return smtpd.AuthCheckResult{Result: smtpd.AuthResultNone, Reason: "no dkim signature"}
	}
	best := smtpd.AuthCheckResult{Result: smtpd.AuthResultFail, Reason: "no valid dkim signature"}
	for _, verification := range verifications {
		if verification == nil {
			continue
		}
		if verification.Err == nil {
			return smtpd.AuthCheckResult{
				Result:     smtpd.AuthResultPass,
				Reason:     "dkim signature verified",
				Domain:     strings.ToLower(verification.Domain),
				Identifier: strings.ToLower(verification.Identifier),
			}
		}
		best.Domain = strings.ToLower(verification.Domain)
		best.Identifier = strings.ToLower(verification.Identifier)
		best.Reason = verification.Err.Error()
		if dkim.IsTempFail(verification.Err) {
			best.Result = smtpd.AuthResultTemporary
		} else if dkim.IsPermFail(verification.Err) {
			best.Result = smtpd.AuthResultPermanent
		}
	}
	if errors.Is(err, dkim.ErrTooManySignatures) && best.Result != smtpd.AuthResultPass {
		best.Reason = best.Reason + "; too many dkim signatures"
	}
	return best
}

func (v Verifier) evaluateDMARC(ctx context.Context, dns DNSResolver, req smtpd.AuthenticationRequest, spf smtpd.AuthCheckResult, dkimResult smtpd.AuthCheckResult) smtpd.AuthCheckResult {
	fromDomain := strings.ToLower(strings.TrimSpace(domainFromAddress(req.Parsed.From.Address)))
	if fromDomain == "" {
		return smtpd.AuthCheckResult{Result: smtpd.AuthResultPermanent, Reason: "missing RFC5322 From domain"}
	}
	record, err := dmarc.LookupWithOptions(fromDomain, &dmarc.LookupOptions{
		LookupTXT: func(domain string) ([]string, error) { return dns.LookupTXT(ctx, domain) },
	})
	if err != nil {
		if errors.Is(err, dmarc.ErrNoPolicy) {
			return smtpd.AuthCheckResult{Result: smtpd.AuthResultNone, Reason: "no dmarc policy", Domain: fromDomain}
		}
		if dmarc.IsTempFail(err) {
			return smtpd.AuthCheckResult{Result: smtpd.AuthResultTemporary, Reason: err.Error(), Domain: fromDomain}
		}
		return smtpd.AuthCheckResult{Result: smtpd.AuthResultPermanent, Reason: err.Error(), Domain: fromDomain}
	}
	if spf.Result == smtpd.AuthResultPass && aligned(fromDomain, spf.Domain, record.SPFAlignment) {
		return smtpd.AuthCheckResult{Result: smtpd.AuthResultPass, Reason: "spf aligned", Domain: fromDomain, Policy: string(record.Policy)}
	}
	if dkimResult.Result == smtpd.AuthResultPass && aligned(fromDomain, dkimResult.Domain, record.DKIMAlignment) {
		return smtpd.AuthCheckResult{Result: smtpd.AuthResultPass, Reason: "dkim aligned", Domain: fromDomain, Policy: string(record.Policy)}
	}
	return smtpd.AuthCheckResult{Result: smtpd.AuthResultFail, Reason: "no aligned spf or dkim pass", Domain: fromDomain, Policy: string(record.Policy)}
}

func parseRemoteIP(remote string) (netip.Addr, bool) {
	remote = strings.TrimSpace(remote)
	if host, _, err := net.SplitHostPort(remote); err == nil {
		remote = host
	}
	addr, err := netip.ParseAddr(strings.Trim(remote, "[]"))
	if err != nil {
		return netip.Addr{}, false
	}
	return addr.Unmap(), true
}

func domainFromAddress(address string) string {
	_, domain, ok := strings.Cut(strings.TrimSpace(address), "@")
	if !ok {
		return ""
	}
	return strings.ToLower(strings.Trim(domain, " <>"))
}

func splitSPFTerm(term string) (byte, string) {
	if term == "" {
		return '+', term
	}
	switch term[0] {
	case '+', '-', '~', '?':
		return term[0], term[1:]
	default:
		return '+', term
	}
}

func spfQualifierResult(q byte) smtpd.AuthResult {
	switch q {
	case '-':
		return smtpd.AuthResultFail
	case '~':
		return smtpd.AuthResultNeutral
	case '?':
		return smtpd.AuthResultNeutral
	default:
		return smtpd.AuthResultPass
	}
}

func parseMechanism(mechanism string) (name string, value string, cidr int, err error) {
	cidr = -1
	if before, after, ok := strings.Cut(mechanism, "/"); ok {
		mechanism = before
		if _, err := fmt.Sscanf(after, "%d", &cidr); err != nil {
			return "", "", -1, fmt.Errorf("invalid spf cidr: %s", after)
		}
	}
	name, value, _ = strings.Cut(mechanism, ":")
	return strings.ToLower(name), strings.ToLower(value), cidr, nil
}

func matchIPMechanism(ip netip.Addr, value string, cidr int) bool {
	network, err := netip.ParsePrefix(value)
	if err != nil {
		addr, err := netip.ParseAddr(value)
		if err != nil {
			return false
		}
		bits := addr.BitLen()
		if cidr >= 0 {
			bits = cidr
		}
		network = netip.PrefixFrom(addr.Unmap(), bits)
	}
	return network.Contains(ip)
}

func matchA(ctx context.Context, dns DNSResolver, ip netip.Addr, host string, cidr int) (bool, error) {
	ips, err := dns.LookupIP(ctx, host)
	if err != nil {
		return false, err
	}
	for _, candidate := range ips {
		if ipMatches(ip, candidate, cidr) {
			return true, nil
		}
	}
	return false, nil
}

func matchMX(ctx context.Context, dns DNSResolver, ip netip.Addr, host string, cidr int) (bool, error) {
	mxs, err := dns.LookupMX(ctx, host)
	if err != nil {
		return false, err
	}
	for _, mx := range mxs {
		if matched, err := matchA(ctx, dns, ip, strings.TrimSuffix(mx.Host, "."), cidr); err != nil {
			return false, err
		} else if matched {
			return true, nil
		}
	}
	return false, nil
}

func ipMatches(ip, candidate netip.Addr, cidr int) bool {
	candidate = candidate.Unmap()
	if cidr < 0 {
		return ip == candidate
	}
	return netip.PrefixFrom(candidate, cidr).Contains(ip)
}

func dnsAuthResult(err error) smtpd.AuthResult {
	var dnsErr *net.DNSError
	if errors.As(err, &dnsErr) && dnsErr.IsTemporary {
		return smtpd.AuthResultTemporary
	}
	return smtpd.AuthResultPermanent
}

func aligned(fromDomain, authDomain string, mode dmarc.AlignmentMode) bool {
	fromDomain = strings.ToLower(strings.TrimSuffix(strings.TrimSpace(fromDomain), "."))
	authDomain = strings.ToLower(strings.TrimSuffix(strings.TrimSpace(authDomain), "."))
	if fromDomain == "" || authDomain == "" {
		return false
	}
	if mode == dmarc.AlignmentStrict {
		return fromDomain == authDomain
	}
	return fromDomain == authDomain || strings.HasSuffix(authDomain, "."+fromDomain) || strings.HasSuffix(fromDomain, "."+authDomain)
}
