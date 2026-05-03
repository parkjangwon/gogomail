package maildb

import "testing"

func TestNormalizeOutgoingIntent(t *testing.T) {
	t.Parallel()

	if got := normalizeOutgoingIntent(" Reply "); got != "reply" {
		t.Fatalf("reply intent = %q", got)
	}
	if got := normalizeOutgoingIntent("unknown"); got != "new" {
		t.Fatalf("unknown intent = %q", got)
	}
}
