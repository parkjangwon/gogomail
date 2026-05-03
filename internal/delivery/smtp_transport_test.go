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

func TestNormalizeDeliveryTLSMode(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		in   DeliveryTLSMode
		want DeliveryTLSMode
	}{
		{name: "empty defaults opportunistic", in: "", want: DeliveryTLSOpportunistic},
		{name: "opportunistic", in: "opportunistic", want: DeliveryTLSOpportunistic},
		{name: "require", in: "require", want: DeliveryTLSRequire},
		{name: "disable", in: "disable", want: DeliveryTLSDisable},
		{name: "invalid defaults opportunistic", in: "bad", want: DeliveryTLSOpportunistic},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := normalizeDeliveryTLSMode(tt.in); got != tt.want {
				t.Fatalf("normalizeDeliveryTLSMode(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}
