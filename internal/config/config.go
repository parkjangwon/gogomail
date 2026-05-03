package config

import (
	"os"
	"strings"
)

type Config struct {
	Environment     string
	HTTPAddr        string
	SMTPAddr        string
	DatabaseURL     string
	RedisAddr       string
	StorageBackend  string
	MigrationDir    string
	SMTPDomain      string
	MailstoreRoot   string
	LocalRecipients []string
}

func Load() Config {
	return Config{
		Environment:     envOrDefault("GOGOMAIL_ENV", "development"),
		HTTPAddr:        envOrDefault("GOGOMAIL_HTTP_ADDR", ":8080"),
		SMTPAddr:        envOrDefault("GOGOMAIL_SMTP_ADDR", ":2525"),
		DatabaseURL:     envOrDefault("GOGOMAIL_DATABASE_URL", "postgres://gogomail:gogomail@localhost:5432/gogomail?sslmode=disable"),
		RedisAddr:       envOrDefault("GOGOMAIL_REDIS_ADDR", "localhost:6379"),
		StorageBackend:  envOrDefault("GOGOMAIL_STORAGE_BACKEND", "local"),
		MigrationDir:    envOrDefault("GOGOMAIL_MIGRATION_DIR", "migrations"),
		SMTPDomain:      envOrDefault("GOGOMAIL_SMTP_DOMAIN", "localhost"),
		MailstoreRoot:   envOrDefault("GOGOMAIL_MAILSTORE_ROOT", "var/mailstore"),
		LocalRecipients: splitCSV(os.Getenv("GOGOMAIL_LOCAL_RECIPIENTS")),
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
