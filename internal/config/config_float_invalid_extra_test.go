package config

import "testing"

func TestFloatEnvOrDefaultFallsBackForInvalidValue(t *testing.T) {
	t.Setenv("GOGOMAIL_TEST_FLOAT", "nan?")
	if got := floatEnvOrDefault("GOGOMAIL_TEST_FLOAT", 0.25); got != 0.25 {
		t.Fatalf("floatEnvOrDefault = %v, want fallback", got)
	}
}
