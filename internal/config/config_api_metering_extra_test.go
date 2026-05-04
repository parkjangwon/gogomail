package config

import "testing"

func TestValidateRejectsUnknownAPIMeteringBackend(t *testing.T) {
	cfg := Load()
	cfg.APIMeteringBackend = "database"

	if err := cfg.Validate(); err == nil {
		t.Fatal("Validate accepted unknown API metering backend")
	}
}

func TestValidateRejectsNonpositiveAPIMeteringTimeout(t *testing.T) {
	cfg := Load()
	cfg.APIMeteringTimeout = 0

	if err := cfg.Validate(); err == nil {
		t.Fatal("Validate accepted nonpositive API metering timeout")
	}
}
