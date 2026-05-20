package main

import (
	"bytes"
	"context"
	"log/slog"
	"os"
	"slices"
	"strings"
	"testing"

	"github.com/gogomail/gogomail/internal/app"
	"github.com/gogomail/gogomail/internal/config"
)

func TestRunPassesYAMLConfigToApp(t *testing.T) {
	configFile := writeCommandConfig(t, `
environment: test
submission_allow_insecure_auth: false
imap_allow_insecure_auth: false
caldav_allow_insecure_auth: false
carddav_allow_insecure_auth: false
storage_backend: minio
storage_s3_endpoint: http://localhost:19000
storage_s3_region: us-east-1
storage_s3_bucket: gogomail
storage_s3_access_key_id: access
storage_s3_secret_access_key: secret
`)

	var gotMode app.Mode
	var gotConfig config.Config
	exitCode := run([]string{"--config", configFile, "--mode", "batch-worker"}, &bytes.Buffer{}, &bytes.Buffer{}, func(ctx context.Context, mode app.Mode, cfg config.Config, logger *slog.Logger) error {
		gotMode = mode
		gotConfig = cfg
		return nil
	})
	if exitCode != 0 {
		t.Fatalf("run exit code = %d, want 0", exitCode)
	}
	if gotMode != app.ModeBatchWorker {
		t.Fatalf("mode = %q, want batch-worker", gotMode)
	}
	if gotConfig.StorageBackend != "minio" || gotConfig.StorageS3Endpoint != "http://localhost:19000" || gotConfig.StorageS3Bucket != "gogomail" {
		t.Fatalf("storage config = backend:%q endpoint:%q bucket:%q", gotConfig.StorageBackend, gotConfig.StorageS3Endpoint, gotConfig.StorageS3Bucket)
	}
}

func TestRunAcceptsStorageProfileConfigs(t *testing.T) {
	tests := []struct {
		path        string
		backend     string
		endpoint    string
		region      string
		bucket      string
		prefix      string
		accessKeyID string
		secretKey   string
		root        string
		compat      []string
		environment string
	}{
		{path: "configs/storage.local.yaml", backend: "local"},
		{path: "configs/storage.nfs.yaml", backend: "nfs", root: "/mnt/gogomail-storage", compat: []string{"local"}},
		{path: "configs/storage.minio.yaml", backend: "minio", endpoint: "http://localhost:19000", region: "us-east-1", bucket: "gogomail", accessKeyID: "gogomail", secretKey: "gogomail123"},
		{path: "configs/storage.s3.yaml", backend: "s3", endpoint: "https://s3.us-east-1.amazonaws.com", region: "us-east-1", bucket: "gogomail-prod", prefix: "mail", accessKeyID: "CHANGE_ME_ACCESS_KEY_ID", secretKey: "CHANGE_ME_SECRET_ACCESS_KEY", environment: "production"},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.backend, func(t *testing.T) {
			var gotConfig config.Config
			exitCode := run([]string{"--config", "../../" + tt.path, "--mode", "batch-worker"}, &bytes.Buffer{}, &bytes.Buffer{}, func(ctx context.Context, mode app.Mode, cfg config.Config, logger *slog.Logger) error {
				if mode != app.ModeBatchWorker {
					t.Fatalf("mode = %q, want batch-worker", mode)
				}
				gotConfig = cfg
				return nil
			})
			if exitCode != 0 {
				t.Fatalf("run exit code = %d, want 0", exitCode)
			}
			if gotConfig.StorageBackend != tt.backend {
				t.Fatalf("StorageBackend = %q, want %q", gotConfig.StorageBackend, tt.backend)
			}
			if gotConfig.StorageS3Endpoint != tt.endpoint {
				t.Fatalf("StorageS3Endpoint = %q, want %q", gotConfig.StorageS3Endpoint, tt.endpoint)
			}
			if tt.region != "" && gotConfig.StorageS3Region != tt.region {
				t.Fatalf("StorageS3Region = %q, want %q", gotConfig.StorageS3Region, tt.region)
			}
			if gotConfig.StorageS3Bucket != tt.bucket {
				t.Fatalf("StorageS3Bucket = %q, want %q", gotConfig.StorageS3Bucket, tt.bucket)
			}
			if gotConfig.StorageS3Prefix != tt.prefix {
				t.Fatalf("StorageS3Prefix = %q, want %q", gotConfig.StorageS3Prefix, tt.prefix)
			}
			if gotConfig.StorageS3AccessKeyID != tt.accessKeyID {
				t.Fatalf("StorageS3AccessKeyID = %q, want %q", gotConfig.StorageS3AccessKeyID, tt.accessKeyID)
			}
			if gotConfig.StorageS3SecretAccessKey != tt.secretKey {
				t.Fatalf("StorageS3SecretAccessKey = %q, want %q", gotConfig.StorageS3SecretAccessKey, tt.secretKey)
			}
			if tt.root != "" && gotConfig.MailstoreRoot != tt.root {
				t.Fatalf("MailstoreRoot = %q, want %q", gotConfig.MailstoreRoot, tt.root)
			}
			if !slices.Equal(gotConfig.StorageBackendCompatLabels, tt.compat) {
				t.Fatalf("StorageBackendCompatLabels = %#v, want %#v", gotConfig.StorageBackendCompatLabels, tt.compat)
			}
			if tt.environment != "" && gotConfig.Environment != tt.environment {
				t.Fatalf("Environment = %q, want %q", gotConfig.Environment, tt.environment)
			}
		})
	}
}

func TestRunRejectsInvalidYAMLConfigBeforeAppStart(t *testing.T) {
	configFile := writeCommandConfig(t, "storage_backend: minio\n")
	var stderr bytes.Buffer
	called := false

	exitCode := run([]string{"--config", configFile, "--mode", "batch-worker"}, &bytes.Buffer{}, &stderr, func(ctx context.Context, mode app.Mode, cfg config.Config, logger *slog.Logger) error {
		called = true
		return nil
	})
	if exitCode != 2 {
		t.Fatalf("run exit code = %d, want 2", exitCode)
	}
	if called {
		t.Fatal("runApp was called after invalid config")
	}
	if !strings.Contains(stderr.String(), "GOGOMAIL_STORAGE_S3_ENDPOINT is required") {
		t.Fatalf("stderr = %q, want config validation error", stderr.String())
	}
}

func TestRunRejectsUnknownModeBeforeAppStart(t *testing.T) {
	var stderr bytes.Buffer
	called := false

	exitCode := run([]string{"--mode", "warp-drive"}, &bytes.Buffer{}, &stderr, func(ctx context.Context, mode app.Mode, cfg config.Config, logger *slog.Logger) error {
		called = true
		return nil
	})
	if exitCode != 2 {
		t.Fatalf("run exit code = %d, want 2", exitCode)
	}
	if called {
		t.Fatal("runApp was called after invalid mode")
	}
	if !strings.Contains(stderr.String(), "unknown mode") {
		t.Fatalf("stderr = %q, want mode parse error", stderr.String())
	}
}

func TestRunUsesAppModeEnvWhenModeFlagUnset(t *testing.T) {
	t.Setenv("APP_MODE", "outbox-relay")

	var gotMode app.Mode
	exitCode := run(nil, &bytes.Buffer{}, &bytes.Buffer{}, func(ctx context.Context, mode app.Mode, cfg config.Config, logger *slog.Logger) error {
		gotMode = mode
		return nil
	})
	if exitCode != 0 {
		t.Fatalf("run exit code = %d, want 0", exitCode)
	}
	if gotMode != app.ModeOutboxRelay {
		t.Fatalf("mode = %q, want %q", gotMode, app.ModeOutboxRelay)
	}
}

func TestRunModeFlagOverridesAppModeEnv(t *testing.T) {
	t.Setenv("APP_MODE", "outbox-relay")

	var gotMode app.Mode
	exitCode := run([]string{"--mode", "mail-api"}, &bytes.Buffer{}, &bytes.Buffer{}, func(ctx context.Context, mode app.Mode, cfg config.Config, logger *slog.Logger) error {
		gotMode = mode
		return nil
	})
	if exitCode != 0 {
		t.Fatalf("run exit code = %d, want 0", exitCode)
	}
	if gotMode != app.ModeMailAPI {
		t.Fatalf("mode = %q, want %q", gotMode, app.ModeMailAPI)
	}
}

func writeCommandConfig(t *testing.T, raw string) string {
	t.Helper()
	path := t.TempDir() + "/gogomail.yaml"
	if err := os.WriteFile(path, []byte(strings.TrimLeft(raw, "\n")), 0o600); err != nil {
		t.Fatalf("write config file: %v", err)
	}
	return path
}
