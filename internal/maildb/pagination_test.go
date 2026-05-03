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
