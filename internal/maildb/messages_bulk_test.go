package maildb

import (
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/lib/pq"
)

func TestValidateBulkMessageFlagRequestRejectsDuplicateIDs(t *testing.T) {
	t.Parallel()

	err := ValidateBulkMessageFlagRequest(BulkMessageFlagRequest{
		UserID:     "user-1",
		MessageIDs: []string{"msg-1", "msg-1"},
		Flag:       "read",
		Value:      true,
	})
	if err == nil {
		t.Fatal("ValidateBulkMessageFlagRequest accepted duplicate message IDs")
	}
}

func TestValidateBulkMessageFlagRequestRejectsUnsafeIDs(t *testing.T) {
	t.Parallel()

	tests := [][]string{
		{"msg-1\r\nmsg-2"},
		{strings.Repeat("x", maxMailboxResourceIDBytes+1)},
	}
	for _, ids := range tests {
		err := ValidateBulkMessageFlagRequest(BulkMessageFlagRequest{
			UserID:     "user-1",
			MessageIDs: ids,
			Flag:       "read",
			Value:      true,
		})
		if err == nil {
			t.Fatalf("ValidateBulkMessageFlagRequest accepted unsafe ids %+v", ids)
		}
	}
}

func TestValidateBulkThreadFlagRequestRejectsDuplicateIDs(t *testing.T) {
	t.Parallel()

	err := ValidateBulkThreadFlagRequest(BulkThreadFlagRequest{
		UserID:    "user-1",
		ThreadIDs: []string{"thread-1", "thread-1"},
		Flag:      "read",
		Value:     true,
	})
	if err == nil {
		t.Fatal("ValidateBulkThreadFlagRequest accepted duplicate thread IDs")
	}
}

func TestValidateBulkThreadFlagRequestRejectsUnsafeIDs(t *testing.T) {
	t.Parallel()

	tests := [][]string{
		{"thread-1\r\nthread-2"},
		{strings.Repeat("x", maxMailboxResourceIDBytes+1)},
	}
	for _, ids := range tests {
		err := ValidateBulkThreadFlagRequest(BulkThreadFlagRequest{
			UserID:    "user-1",
			ThreadIDs: ids,
			Flag:      "read",
			Value:     true,
		})
		if err == nil {
			t.Fatalf("ValidateBulkThreadFlagRequest accepted unsafe ids %+v", ids)
		}
	}
}

func TestBulkSetThreadFlagSQLUpdatesActiveThreadMessages(t *testing.T) {
	t.Parallel()

	for _, want := range []string{
		"WITH requested AS (",
		"unnest($2::uuid[])",
		"JOIN requested ON messages.thread_id = requested.id",
		"JOIN requested ON messages.id = requested.id",
		"RETURNING id::text",
	} {
		if !strings.Contains(bulkSetThreadFlagSQL, want) {
			t.Fatalf("bulk thread flag SQL does not include %q:\n%s", want, bulkSetThreadFlagSQL)
		}
	}
}

func BenchmarkValidateBulkThreadIDs1K(b *testing.B) {
	benchValidateBulkThreadIDs(b, 1_000)
}

func BenchmarkValidateBulkThreadIDs10K(b *testing.B) {
	benchValidateBulkThreadIDs(b, 10_000)
}

func BenchmarkBulkThreadIDsArrayValue1K(b *testing.B) {
	benchBulkThreadIDsArrayValue(b, 1_000)
}

func BenchmarkBulkThreadIDsArrayValue10K(b *testing.B) {
	benchBulkThreadIDsArrayValue(b, 10_000)
}

func BenchmarkBulkMessageIDsArrayValue1K(b *testing.B) {
	benchBulkMessageIDsArrayValue(b, 1_000)
}

func BenchmarkBulkMessageIDsArrayValue10K(b *testing.B) {
	benchBulkMessageIDsArrayValue(b, 10_000)
}

func benchValidateBulkThreadIDs(b *testing.B, count int) {
	b.Helper()
	threadIDs := benchmarkBulkThreadIDs(count)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if err := validateBulkThreadIDs(threadIDs); err != nil {
			b.Fatalf("validateBulkThreadIDs returned error: %v", err)
		}
	}
}

func benchBulkThreadIDsArrayValue(b *testing.B, count int) {
	b.Helper()
	threadIDs := benchmarkBulkThreadIDs(count)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		value, err := pq.Array(threadIDs).Value()
		if err != nil {
			b.Fatalf("pq.Array.Value returned error: %v", err)
		}
		if value == nil {
			b.Fatal("pq.Array.Value returned nil")
		}
	}
}

func benchBulkMessageIDsArrayValue(b *testing.B, count int) {
	b.Helper()
	messageIDs := benchmarkBulkThreadIDs(count)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		value, err := pq.Array(messageIDs).Value()
		if err != nil {
			b.Fatalf("pq.Array.Value returned error: %v", err)
		}
		if value == nil {
			b.Fatal("pq.Array.Value returned nil")
		}
	}
}

func benchmarkBulkThreadIDs(count int) []string {
	ids := make([]string, 0, count)
	for len(ids) < count {
		ids = append(ids, uuid.NewString())
	}
	return ids
}

func TestListMessageIDsForThreadsSQLUsesUuidUnnest(t *testing.T) {
	t.Parallel()

	for _, want := range []string{
		"WITH requested AS (",
		"unnest($2::uuid[])",
		"JOIN requested ON messages.thread_id = requested.id",
		"JOIN requested ON messages.id = requested.id",
		"ORDER BY id",
	} {
		if !strings.Contains(listMessageIDsForThreadsSQL, want) {
			t.Fatalf("list message ids for threads SQL does not include %q:\n%s", want, listMessageIDsForThreadsSQL)
		}
	}
	if strings.Contains(listMessageIDsForThreadsSQL, "jsonb_array_elements_text") {
		t.Fatalf("listMessageIDsForThreadsSQL still uses JSON array expansion:\n%s", listMessageIDsForThreadsSQL)
	}
}

func TestValidateBulkThreadMoveRequestRejectsUnsafeIDs(t *testing.T) {
	t.Parallel()

	tests := []BulkThreadMoveRequest{
		{UserID: "user-1", FolderID: "folder-1", ThreadIDs: []string{"thread-1", "thread-1"}},
		{UserID: "user-1", FolderID: "folder-1", ThreadIDs: []string{"thread-1\r\nthread-2"}},
		{UserID: "user-1", FolderID: strings.Repeat("x", maxMailboxResourceIDBytes+1), ThreadIDs: []string{"thread-1"}},
	}
	for _, req := range tests {
		req := req
		t.Run(strings.Join(req.ThreadIDs, ","), func(t *testing.T) {
			t.Parallel()

			if err := ValidateBulkThreadMoveRequest(req); err == nil {
				t.Fatalf("ValidateBulkThreadMoveRequest accepted unsafe request %+v", req)
			}
		})
	}
}

func TestBulkMoveThreadsSQLUpdatesActiveThreadMessages(t *testing.T) {
	t.Parallel()

	for _, want := range []string{
		"COALESCE(thread_id, id)::text IN",
		"jsonb_array_elements_text($2::jsonb)",
		"EXISTS (",
		"RETURNING id::text",
	} {
		if !strings.Contains(bulkMoveThreadsSQL, want) {
			t.Fatalf("bulk thread move SQL does not include %q:\n%s", want, bulkMoveThreadsSQL)
		}
	}
}

func TestValidateBulkThreadDeleteRequestRejectsUnsafeIDs(t *testing.T) {
	t.Parallel()

	tests := []BulkThreadDeleteRequest{
		{UserID: "user-1", ThreadIDs: []string{"thread-1", "thread-1"}},
		{UserID: "user-1", ThreadIDs: []string{"thread-1\r\nthread-2"}},
		{UserID: "user-1", ThreadIDs: []string{strings.Repeat("x", maxMailboxResourceIDBytes+1)}},
	}
	for _, req := range tests {
		req := req
		t.Run(strings.Join(req.ThreadIDs, ","), func(t *testing.T) {
			t.Parallel()

			if err := ValidateBulkThreadDeleteRequest(req); err == nil {
				t.Fatalf("ValidateBulkThreadDeleteRequest accepted unsafe request %+v", req)
			}
		})
	}
}

func TestBulkDeleteThreadsSQLDeletesActiveThreadMessages(t *testing.T) {
	t.Parallel()

	for _, want := range []string{
		"COALESCE(thread_id, id)::text IN",
		"jsonb_array_elements_text($2::jsonb)",
		"status = 'deleted'",
		"RETURNING id::text, COALESCE(size, 0)",
	} {
		if !strings.Contains(bulkDeleteThreadsSQL, want) {
			t.Fatalf("bulk thread delete SQL does not include %q:\n%s", want, bulkDeleteThreadsSQL)
		}
	}
}

func TestValidateBulkThreadRestoreRequestRejectsUnsafeIDs(t *testing.T) {
	t.Parallel()

	tests := []BulkThreadRestoreRequest{
		{UserID: "user-1", ThreadIDs: []string{"thread-1", "thread-1"}},
		{UserID: "user-1", ThreadIDs: []string{"thread-1\r\nthread-2"}},
		{UserID: "user-1", ThreadIDs: []string{strings.Repeat("x", maxMailboxResourceIDBytes+1)}},
	}
	for _, req := range tests {
		req := req
		t.Run(strings.Join(req.ThreadIDs, ","), func(t *testing.T) {
			t.Parallel()

			if err := ValidateBulkThreadRestoreRequest(req); err == nil {
				t.Fatalf("ValidateBulkThreadRestoreRequest accepted unsafe request %+v", req)
			}
		})
	}
}

func TestBulkRestoreThreadsSQLLocksDeletedThreadMessages(t *testing.T) {
	t.Parallel()

	for _, want := range []string{
		"COALESCE(thread_id, id)::text IN",
		"jsonb_array_elements_text($2::jsonb)",
		"status = 'deleted'",
		"FOR UPDATE",
	} {
		if !strings.Contains(bulkRestoreThreadsSQL, want) {
			t.Fatalf("bulk thread restore SQL does not include %q:\n%s", want, bulkRestoreThreadsSQL)
		}
	}
}

func TestValidateBulkMessageMoveRequestRejectsTooManyIDs(t *testing.T) {
	t.Parallel()

	ids := make([]string, 501)
	for i := range ids {
		ids[i] = "msg"
	}
	err := ValidateBulkMessageMoveRequest(BulkMessageMoveRequest{
		UserID:     "user-1",
		FolderID:   "folder-1",
		MessageIDs: ids,
	})
	if err == nil {
		t.Fatal("ValidateBulkMessageMoveRequest accepted too many message IDs")
	}
}

func TestValidateBulkMessageMoveRequestRejectsUnsafeFolderID(t *testing.T) {
	t.Parallel()

	err := ValidateBulkMessageMoveRequest(BulkMessageMoveRequest{
		UserID:     "user-1",
		FolderID:   strings.Repeat("x", maxMailboxResourceIDBytes+1),
		MessageIDs: []string{"msg-1"},
	})
	if err == nil {
		t.Fatal("ValidateBulkMessageMoveRequest accepted oversized folder ID")
	}
}

func TestValidateBulkMessageDeleteRequestRequiresIDs(t *testing.T) {
	t.Parallel()

	err := ValidateBulkMessageDeleteRequest(BulkMessageDeleteRequest{UserID: "user-1"})
	if err == nil {
		t.Fatal("ValidateBulkMessageDeleteRequest accepted missing message IDs")
	}
}

func TestValidateBulkMessageRestoreRequestRejectsUnsafeIDs(t *testing.T) {
	t.Parallel()

	tests := []BulkMessageRestoreRequest{
		{UserID: "user-1", MessageIDs: []string{"msg-1", "msg-1"}},
		{UserID: "user-1", MessageIDs: []string{"msg-1\r\nmsg-2"}},
		{UserID: "user-1", MessageIDs: []string{strings.Repeat("x", maxMailboxResourceIDBytes+1)}},
	}
	for _, req := range tests {
		req := req
		t.Run(strings.Join(req.MessageIDs, ","), func(t *testing.T) {
			t.Parallel()

			if err := ValidateBulkMessageRestoreRequest(req); err == nil {
				t.Fatalf("ValidateBulkMessageRestoreRequest accepted unsafe request %+v", req)
			}
		})
	}
}
