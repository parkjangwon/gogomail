package outbox

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"unicode/utf8"
)

func TestTruncateUTF8BytesPreservesValidStrings(t *testing.T) {
	t.Parallel()

	got := truncateUTF8Bytes(strings.Repeat("a", 1999)+"한", 2000)
	if len(got) > 2000 {
		t.Fatalf("length = %d, want <= 2000 bytes", len(got))
	}
	if !utf8.ValidString(got) {
		t.Fatalf("got invalid UTF-8: %q", got)
	}
}

func TestFetchPendingLocksOutboxRowsAfterCandidateUnion(t *testing.T) {
	t.Parallel()

	if strings.Contains(fetchPendingSQL, "LIMIT $1\n  FOR UPDATE") {
		t.Fatalf("fetchPendingSQL locks the UNION candidate query directly:\n%s", fetchPendingSQL)
	}
	for _, want := range []string{
		"JOIN candidate ON candidate.id = o.id",
		"FOR UPDATE OF o SKIP LOCKED",
		"UPDATE outbox AS o",
	} {
		if !strings.Contains(fetchPendingSQL, want) {
			t.Fatalf("fetchPendingSQL missing %q:\n%s", want, fetchPendingSQL)
		}
	}
}

func TestFetchPendingUsesStableClaimOrdering(t *testing.T) {
	t.Parallel()

	for _, want := range []string{
		"ORDER BY created_at, id",
		"ORDER BY candidate.created_at, candidate.id",
	} {
		if !strings.Contains(fetchPendingSQL, want) {
			t.Fatalf("fetchPendingSQL missing stable order %q:\n%s", want, fetchPendingSQL)
		}
	}
}

func TestFetchPendingHasClaimIndexMigration(t *testing.T) {
	t.Parallel()

	path := filepath.Join("..", "..", "migrations", "0120_outbox_claim_indexes.sql")
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read claim index migration: %v", err)
	}
	sql := string(body)
	for _, want := range []string{
		"idx_outbox_pending_available_claim",
		"ON outbox (available_at, created_at, id)",
		"WHERE status = 'pending'",
		"idx_outbox_processing_locked_claim",
		"ON outbox (locked_at, created_at, id)",
		"WHERE status = 'processing'",
	} {
		if !strings.Contains(sql, want) {
			t.Fatalf("claim index migration missing %q:\n%s", want, sql)
		}
	}
}

func TestPostgresStoreImplementsBatchStore(t *testing.T) {
	t.Parallel()

	var _ BatchStore = (*PostgresStore)(nil)
}

func TestMarkFailedBatchSQLProjectsUnnestColumns(t *testing.T) {
	t.Parallel()

	for _, want := range []string{
		"SELECT id, last_error",
		"FROM unnest($1::uuid[], $3::text[]) AS input(id, last_error)",
	} {
		if !strings.Contains(markFailedBatchSQL, want) {
			t.Fatalf("markFailedBatchSQL missing %q:\n%s", want, markFailedBatchSQL)
		}
	}
	if strings.Contains(markFailedBatchSQL, "SELECT *") {
		t.Fatalf("markFailedBatchSQL still projects all unnest columns:\n%s", markFailedBatchSQL)
	}
}
