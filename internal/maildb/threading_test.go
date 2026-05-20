package maildb

import (
	"strings"
	"testing"
)

func TestThreadCandidatesPrefersReferencesThenInReplyTo(t *testing.T) {
	t.Parallel()

	got := threadCandidates("<parent@example.com>", []string{"<root@example.com>", "<parent@example.com>"})
	if strings.Join(got, ",") != "<root@example.com>,<parent@example.com>,<parent@example.com>" {
		t.Fatalf("threadCandidates = %v", got)
	}
}

func TestNormalizeThreadCandidatesWrapsAndDeduplicates(t *testing.T) {
	t.Parallel()

	got := normalizeThreadCandidates([]string{" root@example.com ", "<ROOT@example.com>", "child@example.com"})
	if strings.Join(got, ",") != "<root@example.com>,<child@example.com>" {
		t.Fatalf("normalizeThreadCandidates = %v", got)
	}
}

func TestResolveThreadIDSQLUsesOrdinalityArray(t *testing.T) {
	t.Parallel()

	for _, want := range []string{
		"unnest($2::text[]) WITH ORDINALITY",
		"JOIN requested ON requested.rfc_message_id = messages.rfc_message_id",
		"ORDER BY requested.ordinality",
	} {
		if !strings.Contains(resolveThreadIDSQL, want) {
			t.Fatalf("resolveThreadIDSQL does not include %q:\n%s", want, resolveThreadIDSQL)
		}
	}
	if strings.Contains(resolveThreadIDSQL, "array_position") {
		t.Fatalf("resolveThreadIDSQL still asks PostgreSQL to rescan candidate arrays:\n%s", resolveThreadIDSQL)
	}
}
