package smtpd

import (
	"strings"
	"testing"
)

func TestRFC5322ValidMessage(t *testing.T) {
	rc := NewRFCCompliant()

	msg := "From: sender@example.com\r\nTo: user@example.com\r\nDate: Mon, 14 May 2026 10:00:00 +0000\r\nSubject: Test\r\n\r\nBody"

	compliance := RFCCompliance{}
	rc.ValidateRFC5322(msg, &compliance)

	if !compliance.RFC5322Valid {
		t.Errorf("valid message should pass RFC 5322: %v", compliance.Errors)
	}
}

func TestRFC5322MissingFrom(t *testing.T) {
	rc := NewRFCCompliant()

	msg := "To: user@example.com\r\nDate: Mon, 14 May 2026 10:00:00 +0000\r\n\r\nBody"

	compliance := RFCCompliance{}
	rc.ValidateRFC5322(msg, &compliance)

	if compliance.RFC5322Valid {
		t.Error("message without From header should fail RFC 5322")
	}

	if len(compliance.Errors) == 0 {
		t.Error("expected error messages")
	}
}

func TestRFC5321ValidEnvelope(t *testing.T) {
	rc := NewRFCCompliant()

	compliance := RFCCompliance{}
	rc.ValidateSMTPEnvelope("sender@example.com", []string{"user@example.com"}, &compliance)

	if !compliance.RFC5321Valid {
		t.Errorf("valid envelope should pass RFC 5321: %v", compliance.Errors)
	}
}

func TestRFC5321InvalidSender(t *testing.T) {
	rc := NewRFCCompliant()

	compliance := RFCCompliance{}
	rc.ValidateSMTPEnvelope("invalid-email", []string{"user@example.com"}, &compliance)

	if compliance.RFC5321Valid {
		t.Error("invalid sender should fail RFC 5321")
	}
}

func TestRFC5321NoRecipients(t *testing.T) {
	rc := NewRFCCompliant()

	compliance := RFCCompliance{}
	rc.ValidateSMTPEnvelope("sender@example.com", []string{}, &compliance)

	if compliance.RFC5321Valid {
		t.Error("envelope with no recipients should fail RFC 5321")
	}
}

func TestRFC3461ValidDSN(t *testing.T) {
	rc := NewRFCCompliant()

	compliance := RFCCompliance{}
	rc.ValidateDSNOptions("FULL", []string{"SUCCESS", "FAILURE"}, &compliance)

	if !compliance.RFC3461Valid {
		t.Errorf("valid DSN should pass RFC 3461: %v", compliance.Errors)
	}
}

func TestRFC3461InvalidReturnOption(t *testing.T) {
	rc := NewRFCCompliant()

	compliance := RFCCompliance{}
	rc.ValidateDSNOptions("INVALID", []string{"SUCCESS"}, &compliance)

	if compliance.RFC3461Valid {
		t.Error("invalid RETURN option should fail RFC 3461")
	}
}

func TestRFC3461InvalidNotifyOption(t *testing.T) {
	rc := NewRFCCompliant()

	compliance := RFCCompliance{}
	rc.ValidateDSNOptions("FULL", []string{"INVALID"}, &compliance)

	if compliance.RFC3461Valid {
		t.Error("invalid NOTIFY option should fail RFC 3461")
	}
}

func TestRFC6376ValidDKIM(t *testing.T) {
	rc := NewRFCCompliant()

	dkimSig := "v=1; a=rsa-sha256; c=relaxed/relaxed; d=example.com; s=default; b=xyz; bh=abc;"

	compliance := RFCCompliance{}
	rc.ValidateDKIM(dkimSig, &compliance)

	if !compliance.RFC6376Valid {
		t.Errorf("valid DKIM should pass RFC 6376: %v", compliance.Errors)
	}
}

func TestRFC6376MissingTag(t *testing.T) {
	rc := NewRFCCompliant()

	dkimSig := "v=1; a=rsa-sha256; b=xyz; bh=abc;" // missing d=, s=

	compliance := RFCCompliance{}
	rc.ValidateDKIM(dkimSig, &compliance)

	if compliance.RFC6376Valid {
		t.Error("incomplete DKIM should fail RFC 6376")
	}
}

func TestRFC6376NoSignature(t *testing.T) {
	rc := NewRFCCompliant()

	compliance := RFCCompliance{}
	rc.ValidateDKIM("", &compliance)

	if !compliance.RFC6376Valid {
		t.Error("missing DKIM is optional and should be valid")
	}
}

func TestRFC5891AsciiDomain(t *testing.T) {
	rc := NewRFCCompliant()

	compliance := RFCCompliance{}
	rc.ValidateIDN([]string{"user@example.com"}, &compliance)

	if !compliance.RFC5891Valid {
		t.Errorf("ASCII domain should pass RFC 5891: %v", compliance.Errors)
	}
}

func TestRFC5891InvalidDomain(t *testing.T) {
	rc := NewRFCCompliant()

	compliance := RFCCompliance{}
	rc.ValidateIDN([]string{"user@invalid..com"}, &compliance)

	if compliance.RFC5891Valid {
		t.Error("invalid domain should fail RFC 5891")
	}
}

func TestComplianceIsCompliant(t *testing.T) {
	tests := []struct {
		name       string
		compliance RFCCompliance
		expected   bool
	}{
		{
			name:       "all valid",
			compliance: RFCCompliance{RFC5322Valid: true, RFC5321Valid: true, RFC3461Valid: true, RFC6376Valid: true, RFC5891Valid: true},
			expected:   true,
		},
		{
			name:       "one invalid",
			compliance: RFCCompliance{RFC5322Valid: false, RFC5321Valid: true, RFC3461Valid: true, RFC6376Valid: true, RFC5891Valid: true},
			expected:   false,
		},
		{
			name:       "multiple invalid",
			compliance: RFCCompliance{RFC5322Valid: false, RFC5321Valid: false, RFC3461Valid: true, RFC6376Valid: true, RFC5891Valid: true},
			expected:   false,
		},
	}

	for _, test := range tests {
		result := test.compliance.IsCompliant()
		if result != test.expected {
			t.Errorf("%s: expected %v, got %v", test.name, test.expected, result)
		}
	}
}

func TestComplianceGetSummary(t *testing.T) {
	compliance := RFCCompliance{
		RFC5322Valid: true,
		RFC5321Valid: false,
		RFC3461Valid: true,
		RFC6376Valid: true,
		RFC5891Valid: true,
		Errors:       []string{"RFC 5321: Missing required field"},
	}

	summary := compliance.GetSummary()

	if !strings.Contains(summary, "RFC Compliance Report") {
		t.Error("summary should contain header")
	}

	if !strings.Contains(summary, "false") {
		t.Error("summary should show RFC 5321 as false")
	}

	if !strings.Contains(summary, "Missing required field") {
		t.Error("summary should include error details")
	}
}

func TestRFCCompliantFullValidation(t *testing.T) {
	rc := NewRFCCompliant()

	msg := "From: sender@example.com\r\nTo: user@example.com\r\nDate: Mon, 14 May 2026 10:00:00 +0000\r\nSubject: Test\r\n\r\nBody"
	dkimSig := "v=1; a=rsa-sha256; c=relaxed/relaxed; d=example.com; s=default; b=xyz; bh=abc;"

	compliance := rc.ValidateMessage(
		msg,
		"sender@example.com",
		[]string{"user@example.com"},
		"FULL",
		[]string{"SUCCESS"},
		dkimSig,
	)

	if !compliance.IsCompliant() {
		t.Errorf("valid message should be fully compliant: %v", compliance.Errors)
	}
}

func BenchmarkRFCCompliantValidateSMTPEnvelope(b *testing.B) {
	rc := NewRFCCompliant()
	compliance := &RFCCompliance{}

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		rc.ValidateSMTPEnvelope("sender@example.com", []string{"user@example.com"}, compliance)
	}
}

func BenchmarkRFCCompliantValidateRFC5322(b *testing.B) {
	rc := NewRFCCompliant()
	msg := "From: sender@example.com\r\nTo: user@example.com\r\nDate: Mon, 14 May 2026 10:00:00 +0000\r\n\r\nBody"
	compliance := &RFCCompliance{}

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		rc.ValidateRFC5322(msg, compliance)
	}
}
