package smtpd

import (
	"errors"
	"testing"

	gosmtp "github.com/emersion/go-smtp"
)

func requireSMTPStatus(t *testing.T, err error, code int, enhanced gosmtp.EnhancedCode) {
	t.Helper()

	var smtpErr *gosmtp.SMTPError
	if !errors.As(err, &smtpErr) {
		t.Fatalf("err = %T %v, want *smtp.SMTPError", err, err)
	}
	if smtpErr.Code != code {
		t.Fatalf("SMTP code = %d, want %d (%v)", smtpErr.Code, code, smtpErr)
	}
	if smtpErr.EnhancedCode != enhanced {
		t.Fatalf("enhanced code = %v, want %v", smtpErr.EnhancedCode, enhanced)
	}
}

func TestSMTPStatusHelpersUseExpectedCodes(t *testing.T) {
	t.Parallel()

	requireSMTPStatus(t, smtpBadSequence("DATA"), 503, gosmtp.EnhancedCode{5, 5, 1})
	requireSMTPStatus(t, smtpMailboxUnavailable("missing"), 550, gosmtp.EnhancedCode{5, 1, 1})
	requireSMTPStatus(t, smtpPolicyReject("blocked"), 550, gosmtp.EnhancedCode{5, 7, 1})
	requireSMTPStatus(t, smtpTooManyRecipients(1), 452, gosmtp.EnhancedCode{4, 5, 3})
	requireSMTPStatus(t, smtpRateLimited("user@example.com"), 451, gosmtp.EnhancedCode{4, 7, 1})
	requireSMTPStatus(t, smtpBackpressure(), 421, gosmtp.EnhancedCode{4, 3, 2})
}
