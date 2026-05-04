package maildb

import (
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
