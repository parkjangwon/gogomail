package config

import "os"

type Config struct {
	Environment    string
	HTTPAddr       string
	SMTPAddr       string
	DatabaseURL    string
	RedisAddr      string
	StorageBackend string
}

func Load() Config {
	return Config{
		Environment:    envOrDefault("GOGOMAIL_ENV", "development"),
		HTTPAddr:       envOrDefault("GOGOMAIL_HTTP_ADDR", ":8080"),
		SMTPAddr:       envOrDefault("GOGOMAIL_SMTP_ADDR", ":2525"),
		DatabaseURL:    envOrDefault("GOGOMAIL_DATABASE_URL", "postgres://gogomail:gogomail@localhost:5432/gogomail?sslmode=disable"),
		RedisAddr:      envOrDefault("GOGOMAIL_REDIS_ADDR", "localhost:6379"),
		StorageBackend: envOrDefault("GOGOMAIL_STORAGE_BACKEND", "local"),
	}
}

func envOrDefault(key string, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
