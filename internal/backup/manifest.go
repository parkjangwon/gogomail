// Package backup provides first-class backup and restore tooling for gogomail.
//
// A backup bundle is a directory (optionally tarred/compressed by the caller)
// containing:
//
//   - manifest.json      — the [Manifest] describing bundle contents and checksums
//   - database.sql.gz     — a gzip-compressed pg_dump (plain SQL format)
//   - config.json         — a redacted [ConfigSnapshot] of the running configuration
//   - storage-manifest.json — a [StorageSnapshot] listing object keys and content hashes
//
// The manifest records SHA-256 checksums for every file so that restore can
// validate bundle integrity before touching a target, and it records the
// database migration version so that restore can refuse to load a dump that is
// incompatible with the running binary's migration set.
//
// The package deliberately contains no direct process or database side effects
// beyond building/validating manifests and computing storage snapshots; the
// pg_dump / psql invocation and bundle assembly live in the cmd layer so the
// core logic stays unit-testable without a live database.
package backup

import (
	"time"
)

// ManifestVersion is the schema version of the backup manifest. Restore refuses
// bundles whose ManifestVersion it does not understand.
const ManifestVersion = 1

// Standard file names inside a backup bundle.
const (
	ManifestFileName        = "manifest.json"
	DatabaseFileName        = "database.sql.gz"
	ConfigFileName          = "config.json"
	StorageManifestFileName = "storage-manifest.json"
)

// RedactedPlaceholder is written into a [ConfigSnapshot] wherever a secret value
// was present in the source configuration. It is intentionally distinct from an
// empty string so that operators can tell "unset" apart from "redacted".
const RedactedPlaceholder = "[REDACTED]"

// FileEntry records a single file within a backup bundle and its SHA-256
// checksum, used to validate integrity at restore time.
type FileEntry struct {
	Name   string `json:"name"`
	Size   int64  `json:"size"`
	SHA256 string `json:"sha256"`
}

// Manifest is the top-level descriptor for a backup bundle.
type Manifest struct {
	ManifestVersion int    `json:"manifest_version"`
	CreatedAt       string `json:"created_at"`
	// ToolVersion identifies the gogomail build that produced the bundle. It is
	// informational only and does not gate restore.
	ToolVersion string `json:"tool_version,omitempty"`

	// MigrationVersion is the goose migration version_id present in the source
	// database at backup time (the current applied version).
	MigrationVersion int64 `json:"migration_version"`
	// ExpectedMigrationVersion is the highest migration version bundled with the
	// binary that produced the backup. It equals MigrationVersion for a healthy
	// source.
	ExpectedMigrationVersion int64 `json:"expected_migration_version"`

	// StorageBackend is the normalized storage backend name (e.g. "local", "s3").
	StorageBackend string `json:"storage_backend"`
	// StorageObjectCount is the number of objects captured in the storage manifest.
	StorageObjectCount int `json:"storage_object_count"`

	// Files lists every non-manifest file in the bundle with its checksum.
	Files []FileEntry `json:"files"`

	// Scope describes what the bundle covers. gogomail backups are always
	// instance-wide (see docs/BACKUP_RESTORE.md); this field documents that
	// contract in the artifact itself.
	Scope string `json:"scope"`
}

// ScopeInstance is the only supported backup scope: the whole instance
// (all tenants), because a PostgreSQL logical dump and object-store snapshot
// span every tenant in the shared schema.
const ScopeInstance = "instance"

// NewManifest returns a Manifest pre-populated with the schema version,
// creation timestamp, and instance scope. Callers fill in the remaining fields.
func NewManifest(now time.Time) Manifest {
	return Manifest{
		ManifestVersion: ManifestVersion,
		CreatedAt:       now.UTC().Format(time.RFC3339),
		Scope:           ScopeInstance,
	}
}

// FindFile returns the FileEntry with the given name and whether it was found.
func (m Manifest) FindFile(name string) (FileEntry, bool) {
	for _, f := range m.Files {
		if f.Name == name {
			return f, true
		}
	}
	return FileEntry{}, false
}
