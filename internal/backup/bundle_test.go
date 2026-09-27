package backup

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestBundleBuildAndValidateRoundTrip(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	m := NewManifest(time.Now())
	m.MigrationVersion = 159
	m.ExpectedMigrationVersion = 159
	m.StorageBackend = "local"

	b, err := NewBundle(dir, m)
	if err != nil {
		t.Fatalf("NewBundle: %v", err)
	}

	if _, err := b.AddFile(DatabaseFileName, strings.NewReader("fake gzip dump")); err != nil {
		t.Fatalf("AddFile db: %v", err)
	}
	if _, err := b.AddJSON(ConfigFileName, map[string]string{"environment": "test"}); err != nil {
		t.Fatalf("AddJSON config: %v", err)
	}
	final, err := b.Finalize()
	if err != nil {
		t.Fatalf("Finalize: %v", err)
	}
	if len(final.Files) != 2 {
		t.Fatalf("final.Files = %d, want 2", len(final.Files))
	}

	validated, err := ValidateBundle(dir)
	if err != nil {
		t.Fatalf("ValidateBundle: %v", err)
	}
	if validated.MigrationVersion != 159 || validated.Scope != ScopeInstance {
		t.Fatalf("validated manifest = %+v", validated)
	}
}

func TestValidateBundleDetectsChecksumTampering(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	b, err := NewBundle(dir, NewManifest(time.Now()))
	if err != nil {
		t.Fatalf("NewBundle: %v", err)
	}
	if _, err := b.AddFile(DatabaseFileName, strings.NewReader("original dump")); err != nil {
		t.Fatalf("AddFile: %v", err)
	}
	if _, err := b.Finalize(); err != nil {
		t.Fatalf("Finalize: %v", err)
	}

	// Tamper with the file after the manifest recorded its checksum.
	if err := os.WriteFile(filepath.Join(dir, DatabaseFileName), []byte("tampered dump!"), 0o640); err != nil {
		t.Fatalf("tamper: %v", err)
	}

	if _, err := ValidateBundle(dir); err == nil {
		t.Fatal("ValidateBundle accepted a tampered bundle")
	} else if !strings.Contains(err.Error(), "checksum mismatch") && !strings.Contains(err.Error(), "size mismatch") {
		t.Fatalf("error = %v, want checksum/size mismatch", err)
	}
}

func TestValidateBundleRequiresDatabaseFile(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	b, err := NewBundle(dir, NewManifest(time.Now()))
	if err != nil {
		t.Fatalf("NewBundle: %v", err)
	}
	if _, err := b.AddJSON(ConfigFileName, map[string]string{"environment": "test"}); err != nil {
		t.Fatalf("AddJSON: %v", err)
	}
	if _, err := b.Finalize(); err != nil {
		t.Fatalf("Finalize: %v", err)
	}
	if _, err := ValidateBundle(dir); err == nil || !strings.Contains(err.Error(), "database file") {
		t.Fatalf("error = %v, want missing database file", err)
	}
}

func TestValidateBundleRejectsUnknownManifestVersion(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	m := NewManifest(time.Now())
	b, err := NewBundle(dir, m)
	if err != nil {
		t.Fatalf("NewBundle: %v", err)
	}
	if _, err := b.AddFile(DatabaseFileName, strings.NewReader("x")); err != nil {
		t.Fatalf("AddFile: %v", err)
	}
	if _, err := b.Finalize(); err != nil {
		t.Fatalf("Finalize: %v", err)
	}
	// Rewrite the manifest with a future schema version.
	raw, err := os.ReadFile(filepath.Join(dir, ManifestFileName))
	if err != nil {
		t.Fatalf("read manifest: %v", err)
	}
	bumped := strings.Replace(string(raw), `"manifest_version": 1`, `"manifest_version": 999`, 1)
	if err := os.WriteFile(filepath.Join(dir, ManifestFileName), []byte(bumped), 0o640); err != nil {
		t.Fatalf("write manifest: %v", err)
	}
	if _, err := ValidateBundle(dir); err == nil || !strings.Contains(err.Error(), "unsupported manifest version") {
		t.Fatalf("error = %v, want unsupported manifest version", err)
	}
}

func TestAddFileRejectsReservedAndPathNames(t *testing.T) {
	t.Parallel()

	b, err := NewBundle(t.TempDir(), NewManifest(time.Now()))
	if err != nil {
		t.Fatalf("NewBundle: %v", err)
	}
	if _, err := b.AddFile(ManifestFileName, strings.NewReader("x")); err == nil {
		t.Fatal("AddFile accepted reserved manifest name")
	}
	if _, err := b.AddFile("../escape", strings.NewReader("x")); err == nil {
		t.Fatal("AddFile accepted a path-traversal name")
	}
}

func TestNewBundleRejectsExistingManifest(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, ManifestFileName), []byte("{}"), 0o640); err != nil {
		t.Fatalf("seed manifest: %v", err)
	}
	if _, err := NewBundle(dir, NewManifest(time.Now())); err == nil {
		t.Fatal("NewBundle overwrote an existing bundle")
	}
}
