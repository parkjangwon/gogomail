package config

import "testing"

func TestLoadAppliesDefaults(t *testing.T) {
	t.Setenv("GOGOMAIL_ENV", "")
	t.Setenv("GOGOMAIL_HTTP_ADDR", "")
	t.Setenv("GOGOMAIL_SMTP_ADDR", "")
	t.Setenv("GOGOMAIL_DATABASE_URL", "")
	t.Setenv("GOGOMAIL_REDIS_ADDR", "")
	t.Setenv("GOGOMAIL_STORAGE_BACKEND", "")
	t.Setenv("GOGOMAIL_MIGRATION_DIR", "")

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
}

func TestLoadReadsEnvironmentOverrides(t *testing.T) {
	t.Setenv("GOGOMAIL_ENV", "test")
	t.Setenv("GOGOMAIL_HTTP_ADDR", ":18080")
	t.Setenv("GOGOMAIL_SMTP_ADDR", ":10025")
	t.Setenv("GOGOMAIL_DATABASE_URL", "postgres://example")
	t.Setenv("GOGOMAIL_REDIS_ADDR", "redis:6379")
	t.Setenv("GOGOMAIL_STORAGE_BACKEND", "minio")
	t.Setenv("GOGOMAIL_MIGRATION_DIR", "db/migrations")

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
}
