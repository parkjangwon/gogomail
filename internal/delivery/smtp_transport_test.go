package delivery

import (
	"context"
	"net"
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

func TestOrderedMXHostsSortsByPreferenceAndHost(t *testing.T) {
	t.Parallel()

	hosts := orderedMXHosts([]*net.MX{
		{Host: "mx-b.example.net.", Pref: 20},
		{Host: "mx-c.example.net.", Pref: 10},
		{Host: "mx-a.example.net.", Pref: 10},
	})
	want := []string{"mx-a.example.net", "mx-c.example.net", "mx-b.example.net"}
	if len(hosts) != len(want) {
		t.Fatalf("hosts = %+v, want %+v", hosts, want)
	}
	for i := range want {
		if hosts[i] != want[i] {
			t.Fatalf("hosts = %+v, want %+v", hosts, want)
		}
	}
}

func TestMXHostsRejectsNullMX(t *testing.T) {
	t.Parallel()

	transport := DirectSMTPTransport{Resolver: staticMXResolver{
		records: []*net.MX{{Host: ".", Pref: 0}},
	}}
	_, err := transport.mxHosts(context.Background(), "example.invalid")
	if err == nil {
		t.Fatal("mxHosts accepted null MX")
	}
	if !IsPermanentFailure(err) {
		t.Fatalf("err = %v, want permanent SMTP failure", err)
	}
}

func TestMXHostsFallsBackToDomainWhenMXLookupFails(t *testing.T) {
	t.Parallel()

	transport := DirectSMTPTransport{Resolver: staticMXResolver{err: net.ErrClosed}}
	hosts, err := transport.mxHosts(context.Background(), "example.net")
	if err != nil {
		t.Fatalf("mxHosts returned error: %v", err)
	}
	if len(hosts) != 1 || hosts[0] != "example.net" {
		t.Fatalf("hosts = %+v, want fallback domain", hosts)
	}
}

type staticMXResolver struct {
	records []*net.MX
	err     error
}

func (r staticMXResolver) LookupMX(context.Context, string) ([]*net.MX, error) {
	if r.err != nil {
		return nil, r.err
	}
	return r.records, nil
}
