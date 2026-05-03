package smtpd

import (
	"strings"
	"testing"
)

func TestFormatAuthenticationResults(t *testing.T) {
	header := FormatAuthenticationResults(AuthenticationResults{
		AuthservID: "mx.example.com",
		SPF: AuthCheckResult{
			Result:     AuthResultPass,
			Reason:     "ip matched",
			Identifier: "sender@example.net",
		},
		DKIM: AuthCheckResult{
			Result:     AuthResultPass,
			Reason:     "signature verified",
			Domain:     "example.net",
			Identifier: "@example.net",
		},
		DMARC: AuthCheckResult{
			Result: AuthResultPass,
			Reason: "spf aligned",
			Domain: "example.net",
		},
	})
	if !strings.HasPrefix(header, "Authentication-Results: mx.example.com;") {
		t.Fatalf("header = %q, want Authentication-Results prefix", header)
	}
	for _, want := range []string{"spf=pass", "dkim=pass", "dmarc=pass", "\r\n"} {
		if !strings.Contains(header, want) {
			t.Fatalf("header = %q, want %q", header, want)
		}
	}
}

func TestFormatAuthenticationResultsDefaultsAuthservID(t *testing.T) {
	header := FormatAuthenticationResults(AuthenticationResults{})
	if !strings.HasPrefix(header, "Authentication-Results: localhost;") {
		t.Fatalf("header = %q, want localhost default", header)
	}
}
