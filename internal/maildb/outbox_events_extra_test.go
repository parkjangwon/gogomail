package maildb

import (
	"strings"
	"testing"
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
