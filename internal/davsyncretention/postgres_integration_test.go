package davsyncretention

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

func TestPostgresDAVSyncRetentionRunsRoundTrip(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	db := openMigratedPostgresTestDB(t)
	repo := NewRepository(db)
	created := time.Date(2026, 5, 5, 1, 0, 0, 0, time.UTC)
	repo.now = func() time.Time { return created }

	completed, err := repo.RecordRun(ctx, RunRecord{
		Cutoff:            created.Add(-90 * 24 * time.Hour),
		Limit:             1000,
		DryRun:            true,
		Status:            RunStatusCompleted,
		CalDAVCandidates:  7,
		CardDAVCandidates: 11,
	})
	if err != nil {
		t.Fatalf("RecordRun completed returned error: %v", err)
	}
	failed, err := repo.RecordRun(ctx, RunRecord{
		Cutoff:            created.Add(-90 * 24 * time.Hour),
		Limit:             500,
		DryRun:            false,
		ConfirmReady:      true,
		Status:            RunStatusFailed,
		ErrorMessage:      "carddav failed\nwith detail",
		CalDAVCandidates:  7,
		CalDAVDeleted:     3,
		CardDAVCandidates: 0,
		CardDAVDeleted:    0,
	})
	if err != nil {
		t.Fatalf("RecordRun failed returned error: %v", err)
	}
	if strings.ContainsAny(failed.ErrorMessage, "\r\n") || !strings.Contains(failed.ErrorMessage, "with detail") {
		t.Fatalf("failed error message = %q", failed.ErrorMessage)
	}

	runs, err := repo.ListRuns(ctx, RunListRequest{
		Limit:       10,
		Status:      RunStatusFailed,
		CreatedFrom: created.Add(-time.Minute),
		CreatedTo:   created.Add(time.Minute),
	})
	if err != nil {
		t.Fatalf("ListRuns returned error: %v", err)
	}
	if len(runs) != 1 || runs[0].ID != failed.ID || runs[0].CalDAVDeleted != 3 || runs[0].Status != RunStatusFailed {
		t.Fatalf("failed runs = %+v", runs)
	}

	got, err := repo.GetRun(ctx, completed.ID)
	if err != nil {
		t.Fatalf("GetRun returned error: %v", err)
	}
	if got.ID != completed.ID || got.Status != RunStatusCompleted || got.ErrorMessage != "" || got.CardDAVCandidates != 11 {
		t.Fatalf("completed run = %+v", got)
	}
}

func openMigratedPostgresTestDB(t *testing.T) *sql.DB {
	t.Helper()

	baseURL := strings.TrimSpace(os.Getenv("GOGOMAIL_TEST_DATABASE_URL"))
	if baseURL == "" {
		t.Skip("set GOGOMAIL_TEST_DATABASE_URL to run PostgreSQL DAV sync retention integration tests")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	t.Cleanup(cancel)

	adminDB, err := sql.Open("pgx", baseURL)
	if err != nil {
		t.Fatalf("open postgres admin connection: %v", err)
	}
	t.Cleanup(func() { _ = adminDB.Close() })

	schema := fmt.Sprintf("gogomail_davsync_test_%d", time.Now().UnixNano())
	if _, err := adminDB.ExecContext(ctx, `CREATE SCHEMA `+schema); err != nil {
		t.Fatalf("create test schema: %v", err)
	}
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cleanupCancel()
		_, _ = adminDB.ExecContext(cleanupCtx, `DROP SCHEMA IF EXISTS `+schema+` CASCADE`)
	})

	dbURL := postgresURLWithSearchPath(t, baseURL, schema)
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

func postgresURLWithSearchPath(t *testing.T, rawURL string, schema string) string {
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
