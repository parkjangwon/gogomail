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

func TestLDAPSyncRunsSQLUsesStableOrdering(t *testing.T) {
	t.Parallel()

	query, args := buildLDAPSyncRunsSQL(LDAPSyncRunListRequest{
		DomainID: "11111111-1111-1111-1111-111111111111",
		Status:   "success",
		Limit:    50,
		Offset:   100,
	})
	for _, want := range []string{
		"WHERE domain_id = $1",
		"AND status = $2",
		"ORDER BY started_at DESC, id DESC",
		"LIMIT $3",
		"OFFSET $4",
	} {
		if !strings.Contains(query, want) {
			t.Fatalf("sync run query missing %q:\n%s", want, query)
		}
	}
	if len(args) != 4 {
		t.Fatalf("args length = %d, want 4", len(args))
	}
}

func TestLastLDAPSyncTimeSQLUsesStableOrdering(t *testing.T) {
	t.Parallel()

	if !strings.Contains(lastLDAPSyncTimeSQL, "ORDER BY last_success_at DESC, id DESC") {
		t.Fatalf("last sync query missing id tie-breaker:\n%s", lastLDAPSyncTimeSQL)
	}
}
