package maildb

import (
	"strings"
	"testing"
	"time"
	"unicode/utf8"
)

func TestTruncateUTF8BytesKeepsValidString(t *testing.T) {
	t.Parallel()

	got := truncateUTF8Bytes(strings.Repeat("a", 511)+"한", outboxEventListErrorPreviewBytes)
	if len(got) > outboxEventListErrorPreviewBytes {
		t.Fatalf("len(got) = %d, want <= %d", len(got), outboxEventListErrorPreviewBytes)
	}
	if !utf8.ValidString(got) {
		t.Fatalf("got invalid UTF-8: %q", got)
	}
}

func TestListOutboxEventsQueryUsesSargableFilters(t *testing.T) {
	t.Parallel()

	query, args := buildListOutboxEventsQuery(OutboxEventListRequest{
		Topic:        "mail.event",
		PartitionKey: "message-1",
		Status:       "pending",
		Since:        time.Date(2026, 5, 21, 1, 2, 3, 0, time.FixedZone("KST", 9*60*60)),
	}, 101)

	for _, want := range []string{
		"WHERE topic = $1",
		"AND partition_key = $2",
		"AND status = $3",
		"AND created_at >= $4",
		"ORDER BY created_at DESC, id DESC",
		"LIMIT $5",
	} {
		if !strings.Contains(query, want) {
			t.Fatalf("list outbox query missing %q:\n%s", want, query)
		}
	}
	for _, forbidden := range []string{
		"NULLIF",
		" OR ",
		"$5::timestamptz IS NULL",
	} {
		if strings.Contains(query, forbidden) {
			t.Fatalf("list outbox query contains non-sargable filter %q:\n%s", forbidden, query)
		}
	}
	if len(args) != 5 {
		t.Fatalf("args len = %d, want 5", len(args))
	}
	if got := args[3].(time.Time).Location(); got != time.UTC {
		t.Fatalf("since arg location = %v, want UTC", got)
	}
}
