package maildb

import (
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestLDAPSyncConflictCursorRoundTrip(t *testing.T) {
	t.Parallel()

	conflictID := uuid.MustParse("77777777-7777-7777-7777-777777777777")
	createdAt := time.Date(2026, 5, 21, 12, 0, 0, 0, time.UTC)
	encoded, err := EncodeLDAPSyncConflictCursor(LDAPSyncConflictView{ID: conflictID, CreatedAt: createdAt})
	if err != nil {
		t.Fatalf("EncodeLDAPSyncConflictCursor returned error: %v", err)
	}
	decoded, err := DecodeLDAPSyncConflictCursor(encoded)
	if err != nil {
		t.Fatalf("DecodeLDAPSyncConflictCursor returned error: %v", err)
	}
	if decoded.ID != conflictID || !decoded.CreatedAt.Equal(createdAt) {
		t.Fatalf("decoded cursor = %+v", decoded)
	}
}

func TestLDAPSyncConflictsSQLUsesSeekCursorWhenPresent(t *testing.T) {
	t.Parallel()

	cursor := LDAPSyncConflictCursor{
		CreatedAt: time.Date(2026, 5, 21, 12, 30, 0, 0, time.UTC),
		ID:        uuid.MustParse("88888888-8888-8888-8888-888888888888"),
	}
	query := buildLDAPSyncConflictsSQL(LDAPSyncConflictListRequest{
		SyncRunID:      "99999999-9999-9999-9999-999999999999",
		UnresolvedOnly: true,
		Cursor:         cursor,
	})
	for _, want := range []string{
		"AND sync_run_id = $3::uuid",
		"AND resolved_at IS NULL",
		"AND (created_at, id) < ($4::timestamptz, $5::uuid)",
		"ORDER BY created_at DESC, id DESC",
		"LIMIT $2",
	} {
		if !strings.Contains(query, want) {
			t.Fatalf("cursor query missing %q:\n%s", want, query)
		}
	}
	if strings.Contains(query, "OFFSET") {
		t.Fatalf("cursor query should not use OFFSET:\n%s", query)
	}

	query = buildLDAPSyncConflictsSQL(LDAPSyncConflictListRequest{})
	if !strings.Contains(query, "LIMIT $2 OFFSET $3") {
		t.Fatalf("offset query missing legacy pagination:\n%s", query)
	}
	if strings.Contains(query, "$4::uuid") {
		t.Fatalf("offset query unexpectedly includes cursor predicate:\n%s", query)
	}
}
