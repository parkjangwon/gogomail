package backup

import (
	"strings"
	"testing"
)

func validManifest() Manifest {
	m := Manifest{ManifestVersion: ManifestVersion, MigrationVersion: 159}
	return m
}

func TestCheckRestoreGuardsAllowsEmptyCompatibleTarget(t *testing.T) {
	t.Parallel()

	err := CheckRestoreGuards(RestoreGuardInput{
		Manifest:              validManifest(),
		TargetIsEmpty:         true,
		BinaryExpectedVersion: 159,
	})
	if err != nil {
		t.Fatalf("CheckRestoreGuards = %v, want nil", err)
	}
}

func TestCheckRestoreGuardsRefusesNonEmptyTargetWithoutForce(t *testing.T) {
	t.Parallel()

	err := CheckRestoreGuards(RestoreGuardInput{
		Manifest:               validManifest(),
		TargetIsEmpty:          false,
		TargetMigrationVersion: 159,
		BinaryExpectedVersion:  159,
	})
	if err == nil || !strings.Contains(err.Error(), "not empty") {
		t.Fatalf("error = %v, want non-empty refusal", err)
	}
}

func TestCheckRestoreGuardsForceBypassesNonEmptyTarget(t *testing.T) {
	t.Parallel()

	err := CheckRestoreGuards(RestoreGuardInput{
		Manifest:               validManifest(),
		TargetIsEmpty:          false,
		TargetMigrationVersion: 159,
		BinaryExpectedVersion:  159,
		Force:                  true,
	})
	if err != nil {
		t.Fatalf("CheckRestoreGuards with Force = %v, want nil", err)
	}
}

func TestCheckRestoreGuardsRejectsForwardMigrationEvenWithForce(t *testing.T) {
	t.Parallel()

	m := validManifest()
	m.MigrationVersion = 200 // bundle newer than the binary understands
	err := CheckRestoreGuards(RestoreGuardInput{
		Manifest:              m,
		TargetIsEmpty:         true,
		BinaryExpectedVersion: 159,
		Force:                 true, // must NOT bypass a forward-version mismatch
	})
	if err == nil || !strings.Contains(err.Error(), "newer than this binary") {
		t.Fatalf("error = %v, want forward-version refusal", err)
	}
}

func TestCheckRestoreGuardsRejectsUnknownManifestVersion(t *testing.T) {
	t.Parallel()

	m := validManifest()
	m.ManifestVersion = 999
	err := CheckRestoreGuards(RestoreGuardInput{
		Manifest:              m,
		TargetIsEmpty:         true,
		BinaryExpectedVersion: 159,
		Force:                 true,
	})
	if err == nil || !strings.Contains(err.Error(), "unsupported manifest version") {
		t.Fatalf("error = %v, want unsupported manifest version", err)
	}
}

func TestCheckRestoreGuardsReportsMultipleProblems(t *testing.T) {
	t.Parallel()

	m := validManifest()
	m.ManifestVersion = 999
	m.MigrationVersion = 500
	err := CheckRestoreGuards(RestoreGuardInput{
		Manifest:              m,
		TargetIsEmpty:         false,
		BinaryExpectedVersion: 159,
	})
	if err == nil {
		t.Fatal("expected combined error")
	}
	for _, want := range []string{"unsupported manifest version", "newer than this binary", "not empty"} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("error %q missing %q", err.Error(), want)
		}
	}
}
