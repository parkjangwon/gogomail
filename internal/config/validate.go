package config

import (
	"fmt"
	"strings"
)

func (c Config) Validate() error {
	if c.SubmissionAllowInsecureAuth && strings.EqualFold(strings.TrimSpace(c.Environment), "production") {
		return fmt.Errorf("GOGOMAIL_SUBMISSION_ALLOW_INSECURE_AUTH must be false in production")
	}
	if (c.SMTPTLSCertFile == "") != (c.SMTPTLSKeyFile == "") {
		return fmt.Errorf("both SMTP TLS certificate and key files are required")
	}
	if strings.TrimSpace(c.SubmissionSMTPSAddr) != "" && (c.SMTPTLSCertFile == "" || c.SMTPTLSKeyFile == "") {
		return fmt.Errorf("GOGOMAIL_SUBMISSION_SMTPS_ADDR requires SMTP TLS certificate and key files")
	}
	if c.SMTPMaxRecipients <= 0 {
		return fmt.Errorf("GOGOMAIL_SMTP_MAX_RECIPIENTS must be positive")
	}
	if c.SubmissionMaxRecipients <= 0 {
		return fmt.Errorf("GOGOMAIL_SUBMISSION_MAX_RECIPIENTS must be positive")
	}
	if c.SMTPMaxMessageBytes <= 0 {
		return fmt.Errorf("GOGOMAIL_SMTP_MAX_MESSAGE_BYTES must be positive")
	}
	if c.SubmissionMaxMessageBytes <= 0 {
		return fmt.Errorf("GOGOMAIL_SUBMISSION_MAX_MESSAGE_BYTES must be positive")
	}
	if c.SMTPReadTimeout <= 0 {
		return fmt.Errorf("GOGOMAIL_SMTP_READ_TIMEOUT must be positive")
	}
	if c.SMTPWriteTimeout <= 0 {
		return fmt.Errorf("GOGOMAIL_SMTP_WRITE_TIMEOUT must be positive")
	}
	if c.DeliveryTimeout <= 0 {
		return fmt.Errorf("GOGOMAIL_DELIVERY_TIMEOUT must be positive")
	}
	if c.SMTPMaxDKIMVerifications <= 0 {
		return fmt.Errorf("GOGOMAIL_SMTP_MAX_DKIM_VERIFICATIONS must be positive")
	}
	if err := validateEnum("GOGOMAIL_SMTP_DMARC_ENFORCEMENT", c.SMTPDMARCEnforcement, "monitor", "quarantine", "reject"); err != nil {
		return err
	}
	if err := validateEnum("GOGOMAIL_METRICS_BACKEND", c.MetricsBackend, "none", "slog"); err != nil {
		return err
	}
	if err := validateEnum("GOGOMAIL_DELIVERY_TLS_MODE", c.DeliveryTLSMode, "opportunistic", "require", "disable"); err != nil {
		return err
	}
	if strings.TrimSpace(c.DeliverySmartHostTLSMode) != "" {
		if err := validateEnum("GOGOMAIL_DELIVERY_SMARTHOST_TLS_MODE", c.DeliverySmartHostTLSMode, "opportunistic", "require", "disable"); err != nil {
			return err
		}
	}
	if c.DeliverySmartHostPort < 0 || c.DeliverySmartHostPort > 65535 {
		return fmt.Errorf("GOGOMAIL_DELIVERY_SMARTHOST_PORT must be between 0 and 65535")
	}
	if strings.TrimSpace(c.DeliverySmartHost) == "" && (strings.TrimSpace(c.DeliverySmartHostUsername) != "" || strings.TrimSpace(c.DeliverySmartHostPassword) != "" || strings.TrimSpace(c.DeliverySmartHostIdentity) != "" || c.DeliverySmartHostPort > 0 || strings.TrimSpace(c.DeliverySmartHostTLSMode) != "") {
		return fmt.Errorf("GOGOMAIL_DELIVERY_SMARTHOST is required when smart-host options are set")
	}
	if c.DeliveryRetryJitterRatio < 0 || c.DeliveryRetryJitterRatio > 1 {
		return fmt.Errorf("GOGOMAIL_DELIVERY_RETRY_JITTER_RATIO must be between 0 and 1")
	}
	if c.DeliveryThrottleEnabled && c.DeliveryDefaultConcurrency == 0 && len(c.DeliveryFarmConcurrency) == 0 && len(c.DeliveryDomainConcurrency) == 0 {
		return fmt.Errorf("delivery throttling requires at least one default, farm, or domain concurrency limit")
	}
	return nil
}

func validateEnum(name string, value string, allowed ...string) error {
	normalized := strings.ToLower(strings.TrimSpace(value))
	for _, candidate := range allowed {
		if normalized == candidate {
			return nil
		}
	}
	return fmt.Errorf("%s has unsupported value %q", name, value)
}
