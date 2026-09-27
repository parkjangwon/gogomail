package main

import (
	"bytes"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/gogomail/gogomail/internal/backup"
)

func TestRestoreCommandRequiresBundle(t *testing.T) {
	var stderr bytes.Buffer
	code := runRestoreCommand([]string{}, &bytes.Buffer{}, &stderr)
	if code != 2 {
		t.Fatalf("exit = %d, want 2", code)
	}
	if !strings.Contains(stderr.String(), "--bundle is required") {
		t.Fatalf("stderr = %q", stderr.String())
	}
}

func TestRestoreCommandRejectsInvalidBundle(t *testing.T) {
	// A directory without a manifest fails validation before any DB access.
	var stderr bytes.Buffer
	code := runRestoreCommand([]string{"--bundle", t.TempDir()}, &bytes.Buffer{}, &stderr)
	if code != 1 {
		t.Fatalf("exit = %d, want 1", code)
	}
	if !strings.Contains(stderr.String(), "read manifest") {
		t.Fatalf("stderr = %q, want manifest read error", stderr.String())
	}
}

func TestRestoreCommandRejectsTamperedBundle(t *testing.T) {
	dir := t.TempDir()
	b, err := backup.NewBundle(dir, backup.NewManifest(time.Now()))
	if err != nil {
		t.Fatalf("NewBundle: %v", err)
	}
	if _, err := b.AddFile(backup.DatabaseFileName, strings.NewReader("dump")); err != nil {
		t.Fatalf("AddFile: %v", err)
	}
	if _, err := b.Finalize(); err != nil {
		t.Fatalf("Finalize: %v", err)
	}
	// Corrupt the dump so checksum validation fails before DB access.
	if err := os.WriteFile(dir+"/"+backup.DatabaseFileName, []byte("corrupted"), 0o640); err != nil {
		t.Fatalf("corrupt: %v", err)
	}

	var stderr bytes.Buffer
	code := runRestoreCommand([]string{"--bundle", dir}, &bytes.Buffer{}, &stderr)
	if code != 1 {
		t.Fatalf("exit = %d, want 1", code)
	}
	if !strings.Contains(stderr.String(), "mismatch") {
		t.Fatalf("stderr = %q, want checksum/size mismatch", stderr.String())
	}
}

func TestPrintRestoreChecklistMarksForceBypass(t *testing.T) {
	var out bytes.Buffer
	m := backup.Manifest{ManifestVersion: backup.ManifestVersion, MigrationVersion: 159, StorageBackend: "local"}
	printRestoreChecklist(&out, m, backup.TargetState{IsEmpty: false, MigrationVersion: 159}, 159, true)
	got := out.String()
	if !strings.Contains(got, "INSTANCE-WIDE") {
		t.Fatalf("checklist missing instance-wide note: %q", got)
	}
	if !strings.Contains(got, "[x] Target database is empty (current version 159; --force=true)") {
		t.Fatalf("checklist did not mark force bypass: %q", got)
	}
}
