package config

import (
	"testing"
	"time"
)

func TestLoadAppliesDefaults(t *testing.T) {
	t.Setenv("GOGOMAIL_ENV", "")
	t.Setenv("GOGOMAIL_HTTP_ADDR", "")
	t.Setenv("GOGOMAIL_SMTP_ADDR", "")
	t.Setenv("GOGOMAIL_DATABASE_URL", "")
	t.Setenv("GOGOMAIL_REDIS_ADDR", "")
	t.Setenv("GOGOMAIL_STORAGE_BACKEND", "")
	t.Setenv("GOGOMAIL_MIGRATION_DIR", "")
	t.Setenv("GOGOMAIL_SMTP_DOMAIN", "")
	t.Setenv("GOGOMAIL_MAILSTORE_ROOT", "")
	t.Setenv("GOGOMAIL_LOCAL_RECIPIENTS", "")
	t.Setenv("GOGOMAIL_DEDUP_BACKEND", "")
	t.Setenv("GOGOMAIL_RATELIMIT_BACKEND", "")
	t.Setenv("GOGOMAIL_RCPT_RATE_LIMIT_PER_MINUTE", "")
	t.Setenv("GOGOMAIL_OUTBOX_RELAY_BATCH_SIZE", "")
	t.Setenv("GOGOMAIL_OUTBOX_RELAY_POLL_INTERVAL", "")
	t.Setenv("GOGOMAIL_OUTBOX_RELAY_MAX_ATTEMPTS", "")
	t.Setenv("GOGOMAIL_EVENT_STREAM", "")
	t.Setenv("GOGOMAIL_EVENT_CONSUMER_GROUP", "")
	t.Setenv("GOGOMAIL_EVENT_CONSUMER_NAME", "")
	t.Setenv("GOGOMAIL_EVENT_CONSUMER_COUNT", "")
	t.Setenv("GOGOMAIL_EVENT_CONSUMER_BLOCK", "")
	t.Setenv("GOGOMAIL_DELIVERY_STREAM", "")
	t.Setenv("GOGOMAIL_DELIVERY_CONSUMER_GROUP", "")
	t.Setenv("GOGOMAIL_DELIVERY_CONSUMER_NAME", "")
	t.Setenv("GOGOMAIL_DELIVERY_CONSUMER_COUNT", "")
	t.Setenv("GOGOMAIL_DELIVERY_CONSUMER_BLOCK", "")
	t.Setenv("GOGOMAIL_DELIVERY_SMTP_HELLO", "")
	t.Setenv("GOGOMAIL_ADMIN_TOKEN", "")

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
	if cfg.StorageBackend != "local" {
		t.Fatalf("StorageBackend = %q, want local", cfg.StorageBackend)
	}
	if cfg.MigrationDir != "migrations" {
		t.Fatalf("MigrationDir = %q, want migrations", cfg.MigrationDir)
	}
	if cfg.SMTPDomain != "localhost" {
		t.Fatalf("SMTPDomain = %q, want localhost", cfg.SMTPDomain)
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
	if cfg.DeliverySMTPHello != "localhost" {
		t.Fatalf("DeliverySMTPHello = %q, want localhost", cfg.DeliverySMTPHello)
	}
	if cfg.AdminToken != "" {
		t.Fatalf("AdminToken = %q, want empty", cfg.AdminToken)
	}
}

func TestLoadReadsEnvironmentOverrides(t *testing.T) {
	t.Setenv("GOGOMAIL_ENV", "test")
	t.Setenv("GOGOMAIL_HTTP_ADDR", ":18080")
	t.Setenv("GOGOMAIL_SMTP_ADDR", ":10025")
	t.Setenv("GOGOMAIL_DATABASE_URL", "postgres://example")
	t.Setenv("GOGOMAIL_REDIS_ADDR", "redis:6379")
	t.Setenv("GOGOMAIL_STORAGE_BACKEND", "minio")
	t.Setenv("GOGOMAIL_MIGRATION_DIR", "db/migrations")
	t.Setenv("GOGOMAIL_SMTP_DOMAIN", "mail.example.com")
	t.Setenv("GOGOMAIL_MAILSTORE_ROOT", "/tmp/gogomail-mailstore")
	t.Setenv("GOGOMAIL_LOCAL_RECIPIENTS", "Admin@Example.COM, user@example.com ")
	t.Setenv("GOGOMAIL_DEDUP_BACKEND", "redis")
	t.Setenv("GOGOMAIL_RATELIMIT_BACKEND", "redis")
	t.Setenv("GOGOMAIL_RCPT_RATE_LIMIT_PER_MINUTE", "5")
	t.Setenv("GOGOMAIL_OUTBOX_RELAY_BATCH_SIZE", "25")
	t.Setenv("GOGOMAIL_OUTBOX_RELAY_POLL_INTERVAL", "250ms")
	t.Setenv("GOGOMAIL_OUTBOX_RELAY_MAX_ATTEMPTS", "3")
	t.Setenv("GOGOMAIL_EVENT_STREAM", "custom.event")
	t.Setenv("GOGOMAIL_EVENT_CONSUMER_GROUP", "custom-group")
	t.Setenv("GOGOMAIL_EVENT_CONSUMER_NAME", "worker-a")
	t.Setenv("GOGOMAIL_EVENT_CONSUMER_COUNT", "10")
	t.Setenv("GOGOMAIL_EVENT_CONSUMER_BLOCK", "500ms")
	t.Setenv("GOGOMAIL_DELIVERY_STREAM", "custom.outbound")
	t.Setenv("GOGOMAIL_DELIVERY_CONSUMER_GROUP", "delivery-group")
	t.Setenv("GOGOMAIL_DELIVERY_CONSUMER_NAME", "delivery-a")
	t.Setenv("GOGOMAIL_DELIVERY_CONSUMER_COUNT", "5")
	t.Setenv("GOGOMAIL_DELIVERY_CONSUMER_BLOCK", "750ms")
	t.Setenv("GOGOMAIL_DELIVERY_SMTP_HELLO", "mx.example.com")
	t.Setenv("GOGOMAIL_ADMIN_TOKEN", "secret")

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
	if cfg.DeliverySMTPHello != "mx.example.com" {
		t.Fatalf("DeliverySMTPHello = %q, want mx.example.com", cfg.DeliverySMTPHello)
	}
	if cfg.AdminToken != "secret" {
		t.Fatalf("AdminToken = %q, want secret", cfg.AdminToken)
	}
}
