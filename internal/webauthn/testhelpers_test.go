package webauthn_test

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

	_ "github.com/jackc/pgx/v5/stdlib"

	"github.com/gogomail/gogomail/internal/database"
)

// openTestDB opens a migrated Postgres test database in an isolated schema.
// The test is skipped when GOGOMAIL_TEST_DATABASE_URL is not set.
func openTestDB(t *testing.T) *sql.DB {
	t.Helper()

	baseURL := strings.TrimSpace(os.Getenv("GOGOMAIL_TEST_DATABASE_URL"))
	if baseURL == "" {
		t.Skip("set GOGOMAIL_TEST_DATABASE_URL to run WebAuthn store integration tests")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	t.Cleanup(cancel)

	adminDB, err := sql.Open("pgx", baseURL)
	if err != nil {
		t.Fatalf("open postgres admin connection: %v", err)
	}
	t.Cleanup(func() { _ = adminDB.Close() })

	schema := fmt.Sprintf("gogomail_webauthn_test_%d", time.Now().UnixNano())
	if _, err := adminDB.ExecContext(ctx, "CREATE SCHEMA "+schema); err != nil {
		t.Fatalf("create test schema: %v", err)
	}
	t.Cleanup(func() {
		cleanCtx, cc := context.WithTimeout(context.Background(), 5*time.Second)
		defer cc()
		_, _ = adminDB.ExecContext(cleanCtx, "DROP SCHEMA IF EXISTS "+schema+" CASCADE")
	})

	dbURL := testDBURLWithSchema(t, baseURL, schema)
	db, err := database.Open(ctx, dbURL)
	if err != nil {
		t.Fatalf("open test database: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	migrationDir, err := filepath.Abs(filepath.Join("..", "..", "migrations"))
	if err != nil {
		t.Fatalf("resolve migration dir: %v", err)
	}
	if err := database.MigrateUp(ctx, db, migrationDir); err != nil {
		t.Fatalf("migrate test database: %v", err)
	}
	return db
}

func testDBURLWithSchema(t *testing.T, rawURL, schema string) string {
	t.Helper()

	parsed, err := url.Parse(rawURL)
	if err != nil {
		t.Fatalf("parse GOGOMAIL_TEST_DATABASE_URL: %v", err)
	}
	q := parsed.Query()
	opts := strings.TrimSpace(q.Get("options"))
	searchPath := fmt.Sprintf("-csearch_path=%s,public", schema)
	if opts == "" {
		q.Set("options", searchPath)
	} else {
		q.Set("options", opts+" "+searchPath)
	}
	parsed.RawQuery = q.Encode()
	return parsed.String()
}

// mustCreateTestUser inserts a minimal user row and returns the UUID.
func mustCreateTestUser(t *testing.T, db *sql.DB) string {
	t.Helper()
	ctx := context.Background()

	var userID string
	err := db.QueryRowContext(ctx, `
		WITH company AS (
			INSERT INTO companies (name) VALUES ('WebAuthn Test Co') RETURNING id
		), domain AS (
			INSERT INTO domains (company_id, name, name_ace)
			SELECT id, 'wa.test', 'wa.test' FROM company RETURNING id
		), u AS (
			INSERT INTO users (domain_id, username, display_name)
			SELECT id, 'wauser', 'WA User' FROM domain RETURNING id
		)
		SELECT id::text FROM u`,
	).Scan(&userID)
	if err != nil {
		t.Fatalf("create test user: %v", err)
	}
	return userID
}
