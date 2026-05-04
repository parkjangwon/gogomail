package database

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func TestMigrationFilenamesUseUniqueVersions(t *testing.T) {
	t.Parallel()

	matches, err := filepath.Glob(filepath.Join("..", "..", "migrations", "*.sql"))
	if err != nil {
		t.Fatalf("glob migrations: %v", err)
	}
	if len(matches) == 0 {
		t.Fatal("no migration files found")
	}

	seen := make(map[string]string, len(matches))
	for _, path := range matches {
		name := filepath.Base(path)
		version, _, ok := strings.Cut(name, "_")
		if !ok || version == "" {
			t.Fatalf("migration %s must start with a numeric version prefix", name)
		}
		if previous := seen[version]; previous != "" {
			t.Fatalf("migration version %s is duplicated by %s and %s", version, previous, name)
		}
		seen[version] = name
	}
}

func TestMigrationVersionsAreContiguous(t *testing.T) {
	t.Parallel()

	matches, err := filepath.Glob(filepath.Join("..", "..", "migrations", "*.sql"))
	if err != nil {
		t.Fatalf("glob migrations: %v", err)
	}

	seen := make(map[int]string, len(matches))
	maxVersion := 0
	for _, path := range matches {
		name := filepath.Base(path)
		rawVersion, _, ok := strings.Cut(name, "_")
		if !ok {
			t.Fatalf("migration %s must start with a numeric version prefix", name)
		}
		version, err := strconv.Atoi(rawVersion)
		if err != nil || version <= 0 {
			t.Fatalf("migration %s has invalid numeric version %q", name, rawVersion)
		}
		seen[version] = name
		if version > maxVersion {
			maxVersion = version
		}
	}
	for version := 1; version <= maxVersion; version++ {
		if seen[version] == "" {
			t.Fatalf("migration version %04d is missing", version)
		}
	}
}

func TestRecentMigrationsDeclareGooseSections(t *testing.T) {
	t.Parallel()

	for _, name := range []string{
		"0042_push_notification_provider_outcomes.sql",
		"0043_delivery_attempt_diagnostics.sql",
		"0044_message_search_index.sql",
		"0045_attachment_upload_sessions.sql",
	} {
		raw, err := os.ReadFile(filepath.Join("..", "..", "migrations", name))
		if err != nil {
			t.Fatalf("read migration %s: %v", name, err)
		}
		text := string(raw)
		if !strings.Contains(text, "-- +goose Up") || !strings.Contains(text, "-- +goose Down") {
			t.Fatalf("migration %s must declare goose Up and Down sections", name)
		}
	}
}
