package config

import (
	"testing"
	"time"
)

func TestLoadSMTPTimeoutOverrides(t *testing.T) {
	t.Setenv("GOGOMAIL_SMTP_READ_TIMEOUT", "45s")
	t.Setenv("GOGOMAIL_SMTP_WRITE_TIMEOUT", "1m")

	cfg := Load()
	if cfg.SMTPReadTimeout != 45*time.Second {
		t.Fatalf("SMTPReadTimeout = %s, want 45s", cfg.SMTPReadTimeout)
	}
	if cfg.SMTPWriteTimeout != time.Minute {
		t.Fatalf("SMTPWriteTimeout = %s, want 1m", cfg.SMTPWriteTimeout)
	}
}
