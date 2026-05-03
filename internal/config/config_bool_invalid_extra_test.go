package config

import "testing"

func TestBoolEnvOrDefaultFallsBackForInvalidValue(t *testing.T) {
	t.Setenv("GOGOMAIL_TEST_BOOL", "definitely")
	if got := boolEnvOrDefault("GOGOMAIL_TEST_BOOL", true); !got {
		t.Fatal("boolEnvOrDefault did not use fallback for invalid bool")
	}
}
