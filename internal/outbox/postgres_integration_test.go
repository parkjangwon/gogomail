package outbox

import (
	"context"
	"database/sql"
	"errors"
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

func TestPostgresStoreClaimsOnlyAvailablePendingRows(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	db := openMigratedOutboxPostgresTestDB(t)
	store := NewPostgresStore(db, 2)

	var readyID, futureID string
	if err := db.QueryRowContext(ctx, `
INSERT INTO outbox (topic, partition_key, payload, status, available_at)
VALUES ('mail.event', 'ready', '{"event":"ready"}'::jsonb, 'pending', now() - interval '1 minute')
RETURNING id::text`).Scan(&readyID); err != nil {
		t.Fatalf("insert ready outbox row: %v", err)
	}
	if err := db.QueryRowContext(ctx, `
INSERT INTO outbox (topic, partition_key, payload, status, available_at)
VALUES ('mail.event', 'future', '{"event":"future"}'::jsonb, 'pending', now() + interval '1 hour')
RETURNING id::text`).Scan(&futureID); err != nil {
		t.Fatalf("insert future outbox row: %v", err)
	}

	events, err := store.FetchPending(ctx, 10)
	if err != nil {
		t.Fatalf("FetchPending returned error: %v", err)
	}
	if len(events) != 1 || events[0].ID != readyID {
		t.Fatalf("events = %+v, want only ready id %q", events, readyID)
	}

	var readyStatus string
	var readyAttempts int
	if err := db.QueryRowContext(ctx, `SELECT status, attempts FROM outbox WHERE id = $1`, readyID).Scan(&readyStatus, &readyAttempts); err != nil {
		t.Fatalf("query ready status: %v", err)
	}
	if readyStatus != "processing" || readyAttempts != 1 {
		t.Fatalf("ready status/attempts = %q/%d, want processing/1", readyStatus, readyAttempts)
	}

	var futureStatus string
	var futureAttempts int
	if err := db.QueryRowContext(ctx, `SELECT status, attempts FROM outbox WHERE id = $1`, futureID).Scan(&futureStatus, &futureAttempts); err != nil {
		t.Fatalf("query future status: %v", err)
	}
	if futureStatus != "pending" || futureAttempts != 0 {
		t.Fatalf("future status/attempts = %q/%d, want pending/0", futureStatus, futureAttempts)
	}
}

func TestPostgresStoreFailureRetryAndExhaustion(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	db := openMigratedOutboxPostgresTestDB(t)
	store := NewPostgresStore(db, 2)

	var eventID string
	if err := db.QueryRowContext(ctx, `
INSERT INTO outbox (topic, partition_key, payload, status, attempts, available_at)
VALUES ('mail.event', 'retry', '{"event":"retry"}'::jsonb, 'processing', 1, now())
RETURNING id::text`).Scan(&eventID); err != nil {
		t.Fatalf("insert retry outbox row: %v", err)
	}

	if err := store.MarkFailed(ctx, eventID, errors.New(strings.Repeat("한", 1200))); err != nil {
		t.Fatalf("MarkFailed returned error: %v", err)
	}

	var status string
	var lastError string
	var processedAt sql.NullTime
	if err := db.QueryRowContext(ctx, `SELECT status, last_error, processed_at FROM outbox WHERE id = $1`, eventID).Scan(&status, &lastError, &processedAt); err != nil {
		t.Fatalf("query failed outbox state: %v", err)
	}
	if status != "pending" || processedAt.Valid {
		t.Fatalf("status/processed_at = %q/%v, want pending/null", status, processedAt.Valid)
	}
	if len(lastError) > 2000 || !strings.HasPrefix(lastError, "한") {
		t.Fatalf("last_error length/prefix = %d/%q", len(lastError), lastError)
	}

	events, err := store.FetchPending(ctx, 1)
	if err != nil {
		t.Fatalf("FetchPending retry returned error: %v", err)
	}
	if len(events) != 1 || events[0].ID != eventID {
		t.Fatalf("retry events = %+v, want id %q", events, eventID)
	}
	if err := store.MarkFailed(ctx, eventID, errors.New("redis down")); err != nil {
		t.Fatalf("MarkFailed exhausted returned error: %v", err)
	}
	if err := db.QueryRowContext(ctx, `SELECT status, processed_at FROM outbox WHERE id = $1`, eventID).Scan(&status, &processedAt); err != nil {
		t.Fatalf("query exhausted outbox state: %v", err)
	}
	if status != "failed" || !processedAt.Valid {
		t.Fatalf("status/processed_at = %q/%v, want failed/non-null", status, processedAt.Valid)
	}
}

func openMigratedOutboxPostgresTestDB(t *testing.T) *sql.DB {
	t.Helper()

	baseURL := strings.TrimSpace(os.Getenv("GOGOMAIL_TEST_DATABASE_URL"))
	if baseURL == "" {
		t.Skip("set GOGOMAIL_TEST_DATABASE_URL to run PostgreSQL outbox integration tests")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	t.Cleanup(cancel)

	adminDB, err := sql.Open("pgx", baseURL)
	if err != nil {
		t.Fatalf("open postgres admin connection: %v", err)
	}
	t.Cleanup(func() { _ = adminDB.Close() })

	schema := fmt.Sprintf("gogomail_outbox_test_%d", time.Now().UnixNano())
	if _, err := adminDB.ExecContext(ctx, `CREATE SCHEMA `+schema); err != nil {
		t.Fatalf("create test schema: %v", err)
	}
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cleanupCancel()
		_, _ = adminDB.ExecContext(cleanupCtx, `DROP SCHEMA IF EXISTS `+schema+` CASCADE`)
	})

	dbURL := outboxPostgresURLWithSearchPath(t, baseURL, schema)
	db, err := database.Open(ctx, dbURL)
	if err != nil {
		t.Fatalf("open postgres test database: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	migrationDir, err := filepath.Abs(filepath.Join("..", "..", "migrations"))
	if err != nil {
		t.Fatalf("resolve migration directory: %v", err)
	}
	if err := database.MigrateUp(ctx, db, migrationDir); err != nil {
		t.Fatalf("migrate postgres test database: %v", err)
	}
	return db
}

func outboxPostgresURLWithSearchPath(t *testing.T, rawURL string, schema string) string {
	t.Helper()

	parsed, err := url.Parse(rawURL)
	if err != nil {
		t.Fatalf("parse GOGOMAIL_TEST_DATABASE_URL: %v", err)
	}
	query := parsed.Query()
	options := strings.TrimSpace(query.Get("options"))
	if options != "" {
		options += " "
	}
	options += "-c search_path=" + schema + ",public"
	query.Set("options", options)
	parsed.RawQuery = query.Encode()
	return parsed.String()
}
