package main

import (
	"bytes"
	"context"
	"log/slog"
	"os"
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
		environment string
	}{
		{path: "configs/storage.local.yaml", backend: "local"},
		{path: "configs/storage.nfs.yaml", backend: "nfs"},
		{path: "configs/storage.minio.yaml", backend: "minio", endpoint: "http://localhost:19000"},
		{path: "configs/storage.s3.yaml", backend: "s3", endpoint: "https://s3.us-east-1.amazonaws.com", environment: "production"},
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

func writeCommandConfig(t *testing.T, raw string) string {
	t.Helper()
	path := t.TempDir() + "/gogomail.yaml"
	if err := os.WriteFile(path, []byte(strings.TrimLeft(raw, "\n")), 0o600); err != nil {
		t.Fatalf("write config file: %v", err)
	}
	return path
}
