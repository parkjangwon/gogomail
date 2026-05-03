package mailauth

import (
	"context"
	"strings"
	"testing"

	smtpd "github.com/gogomail/gogomail/internal/smtp"
)

func TestEnforcementHookRejectsDMARCRejectPolicy(t *testing.T) {
	hook := EnforcementHook(EnforcementOptions{Mode: EnforcementReject})
	err := hook(context.Background(), smtpd.Event{
		Stage: smtpd.StageAuthenticationChecked,
		Authentication: smtpd.AuthenticationResults{
			DMARC: smtpd.AuthCheckResult{
				Result: smtpd.AuthResultFail,
				Domain: "example.com",
				Policy: "reject",
				Reason: "no aligned pass",
			},
		},
	})
	if err == nil || !strings.Contains(err.Error(), "example.com") {
		t.Fatalf("hook error = %v, want DMARC rejection", err)
	}
}

func TestEnforcementHookMonitorAllowsDMARCRejectPolicy(t *testing.T) {
	hook := EnforcementHook(EnforcementOptions{Mode: EnforcementMonitor})
	err := hook(context.Background(), smtpd.Event{
		Stage: smtpd.StageAuthenticationChecked,
		Authentication: smtpd.AuthenticationResults{
			DMARC: smtpd.AuthCheckResult{Result: smtpd.AuthResultFail, Policy: "reject"},
		},
	})
	if err != nil {
		t.Fatalf("monitor hook returned error: %v", err)
	}
}

func TestEnforcementHookQuarantineOnlyRejectsRejectPolicy(t *testing.T) {
	hook := EnforcementHook(EnforcementOptions{Mode: EnforcementQuarantine})
	err := hook(context.Background(), smtpd.Event{
		Stage: smtpd.StageAuthenticationChecked,
		Authentication: smtpd.AuthenticationResults{
			DMARC: smtpd.AuthCheckResult{Result: smtpd.AuthResultFail, Policy: "quarantine"},
		},
	})
	if err != nil {
		t.Fatalf("quarantine hook returned error for quarantine policy: %v", err)
	}
}
