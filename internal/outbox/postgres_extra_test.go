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

func TestFetchPendingLocksOutboxRowsAfterCandidateUnion(t *testing.T) {
	t.Parallel()

	if strings.Contains(fetchPendingSQL, "LIMIT $1\n  FOR UPDATE") {
		t.Fatalf("fetchPendingSQL locks the UNION candidate query directly:\n%s", fetchPendingSQL)
	}
	for _, want := range []string{
		"JOIN candidate ON candidate.id = o.id",
		"FOR UPDATE OF o SKIP LOCKED",
		"UPDATE outbox AS o",
	} {
		if !strings.Contains(fetchPendingSQL, want) {
			t.Fatalf("fetchPendingSQL missing %q:\n%s", want, fetchPendingSQL)
		}
	}
}
