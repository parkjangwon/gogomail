package database

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestOpenRejectsEmptyDatabaseURL(t *testing.T) {
	t.Parallel()

	if _, err := Open(context.Background(), ""); err == nil {
		t.Fatal("Open accepted empty database URL")
	}
}

func TestValidateMigrationDirAcceptsExistingDirectory(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	got, err := ValidateMigrationDir(dir)
	if err != nil {
		t.Fatalf("ValidateMigrationDir returned error: %v", err)
	}
	if got != dir {
		t.Fatalf("ValidateMigrationDir = %q, want %q", got, dir)
	}
}

func TestValidateMigrationDirRejectsMissingDirectory(t *testing.T) {
	t.Parallel()

	_, err := ValidateMigrationDir(t.TempDir() + "/missing")
	if err == nil {
		t.Fatal("ValidateMigrationDir accepted missing directory")
	}
	if !strings.Contains(err.Error(), "migration directory") {
		t.Fatalf("error = %q, want migration directory context", err.Error())
	}
}

func TestExpectedMigrationVersionUsesHighestSQLPrefix(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	for _, name := range []string{
		"0001_initial.sql",
		"0003_delivery.sql",
		"0002_mail.sql",
	} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("-- +goose Up\n-- +goose Down\n"), 0o644); err != nil {
			t.Fatalf("write migration %s: %v", name, err)
		}
	}

	got, err := ExpectedMigrationVersion(dir)
	if err != nil {
		t.Fatalf("ExpectedMigrationVersion returned error: %v", err)
	}
	if got != 3 {
		t.Fatalf("ExpectedMigrationVersion = %d, want 3", got)
	}
}

func TestExpectedMigrationVersionRejectsEmptyDir(t *testing.T) {
	t.Parallel()

	_, err := ExpectedMigrationVersion(t.TempDir())
	if err == nil {
		t.Fatal("ExpectedMigrationVersion accepted empty migration directory")
	}
	if !strings.Contains(err.Error(), "contains no sql migrations") {
		t.Fatalf("error = %q, want empty migration context", err.Error())
	}
}

func TestExpectedMigrationVersionRejectsInvalidPrefix(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "latest_mail.sql"), []byte("-- +goose Up\n-- +goose Down\n"), 0o644); err != nil {
		t.Fatalf("write migration: %v", err)
	}

	_, err := ExpectedMigrationVersion(dir)
	if err == nil {
		t.Fatal("ExpectedMigrationVersion accepted invalid migration filename")
	}
	if !strings.Contains(err.Error(), "invalid numeric version") {
		t.Fatalf("error = %q, want invalid numeric version context", err.Error())
	}
}
