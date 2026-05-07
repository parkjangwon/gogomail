package mailauth

import (
	"context"
	"net"
	"net/netip"
	"strings"
	"testing"

	"github.com/gogomail/gogomail/internal/message"
	smtpd "github.com/gogomail/gogomail/internal/smtp"
)

type fakeDNS struct {
	txt map[string][]string
	ip  map[string][]netip.Addr
	mx  map[string][]*net.MX
}

func (d fakeDNS) LookupTXT(_ context.Context, name string) ([]string, error) {
	if records, ok := d.txt[strings.ToLower(name)]; ok {
		return records, nil
	}
	return nil, &net.DNSError{Name: name, IsNotFound: true}
}

func (d fakeDNS) LookupIP(_ context.Context, host string) ([]netip.Addr, error) {
	if records, ok := d.ip[strings.ToLower(host)]; ok {
		return records, nil
	}
	return nil, &net.DNSError{Name: host, IsNotFound: true}
}

func (d fakeDNS) LookupMX(_ context.Context, name string) ([]*net.MX, error) {
	if records, ok := d.mx[strings.ToLower(name)]; ok {
		return records, nil
	}
	return nil, &net.DNSError{Name: name, IsNotFound: true}
}

func TestVerifierSPFPassAndDMARCAligned(t *testing.T) {
	verifier := Verifier{DNS: fakeDNS{txt: map[string][]string{
		"example.com":        {"v=spf1 ip4:192.0.2.10 -all"},
		"_dmarc.example.com": {"v=DMARC1; p=reject; aspf=s"},
	}}}

	results, err := verifier.VerifyAuthentication(context.Background(), smtpd.AuthenticationRequest{
		RemoteAddr:   "192.0.2.10:25",
		EnvelopeFrom: "sender@example.com",
		Parsed:       parsedFrom("sender@example.com"),
	})
	if err != nil {
		t.Fatalf("VerifyAuthentication() error = %v", err)
	}
	if results.SPF.Result != smtpd.AuthResultPass {
		t.Fatalf("SPF = %+v, want pass", results.SPF)
	}
	if results.DMARC.Result != smtpd.AuthResultPass {
		t.Fatalf("DMARC = %+v, want pass", results.DMARC)
	}
	if results.DMARC.Policy != "reject" {
		t.Fatalf("DMARC policy = %q, want reject", results.DMARC.Policy)
	}
}

func TestVerifierSPFFailAndDMARCRejectPolicy(t *testing.T) {
	verifier := Verifier{DNS: fakeDNS{txt: map[string][]string{
		"example.com":        {"v=spf1 ip4:192.0.2.10 -all"},
		"_dmarc.example.com": {"v=DMARC1; p=reject; aspf=s"},
	}}}

	results, err := verifier.VerifyAuthentication(context.Background(), smtpd.AuthenticationRequest{
		RemoteAddr:   "198.51.100.10:25",
		EnvelopeFrom: "sender@example.com",
		Parsed:       parsedFrom("sender@example.com"),
	})
	if err != nil {
		t.Fatalf("VerifyAuthentication() error = %v", err)
	}
	if results.SPF.Result != smtpd.AuthResultFail {
		t.Fatalf("SPF = %+v, want fail", results.SPF)
	}
	if results.DMARC.Result != smtpd.AuthResultFail || results.DMARC.Policy != "reject" {
		t.Fatalf("DMARC = %+v, want fail with reject policy", results.DMARC)
	}
}

func TestVerifierSPFInclude(t *testing.T) {
	verifier := Verifier{DNS: fakeDNS{txt: map[string][]string{
		"example.com":        {"v=spf1 include:_spf.example.net -all"},
		"_spf.example.net":   {"v=spf1 ip6:2001:db8::1 -all"},
		"_dmarc.example.com": {"v=DMARC1; p=none"},
	}}}

	results, err := verifier.VerifyAuthentication(context.Background(), smtpd.AuthenticationRequest{
		RemoteAddr:   "[2001:db8::1]:25",
		EnvelopeFrom: "sender@example.com",
		Parsed:       parsedFrom("sender@example.com"),
	})
	if err != nil {
		t.Fatalf("VerifyAuthentication() error = %v", err)
	}
	if results.SPF.Result != smtpd.AuthResultPass {
		t.Fatalf("SPF = %+v, want pass", results.SPF)
	}
}

func TestVerifierDMARCStrictAlignmentFailsSubdomain(t *testing.T) {
	verifier := Verifier{DNS: fakeDNS{txt: map[string][]string{
		"bounce.example.com": {"v=spf1 ip4:192.0.2.10 -all"},
		"_dmarc.example.com": {"v=DMARC1; p=reject; aspf=s"},
	}}}

	results, err := verifier.VerifyAuthentication(context.Background(), smtpd.AuthenticationRequest{
		RemoteAddr:   "192.0.2.10",
		EnvelopeFrom: "sender@bounce.example.com",
		Parsed:       parsedFrom("sender@example.com"),
	})
	if err != nil {
		t.Fatalf("VerifyAuthentication() error = %v", err)
	}
	if results.SPF.Result != smtpd.AuthResultPass {
		t.Fatalf("SPF = %+v, want pass", results.SPF)
	}
	if results.DMARC.Result != smtpd.AuthResultFail {
		t.Fatalf("DMARC = %+v, want fail", results.DMARC)
	}
}

func TestVerifierSPFAAndMXMechanisms(t *testing.T) {
	ip := netip.MustParseAddr("192.0.2.44")
	verifier := Verifier{DNS: fakeDNS{
		txt: map[string][]string{
			"example.com":        {"v=spf1 mx -all"},
			"_dmarc.example.com": {"v=DMARC1; p=none"},
		},
		mx: map[string][]*net.MX{
			"example.com": {{Host: "mx.example.com.", Pref: 10}},
		},
		ip: map[string][]netip.Addr{
			"mx.example.com": {ip},
		},
	}}

	results, err := verifier.VerifyAuthentication(context.Background(), smtpd.AuthenticationRequest{
		RemoteAddr:   "192.0.2.44",
		EnvelopeFrom: "sender@example.com",
		Parsed:       parsedFrom("sender@example.com"),
	})
	if err != nil {
		t.Fatalf("VerifyAuthentication() error = %v", err)
	}
	if results.SPF.Result != smtpd.AuthResultPass {
		t.Fatalf("SPF = %+v, want pass", results.SPF)
	}
}

func parsedFrom(address string) message.ParsedMessage {
	return message.ParsedMessage{From: message.Address{Address: address}}
}

func TestSplitSPFTerm(t *testing.T) {
	t.Parallel()

	tests := []struct {
		term          string
		wantQualifier byte
		wantRest      string
	}{
		{"+ip4:192.0.2.0/24", '+', "ip4:192.0.2.0/24"},
		{"-all", '-', "all"},
		{"~all", '~', "all"},
		{"?all", '?', "all"},
		{"all", '+', "all"},
		{"include:_spf.example.net", '+', "include:_spf.example.net"},
		{"redirect=example.com", '+', "redirect=example.com"},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.term, func(t *testing.T) {
			t.Parallel()
			q, rest := splitSPFTerm(tt.term)
			if q != tt.wantQualifier {
				t.Fatalf("splitSPFTerm(%q) qualifier = %c, want %c", tt.term, q, tt.wantQualifier)
			}
			if rest != tt.wantRest {
				t.Fatalf("splitSPFTerm(%q) rest = %q, want %q", tt.term, rest, tt.wantRest)
			}
		})
	}
}

func TestSPFQualifierResult(t *testing.T) {
	t.Parallel()

	tests := []struct {
		q    byte
		want smtpd.AuthResult
	}{
		{'-', smtpd.AuthResultFail},
		{'~', smtpd.AuthResultNeutral},
		{'?', smtpd.AuthResultNeutral},
		{'+', smtpd.AuthResultPass},
		{'x', smtpd.AuthResultPass},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(string(tt.q), func(t *testing.T) {
			t.Parallel()
			got := spfQualifierResult(tt.q)
			if got != tt.want {
				t.Fatalf("spfQualifierResult(%c) = %v, want %v", tt.q, got, tt.want)
			}
		})
	}
}

func TestParseMechanism(t *testing.T) {
	t.Parallel()

	tests := []struct {
		mechanism string
		wantName  string
		wantValue string
		wantCIDR  int
		wantErr   bool
	}{
		{"ip4:192.0.2.0/24", "ip4", "192.0.2.0", 24, false},
		{"ip6:2001:db8::/32", "ip6", "2001:db8::", 32, false},
		{"a", "a", "", -1, false},
		{"mx", "mx", "", -1, false},
		{"ptr", "ptr", "", -1, false},
		{"exists:%{d3}", "exists", "%{d3}", -1, false},
		{"include:_spf.example.net", "include", "_spf.example.net", -1, false},
		{"ip4:192.0.2.0", "ip4", "192.0.2.0", -1, false},
		{"ip4:192.0.2.0/", "", "", -1, true},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.mechanism, func(t *testing.T) {
			t.Parallel()
			name, value, cidr, err := parseMechanism(tt.mechanism)
			if (err != nil) != tt.wantErr {
				t.Fatalf("parseMechanism(%q) error = %v, wantErr %v", tt.mechanism, err, tt.wantErr)
			}
			if !tt.wantErr {
				if name != tt.wantName {
					t.Fatalf("parseMechanism(%q) name = %q, want %q", tt.mechanism, name, tt.wantName)
				}
				if value != tt.wantValue {
					t.Fatalf("parseMechanism(%q) value = %q, want %q", tt.mechanism, value, tt.wantValue)
				}
				if cidr != tt.wantCIDR {
					t.Fatalf("parseMechanism(%q) cidr = %d, want %d", tt.mechanism, cidr, tt.wantCIDR)
				}
			}
		})
	}
}

func TestVerifierSPFNeutral(t *testing.T) {
	verifier := Verifier{DNS: fakeDNS{txt: map[string][]string{
		"example.com": {"v=spf1 ?all"},
	}}}

	results, err := verifier.VerifyAuthentication(context.Background(), smtpd.AuthenticationRequest{
		RemoteAddr:   "192.0.2.10:25",
		EnvelopeFrom: "sender@example.com",
		Parsed:       parsedFrom("sender@example.com"),
	})
	if err != nil {
		t.Fatalf("VerifyAuthentication() error = %v", err)
	}
	if results.SPF.Result != smtpd.AuthResultNeutral {
		t.Fatalf("SPF = %+v, want neutral", results.SPF)
	}
}

func TestVerifierSPFAllMechanisms(t *testing.T) {
	ip := netip.MustParseAddr("192.0.2.100")
	verifier := Verifier{DNS: fakeDNS{
		txt: map[string][]string{
			"example.com": {"v=spf1 a mx ptr -all"},
			"_dmarc.example.com": {"v=DMARC1; p=none"},
		},
		ip: map[string][]netip.Addr{
			"example.com": {ip},
			"mx.example.com": {ip},
			"ptr.example.com": {ip},
		},
	}}

	results, err := verifier.VerifyAuthentication(context.Background(), smtpd.AuthenticationRequest{
		RemoteAddr:   "192.0.2.100",
		EnvelopeFrom: "sender@example.com",
		Parsed:       parsedFrom("sender@example.com"),
	})
	if err != nil {
		t.Fatalf("VerifyAuthentication() error = %v", err)
	}
	if results.SPF.Result != smtpd.AuthResultPass {
		t.Fatalf("SPF = %+v, want pass", results.SPF)
	}
}

func TestVerifierDMARCNOnePolicy(t *testing.T) {
	verifier := Verifier{DNS: fakeDNS{txt: map[string][]string{
		"example.com":        {"v=spf1 ip4:192.0.2.10 -all"},
		"_dmarc.example.com": {"v=DMARC1; p=none"},
	}}}

	results, err := verifier.VerifyAuthentication(context.Background(), smtpd.AuthenticationRequest{
		RemoteAddr:   "198.51.100.10:25",
		EnvelopeFrom: "sender@example.com",
		Parsed:       parsedFrom("sender@example.com"),
	})
	if err != nil {
		t.Fatalf("VerifyAuthentication() error = %v", err)
	}
	if results.SPF.Result != smtpd.AuthResultFail {
		t.Fatalf("SPF = %+v, want fail", results.SPF)
	}
	if results.DMARC.Policy != "none" {
		t.Fatalf("DMARC policy = %q, want none", results.DMARC.Policy)
	}
}

func TestVerifierDMARCQuarantinePolicy(t *testing.T) {
	verifier := Verifier{DNS: fakeDNS{txt: map[string][]string{
		"example.com":        {"v=spf1 ip4:192.0.2.10 -all"},
		"_dmarc.example.com": {"v=DMARC1; p=quarantine; aspf=s"},
	}}}

	results, err := verifier.VerifyAuthentication(context.Background(), smtpd.AuthenticationRequest{
		RemoteAddr:   "198.51.100.10:25",
		EnvelopeFrom: "sender@example.com",
		Parsed:       parsedFrom("sender@example.com"),
	})
	if err != nil {
		t.Fatalf("VerifyAuthentication() error = %v", err)
	}
	if results.SPF.Result != smtpd.AuthResultFail {
		t.Fatalf("SPF = %+v, want fail", results.SPF)
	}
	if results.DMARC.Policy != "quarantine" {
		t.Fatalf("DMARC policy = %q, want quarantine", results.DMARC.Policy)
	}
}
