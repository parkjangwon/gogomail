package config

import (
	"testing"
	"time"
)

func TestLoadAppliesDefaults(t *testing.T) {
	t.Setenv("GOGOMAIL_ENV", "")
	t.Setenv("GOGOMAIL_HTTP_ADDR", "")
	t.Setenv("GOGOMAIL_HTTP_READ_TIMEOUT", "")
	t.Setenv("GOGOMAIL_HTTP_WRITE_TIMEOUT", "")
	t.Setenv("GOGOMAIL_HTTP_IDLE_TIMEOUT", "")
	t.Setenv("GOGOMAIL_HTTP_READ_HEADER_TIMEOUT", "")
	t.Setenv("GOGOMAIL_HTTP_MAX_HEADER_BYTES", "")
	t.Setenv("GOGOMAIL_SMTP_ADDR", "")
	t.Setenv("GOGOMAIL_INBOUND_SMTP_ADDR", "")
	t.Setenv("GOGOMAIL_INBOUND_TRUSTED_RELAYS", "")
	t.Setenv("GOGOMAIL_IMAP_ADDR", "")
	t.Setenv("GOGOMAIL_IMAP_TLS_CERT_FILE", "")
	t.Setenv("GOGOMAIL_IMAP_TLS_KEY_FILE", "")
	t.Setenv("GOGOMAIL_IMAP_ALLOW_INSECURE_AUTH", "")
	t.Setenv("GOGOMAIL_IMAP_MAX_CONNECTIONS", "")
	t.Setenv("GOGOMAIL_IMAP_READ_TIMEOUT", "")
	t.Setenv("GOGOMAIL_IMAP_WRITE_TIMEOUT", "")
	t.Setenv("GOGOMAIL_IMAP_IDLE_TIMEOUT", "")
	t.Setenv("GOGOMAIL_IMAP_NOTIFY_CONSUMER_GROUP", "")
	t.Setenv("GOGOMAIL_IMAP_NOTIFY_CONSUMER_NAME", "")
	t.Setenv("GOGOMAIL_IMAP_NOTIFY_CONSUMER_COUNT", "")
	t.Setenv("GOGOMAIL_IMAP_NOTIFY_CONSUMER_BLOCK", "")
	t.Setenv("GOGOMAIL_IMAP_NOTIFY_CONSUMER_CLAIM_IDLE", "")
	t.Setenv("GOGOMAIL_IMAP_NOTIFY_CONSUMER_MAX_DELIVERIES", "")
	t.Setenv("GOGOMAIL_IMAP_NOTIFY_CONSUMER_DEAD_LETTER_STREAM", "")
	t.Setenv("GOGOMAIL_CALDAV_ADDR", "")
	t.Setenv("GOGOMAIL_CALDAV_ALLOW_INSECURE_AUTH", "")
	t.Setenv("GOGOMAIL_CALDAV_TRUST_FORWARDED_PROTO", "")
	t.Setenv("GOGOMAIL_CALDAV_TRUSTED_PROXIES", "")
	t.Setenv("GOGOMAIL_SUBMISSION_ADDR", "")
	t.Setenv("GOGOMAIL_SUBMISSION_SMTPS_ADDR", "")
	t.Setenv("GOGOMAIL_SUBMISSION_MAX_CONNECTIONS", "")
	t.Setenv("GOGOMAIL_SUBMISSION_MAX_RECIPIENTS", "")
	t.Setenv("GOGOMAIL_SUBMISSION_MAX_MESSAGE_BYTES", "")
	t.Setenv("GOGOMAIL_SUBMISSION_ADD_RECEIVED_HEADER", "")
	t.Setenv("GOGOMAIL_SUBMISSION_SUPPORT_SMTPUTF8", "")
	t.Setenv("GOGOMAIL_SUBMISSION_SUPPORT_REQUIRETLS", "")
	t.Setenv("GOGOMAIL_SUBMISSION_SUPPORT_DSN", "")
	t.Setenv("GOGOMAIL_SUBMISSION_SUPPORT_BINARYMIME", "")
	t.Setenv("GOGOMAIL_SMTP_TLS_CERT_FILE", "")
	t.Setenv("GOGOMAIL_SMTP_TLS_KEY_FILE", "")
	t.Setenv("GOGOMAIL_SUBMISSION_ALLOW_INSECURE_AUTH", "")
	t.Setenv("GOGOMAIL_DATABASE_URL", "")
	t.Setenv("GOGOMAIL_REDIS_ADDR", "")
	t.Setenv("GOGOMAIL_STORAGE_BACKEND", "")
	t.Setenv("GOGOMAIL_STORAGE_BACKEND_COMPAT_LABELS", "")
	t.Setenv("GOGOMAIL_STORAGE_S3_ENDPOINT", "")
	t.Setenv("GOGOMAIL_STORAGE_S3_REGION", "")
	t.Setenv("GOGOMAIL_STORAGE_S3_BUCKET", "")
	t.Setenv("GOGOMAIL_STORAGE_S3_PREFIX", "")
	t.Setenv("GOGOMAIL_STORAGE_S3_ACCESS_KEY_ID", "")
	t.Setenv("GOGOMAIL_STORAGE_S3_SECRET_ACCESS_KEY", "")
	t.Setenv("GOGOMAIL_STORAGE_S3_SESSION_TOKEN", "")
	t.Setenv("GOGOMAIL_STORAGE_S3_FORCE_PATH_STYLE", "")
	t.Setenv("GOGOMAIL_STORAGE_S3_CA_CERT_FILE", "")
	t.Setenv("GOGOMAIL_STORAGE_S3_INSECURE_SKIP_VERIFY", "")
	t.Setenv("GOGOMAIL_MIGRATION_DIR", "")
	t.Setenv("GOGOMAIL_SMTP_DOMAIN", "")
	t.Setenv("GOGOMAIL_SMTP_READ_TIMEOUT", "")
	t.Setenv("GOGOMAIL_SMTP_WRITE_TIMEOUT", "")
	t.Setenv("GOGOMAIL_SMTP_MAX_CONNECTIONS", "")
	t.Setenv("GOGOMAIL_SMTP_MAX_RECIPIENTS", "")
	t.Setenv("GOGOMAIL_SMTP_MAX_MESSAGE_BYTES", "")
	t.Setenv("GOGOMAIL_SMTP_REQUIRE_AUTH", "")
	t.Setenv("GOGOMAIL_SMTP_ADD_RECEIVED_HEADER", "")
	t.Setenv("GOGOMAIL_SMTP_SUPPORT_SMTPUTF8", "")
	t.Setenv("GOGOMAIL_SMTP_SUPPORT_REQUIRETLS", "")
	t.Setenv("GOGOMAIL_SMTP_SUPPORT_DSN", "")
	t.Setenv("GOGOMAIL_SMTP_SUPPORT_BINARYMIME", "")
	t.Setenv("GOGOMAIL_MAILSTORE_ROOT", "")
	t.Setenv("GOGOMAIL_STORAGE_ROOT", "")
	t.Setenv("GOGOMAIL_LOCAL_RECIPIENTS", "")
	t.Setenv("GOGOMAIL_DEDUP_BACKEND", "")
	t.Setenv("GOGOMAIL_RATELIMIT_BACKEND", "")
	t.Setenv("GOGOMAIL_ATTACHMENT_SCAN_BACKEND", "")
	t.Setenv("GOGOMAIL_ATTACHMENT_SCAN_WEBHOOK_URL", "")
	t.Setenv("GOGOMAIL_ATTACHMENT_SCAN_WEBHOOK_TOKEN", "")
	t.Setenv("GOGOMAIL_ATTACHMENT_SCAN_TIMEOUT", "")
	t.Setenv("GOGOMAIL_ATTACHMENT_CLEANUP_INTERVAL", "")
	t.Setenv("GOGOMAIL_ATTACHMENT_CLEANUP_STALE_AGE", "")
	t.Setenv("GOGOMAIL_ATTACHMENT_CLEANUP_BATCH_SIZE", "")
	t.Setenv("GOGOMAIL_ATTACHMENT_CLEANUP_RUN_ONCE", "")
	t.Setenv("GOGOMAIL_DRIVE_CLEANUP_INTERVAL", "")
	t.Setenv("GOGOMAIL_DRIVE_CLEANUP_BATCH_SIZE", "")
	t.Setenv("GOGOMAIL_DRIVE_CLEANUP_RUN_ONCE", "")
	t.Setenv("GOGOMAIL_DAV_SYNC_RETENTION_INTERVAL", "")
	t.Setenv("GOGOMAIL_DAV_SYNC_RETENTION_CUTOFF_AGE", "")
	t.Setenv("GOGOMAIL_DAV_SYNC_RETENTION_BATCH_SIZE", "")
	t.Setenv("GOGOMAIL_DAV_SYNC_RETENTION_RUN_ONCE", "")
	t.Setenv("GOGOMAIL_DAV_SYNC_RETENTION_DRY_RUN", "")
	t.Setenv("GOGOMAIL_DAV_SYNC_RETENTION_CONFIRM_READY", "")
	t.Setenv("GOGOMAIL_DRIVE_SHARE_RATELIMIT_BACKEND", "")
	t.Setenv("GOGOMAIL_DRIVE_SHARE_RATELIMIT_PER_MINUTE", "")
	t.Setenv("GOGOMAIL_PUSH_NOTIFICATION_BACKEND", "")
	t.Setenv("GOGOMAIL_PUSH_NOTIFICATION_WEBHOOK_URL", "")
	t.Setenv("GOGOMAIL_PUSH_NOTIFICATION_WEBHOOK_TOKEN", "")
	t.Setenv("GOGOMAIL_PUSH_NOTIFICATION_WEBHOOK_TIMEOUT", "")
	t.Setenv("GOGOMAIL_RCPT_RATE_LIMIT_PER_MINUTE", "")
	t.Setenv("GOGOMAIL_OUTBOX_RELAY_BATCH_SIZE", "")
	t.Setenv("GOGOMAIL_OUTBOX_RELAY_POLL_INTERVAL", "")
	t.Setenv("GOGOMAIL_OUTBOX_RELAY_MAX_ATTEMPTS", "")
	t.Setenv("GOGOMAIL_EVENT_STREAM", "")
	t.Setenv("GOGOMAIL_EVENT_CONSUMER_GROUP", "")
	t.Setenv("GOGOMAIL_EVENT_CONSUMER_NAME", "")
	t.Setenv("GOGOMAIL_EVENT_CONSUMER_COUNT", "")
	t.Setenv("GOGOMAIL_EVENT_CONSUMER_BLOCK", "")
	t.Setenv("GOGOMAIL_EVENT_CONSUMER_CLAIM_IDLE", "")
	t.Setenv("GOGOMAIL_EVENT_CONSUMER_MAX_DELIVERIES", "")
	t.Setenv("GOGOMAIL_EVENT_CONSUMER_DEAD_LETTER_STREAM", "")
	t.Setenv("GOGOMAIL_SEARCH_INDEX_CONSUMER_MAX_DELIVERIES", "")
	t.Setenv("GOGOMAIL_SEARCH_INDEX_CONSUMER_DEAD_LETTER_STREAM", "")
	t.Setenv("GOGOMAIL_API_METERING_CONSUMER_MAX_DELIVERIES", "")
	t.Setenv("GOGOMAIL_API_METERING_CONSUMER_DEAD_LETTER_STREAM", "")
	t.Setenv("GOGOMAIL_PUSH_NOTIFICATION_CONSUMER_MAX_DELIVERIES", "")
	t.Setenv("GOGOMAIL_PUSH_NOTIFICATION_CONSUMER_DEAD_LETTER_STREAM", "")
	t.Setenv("GOGOMAIL_DELIVERY_STREAM", "")
	t.Setenv("GOGOMAIL_DELIVERY_CONSUMER_GROUP", "")
	t.Setenv("GOGOMAIL_DELIVERY_CONSUMER_NAME", "")
	t.Setenv("GOGOMAIL_DELIVERY_CONSUMER_COUNT", "")
	t.Setenv("GOGOMAIL_DELIVERY_CONSUMER_BLOCK", "")
	t.Setenv("GOGOMAIL_DELIVERY_CONSUMER_CLAIM_IDLE", "")
	t.Setenv("GOGOMAIL_DELIVERY_CONSUMER_MAX_DELIVERIES", "")
	t.Setenv("GOGOMAIL_DELIVERY_CONSUMER_DEAD_LETTER_STREAM", "")
	t.Setenv("GOGOMAIL_DELIVERY_SMTP_HELLO", "")
	t.Setenv("GOGOMAIL_DELIVERY_TIMEOUT", "")
	t.Setenv("GOGOMAIL_DELIVERY_TLS_MODE", "")
	t.Setenv("GOGOMAIL_DELIVERY_RETRY_DELAYS", "")
	t.Setenv("GOGOMAIL_DELIVERY_RETRY_JITTER_RATIO", "")
	t.Setenv("GOGOMAIL_DELIVERY_RETRY_MAX_DELAY", "")
	t.Setenv("GOGOMAIL_DKIM_ENABLED", "")
	t.Setenv("GOGOMAIL_ADMIN_TOKEN", "")
	t.Setenv("GOGOMAIL_AUTH_JWT_SECRET", "")
	t.Setenv("GOGOMAIL_PUSH_NOTIFICATION_DEVICE_LIMIT", "")
	t.Setenv("GOGOMAIL_CARDDAV_TRUST_FORWARDED_PROTO", "")
	t.Setenv("GOGOMAIL_CARDDAV_TRUSTED_PROXIES", "")

	cfg := Load()

	if cfg.Environment != "development" {
		t.Fatalf("Environment = %q, want development", cfg.Environment)
	}
	if cfg.HTTPAddr != ":8080" {
		t.Fatalf("HTTPAddr = %q, want :8080", cfg.HTTPAddr)
	}
	if cfg.HTTPReadTimeout != 5*time.Minute {
		t.Fatalf("HTTPReadTimeout = %s, want 5m", cfg.HTTPReadTimeout)
	}
	if cfg.HTTPWriteTimeout != 10*time.Minute {
		t.Fatalf("HTTPWriteTimeout = %s, want 10m", cfg.HTTPWriteTimeout)
	}
	if cfg.HTTPIdleTimeout != 2*time.Minute {
		t.Fatalf("HTTPIdleTimeout = %s, want 2m", cfg.HTTPIdleTimeout)
	}
	if cfg.HTTPReadHeaderTimeout != 5*time.Second {
		t.Fatalf("HTTPReadHeaderTimeout = %s, want 5s", cfg.HTTPReadHeaderTimeout)
	}
	if cfg.HTTPMaxHeaderBytes != 64*1024 {
		t.Fatalf("HTTPMaxHeaderBytes = %d, want 64KiB", cfg.HTTPMaxHeaderBytes)
	}
	if cfg.SMTPAddr != ":2525" {
		t.Fatalf("SMTPAddr = %q, want :2525", cfg.SMTPAddr)
	}
	if cfg.InboundSMTPAddr != ":2526" {
		t.Fatalf("InboundSMTPAddr = %q, want :2526", cfg.InboundSMTPAddr)
	}
	if cfg.SMTPMaxConnections != 0 {
		t.Fatalf("SMTPMaxConnections = %d, want 0 for unlimited default", cfg.SMTPMaxConnections)
	}
	if len(cfg.InboundTrustedRelays) != 2 {
		t.Fatalf("InboundTrustedRelays = %+v, want loopback defaults", cfg.InboundTrustedRelays)
	}
	if cfg.IMAPAddr != ":1143" {
		t.Fatalf("IMAPAddr = %q, want :1143", cfg.IMAPAddr)
	}
	if cfg.IMAPTLSCertFile != "" || cfg.IMAPTLSKeyFile != "" {
		t.Fatalf("IMAP TLS files = %q/%q, want empty", cfg.IMAPTLSCertFile, cfg.IMAPTLSKeyFile)
	}
	if !cfg.IMAPAllowInsecureAuth {
		t.Fatal("IMAPAllowInsecureAuth = false, want true in development defaults")
	}
	if cfg.IMAPMaxConnections != 0 {
		t.Fatalf("IMAPMaxConnections = %d, want 0 for unlimited default", cfg.IMAPMaxConnections)
	}
	if cfg.IMAPReadTimeout != 5*time.Minute {
		t.Fatalf("IMAPReadTimeout = %s, want 5m", cfg.IMAPReadTimeout)
	}
	if cfg.IMAPWriteTimeout != 30*time.Second {
		t.Fatalf("IMAPWriteTimeout = %s, want 30s", cfg.IMAPWriteTimeout)
	}
	if cfg.IMAPIdleTimeout != 30*time.Minute {
		t.Fatalf("IMAPIdleTimeout = %s, want 30m", cfg.IMAPIdleTimeout)
	}
	if cfg.POP3MaxConnections != 0 {
		t.Fatalf("POP3MaxConnections = %d, want 0 for unlimited default", cfg.POP3MaxConnections)
	}
	if cfg.POP3SAddr != "" {
		t.Fatalf("POP3SAddr = %q, want empty default", cfg.POP3SAddr)
	}
	if cfg.IMAPNotifyConsumerGroup != "gogomail.imap-gateway" {
		t.Fatalf("IMAPNotifyConsumerGroup = %q, want gogomail.imap-gateway", cfg.IMAPNotifyConsumerGroup)
	}
	if cfg.IMAPNotifyConsumerName != "imap-gateway-1" {
		t.Fatalf("IMAPNotifyConsumerName = %q, want imap-gateway-1", cfg.IMAPNotifyConsumerName)
	}
	if cfg.IMAPNotifyConsumerCount != 50 {
		t.Fatalf("IMAPNotifyConsumerCount = %d, want 50", cfg.IMAPNotifyConsumerCount)
	}
	if cfg.IMAPNotifyConsumerBlock != time.Second {
		t.Fatalf("IMAPNotifyConsumerBlock = %s, want 1s", cfg.IMAPNotifyConsumerBlock)
	}
	if cfg.IMAPNotifyConsumerClaimIdle != 5*time.Minute {
		t.Fatalf("IMAPNotifyConsumerClaimIdle = %s, want 5m", cfg.IMAPNotifyConsumerClaimIdle)
	}
	if cfg.IMAPNotifyConsumerMaxDeliveries != 10 {
		t.Fatalf("IMAPNotifyConsumerMaxDeliveries = %d, want 10", cfg.IMAPNotifyConsumerMaxDeliveries)
	}
	if cfg.IMAPNotifyConsumerDeadLetterStream != "mail.event.dead" {
		t.Fatalf("IMAPNotifyConsumerDeadLetterStream = %q, want mail.event.dead", cfg.IMAPNotifyConsumerDeadLetterStream)
	}
	if cfg.CalDAVAddr != ":8081" {
		t.Fatalf("CalDAVAddr = %q, want :8081", cfg.CalDAVAddr)
	}
	if !cfg.CalDAVAllowInsecureAuth {
		t.Fatal("CalDAVAllowInsecureAuth = false, want true in development defaults")
	}
	if cfg.CalDAVTrustForwardedProto {
		t.Fatal("CalDAVTrustForwardedProto = true, want false in defaults")
	}
	if len(cfg.CalDAVTrustedProxies) != 0 {
		t.Fatalf("CalDAVTrustedProxies = %#v, want empty", cfg.CalDAVTrustedProxies)
	}
	if cfg.CardDAVAddr != ":8082" {
		t.Fatalf("CardDAVAddr = %q, want :8082", cfg.CardDAVAddr)
	}
	if !cfg.CardDAVAllowInsecureAuth {
		t.Fatal("CardDAVAllowInsecureAuth = false, want true in development defaults")
	}
	if cfg.CardDAVTrustForwardedProto {
		t.Fatal("CardDAVTrustForwardedProto = true, want false in defaults")
	}
	if len(cfg.CardDAVTrustedProxies) != 0 {
		t.Fatalf("CardDAVTrustedProxies = %#v, want empty", cfg.CardDAVTrustedProxies)
	}
	if cfg.SubmissionAddr != ":2587" {
		t.Fatalf("SubmissionAddr = %q, want :2587", cfg.SubmissionAddr)
	}
	if cfg.SubmissionSMTPSAddr != "" {
		t.Fatalf("SubmissionSMTPSAddr = %q, want empty", cfg.SubmissionSMTPSAddr)
	}
	if cfg.SubmissionMaxConnections != 0 {
		t.Fatalf("SubmissionMaxConnections = %d, want 0 for unlimited default", cfg.SubmissionMaxConnections)
	}
	if cfg.SubmissionMaxRecipients != 100 {
		t.Fatalf("SubmissionMaxRecipients = %d, want 100", cfg.SubmissionMaxRecipients)
	}
	if cfg.SubmissionMaxMessageBytes != 25*1024*1024 {
		t.Fatalf("SubmissionMaxMessageBytes = %d, want 25MiB", cfg.SubmissionMaxMessageBytes)
	}
	if !cfg.SubmissionAddReceivedHeader {
		t.Fatal("SubmissionAddReceivedHeader = false, want true")
	}
	if cfg.SubmissionSupportSMTPUTF8 {
		t.Fatal("SubmissionSupportSMTPUTF8 = true, want false")
	}
	if cfg.SubmissionSupportRequireTLS {
		t.Fatal("SubmissionSupportRequireTLS = true, want false")
	}
	if cfg.SubmissionSupportDSN {
		t.Fatal("SubmissionSupportDSN = true, want false")
	}
	if cfg.SubmissionSupportBinaryMIME {
		t.Fatal("SubmissionSupportBinaryMIME = true, want false")
	}
	if cfg.SMTPTLSCertFile != "" {
		t.Fatalf("SMTPTLSCertFile = %q, want empty", cfg.SMTPTLSCertFile)
	}
	if cfg.SMTPTLSKeyFile != "" {
		t.Fatalf("SMTPTLSKeyFile = %q, want empty", cfg.SMTPTLSKeyFile)
	}
	if !cfg.SubmissionAllowInsecureAuth {
		t.Fatal("SubmissionAllowInsecureAuth = false, want true in development defaults")
	}
	if cfg.StorageBackend != "local" {
		t.Fatalf("StorageBackend = %q, want local", cfg.StorageBackend)
	}
	if len(cfg.StorageBackendCompatLabels) != 0 {
		t.Fatalf("StorageBackendCompatLabels = %#v, want empty", cfg.StorageBackendCompatLabels)
	}
	if cfg.StorageS3Region != "us-east-1" || cfg.StorageS3Bucket != "" || cfg.StorageS3ForcePathStyle || cfg.StorageS3CACertFile != "" || cfg.StorageS3InsecureSkipVerify {
		t.Fatalf("S3 storage defaults = region:%q bucket:%q force_path:%v ca:%q insecure:%v", cfg.StorageS3Region, cfg.StorageS3Bucket, cfg.StorageS3ForcePathStyle, cfg.StorageS3CACertFile, cfg.StorageS3InsecureSkipVerify)
	}
	if cfg.MigrationDir != "migrations" {
		t.Fatalf("MigrationDir = %q, want migrations", cfg.MigrationDir)
	}
	if cfg.SMTPDomain != "localhost" {
		t.Fatalf("SMTPDomain = %q, want localhost", cfg.SMTPDomain)
	}
	if cfg.SMTPReadTimeout != 30*time.Second {
		t.Fatalf("SMTPReadTimeout = %s, want 30s", cfg.SMTPReadTimeout)
	}
	if cfg.SMTPWriteTimeout != 30*time.Second {
		t.Fatalf("SMTPWriteTimeout = %s, want 30s", cfg.SMTPWriteTimeout)
	}
	if cfg.SMTPMaxRecipients != 100 {
		t.Fatalf("SMTPMaxRecipients = %d, want 100", cfg.SMTPMaxRecipients)
	}
	if cfg.SMTPMaxMessageBytes != 25*1024*1024 {
		t.Fatalf("SMTPMaxMessageBytes = %d, want 25MiB", cfg.SMTPMaxMessageBytes)
	}
	if cfg.SMTPRequireAuth {
		t.Fatal("SMTPRequireAuth = true, want false")
	}
	if !cfg.SMTPAddReceivedHeader {
		t.Fatal("SMTPAddReceivedHeader = false, want true")
	}
	if cfg.SMTPSupportSMTPUTF8 {
		t.Fatal("SMTPSupportSMTPUTF8 = true, want false")
	}
	if cfg.SMTPSupportRequireTLS {
		t.Fatal("SMTPSupportRequireTLS = true, want false")
	}
	if cfg.SMTPSupportDSN {
		t.Fatal("SMTPSupportDSN = true, want false")
	}
	if cfg.SMTPSupportBinaryMIME {
		t.Fatal("SMTPSupportBinaryMIME = true, want false")
	}
	if cfg.MailstoreRoot != "var/mailstore" {
		t.Fatalf("MailstoreRoot = %q, want var/mailstore", cfg.MailstoreRoot)
	}
	if len(cfg.LocalRecipients) != 0 {
		t.Fatalf("LocalRecipients = %v, want empty", cfg.LocalRecipients)
	}
	if cfg.DedupBackend != "none" {
		t.Fatalf("DedupBackend = %q, want none", cfg.DedupBackend)
	}
	if cfg.RateLimitBackend != "none" {
		t.Fatalf("RateLimitBackend = %q, want none", cfg.RateLimitBackend)
	}
	if cfg.AttachmentScanBackend != "none" {
		t.Fatalf("AttachmentScanBackend = %q, want none", cfg.AttachmentScanBackend)
	}
	if cfg.AttachmentScanWebhookURL != "" {
		t.Fatalf("AttachmentScanWebhookURL = %q, want empty", cfg.AttachmentScanWebhookURL)
	}
	if cfg.AttachmentScanWebhookToken != "" {
		t.Fatalf("AttachmentScanWebhookToken = %q, want empty", cfg.AttachmentScanWebhookToken)
	}
	if cfg.AttachmentScanTimeout != 2*time.Second {
		t.Fatalf("AttachmentScanTimeout = %s, want 2s", cfg.AttachmentScanTimeout)
	}
	if cfg.AttachmentCleanupInterval != time.Hour {
		t.Fatalf("AttachmentCleanupInterval = %s, want 1h", cfg.AttachmentCleanupInterval)
	}
	if cfg.AttachmentCleanupStaleAge != 24*time.Hour {
		t.Fatalf("AttachmentCleanupStaleAge = %s, want 24h", cfg.AttachmentCleanupStaleAge)
	}
	if cfg.AttachmentCleanupBatchSize != 100 {
		t.Fatalf("AttachmentCleanupBatchSize = %d, want 100", cfg.AttachmentCleanupBatchSize)
	}
	if cfg.AttachmentCleanupRunOnce {
		t.Fatal("AttachmentCleanupRunOnce = true, want false")
	}
	if cfg.DriveCleanupInterval != 15*time.Minute {
		t.Fatalf("DriveCleanupInterval = %s, want 15m", cfg.DriveCleanupInterval)
	}
	if cfg.DriveCleanupBatchSize != 100 {
		t.Fatalf("DriveCleanupBatchSize = %d, want 100", cfg.DriveCleanupBatchSize)
	}
	if cfg.DriveCleanupRunOnce {
		t.Fatal("DriveCleanupRunOnce = true, want false")
	}
	if cfg.DAVSyncRetentionInterval != 24*time.Hour {
		t.Fatalf("DAVSyncRetentionInterval = %s, want 24h", cfg.DAVSyncRetentionInterval)
	}
	if cfg.DAVSyncRetentionCutoffAge != 90*24*time.Hour {
		t.Fatalf("DAVSyncRetentionCutoffAge = %s, want 2160h", cfg.DAVSyncRetentionCutoffAge)
	}
	if cfg.DAVSyncRetentionBatchSize != 1000 {
		t.Fatalf("DAVSyncRetentionBatchSize = %d, want 1000", cfg.DAVSyncRetentionBatchSize)
	}
	if cfg.DAVSyncRetentionRunOnce {
		t.Fatal("DAVSyncRetentionRunOnce = true, want false")
	}
	if !cfg.DAVSyncRetentionDryRun {
		t.Fatal("DAVSyncRetentionDryRun = false, want true")
	}
	if cfg.DAVSyncRetentionConfirmReady {
		t.Fatal("DAVSyncRetentionConfirmReady = true, want false")
	}
	if cfg.DriveShareRateLimitBackend != "none" {
		t.Fatalf("DriveShareRateLimitBackend = %q, want none", cfg.DriveShareRateLimitBackend)
	}
	if cfg.DriveShareRateLimitPerMinute != 120 {
		t.Fatalf("DriveShareRateLimitPerMinute = %d, want 120", cfg.DriveShareRateLimitPerMinute)
	}
	if cfg.RcptRateLimitPerMinute != 60 {
		t.Fatalf("RcptRateLimitPerMinute = %d, want 60", cfg.RcptRateLimitPerMinute)
	}
	if cfg.OutboxRelayBatchSize != 100 {
		t.Fatalf("OutboxRelayBatchSize = %d, want 100", cfg.OutboxRelayBatchSize)
	}
	if cfg.OutboxRelayPollInterval != time.Second {
		t.Fatalf("OutboxRelayPollInterval = %s, want 1s", cfg.OutboxRelayPollInterval)
	}
	if cfg.OutboxRelayMaxAttempts != 10 {
		t.Fatalf("OutboxRelayMaxAttempts = %d, want 10", cfg.OutboxRelayMaxAttempts)
	}
	if cfg.EventStream != "mail.event" {
		t.Fatalf("EventStream = %q, want mail.event", cfg.EventStream)
	}
	if cfg.EventConsumerGroup != "gogomail.event-worker" {
		t.Fatalf("EventConsumerGroup = %q, want gogomail.event-worker", cfg.EventConsumerGroup)
	}
	if cfg.EventConsumerName != "event-worker-1" {
		t.Fatalf("EventConsumerName = %q, want event-worker-1", cfg.EventConsumerName)
	}
	if cfg.EventConsumerCount != 100 {
		t.Fatalf("EventConsumerCount = %d, want 100", cfg.EventConsumerCount)
	}
	if cfg.EventConsumerBlock != time.Second {
		t.Fatalf("EventConsumerBlock = %s, want 1s", cfg.EventConsumerBlock)
	}
	if cfg.EventConsumerClaimIdle != 5*time.Minute {
		t.Fatalf("EventConsumerClaimIdle = %s, want 5m", cfg.EventConsumerClaimIdle)
	}
	if cfg.EventConsumerMaxDeliveries != 10 {
		t.Fatalf("EventConsumerMaxDeliveries = %d, want 10", cfg.EventConsumerMaxDeliveries)
	}
	if cfg.EventConsumerDeadLetterStream != "mail.event.dead" {
		t.Fatalf("EventConsumerDeadLetterStream = %q, want mail.event.dead", cfg.EventConsumerDeadLetterStream)
	}
	if cfg.DeliveryStream != "mail.outbound.general" {
		t.Fatalf("DeliveryStream = %q, want mail.outbound.general", cfg.DeliveryStream)
	}
	if cfg.DeliveryConsumerGroup != "gogomail.delivery-worker" {
		t.Fatalf("DeliveryConsumerGroup = %q, want gogomail.delivery-worker", cfg.DeliveryConsumerGroup)
	}
	if cfg.DeliveryConsumerName != "delivery-worker-1" {
		t.Fatalf("DeliveryConsumerName = %q, want delivery-worker-1", cfg.DeliveryConsumerName)
	}
	if cfg.DeliveryConsumerCount != 50 {
		t.Fatalf("DeliveryConsumerCount = %d, want 50", cfg.DeliveryConsumerCount)
	}
	if cfg.DeliveryConsumerBlock != time.Second {
		t.Fatalf("DeliveryConsumerBlock = %s, want 1s", cfg.DeliveryConsumerBlock)
	}
	if cfg.DeliveryConsumerClaimIdle != 5*time.Minute {
		t.Fatalf("DeliveryConsumerClaimIdle = %s, want 5m", cfg.DeliveryConsumerClaimIdle)
	}
	if cfg.DeliveryConsumerMaxDeliveries != 10 {
		t.Fatalf("DeliveryConsumerMaxDeliveries = %d, want 10", cfg.DeliveryConsumerMaxDeliveries)
	}
	if cfg.DeliveryConsumerDeadLetterStream != "mail.outbound.general.dead" {
		t.Fatalf("DeliveryConsumerDeadLetterStream = %q, want mail.outbound.general.dead", cfg.DeliveryConsumerDeadLetterStream)
	}
	if cfg.DeliverySMTPHello != "localhost" {
		t.Fatalf("DeliverySMTPHello = %q, want localhost", cfg.DeliverySMTPHello)
	}
	if cfg.DeliveryTimeout != 30*time.Second {
		t.Fatalf("DeliveryTimeout = %s, want 30s", cfg.DeliveryTimeout)
	}
	if cfg.DeliveryTLSMode != "opportunistic" {
		t.Fatalf("DeliveryTLSMode = %q, want opportunistic", cfg.DeliveryTLSMode)
	}
	wantRetryDelays := []time.Duration{5 * time.Minute, 30 * time.Minute, 2 * time.Hour, 8 * time.Hour, 24 * time.Hour}
	if len(cfg.DeliveryRetryDelays) != len(wantRetryDelays) {
		t.Fatalf("DeliveryRetryDelays = %v, want %v", cfg.DeliveryRetryDelays, wantRetryDelays)
	}
	for i := range wantRetryDelays {
		if cfg.DeliveryRetryDelays[i] != wantRetryDelays[i] {
			t.Fatalf("DeliveryRetryDelays = %v, want %v", cfg.DeliveryRetryDelays, wantRetryDelays)
		}
	}
	if cfg.DeliveryRetryJitterRatio != 0.20 {
		t.Fatalf("DeliveryRetryJitterRatio = %f, want 0.20", cfg.DeliveryRetryJitterRatio)
	}
	if cfg.DeliveryRetryMaxDelay != 24*time.Hour {
		t.Fatalf("DeliveryRetryMaxDelay = %s, want 24h", cfg.DeliveryRetryMaxDelay)
	}
	if cfg.DKIMEnabled {
		t.Fatal("DKIMEnabled = true, want false")
	}
	if cfg.AdminToken != "" {
		t.Fatalf("AdminToken = %q, want empty", cfg.AdminToken)
	}
	if cfg.AuthJWTSecret != "" {
		t.Fatalf("AuthJWTSecret = %q, want empty", cfg.AuthJWTSecret)
	}
	if cfg.PushNotifyDeviceLimit != 200 {
		t.Fatalf("PushNotifyDeviceLimit = %d, want 200", cfg.PushNotifyDeviceLimit)
	}
	if cfg.PushNotifyBackend != "none" {
		t.Fatalf("PushNotifyBackend = %q, want none", cfg.PushNotifyBackend)
	}
	if cfg.PushNotifyWebhookURL != "" {
		t.Fatalf("PushNotifyWebhookURL = %q, want empty", cfg.PushNotifyWebhookURL)
	}
	if cfg.PushNotifyWebhookToken != "" {
		t.Fatalf("PushNotifyWebhookToken = %q, want empty", cfg.PushNotifyWebhookToken)
	}
	if cfg.PushNotifyWebhookTimeout != 2*time.Second {
		t.Fatalf("PushNotifyWebhookTimeout = %s, want 2s", cfg.PushNotifyWebhookTimeout)
	}
}

func TestLoadAcceptsStorageRootAliasForMailstoreRoot(t *testing.T) {
	t.Setenv("GOGOMAIL_MAILSTORE_ROOT", "")
	t.Setenv("GOGOMAIL_STORAGE_ROOT", "/mnt/gogomail-storage")

	cfg := Load()
	if cfg.MailstoreRoot != "/mnt/gogomail-storage" {
		t.Fatalf("MailstoreRoot = %q, want storage root alias", cfg.MailstoreRoot)
	}
}

func TestLoadPrefersMailstoreRootOverStorageRootAlias(t *testing.T) {
	t.Setenv("GOGOMAIL_MAILSTORE_ROOT", "/srv/gogomail-mailstore")
	t.Setenv("GOGOMAIL_STORAGE_ROOT", "/mnt/gogomail-storage")

	cfg := Load()
	if cfg.MailstoreRoot != "/srv/gogomail-mailstore" {
		t.Fatalf("MailstoreRoot = %q, want explicit mailstore root", cfg.MailstoreRoot)
	}
}

func TestLoadReadsEnvironmentOverrides(t *testing.T) {
	t.Setenv("GOGOMAIL_ENV", "test")
	t.Setenv("GOGOMAIL_HTTP_ADDR", ":18080")
	t.Setenv("GOGOMAIL_HTTP_READ_TIMEOUT", "45s")
	t.Setenv("GOGOMAIL_HTTP_WRITE_TIMEOUT", "90s")
	t.Setenv("GOGOMAIL_HTTP_IDLE_TIMEOUT", "75s")
	t.Setenv("GOGOMAIL_HTTP_READ_HEADER_TIMEOUT", "3s")
	t.Setenv("GOGOMAIL_HTTP_MAX_HEADER_BYTES", "32768")
	t.Setenv("GOGOMAIL_SMTP_ADDR", ":10025")
	t.Setenv("GOGOMAIL_SUBMISSION_ADDR", ":10587")
	t.Setenv("GOGOMAIL_POP3_MAX_CONNECTIONS", "96")
	t.Setenv("GOGOMAIL_POP3S_ADDR", ":1995")
	t.Setenv("GOGOMAIL_SUBMISSION_MAX_CONNECTIONS", "128")
	t.Setenv("GOGOMAIL_SUBMISSION_MAX_RECIPIENTS", "25")
	t.Setenv("GOGOMAIL_SUBMISSION_MAX_MESSAGE_BYTES", "1048576")
	t.Setenv("GOGOMAIL_SUBMISSION_ADD_RECEIVED_HEADER", "false")
	t.Setenv("GOGOMAIL_SUBMISSION_SUPPORT_SMTPUTF8", "true")
	t.Setenv("GOGOMAIL_SUBMISSION_SUPPORT_REQUIRETLS", "true")
	t.Setenv("GOGOMAIL_SUBMISSION_SUPPORT_DSN", "true")
	t.Setenv("GOGOMAIL_SUBMISSION_SUPPORT_BINARYMIME", "true")
	t.Setenv("GOGOMAIL_SMTP_TLS_CERT_FILE", "cert.pem")
	t.Setenv("GOGOMAIL_SMTP_TLS_KEY_FILE", "key.pem")
	t.Setenv("GOGOMAIL_SUBMISSION_ALLOW_INSECURE_AUTH", "false")
	t.Setenv("GOGOMAIL_DATABASE_URL", "postgres://example")
	t.Setenv("GOGOMAIL_REDIS_ADDR", "redis:6379")
	t.Setenv("GOGOMAIL_STORAGE_BACKEND", "local")
	t.Setenv("GOGOMAIL_STORAGE_S3_CA_CERT_FILE", "/etc/gogomail/s3-ca.pem")
	t.Setenv("GOGOMAIL_STORAGE_S3_INSECURE_SKIP_VERIFY", "true")
	t.Setenv("GOGOMAIL_MIGRATION_DIR", "db/migrations")
	t.Setenv("GOGOMAIL_SMTP_DOMAIN", "mail.example.com")
	t.Setenv("GOGOMAIL_SMTP_MAX_CONNECTIONS", "256")
	t.Setenv("GOGOMAIL_SMTP_MAX_RECIPIENTS", "50")
	t.Setenv("GOGOMAIL_SMTP_MAX_MESSAGE_BYTES", "2097152")
	t.Setenv("GOGOMAIL_SMTP_REQUIRE_AUTH", "true")
	t.Setenv("GOGOMAIL_SMTP_ADD_RECEIVED_HEADER", "false")
	t.Setenv("GOGOMAIL_SMTP_SUPPORT_SMTPUTF8", "true")
	t.Setenv("GOGOMAIL_SMTP_SUPPORT_REQUIRETLS", "true")
	t.Setenv("GOGOMAIL_SMTP_SUPPORT_DSN", "true")
	t.Setenv("GOGOMAIL_SMTP_SUPPORT_BINARYMIME", "true")
	t.Setenv("GOGOMAIL_MAILSTORE_ROOT", "/tmp/gogomail-mailstore")
	t.Setenv("GOGOMAIL_LOCAL_RECIPIENTS", "Admin@Example.COM, user@example.com ")
	t.Setenv("GOGOMAIL_DEDUP_BACKEND", "redis")
	t.Setenv("GOGOMAIL_RATELIMIT_BACKEND", "redis")
	t.Setenv("GOGOMAIL_ATTACHMENT_SCAN_BACKEND", "webhook")
	t.Setenv("GOGOMAIL_ATTACHMENT_SCAN_WEBHOOK_URL", "http://scanner.internal/scan")
	t.Setenv("GOGOMAIL_ATTACHMENT_SCAN_WEBHOOK_TOKEN", "scanner-token")
	t.Setenv("GOGOMAIL_ATTACHMENT_SCAN_TIMEOUT", "3s")
	t.Setenv("GOGOMAIL_ATTACHMENT_CLEANUP_INTERVAL", "15m")
	t.Setenv("GOGOMAIL_ATTACHMENT_CLEANUP_STALE_AGE", "48h")
	t.Setenv("GOGOMAIL_ATTACHMENT_CLEANUP_BATCH_SIZE", "250")
	t.Setenv("GOGOMAIL_ATTACHMENT_CLEANUP_RUN_ONCE", "true")
	t.Setenv("GOGOMAIL_DRIVE_CLEANUP_INTERVAL", "30m")
	t.Setenv("GOGOMAIL_DRIVE_CLEANUP_BATCH_SIZE", "75")
	t.Setenv("GOGOMAIL_DRIVE_CLEANUP_RUN_ONCE", "true")
	t.Setenv("GOGOMAIL_DAV_SYNC_RETENTION_INTERVAL", "6h")
	t.Setenv("GOGOMAIL_DAV_SYNC_RETENTION_CUTOFF_AGE", "168h")
	t.Setenv("GOGOMAIL_DAV_SYNC_RETENTION_BATCH_SIZE", "600")
	t.Setenv("GOGOMAIL_DAV_SYNC_RETENTION_RUN_ONCE", "true")
	t.Setenv("GOGOMAIL_DAV_SYNC_RETENTION_DRY_RUN", "false")
	t.Setenv("GOGOMAIL_DAV_SYNC_RETENTION_CONFIRM_READY", "true")
	t.Setenv("GOGOMAIL_DRIVE_SHARE_RATELIMIT_BACKEND", "redis")
	t.Setenv("GOGOMAIL_DRIVE_SHARE_RATELIMIT_PER_MINUTE", "42")
	t.Setenv("GOGOMAIL_API_USAGE_RETENTION_INTERVAL", "12h")
	t.Setenv("GOGOMAIL_API_USAGE_RETENTION_CUTOFF_AGE", "720h")
	t.Setenv("GOGOMAIL_API_USAGE_RETENTION_BATCH_SIZE", "500")
	t.Setenv("GOGOMAIL_API_USAGE_RETENTION_RUN_ONCE", "true")
	t.Setenv("GOGOMAIL_API_USAGE_RETENTION_DRY_RUN", "false")
	t.Setenv("GOGOMAIL_API_USAGE_RETENTION_CONFIRM_READY", "true")
	t.Setenv("GOGOMAIL_API_USAGE_RETENTION_TENANT_ID", "tenant-1")
	t.Setenv("GOGOMAIL_API_USAGE_RETENTION_PRINCIPAL_ID", "principal-1")
	t.Setenv("GOGOMAIL_PUSH_NOTIFICATION_BACKEND", "webhook")
	t.Setenv("GOGOMAIL_PUSH_NOTIFICATION_WEBHOOK_URL", "https://push.internal/send")
	t.Setenv("GOGOMAIL_PUSH_NOTIFICATION_WEBHOOK_TOKEN", "push-token")
	t.Setenv("GOGOMAIL_PUSH_NOTIFICATION_WEBHOOK_TIMEOUT", "4s")
	t.Setenv("GOGOMAIL_RCPT_RATE_LIMIT_PER_MINUTE", "5")
	t.Setenv("GOGOMAIL_OUTBOX_RELAY_BATCH_SIZE", "25")
	t.Setenv("GOGOMAIL_OUTBOX_RELAY_POLL_INTERVAL", "250ms")
	t.Setenv("GOGOMAIL_OUTBOX_RELAY_MAX_ATTEMPTS", "3")
	t.Setenv("GOGOMAIL_EVENT_STREAM", "custom.event")
	t.Setenv("GOGOMAIL_EVENT_CONSUMER_GROUP", "custom-group")
	t.Setenv("GOGOMAIL_EVENT_CONSUMER_NAME", "worker-a")
	t.Setenv("GOGOMAIL_EVENT_CONSUMER_COUNT", "10")
	t.Setenv("GOGOMAIL_EVENT_CONSUMER_BLOCK", "500ms")
	t.Setenv("GOGOMAIL_EVENT_CONSUMER_CLAIM_IDLE", "90s")
	t.Setenv("GOGOMAIL_EVENT_CONSUMER_MAX_DELIVERIES", "7")
	t.Setenv("GOGOMAIL_EVENT_CONSUMER_DEAD_LETTER_STREAM", "custom.event.dlq")
	t.Setenv("GOGOMAIL_SEARCH_INDEX_CONSUMER_MAX_DELIVERIES", "8")
	t.Setenv("GOGOMAIL_SEARCH_INDEX_CONSUMER_DEAD_LETTER_STREAM", "search.event.dlq")
	t.Setenv("GOGOMAIL_API_METERING_CONSUMER_MAX_DELIVERIES", "9")
	t.Setenv("GOGOMAIL_API_METERING_CONSUMER_DEAD_LETTER_STREAM", "api.event.dlq")
	t.Setenv("GOGOMAIL_PUSH_NOTIFICATION_CONSUMER_MAX_DELIVERIES", "11")
	t.Setenv("GOGOMAIL_PUSH_NOTIFICATION_CONSUMER_DEAD_LETTER_STREAM", "push.event.dlq")
	t.Setenv("GOGOMAIL_DELIVERY_STREAM", "custom.outbound")
	t.Setenv("GOGOMAIL_DELIVERY_CONSUMER_GROUP", "delivery-group")
	t.Setenv("GOGOMAIL_DELIVERY_CONSUMER_NAME", "delivery-a")
	t.Setenv("GOGOMAIL_DELIVERY_CONSUMER_COUNT", "5")
	t.Setenv("GOGOMAIL_DELIVERY_CONSUMER_BLOCK", "750ms")
	t.Setenv("GOGOMAIL_DELIVERY_CONSUMER_CLAIM_IDLE", "2m")
	t.Setenv("GOGOMAIL_DELIVERY_CONSUMER_MAX_DELIVERIES", "12")
	t.Setenv("GOGOMAIL_DELIVERY_CONSUMER_DEAD_LETTER_STREAM", "delivery.event.dlq")
	t.Setenv("GOGOMAIL_DELIVERY_SMTP_HELLO", "mx.example.com")
	t.Setenv("GOGOMAIL_DELIVERY_TIMEOUT", "45s")
	t.Setenv("GOGOMAIL_DELIVERY_TLS_MODE", "require")
	t.Setenv("GOGOMAIL_DELIVERY_RETRY_DELAYS", "1m, 5m, 1h")
	t.Setenv("GOGOMAIL_DELIVERY_RETRY_JITTER_RATIO", "0.35")
	t.Setenv("GOGOMAIL_DELIVERY_RETRY_MAX_DELAY", "6h")
	t.Setenv("GOGOMAIL_DKIM_ENABLED", "true")
	t.Setenv("GOGOMAIL_ADMIN_TOKEN", "secret")
	t.Setenv("GOGOMAIL_AUTH_JWT_SECRET", "jwt-secret")
	t.Setenv("GOGOMAIL_PUSH_NOTIFICATION_DEVICE_LIMIT", "25")
	t.Setenv("GOGOMAIL_CALDAV_ADDR", ":18081")
	t.Setenv("GOGOMAIL_CALDAV_ALLOW_INSECURE_AUTH", "false")
	t.Setenv("GOGOMAIL_CALDAV_TRUST_FORWARDED_PROTO", "false")
	t.Setenv("GOGOMAIL_CALDAV_TRUSTED_PROXIES", "127.0.0.0/8, ::1/128")
	t.Setenv("GOGOMAIL_CARDDAV_ADDR", ":18082")
	t.Setenv("GOGOMAIL_CARDDAV_ALLOW_INSECURE_AUTH", "false")
	t.Setenv("GOGOMAIL_CARDDAV_TRUST_FORWARDED_PROTO", "false")
	t.Setenv("GOGOMAIL_CARDDAV_TRUSTED_PROXIES", "198.51.100.0/24,2001:db8::/32")

	cfg := Load()

	if cfg.Environment != "test" {
		t.Fatalf("Environment = %q, want test", cfg.Environment)
	}
	if cfg.HTTPAddr != ":18080" {
		t.Fatalf("HTTPAddr = %q, want :18080", cfg.HTTPAddr)
	}
	if cfg.HTTPReadTimeout != 45*time.Second {
		t.Fatalf("HTTPReadTimeout = %s, want 45s", cfg.HTTPReadTimeout)
	}
	if cfg.HTTPWriteTimeout != 90*time.Second {
		t.Fatalf("HTTPWriteTimeout = %s, want 90s", cfg.HTTPWriteTimeout)
	}
	if cfg.HTTPIdleTimeout != 75*time.Second {
		t.Fatalf("HTTPIdleTimeout = %s, want 75s", cfg.HTTPIdleTimeout)
	}
	if cfg.HTTPReadHeaderTimeout != 3*time.Second {
		t.Fatalf("HTTPReadHeaderTimeout = %s, want 3s", cfg.HTTPReadHeaderTimeout)
	}
	if cfg.HTTPMaxHeaderBytes != 32768 {
		t.Fatalf("HTTPMaxHeaderBytes = %d, want 32768", cfg.HTTPMaxHeaderBytes)
	}
	if cfg.CalDAVAddr != ":18081" {
		t.Fatalf("CalDAVAddr = %q, want :18081", cfg.CalDAVAddr)
	}
	if cfg.CalDAVAllowInsecureAuth {
		t.Fatal("CalDAVAllowInsecureAuth = true, want false")
	}
	if cfg.CalDAVTrustForwardedProto {
		t.Fatal("CalDAVTrustForwardedProto = true, want false")
	}
	if len(cfg.CalDAVTrustedProxies) != 2 || cfg.CalDAVTrustedProxies[0] != "127.0.0.0/8" || cfg.CalDAVTrustedProxies[1] != "::1/128" {
		t.Fatalf("CalDAVTrustedProxies = %#v, want [127.0.0.0/8 ::1/128]", cfg.CalDAVTrustedProxies)
	}
	if cfg.CardDAVAddr != ":18082" {
		t.Fatalf("CardDAVAddr = %q, want :18082", cfg.CardDAVAddr)
	}
	if cfg.CardDAVAllowInsecureAuth {
		t.Fatal("CardDAVAllowInsecureAuth = true, want false")
	}
	if cfg.CardDAVTrustForwardedProto {
		t.Fatal("CardDAVTrustForwardedProto = true, want false")
	}
	if len(cfg.CardDAVTrustedProxies) != 2 || cfg.CardDAVTrustedProxies[0] != "198.51.100.0/24" || cfg.CardDAVTrustedProxies[1] != "2001:db8::/32" {
		t.Fatalf("CardDAVTrustedProxies = %#v, want [198.51.100.0/24 2001:db8::/32]", cfg.CardDAVTrustedProxies)
	}
	if cfg.SMTPAddr != ":10025" {
		t.Fatalf("SMTPAddr = %q, want :10025", cfg.SMTPAddr)
	}
	if cfg.SMTPMaxConnections != 256 {
		t.Fatalf("SMTPMaxConnections = %d, want 256", cfg.SMTPMaxConnections)
	}
	if cfg.SubmissionAddr != ":10587" {
		t.Fatalf("SubmissionAddr = %q, want :10587", cfg.SubmissionAddr)
	}
	if cfg.POP3MaxConnections != 96 {
		t.Fatalf("POP3MaxConnections = %d, want 96", cfg.POP3MaxConnections)
	}
	if cfg.POP3SAddr != ":1995" {
		t.Fatalf("POP3SAddr = %q, want :1995", cfg.POP3SAddr)
	}
	if cfg.SubmissionMaxConnections != 128 {
		t.Fatalf("SubmissionMaxConnections = %d, want 128", cfg.SubmissionMaxConnections)
	}
	if cfg.SubmissionMaxRecipients != 25 {
		t.Fatalf("SubmissionMaxRecipients = %d, want 25", cfg.SubmissionMaxRecipients)
	}
	if cfg.SubmissionMaxMessageBytes != 1048576 {
		t.Fatalf("SubmissionMaxMessageBytes = %d, want 1048576", cfg.SubmissionMaxMessageBytes)
	}
	if cfg.SubmissionAddReceivedHeader {
		t.Fatal("SubmissionAddReceivedHeader = true, want false")
	}
	if !cfg.SubmissionSupportSMTPUTF8 {
		t.Fatal("SubmissionSupportSMTPUTF8 = false, want true")
	}
	if !cfg.SubmissionSupportRequireTLS {
		t.Fatal("SubmissionSupportRequireTLS = false, want true")
	}
	if !cfg.SubmissionSupportDSN {
		t.Fatal("SubmissionSupportDSN = false, want true")
	}
	if !cfg.SubmissionSupportBinaryMIME {
		t.Fatal("SubmissionSupportBinaryMIME = false, want true")
	}
	if cfg.SMTPTLSCertFile != "cert.pem" {
		t.Fatalf("SMTPTLSCertFile = %q, want cert.pem", cfg.SMTPTLSCertFile)
	}
	if cfg.SMTPTLSKeyFile != "key.pem" {
		t.Fatalf("SMTPTLSKeyFile = %q, want key.pem", cfg.SMTPTLSKeyFile)
	}
	if cfg.SubmissionAllowInsecureAuth {
		t.Fatal("SubmissionAllowInsecureAuth = true, want false")
	}
	if cfg.DatabaseURL != "postgres://example" {
		t.Fatalf("DatabaseURL = %q, want postgres://example", cfg.DatabaseURL)
	}
	if cfg.RedisAddr != "redis:6379" {
		t.Fatalf("RedisAddr = %q, want redis:6379", cfg.RedisAddr)
	}
	if cfg.StorageBackend != "local" {
		t.Fatalf("StorageBackend = %q, want local", cfg.StorageBackend)
	}
	if cfg.StorageS3CACertFile != "/etc/gogomail/s3-ca.pem" {
		t.Fatalf("StorageS3CACertFile = %q, want configured CA file", cfg.StorageS3CACertFile)
	}
	if !cfg.StorageS3InsecureSkipVerify {
		t.Fatal("StorageS3InsecureSkipVerify = false, want true")
	}
	if cfg.MigrationDir != "db/migrations" {
		t.Fatalf("MigrationDir = %q, want db/migrations", cfg.MigrationDir)
	}
	if cfg.SMTPDomain != "mail.example.com" {
		t.Fatalf("SMTPDomain = %q, want mail.example.com", cfg.SMTPDomain)
	}
	if cfg.SMTPMaxRecipients != 50 {
		t.Fatalf("SMTPMaxRecipients = %d, want 50", cfg.SMTPMaxRecipients)
	}
	if cfg.SMTPMaxMessageBytes != 2097152 {
		t.Fatalf("SMTPMaxMessageBytes = %d, want 2097152", cfg.SMTPMaxMessageBytes)
	}
	if !cfg.SMTPRequireAuth {
		t.Fatal("SMTPRequireAuth = false, want true")
	}
	if cfg.SMTPAddReceivedHeader {
		t.Fatal("SMTPAddReceivedHeader = true, want false")
	}
	if !cfg.SMTPSupportSMTPUTF8 {
		t.Fatal("SMTPSupportSMTPUTF8 = false, want true")
	}
	if !cfg.SMTPSupportRequireTLS {
		t.Fatal("SMTPSupportRequireTLS = false, want true")
	}
	if !cfg.SMTPSupportDSN {
		t.Fatal("SMTPSupportDSN = false, want true")
	}
	if !cfg.SMTPSupportBinaryMIME {
		t.Fatal("SMTPSupportBinaryMIME = false, want true")
	}
	if cfg.MailstoreRoot != "/tmp/gogomail-mailstore" {
		t.Fatalf("MailstoreRoot = %q, want /tmp/gogomail-mailstore", cfg.MailstoreRoot)
	}
	if len(cfg.LocalRecipients) != 2 || cfg.LocalRecipients[0] != "Admin@Example.COM" || cfg.LocalRecipients[1] != "user@example.com" {
		t.Fatalf("LocalRecipients = %v, want two parsed recipients", cfg.LocalRecipients)
	}
	if cfg.DedupBackend != "redis" {
		t.Fatalf("DedupBackend = %q, want redis", cfg.DedupBackend)
	}
	if cfg.RateLimitBackend != "redis" {
		t.Fatalf("RateLimitBackend = %q, want redis", cfg.RateLimitBackend)
	}
	if cfg.AttachmentScanBackend != "webhook" {
		t.Fatalf("AttachmentScanBackend = %q, want webhook", cfg.AttachmentScanBackend)
	}
	if cfg.AttachmentScanWebhookURL != "http://scanner.internal/scan" {
		t.Fatalf("AttachmentScanWebhookURL = %q, want scanner URL", cfg.AttachmentScanWebhookURL)
	}
	if cfg.AttachmentScanWebhookToken != "scanner-token" {
		t.Fatalf("AttachmentScanWebhookToken = %q, want scanner-token", cfg.AttachmentScanWebhookToken)
	}
	if cfg.AttachmentScanTimeout != 3*time.Second {
		t.Fatalf("AttachmentScanTimeout = %s, want 3s", cfg.AttachmentScanTimeout)
	}
	if cfg.AttachmentCleanupInterval != 15*time.Minute {
		t.Fatalf("AttachmentCleanupInterval = %s, want 15m", cfg.AttachmentCleanupInterval)
	}
	if cfg.AttachmentCleanupStaleAge != 48*time.Hour {
		t.Fatalf("AttachmentCleanupStaleAge = %s, want 48h", cfg.AttachmentCleanupStaleAge)
	}
	if cfg.AttachmentCleanupBatchSize != 250 {
		t.Fatalf("AttachmentCleanupBatchSize = %d, want 250", cfg.AttachmentCleanupBatchSize)
	}
	if !cfg.AttachmentCleanupRunOnce {
		t.Fatal("AttachmentCleanupRunOnce = false, want true")
	}
	if cfg.DriveCleanupInterval != 30*time.Minute {
		t.Fatalf("DriveCleanupInterval = %s, want 30m", cfg.DriveCleanupInterval)
	}
	if cfg.DriveCleanupBatchSize != 75 {
		t.Fatalf("DriveCleanupBatchSize = %d, want 75", cfg.DriveCleanupBatchSize)
	}
	if !cfg.DriveCleanupRunOnce {
		t.Fatal("DriveCleanupRunOnce = false, want true")
	}
	if cfg.DAVSyncRetentionInterval != 6*time.Hour {
		t.Fatalf("DAVSyncRetentionInterval = %s, want 6h", cfg.DAVSyncRetentionInterval)
	}
	if cfg.DAVSyncRetentionCutoffAge != 168*time.Hour {
		t.Fatalf("DAVSyncRetentionCutoffAge = %s, want 168h", cfg.DAVSyncRetentionCutoffAge)
	}
	if cfg.DAVSyncRetentionBatchSize != 600 {
		t.Fatalf("DAVSyncRetentionBatchSize = %d, want 600", cfg.DAVSyncRetentionBatchSize)
	}
	if !cfg.DAVSyncRetentionRunOnce {
		t.Fatal("DAVSyncRetentionRunOnce = false, want true")
	}
	if cfg.DAVSyncRetentionDryRun {
		t.Fatal("DAVSyncRetentionDryRun = true, want false")
	}
	if !cfg.DAVSyncRetentionConfirmReady {
		t.Fatal("DAVSyncRetentionConfirmReady = false, want true")
	}
	if cfg.DriveShareRateLimitBackend != "redis" {
		t.Fatalf("DriveShareRateLimitBackend = %q, want redis", cfg.DriveShareRateLimitBackend)
	}
	if cfg.DriveShareRateLimitPerMinute != 42 {
		t.Fatalf("DriveShareRateLimitPerMinute = %d, want 42", cfg.DriveShareRateLimitPerMinute)
	}
	if cfg.APIUsageRetentionInterval != 12*time.Hour {
		t.Fatalf("APIUsageRetentionInterval = %s, want 12h", cfg.APIUsageRetentionInterval)
	}
	if cfg.APIUsageRetentionCutoffAge != 720*time.Hour {
		t.Fatalf("APIUsageRetentionCutoffAge = %s, want 720h", cfg.APIUsageRetentionCutoffAge)
	}
	if cfg.APIUsageRetentionBatchSize != 500 {
		t.Fatalf("APIUsageRetentionBatchSize = %d, want 500", cfg.APIUsageRetentionBatchSize)
	}
	if !cfg.APIUsageRetentionRunOnce {
		t.Fatal("APIUsageRetentionRunOnce = false, want true")
	}
	if cfg.APIUsageRetentionDryRun {
		t.Fatal("APIUsageRetentionDryRun = true, want false")
	}
	if !cfg.APIUsageRetentionConfirmReady {
		t.Fatal("APIUsageRetentionConfirmReady = false, want true")
	}
	if cfg.APIUsageRetentionTenantID != "tenant-1" || cfg.APIUsageRetentionPrincipalID != "principal-1" {
		t.Fatalf("APIUsageRetention filters = %q/%q", cfg.APIUsageRetentionTenantID, cfg.APIUsageRetentionPrincipalID)
	}
	if cfg.RcptRateLimitPerMinute != 5 {
		t.Fatalf("RcptRateLimitPerMinute = %d, want 5", cfg.RcptRateLimitPerMinute)
	}
	if cfg.OutboxRelayBatchSize != 25 {
		t.Fatalf("OutboxRelayBatchSize = %d, want 25", cfg.OutboxRelayBatchSize)
	}
	if cfg.OutboxRelayPollInterval != 250*time.Millisecond {
		t.Fatalf("OutboxRelayPollInterval = %s, want 250ms", cfg.OutboxRelayPollInterval)
	}
	if cfg.OutboxRelayMaxAttempts != 3 {
		t.Fatalf("OutboxRelayMaxAttempts = %d, want 3", cfg.OutboxRelayMaxAttempts)
	}
	if cfg.EventStream != "custom.event" {
		t.Fatalf("EventStream = %q, want custom.event", cfg.EventStream)
	}
	if cfg.EventConsumerGroup != "custom-group" {
		t.Fatalf("EventConsumerGroup = %q, want custom-group", cfg.EventConsumerGroup)
	}
	if cfg.EventConsumerName != "worker-a" {
		t.Fatalf("EventConsumerName = %q, want worker-a", cfg.EventConsumerName)
	}
	if cfg.EventConsumerCount != 10 {
		t.Fatalf("EventConsumerCount = %d, want 10", cfg.EventConsumerCount)
	}
	if cfg.EventConsumerBlock != 500*time.Millisecond {
		t.Fatalf("EventConsumerBlock = %s, want 500ms", cfg.EventConsumerBlock)
	}
	if cfg.EventConsumerClaimIdle != 90*time.Second {
		t.Fatalf("EventConsumerClaimIdle = %s, want 90s", cfg.EventConsumerClaimIdle)
	}
	if cfg.EventConsumerMaxDeliveries != 7 {
		t.Fatalf("EventConsumerMaxDeliveries = %d, want 7", cfg.EventConsumerMaxDeliveries)
	}
	if cfg.EventConsumerDeadLetterStream != "custom.event.dlq" {
		t.Fatalf("EventConsumerDeadLetterStream = %q, want custom.event.dlq", cfg.EventConsumerDeadLetterStream)
	}
	if cfg.SearchIndexConsumerMaxDeliveries != 8 {
		t.Fatalf("SearchIndexConsumerMaxDeliveries = %d, want 8", cfg.SearchIndexConsumerMaxDeliveries)
	}
	if cfg.SearchIndexConsumerDeadLetterStream != "search.event.dlq" {
		t.Fatalf("SearchIndexConsumerDeadLetterStream = %q, want search.event.dlq", cfg.SearchIndexConsumerDeadLetterStream)
	}
	if cfg.APIMeteringConsumerMaxDeliveries != 9 {
		t.Fatalf("APIMeteringConsumerMaxDeliveries = %d, want 9", cfg.APIMeteringConsumerMaxDeliveries)
	}
	if cfg.APIMeteringConsumerDeadLetterStream != "api.event.dlq" {
		t.Fatalf("APIMeteringConsumerDeadLetterStream = %q, want api.event.dlq", cfg.APIMeteringConsumerDeadLetterStream)
	}
	if cfg.PushNotifyConsumerMaxDeliveries != 11 {
		t.Fatalf("PushNotifyConsumerMaxDeliveries = %d, want 11", cfg.PushNotifyConsumerMaxDeliveries)
	}
	if cfg.PushNotifyConsumerDeadLetterStream != "push.event.dlq" {
		t.Fatalf("PushNotifyConsumerDeadLetterStream = %q, want push.event.dlq", cfg.PushNotifyConsumerDeadLetterStream)
	}
	if cfg.DeliveryStream != "custom.outbound" {
		t.Fatalf("DeliveryStream = %q, want custom.outbound", cfg.DeliveryStream)
	}
	if cfg.DeliveryConsumerGroup != "delivery-group" {
		t.Fatalf("DeliveryConsumerGroup = %q, want delivery-group", cfg.DeliveryConsumerGroup)
	}
	if cfg.DeliveryConsumerName != "delivery-a" {
		t.Fatalf("DeliveryConsumerName = %q, want delivery-a", cfg.DeliveryConsumerName)
	}
	if cfg.DeliveryConsumerCount != 5 {
		t.Fatalf("DeliveryConsumerCount = %d, want 5", cfg.DeliveryConsumerCount)
	}
	if cfg.DeliveryConsumerBlock != 750*time.Millisecond {
		t.Fatalf("DeliveryConsumerBlock = %s, want 750ms", cfg.DeliveryConsumerBlock)
	}
	if cfg.DeliveryConsumerClaimIdle != 2*time.Minute {
		t.Fatalf("DeliveryConsumerClaimIdle = %s, want 2m", cfg.DeliveryConsumerClaimIdle)
	}
	if cfg.DeliveryConsumerMaxDeliveries != 12 {
		t.Fatalf("DeliveryConsumerMaxDeliveries = %d, want 12", cfg.DeliveryConsumerMaxDeliveries)
	}
	if cfg.DeliveryConsumerDeadLetterStream != "delivery.event.dlq" {
		t.Fatalf("DeliveryConsumerDeadLetterStream = %q, want delivery.event.dlq", cfg.DeliveryConsumerDeadLetterStream)
	}
	if cfg.DeliverySMTPHello != "mx.example.com" {
		t.Fatalf("DeliverySMTPHello = %q, want mx.example.com", cfg.DeliverySMTPHello)
	}
	if cfg.DeliveryTimeout != 45*time.Second {
		t.Fatalf("DeliveryTimeout = %s, want 45s", cfg.DeliveryTimeout)
	}
	if cfg.DeliveryTLSMode != "require" {
		t.Fatalf("DeliveryTLSMode = %q, want require", cfg.DeliveryTLSMode)
	}
	if len(cfg.DeliveryRetryDelays) != 3 ||
		cfg.DeliveryRetryDelays[0] != time.Minute ||
		cfg.DeliveryRetryDelays[1] != 5*time.Minute ||
		cfg.DeliveryRetryDelays[2] != time.Hour {
		t.Fatalf("DeliveryRetryDelays = %v, want [1m 5m 1h]", cfg.DeliveryRetryDelays)
	}
	if cfg.DeliveryRetryJitterRatio != 0.35 {
		t.Fatalf("DeliveryRetryJitterRatio = %f, want 0.35", cfg.DeliveryRetryJitterRatio)
	}
	if cfg.DeliveryRetryMaxDelay != 6*time.Hour {
		t.Fatalf("DeliveryRetryMaxDelay = %s, want 6h", cfg.DeliveryRetryMaxDelay)
	}
	if !cfg.DKIMEnabled {
		t.Fatal("DKIMEnabled = false, want true")
	}
	if cfg.AdminToken != "secret" {
		t.Fatalf("AdminToken = %q, want secret", cfg.AdminToken)
	}
	if cfg.AuthJWTSecret != "jwt-secret" {
		t.Fatalf("AuthJWTSecret = %q, want jwt-secret", cfg.AuthJWTSecret)
	}
	if cfg.PushNotifyDeviceLimit != 25 {
		t.Fatalf("PushNotifyDeviceLimit = %d, want 25", cfg.PushNotifyDeviceLimit)
	}
	if cfg.PushNotifyBackend != "webhook" {
		t.Fatalf("PushNotifyBackend = %q, want webhook", cfg.PushNotifyBackend)
	}
	if cfg.PushNotifyWebhookURL != "https://push.internal/send" {
		t.Fatalf("PushNotifyWebhookURL = %q, want webhook URL", cfg.PushNotifyWebhookURL)
	}
	if cfg.PushNotifyWebhookToken != "push-token" {
		t.Fatalf("PushNotifyWebhookToken = %q, want push-token", cfg.PushNotifyWebhookToken)
	}
	if cfg.PushNotifyWebhookTimeout != 4*time.Second {
		t.Fatalf("PushNotifyWebhookTimeout = %s, want 4s", cfg.PushNotifyWebhookTimeout)
	}
}

func TestLoadDisablesSubmissionInsecureAuthByDefaultInProduction(t *testing.T) {
	t.Setenv("GOGOMAIL_ENV", "production")
	t.Setenv("GOGOMAIL_SUBMISSION_ALLOW_INSECURE_AUTH", "")

	cfg := Load()

	if cfg.SubmissionAllowInsecureAuth {
		t.Fatal("SubmissionAllowInsecureAuth = true, want false in production defaults")
	}
}

func TestLoadReadsConsumerClaimIdleSettings(t *testing.T) {
	t.Setenv("GOGOMAIL_EVENT_CONSUMER_CLAIM_IDLE", "1m")
	t.Setenv("GOGOMAIL_SEARCH_INDEX_CONSUMER_CLAIM_IDLE", "2m")
	t.Setenv("GOGOMAIL_API_METERING_CONSUMER_CLAIM_IDLE", "3m")
	t.Setenv("GOGOMAIL_PUSH_NOTIFICATION_CONSUMER_CLAIM_IDLE", "4m")
	t.Setenv("GOGOMAIL_DELIVERY_CONSUMER_CLAIM_IDLE", "5m")

	cfg := Load()

	if cfg.EventConsumerClaimIdle != time.Minute {
		t.Fatalf("EventConsumerClaimIdle = %s, want 1m", cfg.EventConsumerClaimIdle)
	}
	if cfg.SearchIndexConsumerClaimIdle != 2*time.Minute {
		t.Fatalf("SearchIndexConsumerClaimIdle = %s, want 2m", cfg.SearchIndexConsumerClaimIdle)
	}
	if cfg.APIMeteringConsumerClaimIdle != 3*time.Minute {
		t.Fatalf("APIMeteringConsumerClaimIdle = %s, want 3m", cfg.APIMeteringConsumerClaimIdle)
	}
	if cfg.PushNotifyConsumerClaimIdle != 4*time.Minute {
		t.Fatalf("PushNotifyConsumerClaimIdle = %s, want 4m", cfg.PushNotifyConsumerClaimIdle)
	}
	if cfg.DeliveryConsumerClaimIdle != 5*time.Minute {
		t.Fatalf("DeliveryConsumerClaimIdle = %s, want 5m", cfg.DeliveryConsumerClaimIdle)
	}
}

func TestLoadReadsConsumerDeadLetterSettings(t *testing.T) {
	t.Setenv("GOGOMAIL_EVENT_STREAM", "mail.events")
	t.Setenv("GOGOMAIL_API_METERING_STREAM", "api.events")
	t.Setenv("GOGOMAIL_DELIVERY_STREAM", "delivery.events")
	t.Setenv("GOGOMAIL_EVENT_CONSUMER_MAX_DELIVERIES", "3")
	t.Setenv("GOGOMAIL_EVENT_CONSUMER_DEAD_LETTER_STREAM", "mail.events.poison")
	t.Setenv("GOGOMAIL_SEARCH_INDEX_CONSUMER_MAX_DELIVERIES", "4")
	t.Setenv("GOGOMAIL_SEARCH_INDEX_CONSUMER_DEAD_LETTER_STREAM", "search.events.poison")
	t.Setenv("GOGOMAIL_API_METERING_CONSUMER_MAX_DELIVERIES", "5")
	t.Setenv("GOGOMAIL_API_METERING_CONSUMER_DEAD_LETTER_STREAM", "api.events.poison")
	t.Setenv("GOGOMAIL_PUSH_NOTIFICATION_CONSUMER_MAX_DELIVERIES", "6")
	t.Setenv("GOGOMAIL_PUSH_NOTIFICATION_CONSUMER_DEAD_LETTER_STREAM", "push.events.poison")
	t.Setenv("GOGOMAIL_DELIVERY_CONSUMER_MAX_DELIVERIES", "7")
	t.Setenv("GOGOMAIL_DELIVERY_CONSUMER_DEAD_LETTER_STREAM", "delivery.events.poison")

	cfg := Load()

	if cfg.EventConsumerMaxDeliveries != 3 || cfg.EventConsumerDeadLetterStream != "mail.events.poison" {
		t.Fatalf("event dead-letter settings = %d/%q, want 3/mail.events.poison", cfg.EventConsumerMaxDeliveries, cfg.EventConsumerDeadLetterStream)
	}
	if cfg.SearchIndexConsumerMaxDeliveries != 4 || cfg.SearchIndexConsumerDeadLetterStream != "search.events.poison" {
		t.Fatalf("search dead-letter settings = %d/%q, want 4/search.events.poison", cfg.SearchIndexConsumerMaxDeliveries, cfg.SearchIndexConsumerDeadLetterStream)
	}
	if cfg.APIMeteringConsumerMaxDeliveries != 5 || cfg.APIMeteringConsumerDeadLetterStream != "api.events.poison" {
		t.Fatalf("api metering dead-letter settings = %d/%q, want 5/api.events.poison", cfg.APIMeteringConsumerMaxDeliveries, cfg.APIMeteringConsumerDeadLetterStream)
	}
	if cfg.PushNotifyConsumerMaxDeliveries != 6 || cfg.PushNotifyConsumerDeadLetterStream != "push.events.poison" {
		t.Fatalf("push dead-letter settings = %d/%q, want 6/push.events.poison", cfg.PushNotifyConsumerMaxDeliveries, cfg.PushNotifyConsumerDeadLetterStream)
	}
	if cfg.DeliveryConsumerMaxDeliveries != 7 || cfg.DeliveryConsumerDeadLetterStream != "delivery.events.poison" {
		t.Fatalf("delivery dead-letter settings = %d/%q, want 7/delivery.events.poison", cfg.DeliveryConsumerMaxDeliveries, cfg.DeliveryConsumerDeadLetterStream)
	}
}

func TestLoadFallsBackForInvalidInt64Environment(t *testing.T) {
	t.Setenv("GOGOMAIL_SMTP_MAX_MESSAGE_BYTES", "not-a-number")
	t.Setenv("GOGOMAIL_SUBMISSION_MAX_MESSAGE_BYTES", "also-bad")

	cfg := Load()

	if cfg.SMTPMaxMessageBytes != 25*1024*1024 {
		t.Fatalf("SMTPMaxMessageBytes = %d, want fallback 25MiB", cfg.SMTPMaxMessageBytes)
	}
	if cfg.SubmissionMaxMessageBytes != 25*1024*1024 {
		t.Fatalf("SubmissionMaxMessageBytes = %d, want fallback 25MiB", cfg.SubmissionMaxMessageBytes)
	}
}

func TestLoadPreservesS3AccessKeyIDWhitespaceForValidation(t *testing.T) {
	t.Setenv("GOGOMAIL_STORAGE_S3_ACCESS_KEY_ID", " access ")

	cfg := Load()

	if cfg.StorageS3AccessKeyID != " access " {
		t.Fatalf("StorageS3AccessKeyID = %q, want raw env value", cfg.StorageS3AccessKeyID)
	}
}

func TestLoadFallsBackForInvalidDurationCSVEnvironment(t *testing.T) {
	t.Setenv("GOGOMAIL_DELIVERY_RETRY_DELAYS", "5m,definitely-bad,1h")

	cfg := Load()

	want := []time.Duration{5 * time.Minute, 30 * time.Minute, 2 * time.Hour, 8 * time.Hour, 24 * time.Hour}
	if len(cfg.DeliveryRetryDelays) != len(want) {
		t.Fatalf("DeliveryRetryDelays = %v, want fallback %v", cfg.DeliveryRetryDelays, want)
	}
	for i := range want {
		if cfg.DeliveryRetryDelays[i] != want[i] {
			t.Fatalf("DeliveryRetryDelays = %v, want fallback %v", cfg.DeliveryRetryDelays, want)
		}
	}
}

func TestLoadFallsBackForBlankDurationCSVEnvironment(t *testing.T) {
	t.Setenv("GOGOMAIL_DELIVERY_RETRY_DELAYS", ",,")

	cfg := Load()

	want := []time.Duration{5 * time.Minute, 30 * time.Minute, 2 * time.Hour, 8 * time.Hour, 24 * time.Hour}
	if len(cfg.DeliveryRetryDelays) != len(want) {
		t.Fatalf("DeliveryRetryDelays = %v, want fallback %v", cfg.DeliveryRetryDelays, want)
	}
	for i := range want {
		if cfg.DeliveryRetryDelays[i] != want[i] {
			t.Fatalf("DeliveryRetryDelays = %v, want fallback %v", cfg.DeliveryRetryDelays, want)
		}
	}
}
