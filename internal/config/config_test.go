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
	t.Setenv("GOGOMAIL_SMTP_DOMAIN", "")
	t.Setenv("GOGOMAIL_MAILSTORE_ROOT", "")
	t.Setenv("GOGOMAIL_LOCAL_RECIPIENTS", "")

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
}
