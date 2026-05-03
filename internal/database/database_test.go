package database

import (
	"context"
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
