package backup

import (
	"context"
	"database/sql"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gogomail/gogomail/internal/database"
	_ "github.com/jackc/pgx/v5/stdlib"
)

// TestInspectTargetIntegration exercises InspectTarget against a real
// PostgreSQL database. It is gated on GOGOMAIL_TEST_DATABASE_URL per repo
// convention and skips when the variable is unset (so `go test -short ./...`
// stays green without infrastructure).
func TestInspectTargetIntegration(t *testing.T) {
	baseURL := strings.TrimSpace(os.Getenv("GOGOMAIL_TEST_DATABASE_URL"))
	if baseURL == "" {
		t.Skip("set GOGOMAIL_TEST_DATABASE_URL to run backup integration tests")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	t.Cleanup(cancel)

	adminDB, err := sql.Open("pgx", baseURL)
	if err != nil {
		t.Fatalf("open admin connection: %v", err)
	}
	t.Cleanup(func() { _ = adminDB.Close() })

	schema := fmt.Sprintf("gogomail_backup_test_%d", time.Now().UnixNano())
	if _, err := adminDB.ExecContext(ctx, `CREATE SCHEMA `+schema); err != nil {
		t.Fatalf("create schema: %v", err)
	}
	t.Cleanup(func() {
		cctx, ccancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer ccancel()
		_, _ = adminDB.ExecContext(cctx, `DROP SCHEMA IF EXISTS `+schema+` CASCADE`)
	})

	dbURL := withSearchPath(t, baseURL, schema)
	db, err := database.Open(ctx, dbURL)
	if err != nil {
		t.Fatalf("open test database: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	// A fresh schema is empty and has no goose version table.
	state, err := InspectTarget(ctx, db)
	if err != nil {
		t.Fatalf("InspectTarget(empty): %v", err)
	}
	if !state.IsEmpty || state.MigrationVersion != 0 {
		t.Fatalf("empty target state = %+v, want empty with version 0", state)
	}

	// After migrating, the target is no longer empty and reports a version.
	migrationDir, err := filepath.Abs(filepath.Join("..", "..", "migrations"))
	if err != nil {
		t.Fatalf("resolve migrations: %v", err)
	}
	if err := database.MigrateUp(ctx, db, migrationDir); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	expected, err := database.ExpectedMigrationVersion(migrationDir)
	if err != nil {
		t.Fatalf("expected version: %v", err)
	}

	state, err = InspectTarget(ctx, db)
	if err != nil {
		t.Fatalf("InspectTarget(migrated): %v", err)
	}
	if state.IsEmpty {
		t.Fatal("migrated target reported empty")
	}
	if state.MigrationVersion != expected {
		t.Fatalf("migration version = %d, want %d", state.MigrationVersion, expected)
	}
}

func withSearchPath(t *testing.T, rawURL, schema string) string {
	t.Helper()
	parsed, err := url.Parse(rawURL)
	if err != nil {
		t.Fatalf("parse GOGOMAIL_TEST_DATABASE_URL: %v", err)
	}
	query := parsed.Query()
	options := strings.TrimSpace(query.Get("options"))
	searchPath := "-c search_path=" + schema + ",public"
	if options != "" {
		options += " "
	}
	options += searchPath
	query.Set("options", options)
	parsed.RawQuery = query.Encode()
	return parsed.String()
}
