package maildb

import (
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestRDBMSSyncRunCursorRoundTrip(t *testing.T) {
	t.Parallel()

	runID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	createdAt := time.Date(2026, 5, 21, 8, 30, 0, 0, time.UTC)
	encoded, err := EncodeRDBMSSyncRunCursor(RDBMSSyncRunView{ID: runID, CreatedAt: createdAt})
	if err != nil {
		t.Fatalf("EncodeRDBMSSyncRunCursor returned error: %v", err)
	}
	decoded, err := DecodeRDBMSSyncRunCursor(encoded)
	if err != nil {
		t.Fatalf("DecodeRDBMSSyncRunCursor returned error: %v", err)
	}
	if decoded.ID != runID || !decoded.CreatedAt.Equal(createdAt) {
		t.Fatalf("decoded cursor = %+v", decoded)
	}
}

func TestRDBMSSyncRunsSQLUsesSeekCursorWhenPresent(t *testing.T) {
	t.Parallel()

	query := buildRDBMSSyncRunsSQL(true)
	for _, want := range []string{
		"AND (created_at, id) < ($3::timestamptz, $4::uuid)",
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

	query = buildRDBMSSyncRunsSQL(false)
	if !strings.Contains(query, "LIMIT $2 OFFSET $3") {
		t.Fatalf("offset query missing legacy pagination:\n%s", query)
	}
	if strings.Contains(query, "$4::uuid") {
		t.Fatalf("offset query unexpectedly includes cursor predicate:\n%s", query)
	}
}
