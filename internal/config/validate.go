package config

import (
	"crypto/ed25519"
	"encoding/base64"
	"fmt"
	"net"
	"net/mail"
	"net/url"
	"strconv"
	"strings"
)

const (
	maxExportManifestSignerKeyIDBytes      = 200
	maxExportManifestSignerCredentialBytes = 4096
	maxWebhookTokenBytes                   = 4096
	maxAttachmentCleanupBatchSize          = 1000
	minHTTPMaxHeaderBytes                  = 4 << 10
	maxHTTPMaxHeaderBytes                  = 1 << 20
)

func (c Config) Validate() error {
	production := strings.EqualFold(strings.TrimSpace(c.Environment), "production")
	if err := validateEnum("GOGOMAIL_ENV", c.Environment, "development", "test", "production"); err != nil {
		return err
	}
	if c.SubmissionAllowInsecureAuth && strings.EqualFold(strings.TrimSpace(c.Environment), "production") {
		return fmt.Errorf("GOGOMAIL_SUBMISSION_ALLOW_INSECURE_AUTH must be false in production")
	}
	if (c.SMTPTLSCertFile == "") != (c.SMTPTLSKeyFile == "") {
		return fmt.Errorf("both SMTP TLS certificate and key files are required")
	}
	if c.HTTPReadTimeout <= 0 {
		return fmt.Errorf("GOGOMAIL_HTTP_READ_TIMEOUT must be positive")
	}
	if c.HTTPWriteTimeout <= 0 {
		return fmt.Errorf("GOGOMAIL_HTTP_WRITE_TIMEOUT must be positive")
	}
	if c.HTTPIdleTimeout <= 0 {
		return fmt.Errorf("GOGOMAIL_HTTP_IDLE_TIMEOUT must be positive")
	}
	if c.HTTPReadHeaderTimeout <= 0 {
		return fmt.Errorf("GOGOMAIL_HTTP_READ_HEADER_TIMEOUT must be positive")
	}
	if c.HTTPMaxHeaderBytes < minHTTPMaxHeaderBytes || c.HTTPMaxHeaderBytes > maxHTTPMaxHeaderBytes {
		return fmt.Errorf("GOGOMAIL_HTTP_MAX_HEADER_BYTES must be between %d and %d", minHTTPMaxHeaderBytes, maxHTTPMaxHeaderBytes)
	}
	if err := validateTCPAddr("GOGOMAIL_HTTP_ADDR", c.HTTPAddr, true); err != nil {
		return err
	}
	if err := validateTCPAddr("GOGOMAIL_SMTP_ADDR", c.SMTPAddr, true); err != nil {
		return err
	}
	if err := validateTCPAddr("GOGOMAIL_INBOUND_SMTP_ADDR", c.InboundSMTPAddr, true); err != nil {
		return err
	}
	if err := validateTCPAddr("GOGOMAIL_SUBMISSION_ADDR", c.SubmissionAddr, true); err != nil {
		return err
	}
	if err := validateTCPAddr("GOGOMAIL_SUBMISSION_SMTPS_ADDR", c.SubmissionSMTPSAddr, false); err != nil {
		return err
	}
	if err := validateEnum("GOGOMAIL_STORAGE_BACKEND", c.StorageBackend, "local"); err != nil {
		return err
	}
	if err := validateEnum("GOGOMAIL_DEDUP_BACKEND", c.DedupBackend, "none", "redis"); err != nil {
		return err
	}
	if err := validateEnum("GOGOMAIL_RATELIMIT_BACKEND", c.RateLimitBackend, "none", "redis"); err != nil {
		return err
	}
	if err := validateEnum("GOGOMAIL_BACKPRESSURE_BACKEND", c.BackpressureBackend, "none", "redis"); err != nil {
		return err
	}
	if c.RcptRateLimitPerMinute <= 0 {
		return fmt.Errorf("GOGOMAIL_RCPT_RATE_LIMIT_PER_MINUTE must be positive")
	}
	if c.OutboxRelayBatchSize <= 0 {
		return fmt.Errorf("GOGOMAIL_OUTBOX_RELAY_BATCH_SIZE must be positive")
	}
	if c.OutboxRelayPollInterval <= 0 {
		return fmt.Errorf("GOGOMAIL_OUTBOX_RELAY_POLL_INTERVAL must be positive")
	}
	if c.OutboxRelayMaxAttempts <= 0 {
		return fmt.Errorf("GOGOMAIL_OUTBOX_RELAY_MAX_ATTEMPTS must be positive")
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
	if strings.TrimSpace(c.SMTPDomain) == "" || strings.ContainsAny(c.SMTPDomain, " \t\r\n") {
		return fmt.Errorf("GOGOMAIL_SMTP_DOMAIN must be a non-empty hostname without whitespace")
	}
	if c.DeliveryTimeout <= 0 {
		return fmt.Errorf("GOGOMAIL_DELIVERY_TIMEOUT must be positive")
	}
	if strings.TrimSpace(c.DeliverySMTPHello) == "" || strings.ContainsAny(c.DeliverySMTPHello, " \t\r\n") {
		return fmt.Errorf("GOGOMAIL_DELIVERY_SMTP_HELLO must be a non-empty hostname without whitespace")
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
	if err := validateEnum("GOGOMAIL_ATTACHMENT_SCAN_BACKEND", c.AttachmentScanBackend, "none", "webhook"); err != nil {
		return err
	}
	if strings.EqualFold(strings.TrimSpace(c.AttachmentScanBackend), "webhook") {
		if err := validateWebhookURL("GOGOMAIL_ATTACHMENT_SCAN_WEBHOOK_URL", c.AttachmentScanWebhookURL, production); err != nil {
			return err
		}
		if err := validateOptionalSecret("GOGOMAIL_ATTACHMENT_SCAN_WEBHOOK_TOKEN", c.AttachmentScanWebhookToken); err != nil {
			return err
		}
	}
	if c.AttachmentScanTimeout <= 0 {
		return fmt.Errorf("GOGOMAIL_ATTACHMENT_SCAN_TIMEOUT must be positive")
	}
	if c.AttachmentCleanupInterval <= 0 {
		return fmt.Errorf("GOGOMAIL_ATTACHMENT_CLEANUP_INTERVAL must be positive")
	}
	if c.AttachmentCleanupStaleAge <= 0 {
		return fmt.Errorf("GOGOMAIL_ATTACHMENT_CLEANUP_STALE_AGE must be positive")
	}
	if c.AttachmentCleanupBatchSize <= 0 || c.AttachmentCleanupBatchSize > maxAttachmentCleanupBatchSize {
		return fmt.Errorf("GOGOMAIL_ATTACHMENT_CLEANUP_BATCH_SIZE must be between 1 and %d", maxAttachmentCleanupBatchSize)
	}
	if err := validateEnum("GOGOMAIL_PUSH_NOTIFICATION_BACKEND", c.PushNotifyBackend, "none", "slog", "webhook"); err != nil {
		return err
	}
	if strings.EqualFold(strings.TrimSpace(c.PushNotifyBackend), "webhook") {
		if err := validateWebhookURL("GOGOMAIL_PUSH_NOTIFICATION_WEBHOOK_URL", c.PushNotifyWebhookURL, production); err != nil {
			return err
		}
		if err := validateOptionalSecret("GOGOMAIL_PUSH_NOTIFICATION_WEBHOOK_TOKEN", c.PushNotifyWebhookToken); err != nil {
			return err
		}
	}
	if c.PushNotifyWebhookTimeout <= 0 {
		return fmt.Errorf("GOGOMAIL_PUSH_NOTIFICATION_WEBHOOK_TIMEOUT must be positive")
	}
	if c.PushNotifyDeviceLimit <= 0 || c.PushNotifyDeviceLimit > 200 {
		return fmt.Errorf("GOGOMAIL_PUSH_NOTIFICATION_DEVICE_LIMIT must be between 1 and 200")
	}
	if c.PushNotifyConsumerCount <= 0 {
		return fmt.Errorf("GOGOMAIL_PUSH_NOTIFICATION_CONSUMER_COUNT must be positive")
	}
	if c.PushNotifyConsumerBlock <= 0 {
		return fmt.Errorf("GOGOMAIL_PUSH_NOTIFICATION_CONSUMER_BLOCK must be positive")
	}
	if c.PushNotifyConsumerClaimIdle < 0 {
		return fmt.Errorf("GOGOMAIL_PUSH_NOTIFICATION_CONSUMER_CLAIM_IDLE must not be negative")
	}
	if c.PushNotifyConsumerMaxDeliveries < 0 {
		return fmt.Errorf("GOGOMAIL_PUSH_NOTIFICATION_CONSUMER_MAX_DELIVERIES must not be negative")
	}
	if err := validateBoundedNoCRLF("GOGOMAIL_PUSH_NOTIFICATION_CONSUMER_DEAD_LETTER_STREAM", c.PushNotifyConsumerDeadLetterStream, 1024); err != nil {
		return err
	}
	if err := validateEnum("GOGOMAIL_API_METERING_BACKEND", c.APIMeteringBackend, "none", "slog", "outbox"); err != nil {
		return err
	}
	if c.APIMeteringTimeout <= 0 {
		return fmt.Errorf("GOGOMAIL_API_METERING_TIMEOUT must be positive")
	}
	if err := validateEnum("GOGOMAIL_API_METERING_AGGREGATE_BACKEND", c.APIMeteringAggregateBackend, "disabled", "postgres"); err != nil {
		return err
	}
	if c.APIMeteringConsumerCount <= 0 {
		return fmt.Errorf("GOGOMAIL_API_METERING_CONSUMER_COUNT must be positive")
	}
	if c.APIMeteringConsumerBlock <= 0 {
		return fmt.Errorf("GOGOMAIL_API_METERING_CONSUMER_BLOCK must be positive")
	}
	if c.APIMeteringConsumerClaimIdle < 0 {
		return fmt.Errorf("GOGOMAIL_API_METERING_CONSUMER_CLAIM_IDLE must not be negative")
	}
	if c.APIMeteringConsumerMaxDeliveries < 0 {
		return fmt.Errorf("GOGOMAIL_API_METERING_CONSUMER_MAX_DELIVERIES must not be negative")
	}
	if err := validateBoundedNoCRLF("GOGOMAIL_API_METERING_CONSUMER_DEAD_LETTER_STREAM", c.APIMeteringConsumerDeadLetterStream, 1024); err != nil {
		return err
	}
	if c.APIUsageRetentionInterval <= 0 {
		return fmt.Errorf("GOGOMAIL_API_USAGE_RETENTION_INTERVAL must be positive")
	}
	if c.APIUsageRetentionCutoffAge <= 0 {
		return fmt.Errorf("GOGOMAIL_API_USAGE_RETENTION_CUTOFF_AGE must be positive")
	}
	if c.APIUsageRetentionBatchSize <= 0 || c.APIUsageRetentionBatchSize > 10000 {
		return fmt.Errorf("GOGOMAIL_API_USAGE_RETENTION_BATCH_SIZE must be between 1 and 10000")
	}
	if !c.APIUsageRetentionDryRun && !c.APIUsageRetentionConfirmReady {
		return fmt.Errorf("GOGOMAIL_API_USAGE_RETENTION_CONFIRM_READY is required when GOGOMAIL_API_USAGE_RETENTION_DRY_RUN=false")
	}
	if err := validateBoundedNoCRLF("GOGOMAIL_API_USAGE_RETENTION_TENANT_ID", c.APIUsageRetentionTenantID, 1024); err != nil {
		return err
	}
	if err := validateBoundedNoCRLF("GOGOMAIL_API_USAGE_RETENTION_PRINCIPAL_ID", c.APIUsageRetentionPrincipalID, 1024); err != nil {
		return err
	}
	if err := validateEnum("GOGOMAIL_API_USAGE_EXPORT_MANIFEST_SIGNER_BACKEND", c.APIUsageExportManifestSignerBackend, "disabled", "local-hmac", "local-ed25519", "remote-ed25519"); err != nil {
		return err
	}
	switch strings.ToLower(strings.TrimSpace(c.APIUsageExportManifestSignerBackend)) {
	case "local-hmac":
		if err := validateExportManifestSignerKeyID(c.APIUsageExportManifestSignerKeyID, "local-hmac"); err != nil {
			return err
		}
		if c.APIUsageExportManifestSignerSecret == "" {
			return fmt.Errorf("GOGOMAIL_API_USAGE_EXPORT_MANIFEST_SIGNER_SECRET is required for local-hmac signer")
		}
		if len(c.APIUsageExportManifestSignerSecret) > maxExportManifestSignerCredentialBytes {
			return fmt.Errorf("GOGOMAIL_API_USAGE_EXPORT_MANIFEST_SIGNER_SECRET is too long")
		}
	case "local-ed25519":
		if err := validateExportManifestSignerKeyID(c.APIUsageExportManifestSignerKeyID, "local-ed25519"); err != nil {
			return err
		}
		privateKey, err := decodeBase64Key("GOGOMAIL_API_USAGE_EXPORT_MANIFEST_SIGNER_PRIVATE_KEY", c.APIUsageExportSignerPrivateKey, ed25519.PrivateKeySize)
		if err != nil {
			return err
		}
		publicKey, err := decodeBase64Key("GOGOMAIL_API_USAGE_EXPORT_MANIFEST_SIGNER_PUBLIC_KEY", c.APIUsageExportSignerPublicKey, ed25519.PublicKeySize)
		if err != nil {
			return err
		}
		if !stringBytesEqual(ed25519.PrivateKey(privateKey).Public().(ed25519.PublicKey), publicKey) {
			return fmt.Errorf("GOGOMAIL_API_USAGE_EXPORT_MANIFEST_SIGNER_PUBLIC_KEY must match GOGOMAIL_API_USAGE_EXPORT_MANIFEST_SIGNER_PRIVATE_KEY")
		}
	case "remote-ed25519":
		if err := validateExportManifestSignerKeyID(c.APIUsageExportManifestSignerKeyID, "remote-ed25519"); err != nil {
			return err
		}
		if err := validateHTTPSURL("GOGOMAIL_API_USAGE_EXPORT_MANIFEST_SIGNER_URL", c.APIUsageExportSignerURL); err != nil {
			return err
		}
		if _, err := decodeBase64Key("GOGOMAIL_API_USAGE_EXPORT_MANIFEST_SIGNER_PUBLIC_KEY", c.APIUsageExportSignerPublicKey, ed25519.PublicKeySize); err != nil {
			return err
		}
		token := strings.TrimSpace(c.APIUsageExportSignerToken)
		if strings.ContainsAny(token, "\r\n") {
			return fmt.Errorf("GOGOMAIL_API_USAGE_EXPORT_MANIFEST_SIGNER_TOKEN cannot contain line breaks")
		}
		if len(token) > maxExportManifestSignerCredentialBytes {
			return fmt.Errorf("GOGOMAIL_API_USAGE_EXPORT_MANIFEST_SIGNER_TOKEN is too long")
		}
	}
	if !c.APIUsageRetentionDryRun && !strings.EqualFold(strings.TrimSpace(c.APIUsageExportManifestSignerBackend), "remote-ed25519") {
		return fmt.Errorf("GOGOMAIL_API_USAGE_RETENTION_DRY_RUN=false requires GOGOMAIL_API_USAGE_EXPORT_MANIFEST_SIGNER_BACKEND=remote-ed25519")
	}
	if err := validateEnum("GOGOMAIL_DELIVERY_TLS_MODE", c.DeliveryTLSMode, "opportunistic", "require", "disable"); err != nil {
		return err
	}
	if err := validateEnum("GOGOMAIL_DELIVERY_ROUTE_BACKEND", c.DeliveryRouteBackend, "env", "postgres"); err != nil {
		return err
	}
	if err := validateEnum("GOGOMAIL_SEARCH_INDEX_BACKEND", c.SearchIndexBackend, "disabled", "postgres", "opensearch"); err != nil {
		return err
	}
	if strings.EqualFold(strings.TrimSpace(c.SearchIndexBackend), "opensearch") {
		if strings.TrimSpace(c.SearchIndexOpenSearchEndpoint) == "" {
			return fmt.Errorf("GOGOMAIL_SEARCH_INDEX_OPENSEARCH_ENDPOINT is required when GOGOMAIL_SEARCH_INDEX_BACKEND=opensearch")
		}
		if err := validateHTTPURL("GOGOMAIL_SEARCH_INDEX_OPENSEARCH_ENDPOINT", c.SearchIndexOpenSearchEndpoint); err != nil {
			return err
		}
		if strings.TrimSpace(c.SearchIndexOpenSearchIndex) == "" {
			return fmt.Errorf("GOGOMAIL_SEARCH_INDEX_OPENSEARCH_INDEX is required when GOGOMAIL_SEARCH_INDEX_BACKEND=opensearch")
		}
		if err := validateOpenSearchIndexName("GOGOMAIL_SEARCH_INDEX_OPENSEARCH_INDEX", c.SearchIndexOpenSearchIndex); err != nil {
			return err
		}
	}
	if c.SearchIndexMaxBodyBytes <= 0 {
		return fmt.Errorf("GOGOMAIL_SEARCH_INDEX_MAX_BODY_BYTES must be positive")
	}
	if c.SearchIndexConsumerCount <= 0 {
		return fmt.Errorf("GOGOMAIL_SEARCH_INDEX_CONSUMER_COUNT must be positive")
	}
	if c.SearchIndexConsumerBlock <= 0 {
		return fmt.Errorf("GOGOMAIL_SEARCH_INDEX_CONSUMER_BLOCK must be positive")
	}
	if c.EventConsumerCount <= 0 {
		return fmt.Errorf("GOGOMAIL_EVENT_CONSUMER_COUNT must be positive")
	}
	if c.EventConsumerBlock <= 0 {
		return fmt.Errorf("GOGOMAIL_EVENT_CONSUMER_BLOCK must be positive")
	}
	if c.EventConsumerClaimIdle < 0 {
		return fmt.Errorf("GOGOMAIL_EVENT_CONSUMER_CLAIM_IDLE must not be negative")
	}
	if c.EventConsumerMaxDeliveries < 0 {
		return fmt.Errorf("GOGOMAIL_EVENT_CONSUMER_MAX_DELIVERIES must not be negative")
	}
	if err := validateBoundedNoCRLF("GOGOMAIL_EVENT_CONSUMER_DEAD_LETTER_STREAM", c.EventConsumerDeadLetterStream, 1024); err != nil {
		return err
	}
	if c.SearchIndexConsumerClaimIdle < 0 {
		return fmt.Errorf("GOGOMAIL_SEARCH_INDEX_CONSUMER_CLAIM_IDLE must not be negative")
	}
	if c.SearchIndexConsumerMaxDeliveries < 0 {
		return fmt.Errorf("GOGOMAIL_SEARCH_INDEX_CONSUMER_MAX_DELIVERIES must not be negative")
	}
	if err := validateBoundedNoCRLF("GOGOMAIL_SEARCH_INDEX_CONSUMER_DEAD_LETTER_STREAM", c.SearchIndexConsumerDeadLetterStream, 1024); err != nil {
		return err
	}
	if c.SearchIndexOpenSearchTimeout <= 0 {
		return fmt.Errorf("GOGOMAIL_SEARCH_INDEX_OPENSEARCH_TIMEOUT must be positive")
	}
	if strings.TrimSpace(c.DeliverySmartHostTLSMode) != "" {
		if err := validateEnum("GOGOMAIL_DELIVERY_SMARTHOST_TLS_MODE", c.DeliverySmartHostTLSMode, "opportunistic", "require", "disable"); err != nil {
			return err
		}
	}
	if strings.EqualFold(strings.TrimSpace(c.DeliveryRouteBackend), "postgres") && strings.TrimSpace(c.DeliverySmartHost) != "" {
		return fmt.Errorf("GOGOMAIL_DELIVERY_SMARTHOST cannot be combined with postgres delivery route backend")
	}
	if c.DeliverySmartHostPort < 0 || c.DeliverySmartHostPort > 65535 {
		return fmt.Errorf("GOGOMAIL_DELIVERY_SMARTHOST_PORT must be between 0 and 65535")
	}
	if strings.TrimSpace(c.DeliverySmartHost) == "" && (strings.TrimSpace(c.DeliverySmartHostUsername) != "" || strings.TrimSpace(c.DeliverySmartHostPassword) != "" || strings.TrimSpace(c.DeliverySmartHostIdentity) != "" || c.DeliverySmartHostPort > 0 || strings.TrimSpace(c.DeliverySmartHostTLSMode) != "" || c.DeliverySmartHostImplicitTLS) {
		return fmt.Errorf("GOGOMAIL_DELIVERY_SMARTHOST is required when smart-host options are set")
	}
	if c.DeliverySmartHostImplicitTLS && strings.EqualFold(strings.TrimSpace(c.DeliverySmartHostTLSMode), "disable") {
		return fmt.Errorf("GOGOMAIL_DELIVERY_SMARTHOST_IMPLICIT_TLS cannot be used with disabled smart-host TLS")
	}
	if strings.TrimSpace(c.DSNPostmaster) != "" {
		if _, err := mail.ParseAddress(strings.TrimSpace(c.DSNPostmaster)); err != nil {
			return fmt.Errorf("GOGOMAIL_DSN_POSTMASTER must be a valid mailbox address")
		}
	}
	if c.DeliveryRetryJitterRatio < 0 || c.DeliveryRetryJitterRatio > 1 {
		return fmt.Errorf("GOGOMAIL_DELIVERY_RETRY_JITTER_RATIO must be between 0 and 1")
	}
	if len(c.DeliveryRetryDelays) == 0 {
		return fmt.Errorf("GOGOMAIL_DELIVERY_RETRY_DELAYS must contain at least one delay")
	}
	for _, delay := range c.DeliveryRetryDelays {
		if delay <= 0 {
			return fmt.Errorf("GOGOMAIL_DELIVERY_RETRY_DELAYS must contain only positive durations")
		}
	}
	if c.DeliveryRetryMaxDelay <= 0 {
		return fmt.Errorf("GOGOMAIL_DELIVERY_RETRY_MAX_DELAY must be positive")
	}
	if c.DeliveryThrottleEnabled && c.DeliveryDefaultConcurrency == 0 && len(c.DeliveryFarmConcurrency) == 0 && len(c.DeliveryDomainConcurrency) == 0 {
		return fmt.Errorf("delivery throttling requires at least one default, farm, or domain concurrency limit")
	}
	if c.DeliveryConsumerCount <= 0 {
		return fmt.Errorf("GOGOMAIL_DELIVERY_CONSUMER_COUNT must be positive")
	}
	if c.DeliveryConsumerBlock <= 0 {
		return fmt.Errorf("GOGOMAIL_DELIVERY_CONSUMER_BLOCK must be positive")
	}
	if c.DeliveryConsumerClaimIdle < 0 {
		return fmt.Errorf("GOGOMAIL_DELIVERY_CONSUMER_CLAIM_IDLE must not be negative")
	}
	if c.DeliveryConsumerMaxDeliveries < 0 {
		return fmt.Errorf("GOGOMAIL_DELIVERY_CONSUMER_MAX_DELIVERIES must not be negative")
	}
	if err := validateBoundedNoCRLF("GOGOMAIL_DELIVERY_CONSUMER_DEAD_LETTER_STREAM", c.DeliveryConsumerDeadLetterStream, 1024); err != nil {
		return err
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

func validateTCPAddr(name string, value string, required bool) error {
	value = strings.TrimSpace(value)
	if value == "" {
		if required {
			return fmt.Errorf("%s is required", name)
		}
		return nil
	}
	if strings.ContainsAny(value, "\r\n") {
		return fmt.Errorf("%s cannot contain line breaks", name)
	}
	_, port, err := net.SplitHostPort(value)
	if err != nil {
		return fmt.Errorf("%s must be a TCP host:port address: %w", name, err)
	}
	parsedPort, err := strconv.Atoi(port)
	if err != nil || parsedPort < 1 || parsedPort > 65535 {
		return fmt.Errorf("%s must include a TCP port between 1 and 65535", name)
	}
	return nil
}

func validateHTTPSURL(name string, value string) error {
	value = strings.TrimSpace(value)
	if value == "" {
		return fmt.Errorf("%s is required", name)
	}
	parsed, err := url.Parse(value)
	if err != nil {
		return fmt.Errorf("%s must be a valid URL: %w", name, err)
	}
	if parsed.Scheme != "https" || parsed.Host == "" {
		return fmt.Errorf("%s must be an https URL", name)
	}
	return nil
}

func validateHTTPURL(name string, value string) error {
	value = strings.TrimSpace(value)
	if value == "" {
		return fmt.Errorf("%s is required", name)
	}
	if strings.ContainsAny(value, "\r\n") {
		return fmt.Errorf("%s cannot contain line breaks", name)
	}
	parsed, err := url.Parse(value)
	if err != nil {
		return fmt.Errorf("%s must be a valid URL: %w", name, err)
	}
	if (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" {
		return fmt.Errorf("%s must be an http or https URL", name)
	}
	return nil
}

func validateOpenSearchIndexName(name string, value string) error {
	value = strings.TrimSpace(value)
	if value == "" {
		return fmt.Errorf("%s is required", name)
	}
	if strings.ContainsAny(value, `/\?#*:,"<>| `) || strings.HasPrefix(value, ".") || strings.HasPrefix(value, "_") {
		return fmt.Errorf("%s is invalid", name)
	}
	return nil
}

func validateWebhookURL(name string, value string, requireHTTPS bool) error {
	value = strings.TrimSpace(value)
	if value == "" {
		return fmt.Errorf("%s is required", name)
	}
	if strings.ContainsAny(value, "\r\n") {
		return fmt.Errorf("%s cannot contain line breaks", name)
	}
	parsed, err := url.Parse(value)
	if err != nil {
		return fmt.Errorf("%s must be a valid URL: %w", name, err)
	}
	if (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" {
		return fmt.Errorf("%s must be an http or https URL", name)
	}
	if requireHTTPS && parsed.Scheme != "https" {
		return fmt.Errorf("%s must be an https URL in production", name)
	}
	return nil
}

func validateOptionalSecret(name string, value string) error {
	value = strings.TrimSpace(value)
	if strings.ContainsAny(value, "\r\n") {
		return fmt.Errorf("%s cannot contain line breaks", name)
	}
	if len(value) > maxWebhookTokenBytes {
		return fmt.Errorf("%s is too long", name)
	}
	return nil
}

func validateBoundedNoCRLF(name string, value string, maxBytes int) error {
	value = strings.TrimSpace(value)
	if strings.ContainsAny(value, "\r\n") {
		return fmt.Errorf("%s cannot contain line breaks", name)
	}
	if len(value) > maxBytes {
		return fmt.Errorf("%s is too long", name)
	}
	return nil
}

func validateExportManifestSignerKeyID(value string, backend string) error {
	value = strings.TrimSpace(value)
	if value == "" {
		return fmt.Errorf("GOGOMAIL_API_USAGE_EXPORT_MANIFEST_SIGNER_KEY_ID is required for %s signer", backend)
	}
	if strings.ContainsAny(value, "\r\n") {
		return fmt.Errorf("GOGOMAIL_API_USAGE_EXPORT_MANIFEST_SIGNER_KEY_ID cannot contain line breaks")
	}
	if len(value) > maxExportManifestSignerKeyIDBytes {
		return fmt.Errorf("GOGOMAIL_API_USAGE_EXPORT_MANIFEST_SIGNER_KEY_ID is too long")
	}
	return nil
}

func decodeBase64Key(name string, value string, size int) ([]byte, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil, fmt.Errorf("%s is required", name)
	}
	if len(value) > base64.StdEncoding.EncodedLen(size) {
		return nil, fmt.Errorf("%s is too long", name)
	}
	decoded, err := base64.StdEncoding.DecodeString(value)
	if err != nil {
		return nil, fmt.Errorf("%s must be base64: %w", name, err)
	}
	if len(decoded) != size {
		return nil, fmt.Errorf("%s must decode to %d bytes", name, size)
	}
	return decoded, nil
}

func stringBytesEqual(a []byte, b []byte) bool {
	if len(a) != len(b) {
		return false
	}
	var diff byte
	for i := range a {
		diff |= a[i] ^ b[i]
	}
	return diff == 0
}
