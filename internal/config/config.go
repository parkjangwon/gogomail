package config

import (
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	Environment                 string
	HTTPAddr                    string
	SMTPAddr                    string
	SubmissionAddr              string
	SubmissionMaxRecipients     int
	SubmissionMaxMessageBytes   int64
	SubmissionSupportSMTPUTF8   bool
	SubmissionSupportRequireTLS bool
	SMTPTLSCertFile             string
	SMTPTLSKeyFile              string
	SubmissionAllowInsecureAuth bool
	DatabaseURL                 string
	RedisAddr                   string
	StorageBackend              string
	MigrationDir                string
	SMTPDomain                  string
	SMTPMaxRecipients           int
	SMTPMaxMessageBytes         int64
	SMTPSupportSMTPUTF8         bool
	SMTPSupportRequireTLS       bool
	SMTPSupportDSN              bool
	SMTPSupportBinaryMIME       bool
	MailstoreRoot               string
	LocalRecipients             []string
	DedupBackend                string
	RateLimitBackend            string
	BackpressureBackend         string
	RcptRateLimitPerMinute      int
	OutboxRelayBatchSize        int
	OutboxRelayPollInterval     time.Duration
	OutboxRelayMaxAttempts      int
	EventStream                 string
	EventConsumerGroup          string
	EventConsumerName           string
	EventConsumerCount          int
	EventConsumerBlock          time.Duration
	DeliveryStream              string
	DeliveryConsumerGroup       string
	DeliveryConsumerName        string
	DeliveryConsumerCount       int
	DeliveryConsumerBlock       time.Duration
	DeliverySMTPHello           string
	DeliveryTimeout             time.Duration
	DeliveryTLSMode             string
	DeliveryRetryDelays         []time.Duration
	DeliveryRetryJitterRatio    float64
	DeliveryRetryMaxDelay       time.Duration
	DKIMEnabled                 bool
	AdminToken                  string
	AuthJWTSecret               string
}

func Load() Config {
	return Config{
		Environment:                 envOrDefault("GOGOMAIL_ENV", "development"),
		HTTPAddr:                    envOrDefault("GOGOMAIL_HTTP_ADDR", ":8080"),
		SMTPAddr:                    envOrDefault("GOGOMAIL_SMTP_ADDR", ":2525"),
		SubmissionAddr:              envOrDefault("GOGOMAIL_SUBMISSION_ADDR", ":2587"),
		SubmissionMaxRecipients:     intEnvOrDefault("GOGOMAIL_SUBMISSION_MAX_RECIPIENTS", 100),
		SubmissionMaxMessageBytes:   int64EnvOrDefault("GOGOMAIL_SUBMISSION_MAX_MESSAGE_BYTES", 25*1024*1024),
		SubmissionSupportSMTPUTF8:   boolEnvOrDefault("GOGOMAIL_SUBMISSION_SUPPORT_SMTPUTF8", false),
		SubmissionSupportRequireTLS: boolEnvOrDefault("GOGOMAIL_SUBMISSION_SUPPORT_REQUIRETLS", false),
		SMTPTLSCertFile:             envOrDefault("GOGOMAIL_SMTP_TLS_CERT_FILE", ""),
		SMTPTLSKeyFile:              envOrDefault("GOGOMAIL_SMTP_TLS_KEY_FILE", ""),
		SubmissionAllowInsecureAuth: boolEnvOrDefault("GOGOMAIL_SUBMISSION_ALLOW_INSECURE_AUTH", defaultSubmissionAllowInsecureAuth()),
		DatabaseURL:                 envOrDefault("GOGOMAIL_DATABASE_URL", "postgres://gogomail:gogomail@localhost:5432/gogomail?sslmode=disable"),
		RedisAddr:                   envOrDefault("GOGOMAIL_REDIS_ADDR", "localhost:6379"),
		StorageBackend:              envOrDefault("GOGOMAIL_STORAGE_BACKEND", "local"),
		MigrationDir:                envOrDefault("GOGOMAIL_MIGRATION_DIR", "migrations"),
		SMTPDomain:                  envOrDefault("GOGOMAIL_SMTP_DOMAIN", "localhost"),
		SMTPMaxRecipients:           intEnvOrDefault("GOGOMAIL_SMTP_MAX_RECIPIENTS", 100),
		SMTPMaxMessageBytes:         int64EnvOrDefault("GOGOMAIL_SMTP_MAX_MESSAGE_BYTES", 25*1024*1024),
		SMTPSupportSMTPUTF8:         boolEnvOrDefault("GOGOMAIL_SMTP_SUPPORT_SMTPUTF8", false),
		SMTPSupportRequireTLS:       boolEnvOrDefault("GOGOMAIL_SMTP_SUPPORT_REQUIRETLS", false),
		SMTPSupportDSN:              boolEnvOrDefault("GOGOMAIL_SMTP_SUPPORT_DSN", false),
		SMTPSupportBinaryMIME:       boolEnvOrDefault("GOGOMAIL_SMTP_SUPPORT_BINARYMIME", false),
		MailstoreRoot:               envOrDefault("GOGOMAIL_MAILSTORE_ROOT", "var/mailstore"),
		LocalRecipients:             splitCSV(os.Getenv("GOGOMAIL_LOCAL_RECIPIENTS")),
		DedupBackend:                envOrDefault("GOGOMAIL_DEDUP_BACKEND", "none"),
		RateLimitBackend:            envOrDefault("GOGOMAIL_RATELIMIT_BACKEND", "none"),
		BackpressureBackend:         envOrDefault("GOGOMAIL_BACKPRESSURE_BACKEND", "none"),
		RcptRateLimitPerMinute:      intEnvOrDefault("GOGOMAIL_RCPT_RATE_LIMIT_PER_MINUTE", 60),
		OutboxRelayBatchSize:        intEnvOrDefault("GOGOMAIL_OUTBOX_RELAY_BATCH_SIZE", 100),
		OutboxRelayPollInterval:     durationEnvOrDefault("GOGOMAIL_OUTBOX_RELAY_POLL_INTERVAL", time.Second),
		OutboxRelayMaxAttempts:      intEnvOrDefault("GOGOMAIL_OUTBOX_RELAY_MAX_ATTEMPTS", 10),
		EventStream:                 envOrDefault("GOGOMAIL_EVENT_STREAM", "mail.event"),
		EventConsumerGroup:          envOrDefault("GOGOMAIL_EVENT_CONSUMER_GROUP", "gogomail.event-worker"),
		EventConsumerName:           envOrDefault("GOGOMAIL_EVENT_CONSUMER_NAME", "event-worker-1"),
		EventConsumerCount:          intEnvOrDefault("GOGOMAIL_EVENT_CONSUMER_COUNT", 100),
		EventConsumerBlock:          durationEnvOrDefault("GOGOMAIL_EVENT_CONSUMER_BLOCK", time.Second),
		DeliveryStream:              envOrDefault("GOGOMAIL_DELIVERY_STREAM", "mail.outbound.general"),
		DeliveryConsumerGroup:       envOrDefault("GOGOMAIL_DELIVERY_CONSUMER_GROUP", "gogomail.delivery-worker"),
		DeliveryConsumerName:        envOrDefault("GOGOMAIL_DELIVERY_CONSUMER_NAME", "delivery-worker-1"),
		DeliveryConsumerCount:       intEnvOrDefault("GOGOMAIL_DELIVERY_CONSUMER_COUNT", 50),
		DeliveryConsumerBlock:       durationEnvOrDefault("GOGOMAIL_DELIVERY_CONSUMER_BLOCK", time.Second),
		DeliverySMTPHello:           envOrDefault("GOGOMAIL_DELIVERY_SMTP_HELLO", "localhost"),
		DeliveryTimeout:             durationEnvOrDefault("GOGOMAIL_DELIVERY_TIMEOUT", 30*time.Second),
		DeliveryTLSMode:             envOrDefault("GOGOMAIL_DELIVERY_TLS_MODE", "opportunistic"),
		DeliveryRetryDelays:         durationCSVEnvOrDefault("GOGOMAIL_DELIVERY_RETRY_DELAYS", []time.Duration{5 * time.Minute, 30 * time.Minute, 2 * time.Hour, 8 * time.Hour, 24 * time.Hour}),
		DeliveryRetryJitterRatio:    floatEnvOrDefault("GOGOMAIL_DELIVERY_RETRY_JITTER_RATIO", 0.20),
		DeliveryRetryMaxDelay:       durationEnvOrDefault("GOGOMAIL_DELIVERY_RETRY_MAX_DELAY", 24*time.Hour),
		DKIMEnabled:                 boolEnvOrDefault("GOGOMAIL_DKIM_ENABLED", false),
		AdminToken:                  envOrDefault("GOGOMAIL_ADMIN_TOKEN", ""),
		AuthJWTSecret:               envOrDefault("GOGOMAIL_AUTH_JWT_SECRET", ""),
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

func int64EnvOrDefault(key string, fallback int64) int64 {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	parsed, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		return fallback
	}
	return parsed
}

func boolEnvOrDefault(key string, fallback bool) bool {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return fallback
	}
	return parsed
}

func defaultSubmissionAllowInsecureAuth() bool {
	return !strings.EqualFold(strings.TrimSpace(os.Getenv("GOGOMAIL_ENV")), "production")
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

func floatEnvOrDefault(key string, fallback float64) float64 {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	parsed, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return fallback
	}
	return parsed
}

func durationCSVEnvOrDefault(key string, fallback []time.Duration) []time.Duration {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return append([]time.Duration(nil), fallback...)
	}
	parts := strings.Split(value, ",")
	durations := make([]time.Duration, 0, len(parts))
	for _, part := range parts {
		parsed, err := time.ParseDuration(strings.TrimSpace(part))
		if err != nil {
			return append([]time.Duration(nil), fallback...)
		}
		durations = append(durations, parsed)
	}
	if len(durations) == 0 {
		return append([]time.Duration(nil), fallback...)
	}
	return durations
}
