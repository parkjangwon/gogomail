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
