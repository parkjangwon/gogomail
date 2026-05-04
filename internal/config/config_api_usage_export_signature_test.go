package config

import "testing"

func TestLoadAPIUsageExportManifestSignerDefaultsDisabled(t *testing.T) {
	t.Setenv("GOGOMAIL_API_USAGE_EXPORT_MANIFEST_SIGNER_BACKEND", "")
	t.Setenv("GOGOMAIL_API_USAGE_EXPORT_MANIFEST_SIGNER_KEY_ID", "")
	t.Setenv("GOGOMAIL_API_USAGE_EXPORT_MANIFEST_SIGNER_SECRET", "")

	cfg := Load()
	if cfg.APIUsageExportManifestSignerBackend != "disabled" {
		t.Fatalf("APIUsageExportManifestSignerBackend = %q", cfg.APIUsageExportManifestSignerBackend)
	}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate returned error: %v", err)
	}
}

func TestValidateRequiresLocalHMACExportManifestSignerSecrets(t *testing.T) {
	cfg := Load()
	cfg.APIUsageExportManifestSignerBackend = "local-hmac"
	if err := cfg.Validate(); err == nil {
		t.Fatal("Validate returned nil error without local-hmac key")
	}

	cfg.APIUsageExportManifestSignerKeyID = "key-1"
	if err := cfg.Validate(); err == nil {
		t.Fatal("Validate returned nil error without local-hmac secret")
	}

	cfg.APIUsageExportManifestSignerSecret = "secret"
	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate returned error: %v", err)
	}
}
