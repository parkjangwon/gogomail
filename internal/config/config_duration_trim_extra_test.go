package config

import (
	"testing"
	"time"
)

func TestDurationEnvOrDefaultTrimsBeforeParsing(t *testing.T) {
	t.Setenv("GOGOMAIL_TEST_DURATION", " 2s ")
	if got := durationEnvOrDefault("GOGOMAIL_TEST_DURATION", time.Minute); got != 2*time.Second {
		t.Fatalf("durationEnvOrDefault = %s, want 2s", got)
	}
}
