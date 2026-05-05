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

func TestFormatAuthenticationResultsSanitizesControlCharacters(t *testing.T) {
	header := FormatAuthenticationResults(AuthenticationResults{
		AuthservID: "mx.example.com\r\nInjected: bad",
		SPF: AuthCheckResult{
			Result:     AuthResultFail,
			Reason:     "dns\r\nInjected: bad\x00\tlookup failed",
			Identifier: "sender@example.net\r\nInjected: bad",
		},
		DKIM: AuthCheckResult{
			Result:     AuthResultTemporary,
			Reason:     "body hash\r\nInjected: bad",
			Domain:     "example.net\r\nInjected: bad",
			Identifier: "@example.net\r\nInjected: bad",
		},
		DMARC: AuthCheckResult{
			Result: AuthResultPermanent,
			Reason: "policy\r\nInjected: bad",
			Domain: "example.net\r\nInjected: bad",
		},
	})
	for _, bad := range []string{
		"\r\nInjected:",
		"\x00",
		"\t",
	} {
		if strings.Contains(header, bad) {
			t.Fatalf("header contains unsanitized %q:\n%s", bad, header)
		}
	}
	unfolded := strings.ReplaceAll(header, "\r\n\t", " ")
	unfolded = strings.ReplaceAll(unfolded, "\r\n ", " ")
	if !strings.Contains(unfolded, "dns Injected: bad lookup failed") {
		t.Fatalf("header missing sanitized reason:\n%s", header)
	}
}

func TestFormatAuthenticationResultsBoundsLongValues(t *testing.T) {
	longReason := strings.Repeat("a", maxAuthenticationValueBytes+200)
	header := FormatAuthenticationResults(AuthenticationResults{
		AuthservID: strings.Repeat("m", maxAuthservIDBytes+200),
		SPF: AuthCheckResult{
			Result:     AuthResultTemporary,
			Reason:     longReason,
			Identifier: "sender@example.net",
		},
	})
	unfolded := strings.ReplaceAll(header, "\r\n\t", "")
	unfolded = strings.ReplaceAll(unfolded, "\r\n ", "")
	if strings.Contains(unfolded, strings.Repeat("a", maxAuthenticationValueBytes+1)) {
		t.Fatalf("header contains unbounded reason:\n%s", header)
	}
	if strings.Contains(unfolded, strings.Repeat("m", maxAuthservIDBytes+1)) {
		t.Fatalf("header contains unbounded authserv-id:\n%s", header)
	}
	if !strings.Contains(unfolded, "reason="+strings.Repeat("a", maxAuthenticationValueBytes)) {
		t.Fatalf("header missing bounded reason:\n%s", header)
	}
	if !strings.HasSuffix(header, "\r\n") {
		t.Fatalf("header missing CRLF terminator: %q", header)
	}
}
