package config

import (
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	Environment                  string
	HTTPAddr                     string
	SMTPAddr                     string
	InboundSMTPAddr              string
	InboundTrustedRelays         []string
	SubmissionAddr               string
	SubmissionSMTPSAddr          string
	SubmissionMaxRecipients      int
	SubmissionMaxMessageBytes    int64
	SubmissionAddReceivedHeader  bool
	SubmissionSupportSMTPUTF8    bool
	SubmissionSupportRequireTLS  bool
	SubmissionSupportDSN         bool
	SubmissionSupportBinaryMIME  bool
	SMTPTLSCertFile              string
	SMTPTLSKeyFile               string
	SubmissionAllowInsecureAuth  bool
	DatabaseURL                  string
	RedisAddr                    string
	StorageBackend               string
	MigrationDir                 string
	SMTPDomain                   string
	SMTPReadTimeout              time.Duration
	SMTPWriteTimeout             time.Duration
	SMTPMaxRecipients            int
	SMTPMaxMessageBytes          int64
	SMTPRequireAuth              bool
	SMTPAddReceivedHeader        bool
	SMTPAuthVerificationEnabled  bool
	SMTPAuthservID               string
	SMTPDMARCEnforcement         string
	SMTPMaxDKIMVerifications     int
	SMTPSupportSMTPUTF8          bool
	SMTPSupportRequireTLS        bool
	SMTPSupportDSN               bool
	SMTPSupportBinaryMIME        bool
	MailstoreRoot                string
	LocalRecipients              []string
	DedupBackend                 string
	RateLimitBackend             string
	BackpressureBackend          string
	MetricsBackend               string
	PushNotifyBackend            string
	PushNotifyConsumerGroup      string
	PushNotifyConsumerName       string
	PushNotifyConsumerCount      int
	PushNotifyConsumerBlock      time.Duration
	APIMeteringBackend           string
	APIMeteringTimeout           time.Duration
	APIMeteringAggregateBackend  string
	APIMeteringStream            string
	APIMeteringConsumerGroup     string
	APIMeteringConsumerName      string
	APIMeteringConsumerCount     int
	APIMeteringConsumerBlock     time.Duration
	RcptRateLimitPerMinute       int
	OutboxRelayBatchSize         int
	OutboxRelayPollInterval      time.Duration
	OutboxRelayMaxAttempts       int
	EventStream                  string
	EventConsumerGroup           string
	EventConsumerName            string
	EventConsumerCount           int
	EventConsumerBlock           time.Duration
	SearchIndexBackend           string
	SearchIndexMaxBodyBytes      int64
	SearchIndexConsumerGroup     string
	SearchIndexConsumerName      string
	SearchIndexConsumerCount     int
	SearchIndexConsumerBlock     time.Duration
	DeliveryStream               string
	DeliveryConsumerGroup        string
	DeliveryConsumerName         string
	DeliveryConsumerCount        int
	DeliveryConsumerBlock        time.Duration
	DeliverySMTPHello            string
	DeliveryTimeout              time.Duration
	DeliveryTLSMode              string
	DeliveryRouteBackend         string
	DeliverySmartHost            string
	DeliverySmartHostPort        int
	DeliverySmartHostTLSMode     string
	DeliverySmartHostImplicitTLS bool
	DeliverySmartHostUsername    string
	DeliverySmartHostPassword    string
	DeliverySmartHostIdentity    string
	DeliveryRetryDelays          []time.Duration
	DeliveryRetryJitterRatio     float64
	DeliveryRetryMaxDelay        time.Duration
	DeliveryThrottleEnabled      bool
	DeliveryDefaultConcurrency   int
	DeliveryFarmConcurrency      map[string]int
	DeliveryDomainConcurrency    map[string]int
	DSNPostmaster                string
	DKIMEnabled                  bool
	AdminToken                   string
	AuthJWTSecret                string
}

func Load() Config {
	return Config{
		Environment:                  envOrDefault("GOGOMAIL_ENV", "development"),
		HTTPAddr:                     envOrDefault("GOGOMAIL_HTTP_ADDR", ":8080"),
		SMTPAddr:                     envOrDefault("GOGOMAIL_SMTP_ADDR", ":2525"),
		InboundSMTPAddr:              envOrDefault("GOGOMAIL_INBOUND_SMTP_ADDR", ":2526"),
		InboundTrustedRelays:         splitCSV(envOrDefault("GOGOMAIL_INBOUND_TRUSTED_RELAYS", "127.0.0.1/32,::1/128")),
		SubmissionAddr:               envOrDefault("GOGOMAIL_SUBMISSION_ADDR", ":2587"),
		SubmissionSMTPSAddr:          envOrDefault("GOGOMAIL_SUBMISSION_SMTPS_ADDR", ""),
		SubmissionMaxRecipients:      intEnvOrDefault("GOGOMAIL_SUBMISSION_MAX_RECIPIENTS", 100),
		SubmissionMaxMessageBytes:    int64EnvOrDefault("GOGOMAIL_SUBMISSION_MAX_MESSAGE_BYTES", 25*1024*1024),
		SubmissionAddReceivedHeader:  boolEnvOrDefault("GOGOMAIL_SUBMISSION_ADD_RECEIVED_HEADER", true),
		SubmissionSupportSMTPUTF8:    boolEnvOrDefault("GOGOMAIL_SUBMISSION_SUPPORT_SMTPUTF8", false),
		SubmissionSupportRequireTLS:  boolEnvOrDefault("GOGOMAIL_SUBMISSION_SUPPORT_REQUIRETLS", false),
		SubmissionSupportDSN:         boolEnvOrDefault("GOGOMAIL_SUBMISSION_SUPPORT_DSN", false),
		SubmissionSupportBinaryMIME:  boolEnvOrDefault("GOGOMAIL_SUBMISSION_SUPPORT_BINARYMIME", false),
		SMTPTLSCertFile:              envOrDefault("GOGOMAIL_SMTP_TLS_CERT_FILE", ""),
		SMTPTLSKeyFile:               envOrDefault("GOGOMAIL_SMTP_TLS_KEY_FILE", ""),
		SubmissionAllowInsecureAuth:  boolEnvOrDefault("GOGOMAIL_SUBMISSION_ALLOW_INSECURE_AUTH", defaultSubmissionAllowInsecureAuth()),
		DatabaseURL:                  envOrDefault("GOGOMAIL_DATABASE_URL", "postgres://gogomail:gogomail@localhost:5432/gogomail?sslmode=disable"),
		RedisAddr:                    envOrDefault("GOGOMAIL_REDIS_ADDR", "localhost:6379"),
		StorageBackend:               envOrDefault("GOGOMAIL_STORAGE_BACKEND", "local"),
		MigrationDir:                 envOrDefault("GOGOMAIL_MIGRATION_DIR", "migrations"),
		SMTPDomain:                   envOrDefault("GOGOMAIL_SMTP_DOMAIN", "localhost"),
		SMTPReadTimeout:              durationEnvOrDefault("GOGOMAIL_SMTP_READ_TIMEOUT", 30*time.Second),
		SMTPWriteTimeout:             durationEnvOrDefault("GOGOMAIL_SMTP_WRITE_TIMEOUT", 30*time.Second),
		SMTPMaxRecipients:            intEnvOrDefault("GOGOMAIL_SMTP_MAX_RECIPIENTS", 100),
		SMTPMaxMessageBytes:          int64EnvOrDefault("GOGOMAIL_SMTP_MAX_MESSAGE_BYTES", 25*1024*1024),
		SMTPRequireAuth:              boolEnvOrDefault("GOGOMAIL_SMTP_REQUIRE_AUTH", false),
		SMTPAddReceivedHeader:        boolEnvOrDefault("GOGOMAIL_SMTP_ADD_RECEIVED_HEADER", true),
		SMTPAuthVerificationEnabled:  boolEnvOrDefault("GOGOMAIL_SMTP_AUTH_VERIFICATION_ENABLED", false),
		SMTPAuthservID:               envOrDefault("GOGOMAIL_SMTP_AUTHSERV_ID", envOrDefault("GOGOMAIL_SMTP_DOMAIN", "localhost")),
		SMTPDMARCEnforcement:         envOrDefault("GOGOMAIL_SMTP_DMARC_ENFORCEMENT", "monitor"),
		SMTPMaxDKIMVerifications:     intEnvOrDefault("GOGOMAIL_SMTP_MAX_DKIM_VERIFICATIONS", 8),
		SMTPSupportSMTPUTF8:          boolEnvOrDefault("GOGOMAIL_SMTP_SUPPORT_SMTPUTF8", false),
		SMTPSupportRequireTLS:        boolEnvOrDefault("GOGOMAIL_SMTP_SUPPORT_REQUIRETLS", false),
		SMTPSupportDSN:               boolEnvOrDefault("GOGOMAIL_SMTP_SUPPORT_DSN", false),
		SMTPSupportBinaryMIME:        boolEnvOrDefault("GOGOMAIL_SMTP_SUPPORT_BINARYMIME", false),
		MailstoreRoot:                envOrDefault("GOGOMAIL_MAILSTORE_ROOT", "var/mailstore"),
		LocalRecipients:              splitCSV(os.Getenv("GOGOMAIL_LOCAL_RECIPIENTS")),
		DedupBackend:                 envOrDefault("GOGOMAIL_DEDUP_BACKEND", "none"),
		RateLimitBackend:             envOrDefault("GOGOMAIL_RATELIMIT_BACKEND", "none"),
		BackpressureBackend:          envOrDefault("GOGOMAIL_BACKPRESSURE_BACKEND", "none"),
		MetricsBackend:               envOrDefault("GOGOMAIL_METRICS_BACKEND", "none"),
		PushNotifyBackend:            envOrDefault("GOGOMAIL_PUSH_NOTIFICATION_BACKEND", "none"),
		PushNotifyConsumerGroup:      envOrDefault("GOGOMAIL_PUSH_NOTIFICATION_CONSUMER_GROUP", "gogomail.push-notification-worker"),
		PushNotifyConsumerName:       envOrDefault("GOGOMAIL_PUSH_NOTIFICATION_CONSUMER_NAME", "push-notification-worker-1"),
		PushNotifyConsumerCount:      intEnvOrDefault("GOGOMAIL_PUSH_NOTIFICATION_CONSUMER_COUNT", 50),
		PushNotifyConsumerBlock:      durationEnvOrDefault("GOGOMAIL_PUSH_NOTIFICATION_CONSUMER_BLOCK", time.Second),
		APIMeteringBackend:           envOrDefault("GOGOMAIL_API_METERING_BACKEND", "none"),
		APIMeteringTimeout:           durationEnvOrDefault("GOGOMAIL_API_METERING_TIMEOUT", 100*time.Millisecond),
		APIMeteringAggregateBackend:  envOrDefault("GOGOMAIL_API_METERING_AGGREGATE_BACKEND", "disabled"),
		APIMeteringStream:            envOrDefault("GOGOMAIL_API_METERING_STREAM", "api.event"),
		APIMeteringConsumerGroup:     envOrDefault("GOGOMAIL_API_METERING_CONSUMER_GROUP", "gogomail.api-metering-worker"),
		APIMeteringConsumerName:      envOrDefault("GOGOMAIL_API_METERING_CONSUMER_NAME", "api-metering-worker-1"),
		APIMeteringConsumerCount:     intEnvOrDefault("GOGOMAIL_API_METERING_CONSUMER_COUNT", 100),
		APIMeteringConsumerBlock:     durationEnvOrDefault("GOGOMAIL_API_METERING_CONSUMER_BLOCK", time.Second),
		RcptRateLimitPerMinute:       intEnvOrDefault("GOGOMAIL_RCPT_RATE_LIMIT_PER_MINUTE", 60),
		OutboxRelayBatchSize:         intEnvOrDefault("GOGOMAIL_OUTBOX_RELAY_BATCH_SIZE", 100),
		OutboxRelayPollInterval:      durationEnvOrDefault("GOGOMAIL_OUTBOX_RELAY_POLL_INTERVAL", time.Second),
		OutboxRelayMaxAttempts:       intEnvOrDefault("GOGOMAIL_OUTBOX_RELAY_MAX_ATTEMPTS", 10),
		EventStream:                  envOrDefault("GOGOMAIL_EVENT_STREAM", "mail.event"),
		EventConsumerGroup:           envOrDefault("GOGOMAIL_EVENT_CONSUMER_GROUP", "gogomail.event-worker"),
		EventConsumerName:            envOrDefault("GOGOMAIL_EVENT_CONSUMER_NAME", "event-worker-1"),
		EventConsumerCount:           intEnvOrDefault("GOGOMAIL_EVENT_CONSUMER_COUNT", 100),
		EventConsumerBlock:           durationEnvOrDefault("GOGOMAIL_EVENT_CONSUMER_BLOCK", time.Second),
		SearchIndexBackend:           envOrDefault("GOGOMAIL_SEARCH_INDEX_BACKEND", "disabled"),
		SearchIndexMaxBodyBytes:      int64EnvOrDefault("GOGOMAIL_SEARCH_INDEX_MAX_BODY_BYTES", 1024*1024),
		SearchIndexConsumerGroup:     envOrDefault("GOGOMAIL_SEARCH_INDEX_CONSUMER_GROUP", "gogomail.search-index-worker"),
		SearchIndexConsumerName:      envOrDefault("GOGOMAIL_SEARCH_INDEX_CONSUMER_NAME", "search-index-worker-1"),
		SearchIndexConsumerCount:     intEnvOrDefault("GOGOMAIL_SEARCH_INDEX_CONSUMER_COUNT", 50),
		SearchIndexConsumerBlock:     durationEnvOrDefault("GOGOMAIL_SEARCH_INDEX_CONSUMER_BLOCK", time.Second),
		DeliveryStream:               envOrDefault("GOGOMAIL_DELIVERY_STREAM", "mail.outbound.general"),
		DeliveryConsumerGroup:        envOrDefault("GOGOMAIL_DELIVERY_CONSUMER_GROUP", "gogomail.delivery-worker"),
		DeliveryConsumerName:         envOrDefault("GOGOMAIL_DELIVERY_CONSUMER_NAME", "delivery-worker-1"),
		DeliveryConsumerCount:        intEnvOrDefault("GOGOMAIL_DELIVERY_CONSUMER_COUNT", 50),
		DeliveryConsumerBlock:        durationEnvOrDefault("GOGOMAIL_DELIVERY_CONSUMER_BLOCK", time.Second),
		DeliverySMTPHello:            envOrDefault("GOGOMAIL_DELIVERY_SMTP_HELLO", "localhost"),
		DeliveryTimeout:              durationEnvOrDefault("GOGOMAIL_DELIVERY_TIMEOUT", 30*time.Second),
		DeliveryTLSMode:              envOrDefault("GOGOMAIL_DELIVERY_TLS_MODE", "opportunistic"),
		DeliveryRouteBackend:         envOrDefault("GOGOMAIL_DELIVERY_ROUTE_BACKEND", "env"),
		DeliverySmartHost:            envOrDefault("GOGOMAIL_DELIVERY_SMARTHOST", ""),
		DeliverySmartHostPort:        intEnvOrDefault("GOGOMAIL_DELIVERY_SMARTHOST_PORT", 0),
		DeliverySmartHostTLSMode:     envOrDefault("GOGOMAIL_DELIVERY_SMARTHOST_TLS_MODE", ""),
		DeliverySmartHostImplicitTLS: boolEnvOrDefault("GOGOMAIL_DELIVERY_SMARTHOST_IMPLICIT_TLS", false),
		DeliverySmartHostUsername:    envOrDefault("GOGOMAIL_DELIVERY_SMARTHOST_USERNAME", ""),
		DeliverySmartHostPassword:    envOrDefault("GOGOMAIL_DELIVERY_SMARTHOST_PASSWORD", ""),
		DeliverySmartHostIdentity:    envOrDefault("GOGOMAIL_DELIVERY_SMARTHOST_IDENTITY", ""),
		DeliveryRetryDelays:          durationCSVEnvOrDefault("GOGOMAIL_DELIVERY_RETRY_DELAYS", []time.Duration{5 * time.Minute, 30 * time.Minute, 2 * time.Hour, 8 * time.Hour, 24 * time.Hour}),
		DeliveryRetryJitterRatio:     floatEnvOrDefault("GOGOMAIL_DELIVERY_RETRY_JITTER_RATIO", 0.20),
		DeliveryRetryMaxDelay:        durationEnvOrDefault("GOGOMAIL_DELIVERY_RETRY_MAX_DELAY", 24*time.Hour),
		DeliveryThrottleEnabled:      boolEnvOrDefault("GOGOMAIL_DELIVERY_THROTTLE_ENABLED", false),
		DeliveryDefaultConcurrency:   intEnvOrDefault("GOGOMAIL_DELIVERY_DEFAULT_CONCURRENCY", 0),
		DeliveryFarmConcurrency:      intMapEnvOrDefault("GOGOMAIL_DELIVERY_FARM_CONCURRENCY", nil),
		DeliveryDomainConcurrency:    intMapEnvOrDefault("GOGOMAIL_DELIVERY_DOMAIN_CONCURRENCY", nil),
		DSNPostmaster:                envOrDefault("GOGOMAIL_DSN_POSTMASTER", ""),
		DKIMEnabled:                  boolEnvOrDefault("GOGOMAIL_DKIM_ENABLED", false),
		AdminToken:                   envOrDefault("GOGOMAIL_ADMIN_TOKEN", ""),
		AuthJWTSecret:                envOrDefault("GOGOMAIL_AUTH_JWT_SECRET", ""),
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

func intMapEnvOrDefault(key string, fallback map[string]int) map[string]int {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return copyStringIntMap(fallback)
	}
	result := make(map[string]int)
	for _, part := range strings.Split(value, ",") {
		name, rawLimit, ok := strings.Cut(part, "=")
		if !ok {
			return copyStringIntMap(fallback)
		}
		name = strings.ToLower(strings.TrimSpace(name))
		limit, err := strconv.Atoi(strings.TrimSpace(rawLimit))
		if name == "" || err != nil || limit <= 0 {
			return copyStringIntMap(fallback)
		}
		result[name] = limit
	}
	return result
}

func copyStringIntMap(in map[string]int) map[string]int {
	if in == nil {
		return nil
	}
	out := make(map[string]int, len(in))
	for key, value := range in {
		out[key] = value
	}
	return out
}
