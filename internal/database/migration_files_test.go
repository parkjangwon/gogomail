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

func TestMigrationsDeclareGooseSections(t *testing.T) {
	t.Parallel()

	matches, err := filepath.Glob(filepath.Join("..", "..", "migrations", "*.sql"))
	if err != nil {
		t.Fatalf("glob migrations: %v", err)
	}
	if len(matches) == 0 {
		t.Fatal("no migration files found")
	}

	for _, path := range matches {
		name := filepath.Base(path)
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read migration %s: %v", name, err)
		}
		text := string(raw)
		if !strings.Contains(text, "-- +goose Up") || !strings.Contains(text, "-- +goose Down") {
			t.Fatalf("migration %s must declare goose Up and Down sections", name)
		}
	}
}

func TestDAVCollectionPropertyLanguageMigrationMatchesRepositoryColumns(t *testing.T) {
	t.Parallel()

	raw, err := os.ReadFile(filepath.Join("..", "..", "migrations", "0097_dav_collection_property_lang.sql"))
	if err != nil {
		t.Fatalf("read DAV collection property language migration: %v", err)
	}
	text := string(raw)

	required := []string{
		"ALTER TABLE caldav_calendars",
		"ALTER TABLE carddav_addressbooks",
		"ADD COLUMN displayname_lang text NOT NULL DEFAULT ''",
		"ADD COLUMN description_lang text NOT NULL DEFAULT ''",
		"caldav_calendars_displayname_lang_check",
		"caldav_calendars_description_lang_check",
		"carddav_addressbooks_displayname_lang_check",
		"carddav_addressbooks_description_lang_check",
		"CHECK (char_length(displayname_lang) <= 64 AND displayname_lang !~ '[[:space:][:cntrl:]]')",
		"CHECK (char_length(description_lang) <= 64 AND description_lang !~ '[[:space:][:cntrl:]]')",
		"DROP CONSTRAINT IF EXISTS carddav_addressbooks_description_lang_check",
		"DROP CONSTRAINT IF EXISTS carddav_addressbooks_displayname_lang_check",
		"DROP COLUMN IF EXISTS description_lang",
		"DROP COLUMN IF EXISTS displayname_lang",
		"DROP CONSTRAINT IF EXISTS caldav_calendars_description_lang_check",
		"DROP CONSTRAINT IF EXISTS caldav_calendars_displayname_lang_check",
	}
	for _, fragment := range required {
		if !strings.Contains(text, fragment) {
			t.Fatalf("migration 0097 must contain %q", fragment)
		}
	}
}
