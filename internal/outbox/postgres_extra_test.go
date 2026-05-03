package outbox

import (
	"strings"
	"testing"
	"unicode/utf8"
)

func TestTruncateUTF8BytesPreservesValidStrings(t *testing.T) {
	t.Parallel()

	got := truncateUTF8Bytes(strings.Repeat("a", 1999)+"한", 2000)
	if len(got) > 2000 {
		t.Fatalf("length = %d, want <= 2000 bytes", len(got))
	}
	if !utf8.ValidString(got) {
		t.Fatalf("got invalid UTF-8: %q", got)
	}
}
