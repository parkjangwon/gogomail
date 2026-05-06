package config

import (
	"os"
	"strings"
	"testing"
	"time"
)

func TestLoadFileAppliesStorageOverlay(t *testing.T) {
	t.Setenv("GOGOMAIL_STORAGE_BACKEND", "local")
	t.Setenv("GOGOMAIL_STORAGE_S3_ENDPOINT", "http://env-minio:9000")
	t.Setenv("GOGOMAIL_STORAGE_S3_ACCESS_KEY_ID", "env-access")
	t.Setenv("GOGOMAIL_STORAGE_S3_SECRET_ACCESS_KEY", "env-secret")

	path := writeYAMLConfig(t, `
environment: test
storage_backend: s3
storage_backend_compat_labels: local,nfs
storage_s3_endpoint: https://s3.us-east-1.amazonaws.com
storage_s3_region: us-east-1
storage_s3_bucket: gogomail-prod
storage_s3_prefix: mail
storage_s3_access_key_id: file-access
storage_s3_secret_access_key: file-secret
storage_s3_session_token: file-session
storage_s3_force_path_style: true
storage_s3_ca_cert_file: /etc/gogomail/s3-ca.pem
storage_s3_insecure_skip_verify: true
mailstore_root: /srv/gogomail
local_recipients:
  - admin@example.com
  - user@example.com
delivery_farm_concurrency:
  general: 50
  bulk: 10
delivery_domain_concurrency: example.com=5,example.net=3
attachment_scan_timeout: 3s
`)

	cfg, err := LoadFile(path)
	if err != nil {
		t.Fatalf("LoadFile returned error: %v", err)
	}
	if cfg.Environment != "test" || cfg.StorageBackend != "s3" {
		t.Fatalf("basic overlay = env:%q backend:%q", cfg.Environment, cfg.StorageBackend)
	}
	if cfg.StorageS3Endpoint != "https://s3.us-east-1.amazonaws.com" || cfg.StorageS3AccessKeyID != "file-access" || cfg.StorageS3SecretAccessKey != "file-secret" {
		t.Fatalf("S3 overlay endpoint/access/secret = %q/%q/%q", cfg.StorageS3Endpoint, cfg.StorageS3AccessKeyID, cfg.StorageS3SecretAccessKey)
	}
	if !cfg.StorageS3ForcePathStyle || !cfg.StorageS3InsecureSkipVerify || cfg.StorageS3CACertFile != "/etc/gogomail/s3-ca.pem" {
		t.Fatalf("S3 TLS/path overlay = force:%v insecure:%v ca:%q", cfg.StorageS3ForcePathStyle, cfg.StorageS3InsecureSkipVerify, cfg.StorageS3CACertFile)
	}
	if len(cfg.StorageBackendCompatLabels) != 2 || cfg.StorageBackendCompatLabels[0] != "local" || cfg.StorageBackendCompatLabels[1] != "nfs" {
		t.Fatalf("compat labels = %#v", cfg.StorageBackendCompatLabels)
	}
	if len(cfg.LocalRecipients) != 2 || cfg.LocalRecipients[0] != "admin@example.com" || cfg.LocalRecipients[1] != "user@example.com" {
		t.Fatalf("local recipients = %#v", cfg.LocalRecipients)
	}
	if cfg.DeliveryFarmConcurrency["general"] != 50 || cfg.DeliveryFarmConcurrency["bulk"] != 10 {
		t.Fatalf("farm concurrency = %#v", cfg.DeliveryFarmConcurrency)
	}
	if cfg.DeliveryDomainConcurrency["example.com"] != 5 || cfg.DeliveryDomainConcurrency["example.net"] != 3 {
		t.Fatalf("domain concurrency = %#v", cfg.DeliveryDomainConcurrency)
	}
	if cfg.AttachmentScanTimeout != 3*time.Second {
		t.Fatalf("AttachmentScanTimeout = %s, want 3s", cfg.AttachmentScanTimeout)
	}
}

func TestLoadFileRejectsUnsupportedKey(t *testing.T) {
	path := writeYAMLConfig(t, "storage_backend: local\nsurprise: true\n")

	_, err := LoadFile(path)
	if err == nil || !strings.Contains(err.Error(), `unsupported key "surprise"`) {
		t.Fatalf("LoadFile error = %v, want unsupported key rejection", err)
	}
}

func TestLoadFileRejectsInvalidTypes(t *testing.T) {
	tests := []struct {
		name string
		raw  string
	}{
		{name: "string bool", raw: "storage_s3_force_path_style: yes-please\n"},
		{name: "bad duration", raw: "attachment_scan_timeout: soon\n"},
		{name: "bad map", raw: "delivery_domain_concurrency: example.com:not-number\n"},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			path := writeYAMLConfig(t, tt.raw)
			if _, err := LoadFile(path); err == nil {
				t.Fatal("LoadFile error = nil, want invalid config file rejection")
			}
		})
	}
}

func TestLoadFileRejectsMalformedYAML(t *testing.T) {
	path := writeYAMLConfig(t, "storage_backend: [")

	if _, err := LoadFile(path); err == nil || !strings.Contains(err.Error(), "parse config file") {
		t.Fatalf("LoadFile error = %v, want parse failure", err)
	}
}

func TestLoadFileParsesExampleConfig(t *testing.T) {
	cfg, err := LoadFile("../../configs/config.example.yaml")
	if err != nil {
		t.Fatalf("LoadFile(config.example.yaml) returned error: %v", err)
	}
	if cfg.StorageBackend != "local" || cfg.DatabaseURL == "" || cfg.RedisAddr == "" {
		t.Fatalf("example config core fields = backend:%q db:%q redis:%q", cfg.StorageBackend, cfg.DatabaseURL, cfg.RedisAddr)
	}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("example config Validate() error = %v", err)
	}
}

func writeYAMLConfig(t *testing.T, raw string) string {
	t.Helper()
	path := t.TempDir() + "/gogomail.yaml"
	if err := os.WriteFile(path, []byte(strings.TrimLeft(raw, "\n")), 0o600); err != nil {
		t.Fatalf("write config file: %v", err)
	}
	return path
}
