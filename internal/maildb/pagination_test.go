package maildb

import (
	"encoding/base64"
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestMessageListCursorRoundTrip(t *testing.T) {
	t.Parallel()

	want := MessageListCursor{At: time.Date(2026, 5, 4, 1, 2, 3, 0, time.UTC), ID: "11111111-1111-1111-1111-111111111111"}
	encoded, err := EncodeMessageListCursor(want)
	if err != nil {
		t.Fatalf("EncodeMessageListCursor returned error: %v", err)
	}
	got, err := DecodeMessageListCursor(encoded)
	if err != nil {
		t.Fatalf("DecodeMessageListCursor returned error: %v", err)
	}
	if !got.At.Equal(want.At) || got.ID != want.ID {
		t.Fatalf("cursor = %+v, want %+v", got, want)
	}
	raw, err := base64.RawURLEncoding.DecodeString(encoded)
	if err != nil {
		t.Fatalf("DecodeString returned error: %v", err)
	}
	if strings.Contains(string(raw), "{") || strings.Contains(string(raw), "created_at") || strings.Contains(string(raw), "at") {
		t.Fatalf("encoded cursor still uses JSON payload: %q", string(raw))
	}
}

func TestDecodeMessageListCursorAcceptsLegacyJSONPayload(t *testing.T) {
	t.Parallel()

	want := MessageListCursor{At: time.Date(2026, 5, 4, 1, 2, 3, 0, time.UTC), ID: "11111111-1111-1111-1111-111111111111"}
	raw, err := json.Marshal(want)
	if err != nil {
		t.Fatalf("json.Marshal returned error: %v", err)
	}
	encoded := base64.RawURLEncoding.EncodeToString(raw)
	got, err := DecodeMessageListCursor(encoded)
	if err != nil {
		t.Fatalf("DecodeMessageListCursor returned error: %v", err)
	}
	if !got.At.Equal(want.At) || got.ID != want.ID {
		t.Fatalf("cursor = %+v, want %+v", got, want)
	}
}

func TestThreadListCursorRoundTrip(t *testing.T) {
	t.Parallel()

	want := ThreadListCursor{At: time.Date(2026, 5, 4, 1, 2, 3, 0, time.UTC), ID: "11111111-1111-1111-1111-111111111111"}
	encoded, err := EncodeThreadListCursor(want)
	if err != nil {
		t.Fatalf("EncodeThreadListCursor returned error: %v", err)
	}
	got, err := DecodeThreadListCursor(encoded)
	if err != nil {
		t.Fatalf("DecodeThreadListCursor returned error: %v", err)
	}
	if !got.At.Equal(want.At) || got.ID != want.ID {
		t.Fatalf("cursor = %+v, want %+v", got, want)
	}
}

func TestDecodeMessageListCursorRejectsMalformedInput(t *testing.T) {
	t.Parallel()

	if _, err := DecodeMessageListCursor("not-base64"); err == nil {
		t.Fatal("DecodeMessageListCursor accepted malformed input")
	}
}

func TestDecodeMessageListCursorRejectsOversizedInput(t *testing.T) {
	t.Parallel()

	if _, err := DecodeMessageListCursor(strings.Repeat("a", MessageListCursorMaxBytes+1)); err == nil {
		t.Fatal("DecodeMessageListCursor accepted oversized input")
	}
}

func TestNormalizeMessageListLimitCapsLargeValues(t *testing.T) {
	t.Parallel()

	if got := NormalizeMessageListLimit(0); got != MessageListDefaultLimit {
		t.Fatalf("default limit = %d", got)
	}
	if got := NormalizeMessageListLimit(500); got != MessageListMaxLimit {
		t.Fatalf("max limit = %d", got)
	}
}

func TestNormalizeListSort(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		in   string
		want string
		ok   bool
	}{
		{name: "default", want: ListSortNewest, ok: true},
		{name: "newest", in: " newest ", want: ListSortNewest, ok: true},
		{name: "oldest", in: "OLDEST", want: ListSortOldest, ok: true},
		{name: "invalid", in: "sideways"},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, ok := NormalizeListSort(tt.in)
			if ok != tt.ok || got != tt.want {
				t.Fatalf("NormalizeListSort(%q) = %q, %v; want %q, %v", tt.in, got, ok, tt.want, tt.ok)
			}
		})
	}
}

func TestNewMessageListPageBuildsNextCursor(t *testing.T) {
	t.Parallel()

	ts := time.Date(2026, 5, 4, 1, 2, 3, 0, time.UTC)
	page, err := NewMessageListPage([]MessageSummary{
		{ID: "11111111-1111-1111-1111-111111111111", ReceivedAt: ts},
	}, 1)
	if err != nil {
		t.Fatalf("NewMessageListPage returned error: %v", err)
	}
	if page.HasMore || page.Limit != 1 || page.NextCursor == "" {
		t.Fatalf("page = %+v", page)
	}
	cursor, err := DecodeMessageListCursor(page.NextCursor)
	if err != nil {
		t.Fatalf("DecodeMessageListCursor returned error: %v", err)
	}
	if cursor.ID != "11111111-1111-1111-1111-111111111111" || !cursor.At.Equal(ts) {
		t.Fatalf("cursor = %+v", cursor)
	}
}

func TestNewThreadListPageBuildsNextCursor(t *testing.T) {
	t.Parallel()

	threads := []ThreadSummary{
		{ID: "11111111-1111-1111-1111-111111111111", LatestAt: time.Date(2026, 5, 4, 2, 0, 0, 0, time.UTC)},
		{ID: "22222222-2222-2222-2222-222222222222", LatestAt: time.Date(2026, 5, 4, 1, 0, 0, 0, time.UTC)},
	}
	page, err := NewThreadListPage(threads, 1)
	if err != nil {
		t.Fatalf("NewThreadListPage returned error: %v", err)
	}
	if !page.HasMore || len(page.Threads) != 1 || page.Threads[0].ID != "11111111-1111-1111-1111-111111111111" {
		t.Fatalf("page = %+v", page)
	}
	cursor, err := DecodeThreadListCursor(page.NextCursor)
	if err != nil {
		t.Fatalf("DecodeThreadListCursor returned error: %v", err)
	}
	if cursor.ID != "11111111-1111-1111-1111-111111111111" {
		t.Fatalf("cursor = %+v", cursor)
	}
}

func TestNewDraftListPageBuildsNextCursor(t *testing.T) {
	t.Parallel()

	drafts := []MessageDetail{
		{ID: "11111111-1111-1111-1111-111111111111", ReceivedAt: time.Date(2026, 5, 4, 2, 0, 0, 0, time.UTC)},
		{ID: "22222222-2222-2222-2222-222222222222", ReceivedAt: time.Date(2026, 5, 4, 1, 0, 0, 0, time.UTC)},
	}
	page, err := NewDraftListPage(drafts, 1)
	if err != nil {
		t.Fatalf("NewDraftListPage returned error: %v", err)
	}
	if !page.HasMore || len(page.Drafts) != 1 || page.Drafts[0].ID != "11111111-1111-1111-1111-111111111111" {
		t.Fatalf("page = %+v", page)
	}
	cursor, err := DecodeMessageListCursor(page.NextCursor)
	if err != nil {
		t.Fatalf("DecodeMessageListCursor returned error: %v", err)
	}
	if cursor.ID != "11111111-1111-1111-1111-111111111111" {
		t.Fatalf("cursor = %+v", cursor)
	}
}

func TestDecodeMessageListCursorRejectsNonUUIDID(t *testing.T) {
	t.Parallel()

	encoded, err := EncodeMessageListCursor(MessageListCursor{
		At: time.Date(2026, 5, 4, 1, 2, 3, 0, time.UTC),
		ID: "not-a-uuid",
	})
	if err != nil {
		t.Fatalf("EncodeMessageListCursor returned error: %v", err)
	}
	if _, err := DecodeMessageListCursor(encoded); err == nil {
		t.Fatal("DecodeMessageListCursor accepted non-UUID id")
	}
}

func TestNewMessageListPageTrimsLimitPlusOne(t *testing.T) {
	t.Parallel()

	ts := time.Date(2026, 5, 4, 1, 2, 3, 0, time.UTC)
	page, err := NewMessageListPage([]MessageSummary{
		{ID: "msg-1", ReceivedAt: ts},
		{ID: "msg-2", ReceivedAt: ts.Add(-time.Minute)},
	}, 1)
	if err != nil {
		t.Fatalf("NewMessageListPage returned error: %v", err)
	}
	if !page.HasMore || len(page.Messages) != 1 || page.Messages[0].ID != "msg-1" {
		t.Fatalf("page = %+v", page)
	}
}
