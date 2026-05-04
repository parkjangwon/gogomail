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

func TestValidateRejectsUnsafeExportManifestSignerCredentials(t *testing.T) {
	t.Parallel()

	publicKey := ed25519.NewKeyFromSeed([]byte(strings.Repeat("r", ed25519.SeedSize))).Public().(ed25519.PublicKey)
	for _, tc := range []struct {
		name    string
		mutate  func(*Config)
		wantErr string
	}{
		{
			name: "key_id_line_break",
			mutate: func(cfg *Config) {
				cfg.APIUsageExportManifestSignerBackend = "local-hmac"
				cfg.APIUsageExportManifestSignerKeyID = "key\nbad"
				cfg.APIUsageExportManifestSignerSecret = "secret"
			},
			wantErr: "KEY_ID cannot contain line breaks",
		},
		{
			name: "key_id_too_long",
			mutate: func(cfg *Config) {
				cfg.APIUsageExportManifestSignerBackend = "local-hmac"
				cfg.APIUsageExportManifestSignerKeyID = strings.Repeat("k", maxExportManifestSignerKeyIDBytes+1)
				cfg.APIUsageExportManifestSignerSecret = "secret"
			},
			wantErr: "KEY_ID is too long",
		},
		{
			name: "hmac_secret_too_long",
			mutate: func(cfg *Config) {
				cfg.APIUsageExportManifestSignerBackend = "local-hmac"
				cfg.APIUsageExportManifestSignerKeyID = "key-1"
				cfg.APIUsageExportManifestSignerSecret = strings.Repeat("s", maxExportManifestSignerCredentialBytes+1)
			},
			wantErr: "SECRET is too long",
		},
		{
			name: "remote_token_line_break",
			mutate: func(cfg *Config) {
				cfg.APIUsageExportManifestSignerBackend = "remote-ed25519"
				cfg.APIUsageExportManifestSignerKeyID = "key-1"
				cfg.APIUsageExportSignerURL = "https://signer.example.test/sign"
				cfg.APIUsageExportSignerPublicKey = base64.StdEncoding.EncodeToString(publicKey)
				cfg.APIUsageExportSignerToken = "token\nbad"
			},
			wantErr: "TOKEN cannot contain line breaks",
		},
		{
			name: "remote_token_too_long",
			mutate: func(cfg *Config) {
				cfg.APIUsageExportManifestSignerBackend = "remote-ed25519"
				cfg.APIUsageExportManifestSignerKeyID = "key-1"
				cfg.APIUsageExportSignerURL = "https://signer.example.test/sign"
				cfg.APIUsageExportSignerPublicKey = base64.StdEncoding.EncodeToString(publicKey)
				cfg.APIUsageExportSignerToken = strings.Repeat("t", maxExportManifestSignerCredentialBytes+1)
			},
			wantErr: "TOKEN is too long",
		},
	} {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			cfg := Load()
			tc.mutate(&cfg)
			if err := cfg.Validate(); err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("Validate error = %v, want %q", err, tc.wantErr)
			}
		})
	}
}

func TestValidateRejectsOversizedEd25519ExportManifestSignerKeysBeforeDecode(t *testing.T) {
	t.Parallel()

	privateKey := ed25519.NewKeyFromSeed([]byte(strings.Repeat("s", ed25519.SeedSize)))
	publicKey := privateKey.Public().(ed25519.PublicKey)
	cfg := Load()
	cfg.APIUsageExportManifestSignerBackend = "local-ed25519"
	cfg.APIUsageExportManifestSignerKeyID = "key-1"
	cfg.APIUsageExportSignerPrivateKey = strings.Repeat("a", base64.StdEncoding.EncodedLen(ed25519.PrivateKeySize)+1)
	cfg.APIUsageExportSignerPublicKey = base64.StdEncoding.EncodeToString(publicKey)
	if err := cfg.Validate(); err == nil || !strings.Contains(err.Error(), "PRIVATE_KEY is too long") {
		t.Fatalf("Validate error = %v, want oversized private key", err)
	}

	cfg.APIUsageExportSignerPrivateKey = base64.StdEncoding.EncodeToString(privateKey)
	cfg.APIUsageExportSignerPublicKey = strings.Repeat("a", base64.StdEncoding.EncodedLen(ed25519.PublicKeySize)+1)
	if err := cfg.Validate(); err == nil || !strings.Contains(err.Error(), "PUBLIC_KEY is too long") {
		t.Fatalf("Validate error = %v, want oversized public key", err)
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
