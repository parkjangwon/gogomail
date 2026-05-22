package config

import (
	"crypto/ed25519"
	"crypto/x509"
	"encoding/base64"
	"fmt"
	"net"
	"net/mail"
	"net/netip"
	"net/url"
	"os"
	"strconv"
	"strings"

	"github.com/gogomail/gogomail/internal/storage"
)

const (
	maxExportManifestSignerKeyIDBytes      = 200
	maxExportManifestSignerCredentialBytes = 4096
	maxOpenSearchCredentialBytes           = 4096
	maxDeliverySmartHostCredentialBytes    = 4096
	maxWebhookTokenBytes                   = 4096
	maxAttachmentCleanupBatchSize          = 1000
	maxDriveCleanupBatchSize               = 1000
	maxDAVSyncRetentionBatchSize           = 10000
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
	if c.IMAPAllowInsecureAuth && strings.EqualFold(strings.TrimSpace(c.Environment), "production") {
		return fmt.Errorf("GOGOMAIL_IMAP_ALLOW_INSECURE_AUTH must be false in production")
	}
	if c.CalDAVAllowInsecureAuth && strings.EqualFold(strings.TrimSpace(c.Environment), "production") {
		return fmt.Errorf("GOGOMAIL_CALDAV_ALLOW_INSECURE_AUTH must be false in production")
	}
	if c.CardDAVAllowInsecureAuth && strings.EqualFold(strings.TrimSpace(c.Environment), "production") {
		return fmt.Errorf("GOGOMAIL_CARDDAV_ALLOW_INSECURE_AUTH must be false in production")
	}
	if err := validateTrustedProxies("GOGOMAIL_CALDAV_TRUSTED_PROXIES", c.CalDAVTrustedProxies); err != nil {
		return err
	}
	if err := validateTrustedProxies("GOGOMAIL_CARDDAV_TRUSTED_PROXIES", c.CardDAVTrustedProxies); err != nil {
		return err
	}
	if (c.SMTPTLSCertFile == "") != (c.SMTPTLSKeyFile == "") {
		return fmt.Errorf("both SMTP TLS certificate and key files are required")
	}
	if (c.IMAPTLSCertFile == "") != (c.IMAPTLSKeyFile == "") {
		return fmt.Errorf("both IMAP TLS certificate and key files are required")
	}
	if (c.LDAPTLSCertFile == "") != (c.LDAPTLSKeyFile == "") {
		return fmt.Errorf("both LDAP TLS certificate and key files are required")
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
	if err := validateTCPAddr("GOGOMAIL_IMAP_ADDR", c.IMAPAddr, true); err != nil {
		return err
	}
	if err := validateTCPAddr("GOGOMAIL_POP3S_ADDR", c.POP3SAddr, false); err != nil {
		return err
	}
	if err := validateTCPAddr("GOGOMAIL_CALDAV_ADDR", c.CalDAVAddr, true); err != nil {
		return err
	}
	if err := validateTCPAddr("GOGOMAIL_CARDDAV_ADDR", c.CardDAVAddr, true); err != nil {
		return err
	}
	if err := validateTCPAddr("GOGOMAIL_WEBDAV_ADDR", c.WebDAVAddr, true); err != nil {
		return err
	}
	if err := validateTCPAddr("GOGOMAIL_LDAP_ADDR", c.LDAPAddr, false); err != nil {
		return err
	}
	if err := validateTCPAddr("GOGOMAIL_LDAPS_ADDR", c.LDAPSAddr, false); err != nil {
		return err
	}
	if strings.TrimSpace(c.LDAPSAddr) != "" && (c.LDAPTLSCertFile == "" || c.LDAPTLSKeyFile == "") {
		return fmt.Errorf("GOGOMAIL_LDAPS_ADDR requires LDAP TLS certificate and key files")
	}
	if err := validateBoundedNoCRLF("GOGOMAIL_IMAP_TLS_CERT_FILE", c.IMAPTLSCertFile, 4096); err != nil {
		return err
	}
	if err := validateBoundedNoCRLF("GOGOMAIL_IMAP_TLS_KEY_FILE", c.IMAPTLSKeyFile, 4096); err != nil {
		return err
	}
	if err := validateBoundedNoCRLF("GOGOMAIL_LDAP_TLS_CERT_FILE", c.LDAPTLSCertFile, 4096); err != nil {
		return err
	}
	if err := validateBoundedNoCRLF("GOGOMAIL_LDAP_TLS_KEY_FILE", c.LDAPTLSKeyFile, 4096); err != nil {
		return err
	}
	for _, referralURL := range c.LDAPReferralURLs {
		if err := validateLDAPReferralURL(referralURL); err != nil {
			return err
		}
	}
	if c.IMAPMaxConnections < 0 {
		return fmt.Errorf("GOGOMAIL_IMAP_MAX_CONNECTIONS must not be negative")
	}
	if c.IMAPReadTimeout < 0 {
		return fmt.Errorf("GOGOMAIL_IMAP_READ_TIMEOUT must not be negative")
	}
	if c.IMAPWriteTimeout < 0 {
		return fmt.Errorf("GOGOMAIL_IMAP_WRITE_TIMEOUT must not be negative")
	}
	if c.IMAPIdleTimeout < 0 {
		return fmt.Errorf("GOGOMAIL_IMAP_IDLE_TIMEOUT must not be negative")
	}
	if c.POP3MaxConnections < 0 {
		return fmt.Errorf("GOGOMAIL_POP3_MAX_CONNECTIONS must not be negative")
	}
	if c.IMAPNotifyConsumerCount <= 0 {
		return fmt.Errorf("GOGOMAIL_IMAP_NOTIFY_CONSUMER_COUNT must be positive")
	}
	if err := validateRequiredBoundedNoCRLF("GOGOMAIL_IMAP_NOTIFY_CONSUMER_GROUP", c.IMAPNotifyConsumerGroup, 1024); err != nil {
		return err
	}
	if err := validateRequiredBoundedNoCRLF("GOGOMAIL_IMAP_NOTIFY_CONSUMER_NAME", c.IMAPNotifyConsumerName, 1024); err != nil {
		return err
	}
	if c.IMAPNotifyConsumerBlock <= 0 {
		return fmt.Errorf("GOGOMAIL_IMAP_NOTIFY_CONSUMER_BLOCK must be positive")
	}
	if c.IMAPNotifyConsumerClaimIdle < 0 {
		return fmt.Errorf("GOGOMAIL_IMAP_NOTIFY_CONSUMER_CLAIM_IDLE must not be negative")
	}
	if c.IMAPNotifyConsumerMaxDeliveries < 0 {
		return fmt.Errorf("GOGOMAIL_IMAP_NOTIFY_CONSUMER_MAX_DELIVERIES must not be negative")
	}
	if err := validateBoundedNoCRLF("GOGOMAIL_IMAP_NOTIFY_CONSUMER_DEAD_LETTER_STREAM", c.IMAPNotifyConsumerDeadLetterStream, 1024); err != nil {
		return err
	}
	if err := validateTCPAddr("GOGOMAIL_SUBMISSION_ADDR", c.SubmissionAddr, true); err != nil {
		return err
	}
	if err := validateTCPAddr("GOGOMAIL_SUBMISSION_SMTPS_ADDR", c.SubmissionSMTPSAddr, false); err != nil {
		return err
	}
	if err := validateEnum("GOGOMAIL_STORAGE_BACKEND", c.StorageBackend, "local", "nfs", "s3", "minio"); err != nil {
		return err
	}
	storageBackend := strings.ToLower(strings.TrimSpace(c.StorageBackend))
	for _, label := range c.StorageBackendCompatLabels {
		if err := validateStorageBackendCompatLabel(label); err != nil {
			return err
		}
	}
	if storageBackend == "local" || storageBackend == "nfs" {
		if err := validateRequiredBoundedNoCRLF("GOGOMAIL_MAILSTORE_ROOT", c.MailstoreRoot, 4096); err != nil {
			return err
		}
	} else if err := validateBoundedNoCRLF("GOGOMAIL_MAILSTORE_ROOT", c.MailstoreRoot, 4096); err != nil {
		return err
	}
	if storageBackend == "s3" || storageBackend == "minio" {
		if storageBackend == "minio" && strings.TrimSpace(c.StorageS3Endpoint) == "" {
			return fmt.Errorf("GOGOMAIL_STORAGE_S3_ENDPOINT is required when GOGOMAIL_STORAGE_BACKEND=minio")
		}
		if production && storageBackend == "s3" && strings.TrimSpace(c.StorageS3Endpoint) == "" {
			return fmt.Errorf("GOGOMAIL_STORAGE_S3_ENDPOINT is required in production when GOGOMAIL_STORAGE_BACKEND=s3")
		}
		if strings.TrimSpace(c.StorageS3Endpoint) != "" {
			if err := validateHTTPURL("GOGOMAIL_STORAGE_S3_ENDPOINT", c.StorageS3Endpoint); err != nil {
				return err
			}
			if _, err := storage.ValidateS3Endpoint(c.StorageS3Endpoint); err != nil {
				return fmt.Errorf("GOGOMAIL_STORAGE_S3_ENDPOINT: %w", err)
			}
			if production && (storageBackend == "s3" || storageBackend == "minio") && !strings.HasPrefix(strings.ToLower(strings.TrimSpace(c.StorageS3Endpoint)), "https://") {
				return fmt.Errorf("GOGOMAIL_STORAGE_S3_ENDPOINT must use https in production (backend=%s)", storageBackend)
			}
		}
		if err := validateRequiredBoundedNoCRLF("GOGOMAIL_STORAGE_S3_REGION", c.StorageS3Region, 128); err != nil {
			return err
		}
		if err := storage.ValidateS3Region(c.StorageS3Region); err != nil {
			return fmt.Errorf("GOGOMAIL_STORAGE_S3_REGION: %w", err)
		}
		if err := validateRequiredBoundedNoCRLF("GOGOMAIL_STORAGE_S3_BUCKET", c.StorageS3Bucket, 255); err != nil {
			return err
		}
		if err := storage.ValidateS3BucketName(c.StorageS3Bucket); err != nil {
			return fmt.Errorf("GOGOMAIL_STORAGE_S3_BUCKET: %w", err)
		}
		if err := validateBoundedNoCRLF("GOGOMAIL_STORAGE_S3_PREFIX", c.StorageS3Prefix, 1024); err != nil {
			return err
		}
		if strings.Trim(strings.TrimSpace(c.StorageS3Prefix), "/") != "" {
			if _, err := storage.ValidateObjectPath(strings.Trim(strings.TrimSpace(c.StorageS3Prefix), "/")); err != nil {
				return fmt.Errorf("GOGOMAIL_STORAGE_S3_PREFIX: %w", err)
			}
		}
		if err := validateS3CredentialNoWhitespace("GOGOMAIL_STORAGE_S3_ACCESS_KEY_ID", c.StorageS3AccessKeyID, 4096, true); err != nil {
			return err
		}
		if err := validateS3CredentialNoWhitespace("GOGOMAIL_STORAGE_S3_SECRET_ACCESS_KEY", c.StorageS3SecretAccessKey, 4096, true); err != nil {
			return err
		}
		if err := validateS3CredentialNoWhitespace("GOGOMAIL_STORAGE_S3_SESSION_TOKEN", c.StorageS3SessionToken, 8192, false); err != nil {
			return err
		}
		if err := validateBoundedNoCRLF("GOGOMAIL_STORAGE_S3_CA_CERT_FILE", c.StorageS3CACertFile, 4096); err != nil {
			return err
		}
		if strings.TrimSpace(c.StorageS3CACertFile) != "" {
			if err := validateCACertFile("GOGOMAIL_STORAGE_S3_CA_CERT_FILE", c.StorageS3CACertFile); err != nil {
				return err
			}
		}
		if production && c.StorageS3InsecureSkipVerify {
			return fmt.Errorf("GOGOMAIL_STORAGE_S3_INSECURE_SKIP_VERIFY must be false in production")
		}
	}
	if err := validateEnum("GOGOMAIL_DEDUP_BACKEND", c.DedupBackend, "none", "redis"); err != nil {
		return err
	}
	if err := validateEnum("GOGOMAIL_RATELIMIT_BACKEND", c.RateLimitBackend, "none", "redis"); err != nil {
		return err
	}
	if err := validateEnum("GOGOMAIL_DRIVE_SHARE_RATELIMIT_BACKEND", c.DriveShareRateLimitBackend, "none", "redis"); err != nil {
		return err
	}
	if err := validateEnum("GOGOMAIL_BACKPRESSURE_BACKEND", c.BackpressureBackend, "none", "redis"); err != nil {
		return err
	}
	if err := validateEnum("GOGOMAIL_DELIVERY_THROTTLE_BACKEND", c.DeliveryThrottleBackend, "local", "redis"); err != nil {
		return err
	}
	if c.RcptRateLimitPerMinute <= 0 {
		return fmt.Errorf("GOGOMAIL_RCPT_RATE_LIMIT_PER_MINUTE must be positive")
	}
	if c.DriveShareRateLimitPerMinute <= 0 {
		return fmt.Errorf("GOGOMAIL_DRIVE_SHARE_RATELIMIT_PER_MINUTE must be positive")
	}
	if c.DeliveryDomainBackoffEnabled {
		if err := validateEnum("GOGOMAIL_DELIVERY_DOMAIN_BACKOFF_BACKEND", c.DeliveryDomainBackoffBackend, "local", "redis"); err != nil {
			return err
		}
		if err := validateEnum("GOGOMAIL_DELIVERY_DOMAIN_BACKOFF_SCOPE", c.DeliveryDomainBackoffScope, "domain", "farm_domain"); err != nil {
			return err
		}
		if c.DeliveryDomainBackoffBaseDelay <= 0 {
			return fmt.Errorf("GOGOMAIL_DELIVERY_DOMAIN_BACKOFF_BASE_DELAY must be positive")
		}
		if c.DeliveryDomainBackoffMaxDelay <= 0 {
			return fmt.Errorf("GOGOMAIL_DELIVERY_DOMAIN_BACKOFF_MAX_DELAY must be positive")
		}
		if c.DeliveryDomainBackoffMaxDelay < c.DeliveryDomainBackoffBaseDelay {
			return fmt.Errorf("GOGOMAIL_DELIVERY_DOMAIN_BACKOFF_MAX_DELAY must be greater than or equal to base delay")
		}
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
	if c.SMTPMaxConnections < 0 {
		return fmt.Errorf("GOGOMAIL_SMTP_MAX_CONNECTIONS must not be negative")
	}
	if c.SubmissionMaxConnections < 0 {
		return fmt.Errorf("GOGOMAIL_SUBMISSION_MAX_CONNECTIONS must not be negative")
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
	if production && isLocalProductionHostname(c.SMTPDomain) {
		return fmt.Errorf("GOGOMAIL_SMTP_DOMAIN must not be localhost, loopback, or unspecified in production")
	}
	if c.DeliveryTimeout <= 0 {
		return fmt.Errorf("GOGOMAIL_DELIVERY_TIMEOUT must be positive")
	}
	if c.DeliveryRecipientBatchSize <= 0 {
		return fmt.Errorf("GOGOMAIL_DELIVERY_RECIPIENT_BATCH_SIZE must be positive")
	}
	if c.MessageBodyCacheEntries < 0 {
		return fmt.Errorf("GOGOMAIL_MESSAGE_BODY_CACHE_ENTRIES must not be negative")
	}
	if c.MessageBodyCacheEntries > 0 && c.MessageBodyCacheTTL <= 0 {
		return fmt.Errorf("GOGOMAIL_MESSAGE_BODY_CACHE_TTL must be positive when message body cache is enabled")
	}
	if strings.TrimSpace(c.DeliverySMTPHello) == "" || strings.ContainsAny(c.DeliverySMTPHello, " \t\r\n") {
		return fmt.Errorf("GOGOMAIL_DELIVERY_SMTP_HELLO must be a non-empty hostname without whitespace")
	}
	if production && isLocalProductionHostname(c.DeliverySMTPHello) {
		return fmt.Errorf("GOGOMAIL_DELIVERY_SMTP_HELLO must not be localhost, loopback, or unspecified in production")
	}
	if c.SMTPMaxDKIMVerifications <= 0 {
		return fmt.Errorf("GOGOMAIL_SMTP_MAX_DKIM_VERIFICATIONS must be positive")
	}
	if err := validateEnum("GOGOMAIL_SMTP_DMARC_ENFORCEMENT", c.SMTPDMARCEnforcement, "monitor", "quarantine", "reject"); err != nil {
		return err
	}
	if err := validateEnum("GOGOMAIL_METRICS_BACKEND", c.MetricsBackend, "none", "slog", "prometheus"); err != nil {
		return err
	}
	if err := validateEnum("GOGOMAIL_ATTACHMENT_SCAN_BACKEND", c.AttachmentScanBackend, "none", "webhook", "clamav"); err != nil {
		return err
	}
	switch strings.ToLower(strings.TrimSpace(c.AttachmentScanBackend)) {
	case "webhook":
		if err := validateWebhookURL("GOGOMAIL_ATTACHMENT_SCAN_WEBHOOK_URL", c.AttachmentScanWebhookURL, production); err != nil {
			return err
		}
		if err := validateOptionalSecret("GOGOMAIL_ATTACHMENT_SCAN_WEBHOOK_TOKEN", c.AttachmentScanWebhookToken); err != nil {
			return err
		}
	case "clamav":
		if strings.TrimSpace(c.AttachmentScanClamAVAddr) == "" || strings.ContainsAny(c.AttachmentScanClamAVAddr, " \t\r\n") {
			return fmt.Errorf("GOGOMAIL_ATTACHMENT_SCAN_CLAMAV_ADDR must be a non-empty host:port without whitespace")
		}
		if _, _, err := net.SplitHostPort(c.AttachmentScanClamAVAddr); err != nil {
			return fmt.Errorf("GOGOMAIL_ATTACHMENT_SCAN_CLAMAV_ADDR must be host:port: %w", err)
		}
	}
	if c.AttachmentScanTimeout <= 0 {
		return fmt.Errorf("GOGOMAIL_ATTACHMENT_SCAN_TIMEOUT must be positive")
	}
	if c.AttachmentScanMaxConcurrency <= 0 {
		return fmt.Errorf("GOGOMAIL_ATTACHMENT_SCAN_MAX_CONCURRENCY must be positive")
	}
	if c.AttachmentScanMaxConcurrency > 1024 {
		return fmt.Errorf("GOGOMAIL_ATTACHMENT_SCAN_MAX_CONCURRENCY must be <= 1024")
	}
	if c.AttachmentScanMaxBytes <= 0 {
		return fmt.Errorf("GOGOMAIL_ATTACHMENT_SCAN_MAX_BYTES must be positive")
	}
	if c.AttachmentScanFailureThreshold <= 0 {
		return fmt.Errorf("GOGOMAIL_ATTACHMENT_SCAN_FAILURE_THRESHOLD must be positive")
	}
	if c.AttachmentScanCircuitOpenDuration <= 0 {
		return fmt.Errorf("GOGOMAIL_ATTACHMENT_SCAN_CIRCUIT_OPEN_DURATION must be positive")
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
	if c.DriveCleanupInterval <= 0 {
		return fmt.Errorf("GOGOMAIL_DRIVE_CLEANUP_INTERVAL must be positive")
	}
	if c.DriveCleanupBatchSize <= 0 || c.DriveCleanupBatchSize > maxDriveCleanupBatchSize {
		return fmt.Errorf("GOGOMAIL_DRIVE_CLEANUP_BATCH_SIZE must be between 1 and %d", maxDriveCleanupBatchSize)
	}
	if c.DAVSyncRetentionInterval <= 0 {
		return fmt.Errorf("GOGOMAIL_DAV_SYNC_RETENTION_INTERVAL must be positive")
	}
	if c.DAVSyncRetentionCutoffAge <= 0 {
		return fmt.Errorf("GOGOMAIL_DAV_SYNC_RETENTION_CUTOFF_AGE must be positive")
	}
	if c.DAVSyncRetentionBatchSize <= 0 || c.DAVSyncRetentionBatchSize > maxDAVSyncRetentionBatchSize {
		return fmt.Errorf("GOGOMAIL_DAV_SYNC_RETENTION_BATCH_SIZE must be between 1 and %d", maxDAVSyncRetentionBatchSize)
	}
	if !c.DAVSyncRetentionDryRun && !c.DAVSyncRetentionConfirmReady {
		return fmt.Errorf("GOGOMAIL_DAV_SYNC_RETENTION_CONFIRM_READY must be true when DAV sync retention dry-run is disabled")
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
	if err := validateRequiredBoundedNoCRLF("GOGOMAIL_PUSH_NOTIFICATION_CONSUMER_GROUP", c.PushNotifyConsumerGroup, 1024); err != nil {
		return err
	}
	if err := validateRequiredBoundedNoCRLF("GOGOMAIL_PUSH_NOTIFICATION_CONSUMER_NAME", c.PushNotifyConsumerName, 1024); err != nil {
		return err
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
	if err := validateRequiredBoundedNoCRLF("GOGOMAIL_API_METERING_STREAM", c.APIMeteringStream, 1024); err != nil {
		return err
	}
	if err := validateRequiredBoundedNoCRLF("GOGOMAIL_API_METERING_CONSUMER_GROUP", c.APIMeteringConsumerGroup, 1024); err != nil {
		return err
	}
	if err := validateRequiredBoundedNoCRLF("GOGOMAIL_API_METERING_CONSUMER_NAME", c.APIMeteringConsumerName, 1024); err != nil {
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
		if err := validateBoundedNoCRLF("GOGOMAIL_SEARCH_INDEX_OPENSEARCH_USERNAME", c.SearchIndexOpenSearchUsername, maxOpenSearchCredentialBytes); err != nil {
			return err
		}
		if err := validateBoundedNoCRLF("GOGOMAIL_SEARCH_INDEX_OPENSEARCH_PASSWORD", c.SearchIndexOpenSearchPassword, maxOpenSearchCredentialBytes); err != nil {
			return err
		}
	}
	if c.SearchIndexMaxBodyBytes <= 0 {
		return fmt.Errorf("GOGOMAIL_SEARCH_INDEX_MAX_BODY_BYTES must be positive")
	}
	if c.SearchIndexConsumerCount <= 0 {
		return fmt.Errorf("GOGOMAIL_SEARCH_INDEX_CONSUMER_COUNT must be positive")
	}
	if err := validateRequiredBoundedNoCRLF("GOGOMAIL_SEARCH_INDEX_CONSUMER_GROUP", c.SearchIndexConsumerGroup, 1024); err != nil {
		return err
	}
	if err := validateRequiredBoundedNoCRLF("GOGOMAIL_SEARCH_INDEX_CONSUMER_NAME", c.SearchIndexConsumerName, 1024); err != nil {
		return err
	}
	if c.SearchIndexConsumerBlock <= 0 {
		return fmt.Errorf("GOGOMAIL_SEARCH_INDEX_CONSUMER_BLOCK must be positive")
	}
	if c.EventConsumerCount <= 0 {
		return fmt.Errorf("GOGOMAIL_EVENT_CONSUMER_COUNT must be positive")
	}
	if err := validateRequiredBoundedNoCRLF("GOGOMAIL_EVENT_STREAM", c.EventStream, 1024); err != nil {
		return err
	}
	if err := validateRequiredBoundedNoCRLF("GOGOMAIL_EVENT_CONSUMER_GROUP", c.EventConsumerGroup, 1024); err != nil {
		return err
	}
	if err := validateRequiredBoundedNoCRLF("GOGOMAIL_EVENT_CONSUMER_NAME", c.EventConsumerName, 1024); err != nil {
		return err
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
	if c.MailFlowOpenSearchBootstrap {
		if strings.TrimSpace(c.SearchIndexOpenSearchEndpoint) == "" {
			return fmt.Errorf("GOGOMAIL_SEARCH_INDEX_OPENSEARCH_ENDPOINT is required when GOGOMAIL_MAIL_FLOW_OPENSEARCH_BOOTSTRAP=true")
		}
		if strings.TrimSpace(c.MailFlowOpenSearchIndex) == "" {
			return fmt.Errorf("GOGOMAIL_MAIL_FLOW_OPENSEARCH_INDEX is required when GOGOMAIL_MAIL_FLOW_OPENSEARCH_BOOTSTRAP=true")
		}
		if err := validateOpenSearchIndexName("GOGOMAIL_MAIL_FLOW_OPENSEARCH_INDEX", c.MailFlowOpenSearchIndex); err != nil {
			return err
		}
	}
	if err := validateEnum("GOGOMAIL_MAIL_FLOW_STATS_BACKEND", c.MailFlowStatsBackend, "auto", "postgres", "opensearch"); err != nil {
		return err
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
	if strings.TrimSpace(c.DeliverySmartHostPassword) != "" && strings.TrimSpace(c.DeliverySmartHostUsername) == "" {
		return fmt.Errorf("GOGOMAIL_DELIVERY_SMARTHOST_USERNAME is required when GOGOMAIL_DELIVERY_SMARTHOST_PASSWORD is set")
	}
	if err := validateBoundedNoCRLF("GOGOMAIL_DELIVERY_SMARTHOST_USERNAME", c.DeliverySmartHostUsername, maxDeliverySmartHostCredentialBytes); err != nil {
		return err
	}
	if err := validateBoundedNoCRLF("GOGOMAIL_DELIVERY_SMARTHOST_PASSWORD", c.DeliverySmartHostPassword, maxDeliverySmartHostCredentialBytes); err != nil {
		return err
	}
	if err := validateBoundedNoCRLF("GOGOMAIL_DELIVERY_SMARTHOST_IDENTITY", c.DeliverySmartHostIdentity, maxDeliverySmartHostCredentialBytes); err != nil {
		return err
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
	farmCoordinatorBackend := strings.ToLower(strings.TrimSpace(c.FarmCoordinatorBackend))
	if err := validateEnum("GOGOMAIL_FARM_COORDINATOR_BACKEND", farmCoordinatorBackend, "noop", "redis"); err != nil {
		return err
	}
	if production && farmCoordinatorBackend == "noop" {
		return fmt.Errorf("GOGOMAIL_FARM_COORDINATOR_BACKEND must be redis in production")
	}
	if farmCoordinatorBackend == "redis" && strings.TrimSpace(c.RedisAddr) == "" && len(c.RedisSentinelAddrs) == 0 {
		return fmt.Errorf("GOGOMAIL_REDIS_ADDR or GOGOMAIL_REDIS_SENTINEL_ADDRS is required when GOGOMAIL_FARM_COORDINATOR_BACKEND=redis")
	}
	if production && farmCoordinatorBackend == "redis" && strings.TrimSpace(c.RedisPassword) == "" {
		return fmt.Errorf("GOGOMAIL_REDIS_PASSWORD must not be empty in production when GOGOMAIL_FARM_COORDINATOR_BACKEND=redis")
	}
	if c.FarmCoordinatorHeartbeatTTL <= 0 {
		return fmt.Errorf("GOGOMAIL_FARM_COORDINATOR_HEARTBEAT_TTL must be positive")
	}
	if c.FarmCoordinatorJobVisibilityTimeout <= 0 {
		return fmt.Errorf("GOGOMAIL_FARM_COORDINATOR_JOB_VISIBILITY_TIMEOUT must be positive")
	}
	if c.DeliveryConsumerCount <= 0 {
		return fmt.Errorf("GOGOMAIL_DELIVERY_CONSUMER_COUNT must be positive")
	}
	if err := validateRequiredBoundedNoCRLF("GOGOMAIL_DELIVERY_STREAM", c.DeliveryStream, 1024); err != nil {
		return err
	}
	if err := validateRequiredBoundedNoCRLF("GOGOMAIL_DELIVERY_CONSUMER_GROUP", c.DeliveryConsumerGroup, 1024); err != nil {
		return err
	}
	if err := validateRequiredBoundedNoCRLF("GOGOMAIL_DELIVERY_CONSUMER_NAME", c.DeliveryConsumerName, 1024); err != nil {
		return err
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
	if err := validatePublicBaseURL(c.PublicBaseURL, production); err != nil {
		return err
	}
	if production {
		if strings.TrimSpace(c.AuthJWTSecret) == "" {
			return fmt.Errorf("GOGOMAIL_AUTH_JWT_SECRET must not be empty in production")
		}
		if len([]byte(strings.TrimSpace(c.AuthJWTSecret))) < 32 {
			return fmt.Errorf("GOGOMAIL_AUTH_JWT_SECRET must be at least 32 bytes in production")
		}
		if strings.TrimSpace(c.AdminToken) == "" {
			return fmt.Errorf("GOGOMAIL_ADMIN_TOKEN must not be empty in production")
		}
		if strings.Contains(c.DatabaseURL, "sslmode=disable") {
			return fmt.Errorf("GOGOMAIL_DATABASE_URL must not use sslmode=disable in production")
		}
	}
	return nil
}

func validatePublicBaseURL(value string, production bool) error {
	value = strings.TrimRight(strings.TrimSpace(value), "/")
	if value == "" {
		if production {
			return fmt.Errorf("GOGOMAIL_PUBLIC_BASE_URL must not be empty in production")
		}
		return nil
	}
	parsed, err := url.Parse(value)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" || parsed.RawQuery != "" || parsed.Fragment != "" {
		return fmt.Errorf("GOGOMAIL_PUBLIC_BASE_URL must be an absolute URL without query or fragment")
	}
	switch parsed.Scheme {
	case "http":
		if production {
			return fmt.Errorf("GOGOMAIL_PUBLIC_BASE_URL must use https in production")
		}
	case "https":
	default:
		return fmt.Errorf("GOGOMAIL_PUBLIC_BASE_URL must use http or https")
	}
	if production && isLocalPublicBaseURLHost(parsed.Hostname()) {
		return fmt.Errorf("GOGOMAIL_PUBLIC_BASE_URL must not point to localhost in production")
	}
	return nil
}

func isLocalPublicBaseURLHost(host string) bool {
	return isLocalProductionHostname(host)
}

func isLocalProductionHostname(host string) bool {
	host = strings.Trim(strings.ToLower(strings.TrimSpace(host)), "[]")
	if host == "localhost" || host == "localhost.localdomain" {
		return true
	}
	ip := net.ParseIP(host)
	if ip == nil {
		return false
	}
	return ip.IsLoopback() || ip.IsUnspecified()
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

func validateStorageBackendCompatLabel(value string) error {
	const name = "GOGOMAIL_STORAGE_BACKEND_COMPAT_LABELS"
	label := strings.ToLower(strings.TrimSpace(value))
	if label == "" {
		return fmt.Errorf("%s label is required", name)
	}
	if len(label) > 64 {
		return fmt.Errorf("%s label is too long", name)
	}
	for _, r := range label {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '.' || r == '_' || r == '-' {
			continue
		}
		return fmt.Errorf("%s label %q must contain only lowercase letters, digits, dot, underscore, or hyphen", name, value)
	}
	return nil
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

func validateRequiredBoundedNoCRLF(name string, value string, maxBytes int) error {
	value = strings.TrimSpace(value)
	if value == "" {
		return fmt.Errorf("%s is required", name)
	}
	return validateBoundedNoCRLF(name, value, maxBytes)
}

func validateTrustedProxies(name string, values []string) error {
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, err := netip.ParsePrefix(value); err == nil {
			continue
		}
		if _, err := netip.ParseAddr(value); err == nil {
			continue
		}
		return fmt.Errorf("%s contains invalid trusted proxy %q", name, value)
	}
	return nil
}

func validateS3CredentialNoWhitespace(name string, value string, maxBytes int, required bool) error {
	if required && value == "" {
		return fmt.Errorf("%s is required", name)
	}
	if value == "" {
		return nil
	}
	if strings.ContainsAny(value, " \t\r\n") {
		return fmt.Errorf("%s cannot contain whitespace", name)
	}
	if len(value) > maxBytes {
		return fmt.Errorf("%s is too long", name)
	}
	return nil
}

func validateLDAPReferralURL(value string) error {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	if strings.ContainsAny(value, "\r\n") {
		return fmt.Errorf("GOGOMAIL_LDAP_REFERRAL_URLS cannot contain line breaks")
	}
	if len(value) > 4096 {
		return fmt.Errorf("GOGOMAIL_LDAP_REFERRAL_URLS entry is too long")
	}
	u, err := url.Parse(value)
	if err != nil {
		return fmt.Errorf("GOGOMAIL_LDAP_REFERRAL_URLS entry is invalid: %w", err)
	}
	if u.Scheme != "ldap" && u.Scheme != "ldaps" {
		return fmt.Errorf("GOGOMAIL_LDAP_REFERRAL_URLS entries must use ldap or ldaps")
	}
	if strings.TrimSpace(u.Host) == "" {
		return fmt.Errorf("GOGOMAIL_LDAP_REFERRAL_URLS entries must include a host")
	}
	return nil
}

func validateCACertFile(name string, path string) error {
	data, err := os.ReadFile(strings.TrimSpace(path))
	if err != nil {
		return fmt.Errorf("%s cannot be read: %w", name, err)
	}
	pool := x509.NewCertPool()
	if !pool.AppendCertsFromPEM(data) {
		return fmt.Errorf("%s must contain at least one PEM-encoded certificate", name)
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
