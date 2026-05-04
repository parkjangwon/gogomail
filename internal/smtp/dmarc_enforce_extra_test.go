package smtpd

import (
	"errors"
	"testing"

	gosmtp "github.com/emersion/go-smtp"
)

func TestEnforceDMARCPolicyNoopWhenDisabled(t *testing.T) {
	t.Parallel()

	results := AuthenticationResults{
		DMARC: AuthCheckResult{Result: AuthResultFail, Policy: "reject", Domain: "example.com"},
	}
	if err := enforceDMARCPolicy(false, results); err != nil {
		t.Errorf("expected no error when enforce=false, got %v", err)
	}
}

func TestEnforceDMARCPolicyNoopWhenPass(t *testing.T) {
	t.Parallel()

	results := AuthenticationResults{
		DMARC: AuthCheckResult{Result: AuthResultPass, Policy: "reject", Domain: "example.com"},
	}
	if err := enforceDMARCPolicy(true, results); err != nil {
		t.Errorf("expected no error when DMARC passes, got %v", err)
	}
}

func TestEnforceDMARCPolicyNoopWhenNone(t *testing.T) {
	t.Parallel()

	results := AuthenticationResults{
		DMARC: AuthCheckResult{Result: AuthResultNone, Policy: "none", Domain: "example.com"},
	}
	if err := enforceDMARCPolicy(true, results); err != nil {
		t.Errorf("expected no error for policy=none, got %v", err)
	}
}

func TestEnforceDMARCPolicyRejects550OnRejectPolicy(t *testing.T) {
	t.Parallel()

	results := AuthenticationResults{
		DMARC: AuthCheckResult{Result: AuthResultFail, Policy: "reject", Domain: "example.com"},
	}
	err := enforceDMARCPolicy(true, results)
	if err == nil {
		t.Fatal("expected SMTP error for DMARC reject policy, got nil")
	}
	var smtpErr *gosmtp.SMTPError
	if !errors.As(err, &smtpErr) {
		t.Fatalf("expected *gosmtp.SMTPError, got %T: %v", err, err)
	}
	if smtpErr.Code != 550 {
		t.Errorf("SMTP code = %d, want 550", smtpErr.Code)
	}
}

func TestEnforceDMARCPolicyAllowsQuarantinePolicy(t *testing.T) {
	t.Parallel()

	results := AuthenticationResults{
		DMARC: AuthCheckResult{Result: AuthResultFail, Policy: "quarantine", Domain: "example.com"},
	}
	if err := enforceDMARCPolicy(true, results); err != nil {
		t.Errorf("expected no error for quarantine policy, got %v", err)
	}
}
