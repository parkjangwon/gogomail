package config

import (
	"crypto/ed25519"
	"encoding/base64"
	"strings"
	"testing"
)

func TestLoadAPIUsageExportManifestSignerDefaultsDisabled(t *testing.T) {
	t.Setenv("GOGOMAIL_API_USAGE_EXPORT_MANIFEST_SIGNER_BACKEND", "")
	t.Setenv("GOGOMAIL_API_USAGE_EXPORT_MANIFEST_SIGNER_KEY_ID", "")
	t.Setenv("GOGOMAIL_API_USAGE_EXPORT_MANIFEST_SIGNER_SECRET", "")
	t.Setenv("GOGOMAIL_API_USAGE_EXPORT_MANIFEST_SIGNER_PRIVATE_KEY", "")
	t.Setenv("GOGOMAIL_API_USAGE_EXPORT_MANIFEST_SIGNER_PUBLIC_KEY", "")

	cfg := Load()
	if cfg.APIUsageExportManifestSignerBackend != "disabled" {
		t.Fatalf("APIUsageExportManifestSignerBackend = %q", cfg.APIUsageExportManifestSignerBackend)
	}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate returned error: %v", err)
	}
}

func TestValidateRequiresLocalEd25519ExportManifestSignerKeys(t *testing.T) {
	privateKey := ed25519.NewKeyFromSeed([]byte(strings.Repeat("s", ed25519.SeedSize)))
	publicKey := privateKey.Public().(ed25519.PublicKey)
	cfg := Load()
	cfg.APIUsageExportManifestSignerBackend = "local-ed25519"
	if err := cfg.Validate(); err == nil {
		t.Fatal("Validate returned nil error without local-ed25519 key id")
	}

	cfg.APIUsageExportManifestSignerKeyID = "key-1"
	if err := cfg.Validate(); err == nil {
		t.Fatal("Validate returned nil error without local-ed25519 private key")
	}

	cfg.APIUsageExportSignerPrivateKey = base64.StdEncoding.EncodeToString(privateKey)
	if err := cfg.Validate(); err == nil {
		t.Fatal("Validate returned nil error without local-ed25519 public key")
	}

	cfg.APIUsageExportSignerPublicKey = base64.StdEncoding.EncodeToString([]byte(strings.Repeat("p", ed25519.PublicKeySize)))
	if err := cfg.Validate(); err == nil {
		t.Fatal("Validate returned nil error with mismatched local-ed25519 public key")
	}

	cfg.APIUsageExportSignerPublicKey = base64.StdEncoding.EncodeToString(publicKey)
	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate returned error: %v", err)
	}
}

func TestValidateRequiresRemoteEd25519ExportManifestSignerConfig(t *testing.T) {
	publicKey := ed25519.NewKeyFromSeed([]byte(strings.Repeat("r", ed25519.SeedSize))).Public().(ed25519.PublicKey)
	cfg := Load()
	cfg.APIUsageExportManifestSignerBackend = "remote-ed25519"
	if err := cfg.Validate(); err == nil {
		t.Fatal("Validate returned nil error without remote-ed25519 key id")
	}

	cfg.APIUsageExportManifestSignerKeyID = "key-1"
	if err := cfg.Validate(); err == nil {
		t.Fatal("Validate returned nil error without remote-ed25519 URL")
	}

	cfg.APIUsageExportSignerURL = "http://signer.example.test/sign"
	cfg.APIUsageExportSignerPublicKey = base64.StdEncoding.EncodeToString(publicKey)
	if err := cfg.Validate(); err == nil {
		t.Fatal("Validate returned nil error with non-https remote-ed25519 URL")
	}

	cfg.APIUsageExportSignerURL = "https://signer.example.test/sign"
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
