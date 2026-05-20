package database

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
)

type Options struct {
	MaxOpenConns    int
	MaxIdleConns    int
	ConnMaxLifetime time.Duration
	ConnMaxIdleTime time.Duration
}

func (o Options) withDefaults() Options {
	if o.MaxOpenConns <= 0 {
		o.MaxOpenConns = 20
	}
	if o.MaxIdleConns <= 0 {
		o.MaxIdleConns = 5
	}
	if o.ConnMaxLifetime <= 0 {
		o.ConnMaxLifetime = 30 * time.Minute
	}
	if o.ConnMaxIdleTime <= 0 {
		o.ConnMaxIdleTime = 5 * time.Minute
	}
	return o
}

func Open(ctx context.Context, databaseURL string, opts ...Options) (*sql.DB, error) {
	if strings.TrimSpace(databaseURL) == "" {
		return nil, fmt.Errorf("database URL is required")
	}

	db, err := sql.Open("pgx", databaseURL)
	if err != nil {
		return nil, fmt.Errorf("open postgres connection: %w", err)
	}

	options := Options{}.withDefaults()
	if len(opts) > 0 {
		options = opts[0].withDefaults()
	}
	db.SetMaxOpenConns(options.MaxOpenConns)
	db.SetMaxIdleConns(options.MaxIdleConns)
	db.SetConnMaxLifetime(options.ConnMaxLifetime)
	db.SetConnMaxIdleTime(options.ConnMaxIdleTime)

	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := db.PingContext(pingCtx); err != nil {
		if closeErr := db.Close(); closeErr != nil {
			fmt.Fprintln(os.Stderr, "close postgres after ping error:", closeErr)
		}
		return nil, fmt.Errorf("ping postgres: %w", err)
	}

	return db, nil
}

func ValidateMigrationDir(dir string) (string, error) {
	cleaned := strings.TrimSpace(dir)
	if cleaned == "" {
		return "", fmt.Errorf("migration directory is required")
	}

	info, err := os.Stat(cleaned)
	if err != nil {
		return "", fmt.Errorf("migration directory %q is not accessible: %w", cleaned, err)
	}
	if !info.IsDir() {
		return "", fmt.Errorf("migration directory %q is not a directory", cleaned)
	}
	return cleaned, nil
}

func MigrateUp(ctx context.Context, db *sql.DB, dir string) error {
	migrationDir, err := ValidateMigrationDir(dir)
	if err != nil {
		return err
	}
	if db == nil {
		return fmt.Errorf("database handle is required")
	}
	if err := goose.SetDialect("postgres"); err != nil {
		return fmt.Errorf("set goose dialect: %w", err)
	}
	if err := goose.UpContext(ctx, db, migrationDir); err != nil {
		return fmt.Errorf("run migrations: %w", err)
	}
	return nil
}

func ExpectedMigrationVersion(dir string) (int64, error) {
	migrationDir, err := ValidateMigrationDir(dir)
	if err != nil {
		return 0, err
	}
	matches, err := filepath.Glob(filepath.Join(migrationDir, "*.sql"))
	if err != nil {
		return 0, fmt.Errorf("glob migrations: %w", err)
	}
	if len(matches) == 0 {
		return 0, fmt.Errorf("migration directory %q contains no sql migrations", migrationDir)
	}

	var expected int64
	for _, path := range matches {
		version, err := migrationVersionFromFilename(filepath.Base(path))
		if err != nil {
			return 0, err
		}
		if version > expected {
			expected = version
		}
	}
	return expected, nil
}

func CurrentMigrationVersion(ctx context.Context, db *sql.DB) (int64, error) {
	if db == nil {
		return 0, fmt.Errorf("database handle is required")
	}
	rows, err := db.QueryContext(ctx, `SELECT version_id, is_applied FROM goose_db_version ORDER BY id DESC`)
	if err != nil {
		return 0, fmt.Errorf("query goose migration version: %w", err)
	}
	defer rows.Close()

	skipped := make(map[int64]struct{})
	for rows.Next() {
		var version int64
		var applied bool
		if err := rows.Scan(&version, &applied); err != nil {
			return 0, fmt.Errorf("scan goose migration version: %w", err)
		}
		if _, ok := skipped[version]; ok {
			continue
		}
		if applied {
			return version, nil
		}
		skipped[version] = struct{}{}
	}
	if err := rows.Err(); err != nil {
		return 0, fmt.Errorf("iterate goose migration versions: %w", err)
	}
	return 0, fmt.Errorf("goose migration version table has no applied version")
}

func MigrationVersionReady(ctx context.Context, db *sql.DB, dir string) (int64, int64, error) {
	expected, err := ExpectedMigrationVersion(dir)
	if err != nil {
		return 0, 0, err
	}
	current, err := CurrentMigrationVersion(ctx, db)
	if err != nil {
		return 0, expected, err
	}
	if current < expected {
		return current, expected, fmt.Errorf("migration version %d is behind expected %d", current, expected)
	}
	return current, expected, nil
}

func migrationVersionFromFilename(name string) (int64, error) {
	rawVersion, _, ok := strings.Cut(name, "_")
	if !ok || rawVersion == "" {
		return 0, fmt.Errorf("migration %s must start with a numeric version prefix", name)
	}
	version, err := strconv.ParseInt(rawVersion, 10, 64)
	if err != nil || version <= 0 {
		return 0, fmt.Errorf("migration %s has invalid numeric version %q", name, rawVersion)
	}
	return version, nil
}
