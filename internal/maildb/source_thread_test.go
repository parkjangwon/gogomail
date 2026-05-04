package maildb

import (
	"strings"
	"testing"
)

func TestSourceThreadReferencesBuildsReplyChain(t *testing.T) {
	t.Parallel()

	source := SourceThreadView{
		MessageID: "<parent@example.com>",
		InReplyTo: "<root@example.com>",
	}
	if got := strings.Join(source.References(), " "); got != "<root@example.com> <parent@example.com>" {
		t.Fatalf("References = %q", got)
	}
}

func TestSourceThreadReferencesSkipsBlankAndDeduplicates(t *testing.T) {
	t.Parallel()

	source := SourceThreadView{
		MessageID: "parent@example.com",
		InReplyTo: "<PARENT@example.com>",
	}
	if got := strings.Join(source.References(), " "); got != "<PARENT@example.com>" {
		t.Fatalf("References = %q", got)
	}
}
