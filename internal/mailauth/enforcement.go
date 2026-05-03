package mailauth

import (
	"context"
	"fmt"
	"strings"

	smtpd "github.com/gogomail/gogomail/internal/smtp"
)

type EnforcementMode string

const (
	EnforcementMonitor    EnforcementMode = "monitor"
	EnforcementQuarantine EnforcementMode = "quarantine"
	EnforcementReject     EnforcementMode = "reject"
)

type EnforcementOptions struct {
	Mode EnforcementMode
}

func EnforcementHook(opts EnforcementOptions) smtpd.Hook {
	mode := normalizeEnforcementMode(opts.Mode)
	return func(_ context.Context, event smtpd.Event) error {
		if event.Stage != smtpd.StageAuthenticationChecked {
			return nil
		}
		dmarc := event.Authentication.DMARC
		if dmarc.Result != smtpd.AuthResultFail {
			return nil
		}
		policy := strings.ToLower(strings.TrimSpace(dmarc.Policy))
		if policy != "reject" && policy != "quarantine" {
			return nil
		}
		switch mode {
		case EnforcementMonitor:
			return nil
		case EnforcementQuarantine:
			if policy == "reject" {
				return fmt.Errorf("dmarc policy rejected message for %s: %s", dmarc.Domain, dmarc.Reason)
			}
			return nil
		case EnforcementReject:
			return fmt.Errorf("dmarc policy %s for %s: %s", policy, dmarc.Domain, dmarc.Reason)
		default:
			return nil
		}
	}
}

func normalizeEnforcementMode(mode EnforcementMode) EnforcementMode {
	switch EnforcementMode(strings.ToLower(strings.TrimSpace(string(mode)))) {
	case EnforcementQuarantine:
		return EnforcementQuarantine
	case EnforcementReject:
		return EnforcementReject
	default:
		return EnforcementMonitor
	}
}
