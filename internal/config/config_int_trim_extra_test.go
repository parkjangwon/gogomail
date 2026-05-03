package config

import "testing"

func TestIntEnvOrDefaultTrimsBeforeParsing(t *testing.T) {
	t.Setenv("GOGOMAIL_TEST_INT", " 42 ")
	if got := intEnvOrDefault("GOGOMAIL_TEST_INT", 7); got != 42 {
		t.Fatalf("intEnvOrDefault = %d, want 42", got)
	}
}
