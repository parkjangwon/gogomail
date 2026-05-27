package config

import "testing"

func TestValidateRejectsUnknownAPIMeteringBackend(t *testing.T) {
	t.Setenv("GOGOMAIL_ENV", "development")
	cfg := Load()
	cfg.APIMeteringBackend = "database"

	if err := cfg.Validate(); err == nil {
		t.Fatal("Validate accepted unknown API metering backend")
	}
}

func TestValidateAcceptsOutboxAPIMeteringBackend(t *testing.T) {
	t.Setenv("GOGOMAIL_ENV", "development")
	cfg := Load()
	cfg.APIMeteringBackend = "outbox"

	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate rejected outbox API metering backend: %v", err)
	}
}

func TestValidateRejectsNonpositiveAPIMeteringTimeout(t *testing.T) {
	t.Setenv("GOGOMAIL_ENV", "development")
	cfg := Load()
	cfg.APIMeteringTimeout = 0

	if err := cfg.Validate(); err == nil {
		t.Fatal("Validate accepted nonpositive API metering timeout")
	}
}
