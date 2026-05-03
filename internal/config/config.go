package config

import (
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	Environment             string
	HTTPAddr                string
	SMTPAddr                string
	DatabaseURL             string
	RedisAddr               string
	StorageBackend          string
	MigrationDir            string
	SMTPDomain              string
	MailstoreRoot           string
	LocalRecipients         []string
	DedupBackend            string
	RateLimitBackend        string
	BackpressureBackend     string
	RcptRateLimitPerMinute  int
	OutboxRelayBatchSize    int
	OutboxRelayPollInterval time.Duration
	OutboxRelayMaxAttempts  int
	EventStream             string
	EventConsumerGroup      string
	EventConsumerName       string
	EventConsumerCount      int
	EventConsumerBlock      time.Duration
}

func Load() Config {
	return Config{
		Environment:             envOrDefault("GOGOMAIL_ENV", "development"),
		HTTPAddr:                envOrDefault("GOGOMAIL_HTTP_ADDR", ":8080"),
		SMTPAddr:                envOrDefault("GOGOMAIL_SMTP_ADDR", ":2525"),
		DatabaseURL:             envOrDefault("GOGOMAIL_DATABASE_URL", "postgres://gogomail:gogomail@localhost:5432/gogomail?sslmode=disable"),
		RedisAddr:               envOrDefault("GOGOMAIL_REDIS_ADDR", "localhost:6379"),
		StorageBackend:          envOrDefault("GOGOMAIL_STORAGE_BACKEND", "local"),
		MigrationDir:            envOrDefault("GOGOMAIL_MIGRATION_DIR", "migrations"),
		SMTPDomain:              envOrDefault("GOGOMAIL_SMTP_DOMAIN", "localhost"),
		MailstoreRoot:           envOrDefault("GOGOMAIL_MAILSTORE_ROOT", "var/mailstore"),
		LocalRecipients:         splitCSV(os.Getenv("GOGOMAIL_LOCAL_RECIPIENTS")),
		DedupBackend:            envOrDefault("GOGOMAIL_DEDUP_BACKEND", "none"),
		RateLimitBackend:        envOrDefault("GOGOMAIL_RATELIMIT_BACKEND", "none"),
		BackpressureBackend:     envOrDefault("GOGOMAIL_BACKPRESSURE_BACKEND", "none"),
		RcptRateLimitPerMinute:  intEnvOrDefault("GOGOMAIL_RCPT_RATE_LIMIT_PER_MINUTE", 60),
		OutboxRelayBatchSize:    intEnvOrDefault("GOGOMAIL_OUTBOX_RELAY_BATCH_SIZE", 100),
		OutboxRelayPollInterval: durationEnvOrDefault("GOGOMAIL_OUTBOX_RELAY_POLL_INTERVAL", time.Second),
		OutboxRelayMaxAttempts:  intEnvOrDefault("GOGOMAIL_OUTBOX_RELAY_MAX_ATTEMPTS", 10),
		EventStream:             envOrDefault("GOGOMAIL_EVENT_STREAM", "mail.event"),
		EventConsumerGroup:      envOrDefault("GOGOMAIL_EVENT_CONSUMER_GROUP", "gogomail.event-worker"),
		EventConsumerName:       envOrDefault("GOGOMAIL_EVENT_CONSUMER_NAME", "event-worker-1"),
		EventConsumerCount:      intEnvOrDefault("GOGOMAIL_EVENT_CONSUMER_COUNT", 100),
		EventConsumerBlock:      durationEnvOrDefault("GOGOMAIL_EVENT_CONSUMER_BLOCK", time.Second),
	}
}

func envOrDefault(key string, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func splitCSV(raw string) []string {
	parts := strings.Split(raw, ",")
	values := make([]string, 0, len(parts))
	for _, part := range parts {
		value := strings.TrimSpace(part)
		if value != "" {
			values = append(values, value)
		}
	}
	return values
}

func intEnvOrDefault(key string, fallback int) int {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}
	return parsed
}

func durationEnvOrDefault(key string, fallback time.Duration) time.Duration {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	parsed, err := time.ParseDuration(value)
	if err != nil {
		return fallback
	}
	return parsed
}
