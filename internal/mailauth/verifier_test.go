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
