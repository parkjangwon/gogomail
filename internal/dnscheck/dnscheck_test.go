package dnscheck

import (
	"context"
	"fmt"
	"net"
	"reflect"
	"testing"
)

func TestVerifierReportsDomainDNS(t *testing.T) {
	t.Parallel()

	resolver := fakeResolver{
		mx: map[string][]*net.MX{
			"example.com": {{Host: "mx.example.com.", Pref: 10}},
		},
		txt: map[string][]string{
			"example.com":                  {"v=spf1 mx -all"},
			"_dmarc.example.com":           {"v=DMARC1; p=quarantine"},
			"s1._domainkey.example.com":    {"v=DKIM1; k=rsa; p=abc"},
			"bad._domainkey.example.com":   {"v=DKIM1; k=rsa; p=other"},
			"empty._domainkey.example.com": {},
		},
	}
	report := Verifier{DNS: resolver}.VerifyDomain(context.Background(), "Example.COM.", []DKIMExpectation{
		{Selector: "s1", PublicKeyDNS: "v=DKIM1; k=rsa; p=abc"},
		{Selector: "bad", PublicKeyDNS: "v=DKIM1; k=rsa; p=abc"},
		{Selector: "empty", PublicKeyDNS: "v=DKIM1; k=rsa; p=abc"},
	})

	if report.Domain != "example.com" {
		t.Fatalf("Domain = %q", report.Domain)
	}
	if report.MX.Status != StatusOK || !reflect.DeepEqual(report.MX.Found, []string{"mx.example.com"}) {
		t.Fatalf("MX = %+v", report.MX)
	}
	if report.SPF.Status != StatusOK {
		t.Fatalf("SPF = %+v", report.SPF)
	}
	if report.DMARC.Status != StatusOK {
		t.Fatalf("DMARC = %+v", report.DMARC)
	}
	if got := []Status{report.DKIM[0].Status, report.DKIM[1].Status, report.DKIM[2].Status}; !reflect.DeepEqual(got, []Status{StatusOK, StatusMismatch, StatusMissing}) {
		t.Fatalf("DKIM statuses = %+v", got)
	}
}

func TestVerifierReportsLookupErrors(t *testing.T) {
	t.Parallel()

	errLookup := fmt.Errorf("lookup failed")
	resolver := fakeResolver{
		mxErr:  map[string]error{"example.com": errLookup},
		txtErr: map[string]error{"example.com": errLookup, "_dmarc.example.com": errLookup, "s1._domainkey.example.com": errLookup},
	}
	report := Verifier{DNS: resolver}.VerifyDomain(context.Background(), "example.com", []DKIMExpectation{
		{Selector: "s1", PublicKeyDNS: "v=DKIM1; p=abc"},
	})
	if report.MX.Status != StatusError || report.SPF.Status != StatusError || report.DMARC.Status != StatusError || report.DKIM[0].Status != StatusError {
		t.Fatalf("report = %+v", report)
	}
}

type fakeResolver struct {
	mx     map[string][]*net.MX
	txt    map[string][]string
	mxErr  map[string]error
	txtErr map[string]error
}

func (r fakeResolver) LookupMX(_ context.Context, name string) ([]*net.MX, error) {
	if err := r.mxErr[name]; err != nil {
		return nil, err
	}
	return append([]*net.MX(nil), r.mx[name]...), nil
}

func (r fakeResolver) LookupTXT(_ context.Context, name string) ([]string, error) {
	if err := r.txtErr[name]; err != nil {
		return nil, err
	}
	return append([]string(nil), r.txt[name]...), nil
}
