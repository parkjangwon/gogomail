package maildb

import (
	"testing"
	"time"
)

func TestMessageListCursorRoundTrip(t *testing.T) {
	t.Parallel()

	want := MessageListCursor{At: time.Date(2026, 5, 4, 1, 2, 3, 0, time.UTC), ID: "msg-1"}
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
		{ID: "msg-1", ReceivedAt: ts},
	}, 1)
	if err != nil {
		t.Fatalf("NewMessageListPage returned error: %v", err)
	}
	if !page.HasMore || page.Limit != 1 || page.NextCursor == "" {
		t.Fatalf("page = %+v", page)
	}
	cursor, err := DecodeMessageListCursor(page.NextCursor)
	if err != nil {
		t.Fatalf("DecodeMessageListCursor returned error: %v", err)
	}
	if cursor.ID != "msg-1" || !cursor.At.Equal(ts) {
		t.Fatalf("cursor = %+v", cursor)
	}
}
