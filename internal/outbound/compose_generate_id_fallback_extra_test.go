package outbound

import (
	"strings"
	"testing"
)

func TestGenerateMessageIDFallsBackToLocalhostDomain(t *testing.T) {
	id := GenerateMessageID(" \t ")
	if !strings.HasPrefix(id, "<") || !strings.HasSuffix(id, "@localhost>") {
		t.Fatalf("GenerateMessageID = %q, want localhost message ID", id)
	}
}
