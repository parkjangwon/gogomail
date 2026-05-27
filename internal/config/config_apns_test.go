package config

import (
	"os"
	"strings"
	"testing"
)

func TestLoadAPNsPrivateKeyFromEnvironmentVariable(t *testing.T) {
	setDevelopmentMode(t)
	privateKey := "-----BEGIN PRIVATE KEY-----\nMIGHAgEAMBMGByqGSM49AgEGCCqGSM49AwEHBG0wawIBAQQg\n-----END PRIVATE KEY-----"
	t.Setenv("GOGOMAIL_APNS_PRIVATE_KEY", privateKey)
	t.Setenv("GOGOMAIL_APNS_PRIVATE_KEY_FILE", "")

	cfg := Load()
	if cfg.APNsPrivateKey != privateKey {
		t.Fatalf("APNsPrivateKey = %q, want %q", cfg.APNsPrivateKey, privateKey)
	}
}

func TestLoadAPNsPrivateKeyFromFile(t *testing.T) {
	setDevelopmentMode(t)
	privateKey := "-----BEGIN PRIVATE KEY-----\nMIGHAgEAMBMGByqGSM49AgEGCCqGSM49AwEHBG0wawIBAQQg\n-----END PRIVATE KEY-----"

	// Create a temporary file with the private key
	tmpFile, err := os.CreateTemp("", "apns-key-*.txt")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	if _, err := tmpFile.WriteString(privateKey); err != nil {
		t.Fatalf("failed to write to temp file: %v", err)
	}
	tmpFile.Close()

	// Set the file path environment variable and unset the inline key
	t.Setenv("GOGOMAIL_APNS_PRIVATE_KEY", "")
	t.Setenv("GOGOMAIL_APNS_PRIVATE_KEY_FILE", tmpFile.Name())

	cfg := Load()
	if cfg.APNsPrivateKey != privateKey {
		t.Fatalf("APNsPrivateKey = %q, want %q", cfg.APNsPrivateKey, privateKey)
	}
	if cfg.APNsPrivateKeyFile != tmpFile.Name() {
		t.Fatalf("APNsPrivateKeyFile = %q, want %q", cfg.APNsPrivateKeyFile, tmpFile.Name())
	}
}

func TestLoadAPNsPrivateKeyFilePathPrecedesEnvironmentVariable(t *testing.T) {
	setDevelopmentMode(t)
	envPrivateKey := "env-key-data"
	filePrivateKey := "file-key-data"

	// Create a temporary file with the private key
	tmpFile, err := os.CreateTemp("", "apns-key-*.txt")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	if _, err := tmpFile.WriteString(filePrivateKey); err != nil {
		t.Fatalf("failed to write to temp file: %v", err)
	}
	tmpFile.Close()

	// Set both environment variable and file path
	t.Setenv("GOGOMAIL_APNS_PRIVATE_KEY", envPrivateKey)
	t.Setenv("GOGOMAIL_APNS_PRIVATE_KEY_FILE", tmpFile.Name())

	cfg := Load()
	// File path should take precedence and override the environment variable
	if cfg.APNsPrivateKey != filePrivateKey {
		t.Fatalf("APNsPrivateKey = %q, want %q (file should override env var)", cfg.APNsPrivateKey, filePrivateKey)
	}
}

func TestValidateRejectsNonexistentAPNsPrivateKeyFile(t *testing.T) {
	setDevelopmentMode(t)
	t.Setenv("GOGOMAIL_APNS_PRIVATE_KEY_FILE", "/nonexistent/path/to/apns-key.txt")
	t.Setenv("GOGOMAIL_APNS_PRIVATE_KEY", "")

	cfg := Load()
	validateErr := cfg.Validate()
	if validateErr == nil {
		t.Fatal("Validate returned nil error for nonexistent APNS private key file")
	}
	if !strings.Contains(validateErr.Error(), "could not be read or is empty") {
		t.Fatalf("Validate error = %v, want error about unreadable file", validateErr)
	}
}

func TestValidateAcceptsAPNsPrivateKeyFromEnvironmentVariable(t *testing.T) {
	setDevelopmentMode(t)
	privateKey := "-----BEGIN PRIVATE KEY-----\nMIGHAgEAMBMGByqGSM49AgEGCCqGSM49AwEHBG0wawIBAQQg\n-----END PRIVATE KEY-----"
	t.Setenv("GOGOMAIL_APNS_PRIVATE_KEY", privateKey)
	t.Setenv("GOGOMAIL_APNS_PRIVATE_KEY_FILE", "")

	cfg := Load()
	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate returned error: %v", err)
	}
}

func TestValidateRejectsEmptyAPNsPrivateKeyFile(t *testing.T) {
	setDevelopmentMode(t)

	// Create an empty temporary file
	tmpFile, err := os.CreateTemp("", "apns-key-empty-*.txt")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())
	tmpFile.Close()

	t.Setenv("GOGOMAIL_APNS_PRIVATE_KEY_FILE", tmpFile.Name())
	t.Setenv("GOGOMAIL_APNS_PRIVATE_KEY", "")

	cfg := Load()
	validateErr := cfg.Validate()
	if validateErr == nil {
		t.Fatal("Validate returned nil error for empty APNS private key file")
	}
	if !strings.Contains(validateErr.Error(), "could not be read or is empty") {
		t.Fatalf("Validate error = %v, want error about empty file", validateErr)
	}
}
