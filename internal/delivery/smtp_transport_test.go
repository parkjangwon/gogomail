package delivery

import (
	"testing"

	"github.com/gogomail/gogomail/internal/outbound"
)

func TestGroupRecipientsByDomain(t *testing.T) {
	t.Parallel()

	groups := groupRecipientsByDomain([]outbound.Address{
		{Email: "a@example.com"},
		{Email: "b@example.com"},
		{Email: "c@example.net"},
	})
	if len(groups["example.com"]) != 2 {
		t.Fatalf("example.com recipients = %d, want 2", len(groups["example.com"]))
	}
	if len(groups["example.net"]) != 1 {
		t.Fatalf("example.net recipients = %d, want 1", len(groups["example.net"]))
	}
}
