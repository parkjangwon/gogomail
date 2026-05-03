package database

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"strings"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
)

func Open(ctx context.Context, databaseURL string) (*sql.DB, error) {
	if strings.TrimSpace(databaseURL) == "" {
		return nil, fmt.Errorf("database URL is required")
	}

	db, err := sql.Open("pgx", databaseURL)
	if err != nil {
		return nil, fmt.Errorf("open postgres connection: %w", err)
	}

	db.SetMaxOpenConns(20)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(30 * time.Minute)
	db.SetConnMaxIdleTime(5 * time.Minute)

	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := db.PingContext(pingCtx); err != nil {
		_ = db.Close()
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
