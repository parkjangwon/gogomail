package config

import "testing"

func TestLoadReadsDSNPostmaster(t *testing.T) {
	t.Setenv("GOGOMAIL_DSN_POSTMASTER", " bounces@example.com ")

	cfg := Load()
	if cfg.DSNPostmaster != " bounces@example.com " {
		t.Fatalf("DSNPostmaster = %q, want raw configured value", cfg.DSNPostmaster)
	}
}
