package deltasync

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

func TestPostgresCursorStoreSaveAndGet(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	db := openMigratedDeltaSyncTestDB(t)
	store := NewPostgresCursorStore(db)

	cursor := &Cursor{
		ID:        "c1",
		DeviceID:  "dev-1",
		UserID:    "user-1",
		MailboxID: "inbox",
		Version:   42,
		CreatedAt: time.Now(),
	}
	if err := store.Save(ctx, cursor); err != nil {
		t.Fatalf("Save: %v", err)
	}

	got, err := store.Get(ctx, "dev-1", "inbox")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Version != 42 || got.DeviceID != "dev-1" || got.UserID != "user-1" {
		t.Fatalf("cursor = %+v", got)
	}
}

func TestPostgresCursorStoreUpdate(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	db := openMigratedDeltaSyncTestDB(t)
	store := NewPostgresCursorStore(db)

	cursor := &Cursor{ID: "c1", DeviceID: "dev-1", UserID: "user-1", MailboxID: "inbox", Version: 1}
	if err := store.Save(ctx, cursor); err != nil {
		t.Fatalf("Save v1: %v", err)
	}

	cursor.Version = 10
	if err := store.Save(ctx, cursor); err != nil {
		t.Fatalf("Save v10: %v", err)
	}

	got, err := store.Get(ctx, "dev-1", "inbox")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Version != 10 {
		t.Fatalf("expected version 10, got %d", got.Version)
	}
}

func TestPostgresCursorStoreListByMailbox(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	db := openMigratedDeltaSyncTestDB(t)
	store := NewPostgresCursorStore(db)

	for i := 1; i <= 3; i++ {
		c := &Cursor{
			ID:        fmt.Sprintf("c%d", i),
			DeviceID:  fmt.Sprintf("dev-%d", i),
			UserID:    "user-1",
			MailboxID: "inbox",
			Version:   int64(i),
		}
		if err := store.Save(ctx, c); err != nil {
			t.Fatalf("Save c%d: %v", i, err)
		}
	}
	// different mailbox
	if err := store.Save(ctx, &Cursor{ID: "cx", DeviceID: "dev-x", UserID: "user-1", MailboxID: "sent", Version: 99}); err != nil {
		t.Fatalf("Save sent: %v", err)
	}

	cursors, err := store.ListByMailbox(ctx, "inbox")
	if err != nil {
		t.Fatalf("ListByMailbox: %v", err)
	}
	if len(cursors) != 3 {
		t.Fatalf("expected 3 cursors, got %d", len(cursors))
	}
}

func TestPostgresCursorStoreDelete(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	db := openMigratedDeltaSyncTestDB(t)
	store := NewPostgresCursorStore(db)

	c := &Cursor{ID: "c1", DeviceID: "dev-1", UserID: "user-1", MailboxID: "inbox", Version: 5}
	if err := store.Save(ctx, c); err != nil {
		t.Fatalf("Save: %v", err)
	}
	if err := store.Delete(ctx, "c1"); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	_, err := store.Get(ctx, "dev-1", "inbox")
	if err == nil {
		t.Fatalf("expected error after delete, got nil")
	}
}

func TestPostgresCursorStoreGetNotFound(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	db := openMigratedDeltaSyncTestDB(t)
	store := NewPostgresCursorStore(db)

	_, err := store.Get(ctx, "no-device", "no-mailbox")
	if err == nil {
		t.Fatalf("expected error for missing cursor")
	}
}

func openMigratedDeltaSyncTestDB(t *testing.T) *sql.DB {
	t.Helper()

	baseURL := strings.TrimSpace(os.Getenv("GOGOMAIL_TEST_DATABASE_URL"))
	if baseURL == "" {
		t.Skip("set GOGOMAIL_TEST_DATABASE_URL to run PostgreSQL delta-sync integration tests")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	t.Cleanup(cancel)

	adminDB, err := sql.Open("pgx", baseURL)
	if err != nil {
		t.Fatalf("open postgres admin connection: %v", err)
	}
	t.Cleanup(func() { _ = adminDB.Close() })

	schema := fmt.Sprintf("gogomail_deltasync_test_%d", time.Now().UnixNano())
	if _, err := adminDB.ExecContext(ctx, `CREATE SCHEMA `+schema); err != nil {
		t.Fatalf("create test schema: %v", err)
	}
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cleanupCancel()
		_, _ = adminDB.ExecContext(cleanupCtx, `DROP SCHEMA IF EXISTS `+schema+` CASCADE`)
	})

	dbURL := deltaSyncTestURLWithSearchPath(t, baseURL, schema)
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

func deltaSyncTestURLWithSearchPath(t *testing.T, rawURL string, schema string) string {
	t.Helper()

	parsed, err := url.Parse(rawURL)
	if err != nil {
		t.Fatalf("parse GOGOMAIL_TEST_DATABASE_URL: %v", err)
	}
	query := parsed.Query()
	options := strings.TrimSpace(query.Get("options"))
	searchPathOption := "-c search_path=" + schema + ",public"
	if options != "" {
		options += " "
	}
	options += searchPathOption
	query.Set("options", options)
	parsed.RawQuery = query.Encode()
	return parsed.String()
}
