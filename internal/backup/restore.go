package backup

import (
	"bytes"
	"fmt"
	"io"
)

// RestoreGuardInput carries the facts a restore needs to decide whether it is
// safe to proceed against a target database.
type RestoreGuardInput struct {
	// Manifest is the validated bundle manifest.
	Manifest Manifest
	// TargetIsEmpty reports whether the target database has no user tables
	// (i.e. is a fresh, empty database). Restoring into a populated database is
	// refused unless Force is set.
	TargetIsEmpty bool
	// TargetMigrationVersion is the migration version currently applied to the
	// target, or 0 if the goose version table does not exist yet (empty target).
	TargetMigrationVersion int64
	// BinaryExpectedVersion is the highest migration version bundled with the
	// restoring binary. The bundle's MigrationVersion must not exceed it, or the
	// running binary is too old to understand the dump.
	BinaryExpectedVersion int64
	// Force bypasses the non-empty-target refusal. It never bypasses the
	// manifest-version or forward-migration-version safety checks, which indicate
	// a structurally incompatible restore.
	Force bool
}

// CheckRestoreGuards evaluates every restore precondition and returns a
// combined error describing all violations, or nil if the restore may proceed.
//
// Hard failures (never bypassable, even with Force):
//   - unsupported manifest version
//   - the bundle was produced by a newer schema than this binary supports
//     (bundle MigrationVersion > BinaryExpectedVersion)
//
// Soft failure (bypassable with Force):
//   - the target database is not empty
func CheckRestoreGuards(in RestoreGuardInput) error {
	var problems []string

	if in.Manifest.ManifestVersion != ManifestVersion {
		problems = append(problems, fmt.Sprintf(
			"unsupported manifest version %d (this build understands %d)",
			in.Manifest.ManifestVersion, ManifestVersion))
	}

	if in.BinaryExpectedVersion > 0 && in.Manifest.MigrationVersion > in.BinaryExpectedVersion {
		problems = append(problems, fmt.Sprintf(
			"bundle migration version %d is newer than this binary's maximum %d; upgrade gogomail before restoring",
			in.Manifest.MigrationVersion, in.BinaryExpectedVersion))
	}

	if !in.TargetIsEmpty {
		if in.Force {
			// allowed, but callers should surface a prominent warning
		} else {
			problems = append(problems, fmt.Sprintf(
				"target database is not empty (migration version %d); refusing to overwrite without --force",
				in.TargetMigrationVersion))
		}
	}

	if len(problems) == 0 {
		return nil
	}
	if len(problems) == 1 {
		return fmt.Errorf("restore refused: %s", problems[0])
	}
	msg := "restore refused for the following reasons:"
	for _, p := range problems {
		msg += "\n  - " + p
	}
	return fmt.Errorf("%s", msg)
}

// bytesReader returns an io.Reader over b. Small helper to keep call sites tidy.
func bytesReader(b []byte) io.Reader { return bytes.NewReader(b) }
