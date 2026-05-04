package config

import (
	"testing"
	"time"
)

func TestLoadAppliesDefaults(t *testing.T) {
	t.Setenv("GOGOMAIL_ENV", "")
	t.Setenv("GOGOMAIL_HTTP_ADDR", "")
	t.Setenv("GOGOMAIL_SMTP_ADDR", "")
	t.Setenv("GOGOMAIL_INBOUND_SMTP_ADDR", "")
	t.Setenv("GOGOMAIL_INBOUND_TRUSTED_RELAYS", "")
	t.Setenv("GOGOMAIL_SUBMISSION_ADDR", "")
	t.Setenv("GOGOMAIL_SUBMISSION_SMTPS_ADDR", "")
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
	t.Setenv("GOGOMAIL_MIGRATION_DIR", "")
	t.Setenv("GOGOMAIL_SMTP_DOMAIN", "")
	t.Setenv("GOGOMAIL_SMTP_READ_TIMEOUT", "")
	t.Setenv("GOGOMAIL_SMTP_WRITE_TIMEOUT", "")
	t.Setenv("GOGOMAIL_SMTP_MAX_RECIPIENTS", "")
	t.Setenv("GOGOMAIL_SMTP_MAX_MESSAGE_BYTES", "")
	t.Setenv("GOGOMAIL_SMTP_REQUIRE_AUTH", "")
	t.Setenv("GOGOMAIL_SMTP_ADD_RECEIVED_HEADER", "")
	t.Setenv("GOGOMAIL_SMTP_SUPPORT_SMTPUTF8", "")
	t.Setenv("GOGOMAIL_SMTP_SUPPORT_REQUIRETLS", "")
	t.Setenv("GOGOMAIL_SMTP_SUPPORT_DSN", "")
	t.Setenv("GOGOMAIL_SMTP_SUPPORT_BINARYMIME", "")
	t.Setenv("GOGOMAIL_MAILSTORE_ROOT", "")
	t.Setenv("GOGOMAIL_LOCAL_RECIPIENTS", "")
	t.Setenv("GOGOMAIL_DEDUP_BACKEND", "")
	t.Setenv("GOGOMAIL_RATELIMIT_BACKEND", "")
	t.Setenv("GOGOMAIL_ATTACHMENT_SCAN_BACKEND", "")
	t.Setenv("GOGOMAIL_ATTACHMENT_SCAN_WEBHOOK_URL", "")
	t.Setenv("GOGOMAIL_ATTACHMENT_SCAN_WEBHOOK_TOKEN", "")
	t.Setenv("GOGOMAIL_ATTACHMENT_SCAN_TIMEOUT", "")
	t.Setenv("GOGOMAIL_PUSH_NOTIFICATION_BACKEND", "")
	t.Setenv("GOGOMAIL_PUSH_NOTIFICATION_WEBHOOK_URL", "")
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
	t.Setenv("GOGOMAIL_DELIVERY_STREAM", "")
	t.Setenv("GOGOMAIL_DELIVERY_CONSUMER_GROUP", "")
	t.Setenv("GOGOMAIL_DELIVERY_CONSUMER_NAME", "")
	t.Setenv("GOGOMAIL_DELIVERY_CONSUMER_COUNT", "")
	t.Setenv("GOGOMAIL_DELIVERY_CONSUMER_BLOCK", "")
	t.Setenv("GOGOMAIL_DELIVERY_CONSUMER_CLAIM_IDLE", "")
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

	cfg := Load()

	if cfg.Environment != "development" {
		t.Fatalf("Environment = %q, want development", cfg.Environment)
	}
	if cfg.HTTPAddr != ":8080" {
		t.Fatalf("HTTPAddr = %q, want :8080", cfg.HTTPAddr)
	}
	if cfg.SMTPAddr != ":2525" {
		t.Fatalf("SMTPAddr = %q, want :2525", cfg.SMTPAddr)
	}
	if cfg.InboundSMTPAddr != ":2526" {
		t.Fatalf("InboundSMTPAddr = %q, want :2526", cfg.InboundSMTPAddr)
	}
	if len(cfg.InboundTrustedRelays) != 2 {
		t.Fatalf("InboundTrustedRelays = %+v, want loopback defaults", cfg.InboundTrustedRelays)
	}
	if cfg.SubmissionAddr != ":2587" {
		t.Fatalf("SubmissionAddr = %q, want :2587", cfg.SubmissionAddr)
	}
	if cfg.SubmissionSMTPSAddr != "" {
		t.Fatalf("SubmissionSMTPSAddr = %q, want empty", cfg.SubmissionSMTPSAddr)
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
	if cfg.PushNotifyWebhookTimeout != 2*time.Second {
		t.Fatalf("PushNotifyWebhookTimeout = %s, want 2s", cfg.PushNotifyWebhookTimeout)
	}
}

func TestLoadReadsEnvironmentOverrides(t *testing.T) {
	t.Setenv("GOGOMAIL_ENV", "test")
	t.Setenv("GOGOMAIL_HTTP_ADDR", ":18080")
	t.Setenv("GOGOMAIL_SMTP_ADDR", ":10025")
	t.Setenv("GOGOMAIL_SUBMISSION_ADDR", ":10587")
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
	t.Setenv("GOGOMAIL_STORAGE_BACKEND", "minio")
	t.Setenv("GOGOMAIL_MIGRATION_DIR", "db/migrations")
	t.Setenv("GOGOMAIL_SMTP_DOMAIN", "mail.example.com")
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
	t.Setenv("GOGOMAIL_PUSH_NOTIFICATION_BACKEND", "webhook")
	t.Setenv("GOGOMAIL_PUSH_NOTIFICATION_WEBHOOK_URL", "https://push.internal/send")
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
	t.Setenv("GOGOMAIL_DELIVERY_STREAM", "custom.outbound")
	t.Setenv("GOGOMAIL_DELIVERY_CONSUMER_GROUP", "delivery-group")
	t.Setenv("GOGOMAIL_DELIVERY_CONSUMER_NAME", "delivery-a")
	t.Setenv("GOGOMAIL_DELIVERY_CONSUMER_COUNT", "5")
	t.Setenv("GOGOMAIL_DELIVERY_CONSUMER_BLOCK", "750ms")
	t.Setenv("GOGOMAIL_DELIVERY_CONSUMER_CLAIM_IDLE", "2m")
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

	cfg := Load()

	if cfg.Environment != "test" {
		t.Fatalf("Environment = %q, want test", cfg.Environment)
	}
	if cfg.HTTPAddr != ":18080" {
		t.Fatalf("HTTPAddr = %q, want :18080", cfg.HTTPAddr)
	}
	if cfg.SMTPAddr != ":10025" {
		t.Fatalf("SMTPAddr = %q, want :10025", cfg.SMTPAddr)
	}
	if cfg.SubmissionAddr != ":10587" {
		t.Fatalf("SubmissionAddr = %q, want :10587", cfg.SubmissionAddr)
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
	if cfg.StorageBackend != "minio" {
		t.Fatalf("StorageBackend = %q, want minio", cfg.StorageBackend)
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
