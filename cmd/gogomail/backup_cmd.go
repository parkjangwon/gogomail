package main

import (
	"compress/gzip"
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/gogomail/gogomail/internal/backup"
	"github.com/gogomail/gogomail/internal/config"
	"github.com/gogomail/gogomail/internal/database"
)

// backupToolVersion is stamped into produced bundles for traceability. It can be
// overridden at build time with -ldflags "-X main.backupToolVersion=...".
var backupToolVersion = "dev"

// runBackupCommand produces a timestamped backup bundle. It supersedes the
// legacy scripts/backup.sh wrapper by additionally capturing a storage manifest,
// a redacted config snapshot, and the migration version alongside the pg_dump.
func runBackupCommand(args []string, stdout io.Writer, stderr io.Writer) int {
	flags := flag.NewFlagSet("backup", flag.ContinueOnError)
	flags.SetOutput(stderr)
	configFile := flags.String("config", "", "optional YAML config file")
	outDir := flags.String("out", "./backups", "directory to write the timestamped bundle into")
	hashStorage := flags.Bool("hash-storage", false, "compute SHA-256 of every storage object (exact but O(total bytes); leave off for large S3 buckets)")
	storagePrefix := flags.String("storage-prefix", "", "restrict the storage manifest to keys under this prefix")
	maxObjects := flags.Int("max-storage-objects", 0, "fail if the storage manifest would exceed this many objects (0 = unlimited)")
	skipStorage := flags.Bool("skip-storage", false, "skip the storage manifest entirely")
	flags.Usage = func() {
		fmt.Fprintln(stderr, "usage: gogomail backup [flags]")
		fmt.Fprintln(stderr, "produces a timestamped backup bundle (pg_dump + storage manifest + redacted config + migration version)")
		flags.PrintDefaults()
	}
	if err := flags.Parse(args); err != nil {
		return 2
	}

	cfg, err := config.LoadFile(*configFile)
	if err != nil {
		fmt.Fprintf(stderr, "error: invalid config file: %v\n", err)
		return 2
	}

	if _, err := exec.LookPath("pg_dump"); err != nil {
		fmt.Fprintln(stderr, "error: pg_dump not found on PATH; install PostgreSQL client tools (see docs/BACKUP_RESTORE.md)")
		return 127
	}

	ctx := context.Background()

	// Capture the current migration version from the source database.
	db, err := database.Open(ctx, cfg.DatabaseURL)
	if err != nil {
		fmt.Fprintf(stderr, "error: database connection failed: %v\n", err)
		return 1
	}
	defer db.Close()

	currentVersion, err := database.CurrentMigrationVersion(ctx, db)
	if err != nil {
		fmt.Fprintf(stderr, "error: read migration version: %v\n", err)
		return 1
	}
	expectedVersion, err := database.ExpectedMigrationVersion(cfg.MigrationDir)
	if err != nil {
		fmt.Fprintf(stderr, "error: read expected migration version: %v\n", err)
		return 1
	}
	if currentVersion < expectedVersion {
		fmt.Fprintf(stderr, "warning: source database migration version %d is behind expected %d; backup captures the current state\n", currentVersion, expectedVersion)
	}

	timestamp := time.Now().UTC().Format("2006-01-02T150405Z")
	bundleDir := filepath.Join(*outDir, "gogomail-backup-"+timestamp)

	manifest := backup.NewManifest(time.Now())
	manifest.ToolVersion = backupToolVersion
	manifest.MigrationVersion = currentVersion
	manifest.ExpectedMigrationVersion = expectedVersion
	manifest.StorageBackend = strings.ToLower(strings.TrimSpace(cfg.StorageBackend))
	if manifest.StorageBackend == "" {
		manifest.StorageBackend = "local"
	}

	bundle, err := backup.NewBundle(bundleDir, manifest)
	if err != nil {
		fmt.Fprintf(stderr, "error: create bundle: %v\n", err)
		return 1
	}

	fmt.Fprintf(stdout, "backup: writing bundle to %s\n", bundleDir)

	// 1. Database dump (pg_dump plain SQL, gzip-compressed).
	if err := writeDatabaseDump(ctx, bundle, cfg.DatabaseURL); err != nil {
		fmt.Fprintf(stderr, "error: database dump failed: %v\n", err)
		return 1
	}
	fmt.Fprintln(stdout, "backup: database dump complete")

	// 2. Storage manifest.
	if !*skipStorage {
		store, err := backup.StoreForConfig(cfg)
		if err != nil {
			fmt.Fprintf(stderr, "error: build storage client: %v\n", err)
			return 1
		}
		snap, err := backup.BuildStorageSnapshot(ctx, store, backup.SnapshotOptions{
			Prefix:     *storagePrefix,
			Hash:       *hashStorage,
			MaxObjects: *maxObjects,
			Backend:    cfg.StorageBackend,
			Bucket:     cfg.StorageS3Bucket,
		})
		if err != nil {
			fmt.Fprintf(stderr, "error: storage snapshot failed: %v\n", err)
			return 1
		}
		if _, err := bundle.AddJSON(backup.StorageManifestFileName, snap); err != nil {
			fmt.Fprintf(stderr, "error: write storage manifest: %v\n", err)
			return 1
		}
		manifest.StorageObjectCount = snap.ObjectCount
		fmt.Fprintf(stdout, "backup: storage manifest complete (%d objects, %d bytes, hashed=%t)\n", snap.ObjectCount, snap.TotalBytes, snap.Hashed)
	} else {
		fmt.Fprintln(stdout, "backup: storage manifest skipped (--skip-storage)")
	}

	// 3. Redacted config snapshot.
	snap := backup.NewConfigSnapshot(cfg)
	if _, err := bundle.AddJSON(backup.ConfigFileName, snap); err != nil {
		fmt.Fprintf(stderr, "error: write config snapshot: %v\n", err)
		return 1
	}
	fmt.Fprintf(stdout, "backup: config snapshot complete (%d secret keys redacted)\n", len(snap.RedactedSecrets))

	// Re-stamp the object count captured after the storage step, then finalize.
	bundle.SetStorageObjectCount(manifest.StorageObjectCount)
	final, err := bundle.Finalize()
	if err != nil {
		fmt.Fprintf(stderr, "error: finalize bundle: %v\n", err)
		return 1
	}

	fmt.Fprintf(stdout, "backup: done — %s (migration version %d, %d files)\n", bundleDir, final.MigrationVersion, len(final.Files))
	return 0
}

func writeDatabaseDump(ctx context.Context, bundle *backup.Bundle, databaseURL string) error {
	// #nosec G204 -- databaseURL comes from operator configuration, not untrusted input.
	cmd := exec.CommandContext(ctx, "pg_dump", "--format=plain", "--no-owner", "--no-privileges", databaseURL)
	var stderr strings.Builder
	cmd.Stderr = &stderr

	pipe, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("pg_dump stdout pipe: %w", err)
	}
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start pg_dump: %w", err)
	}

	// Gzip the dump stream on the way into the bundle.
	pr, pw := io.Pipe()
	go func() {
		gz := gzip.NewWriter(pw)
		_, copyErr := io.Copy(gz, pipe)
		closeErr := gz.Close()
		if copyErr != nil {
			pw.CloseWithError(copyErr)
			return
		}
		pw.CloseWithError(closeErr)
	}()

	if _, err := bundle.AddFile(backup.DatabaseFileName, pr); err != nil {
		_ = cmd.Wait()
		return fmt.Errorf("write database dump: %w", err)
	}
	if err := cmd.Wait(); err != nil {
		return fmt.Errorf("pg_dump failed: %w: %s", err, strings.TrimSpace(stderr.String()))
	}
	return nil
}

// runRestoreCommand validates a bundle, checks restore guards against the target
// database, prints a pre-restore checklist, and (unless --dry-run) restores the
// database dump into the target.
func runRestoreCommand(args []string, stdout io.Writer, stderr io.Writer) int {
	flags := flag.NewFlagSet("restore", flag.ContinueOnError)
	flags.SetOutput(stderr)
	configFile := flags.String("config", "", "optional YAML config file")
	bundleDir := flags.String("bundle", "", "path to the backup bundle directory (required)")
	force := flags.Bool("force", false, "allow restoring into a non-empty target database (DESTRUCTIVE)")
	dryRun := flags.Bool("dry-run", false, "validate the bundle and print the checklist without restoring")
	flags.Usage = func() {
		fmt.Fprintln(stderr, "usage: gogomail restore --bundle <dir> [flags]")
		fmt.Fprintln(stderr, "validates a backup bundle and restores the database into the configured target")
		flags.PrintDefaults()
	}
	if err := flags.Parse(args); err != nil {
		return 2
	}
	if strings.TrimSpace(*bundleDir) == "" {
		fmt.Fprintln(stderr, "error: --bundle is required")
		flags.Usage()
		return 2
	}

	cfg, err := config.LoadFile(*configFile)
	if err != nil {
		fmt.Fprintf(stderr, "error: invalid config file: %v\n", err)
		return 2
	}

	// 1. Validate the bundle (schema version, checksums, required files).
	manifest, err := backup.ValidateBundle(*bundleDir)
	if err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}
	fmt.Fprintf(stdout, "restore: bundle validated — migration version %d, %d files, %d storage objects\n",
		manifest.MigrationVersion, len(manifest.Files), manifest.StorageObjectCount)

	if _, err := exec.LookPath("psql"); err != nil {
		fmt.Fprintln(stderr, "error: psql not found on PATH; install PostgreSQL client tools (see docs/BACKUP_RESTORE.md)")
		return 127
	}

	ctx := context.Background()

	db, err := database.Open(ctx, cfg.DatabaseURL)
	if err != nil {
		fmt.Fprintf(stderr, "error: target database connection failed: %v\n", err)
		return 1
	}
	defer db.Close()

	// 2. Inspect the target and evaluate restore guards.
	target, err := backup.InspectTarget(ctx, db)
	if err != nil {
		fmt.Fprintf(stderr, "error: inspect target: %v\n", err)
		return 1
	}
	binaryExpected, err := database.ExpectedMigrationVersion(cfg.MigrationDir)
	if err != nil {
		fmt.Fprintf(stderr, "error: read expected migration version: %v\n", err)
		return 1
	}

	guardErr := backup.CheckRestoreGuards(backup.RestoreGuardInput{
		Manifest:               manifest,
		TargetIsEmpty:          target.IsEmpty,
		TargetMigrationVersion: target.MigrationVersion,
		BinaryExpectedVersion:  binaryExpected,
		Force:                  *force,
	})

	printRestoreChecklist(stdout, manifest, target, binaryExpected, *force)

	if guardErr != nil {
		fmt.Fprintf(stderr, "\nerror: %v\n", guardErr)
		return 1
	}
	if !target.IsEmpty && *force {
		fmt.Fprintln(stdout, "\nWARNING: target database is NOT empty and --force was supplied; existing data will be overwritten.")
	}

	if *dryRun {
		fmt.Fprintln(stdout, "\nrestore: dry-run complete; no changes made")
		return 0
	}

	// 3. Restore the database dump.
	fmt.Fprintln(stdout, "\nrestore: applying database dump...")
	if err := restoreDatabaseDump(ctx, filepath.Join(*bundleDir, backup.DatabaseFileName), cfg.DatabaseURL); err != nil {
		fmt.Fprintf(stderr, "error: database restore failed: %v\n", err)
		return 1
	}

	fmt.Fprintln(stdout, "restore: database restored")
	fmt.Fprintln(stdout, "restore: NOTE — object storage is not copied by restore; ensure the storage backend is populated (see storage-manifest.json and docs/BACKUP_RESTORE.md)")
	fmt.Fprintln(stdout, "restore: done")
	return 0
}

func restoreDatabaseDump(ctx context.Context, dumpPath string, databaseURL string) error {
	f, err := os.Open(dumpPath)
	if err != nil {
		return fmt.Errorf("open dump: %w", err)
	}
	defer f.Close()

	gz, err := gzip.NewReader(f)
	if err != nil {
		return fmt.Errorf("open gzip dump: %w", err)
	}
	defer gz.Close()

	// #nosec G204 -- databaseURL comes from operator configuration, not untrusted input.
	cmd := exec.CommandContext(ctx, "psql", "--set", "ON_ERROR_STOP=1", databaseURL)
	cmd.Stdin = gz
	var stderr strings.Builder
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("psql restore failed: %w: %s", err, strings.TrimSpace(stderr.String()))
	}
	return nil
}

func printRestoreChecklist(w io.Writer, m backup.Manifest, target backup.TargetState, binaryExpected int64, force bool) {
	fmt.Fprintln(w, "\n=== Pre-restore checklist ===")
	fmt.Fprintf(w, "  [%s] Bundle manifest version understood (%d)\n", checkMark(m.ManifestVersion == backup.ManifestVersion), m.ManifestVersion)
	fmt.Fprintf(w, "  [%s] Bundle migration version %d <= binary maximum %d\n", checkMark(binaryExpected == 0 || m.MigrationVersion <= binaryExpected), m.MigrationVersion, binaryExpected)
	if target.IsEmpty {
		fmt.Fprintf(w, "  [%s] Target database is empty\n", checkMark(true))
	} else {
		fmt.Fprintf(w, "  [%s] Target database is empty (current version %d; --force=%t)\n", checkMark(force), target.MigrationVersion, force)
	}
	fmt.Fprintf(w, "  [ ] Object storage backend %q is provisioned and reachable (restore does NOT copy objects)\n", m.StorageBackend)
	fmt.Fprintln(w, "  [ ] This restore is INSTANCE-WIDE (all tenants). Per-tenant restore is not supported — see docs/BACKUP_RESTORE.md")
	fmt.Fprintln(w, "  [ ] Downstream consumers (IMAP/SMTP/workers) are stopped or drained")
}

func checkMark(ok bool) string {
	if ok {
		return "x"
	}
	return "!"
}
