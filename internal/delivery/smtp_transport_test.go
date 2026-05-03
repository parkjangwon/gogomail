package delivery

import (
	"context"
	"errors"
	"net"
	"strings"
	"testing"
	"time"

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

func TestOrderedMXHostsSkipsNilAndEmptyHosts(t *testing.T) {
	t.Parallel()

	hosts := orderedMXHosts([]*net.MX{
		nil,
		{Host: ".", Pref: 10},
		{Host: "mx.example.net.", Pref: 20},
	})
	if len(hosts) != 1 || hosts[0] != "mx.example.net" {
		t.Fatalf("hosts = %+v, want only valid MX host", hosts)
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

func TestDirectSMTPTransportUsesRouterHostsBeforeMX(t *testing.T) {
	t.Parallel()

	transport := DirectSMTPTransport{
		Router: staticRouter{route: Route{Hosts: []string{"mx-route.example.net."}}},
		Resolver: staticMXResolver{
			records: []*net.MX{{Host: "mx-dns.example.net.", Pref: 10}},
		},
	}
	route, err := transport.route(context.Background(), Job{QueuedMessage: QueuedMessage{Farm: "general"}}, "example.net")
	if err != nil {
		t.Fatalf("route returned error: %v", err)
	}
	if len(route.Hosts) != 1 || route.Hosts[0] != "mx-route.example.net" {
		t.Fatalf("route hosts = %+v, want router host", route.Hosts)
	}
}

func TestDirectSMTPTransportWrapsRouterError(t *testing.T) {
	t.Parallel()

	transport := DirectSMTPTransport{Router: staticRouter{err: errors.New("no farm")}}
	_, err := transport.route(context.Background(), Job{QueuedMessage: QueuedMessage{Farm: "bulk"}}, "example.net")
	if err == nil {
		t.Fatal("route returned nil error")
	}
	if !strings.Contains(err.Error(), "route delivery for example.net") {
		t.Fatalf("error = %v, want domain context", err)
	}
}

func TestPartialDeliveryErrorIsTerminalForMXFailover(t *testing.T) {
	t.Parallel()

	partial := &PartialDeliveryError{
		Delivered: []outbound.Address{{Email: "ok@example.net"}},
		Failed:    []RecipientDeliveryError{{Recipient: outbound.Address{Email: "bad@example.net"}, Err: &SMTPStatusError{Op: "rcpt", Code: 451, Message: "try later"}}},
	}
	errs := []error{partial}
	transport := DirectSMTPTransport{
		Router: staticRouter{route: Route{Hosts: []string{"127.0.0.1", "127.0.0.2"}}},
		Timeout: time.Millisecond,
		deliverHost: func(context.Context, Job, Route, string, []outbound.Address) error {
			err := errs[0]
			errs = errs[1:]
			return err
		},
	}

	err := transport.deliverDomain(context.Background(), Job{QueuedMessage: QueuedMessage{Farm: "general"}}, "example.net", []outbound.Address{{Email: "ok@example.net"}, {Email: "bad@example.net"}})
	if !errors.Is(err, partial) {
		t.Fatalf("deliverDomain error = %v, want partial delivery error", err)
	}
	if len(errs) != 0 {
		t.Fatalf("remaining stub errors = %+v, want no MX retry after partial DATA success", errs)
	}
}

func TestAcceptRecipientsContinuesAfterSingleRecipientFailure(t *testing.T) {
	t.Parallel()

	recipients := []outbound.Address{
		{Email: "ok@example.net"},
		{Email: "bad@example.net"},
		{Email: "also-ok@example.net"},
	}
	accepted, failures := acceptRecipients(recipients, func(recipient outbound.Address) error {
		if recipient.Email == "bad@example.net" {
			return &SMTPStatusError{Op: "rcpt", Code: 550, Message: "no such user"}
		}
		return nil
	})

	if len(failures) != 1 {
		t.Fatalf("recipient failures = %+v, want 1", failures)
	}
	if len(accepted) != 2 {
		t.Fatalf("accepted recipients = %+v, want 2", accepted)
	}
	if accepted[0].Email != "ok@example.net" || accepted[1].Email != "also-ok@example.net" {
		t.Fatalf("accepted recipients = %+v", accepted)
	}
}

func TestAcceptRecipientsReturnsErrorsWhenAllRecipientsFail(t *testing.T) {
	t.Parallel()

	accepted, failures := acceptRecipients([]outbound.Address{
		{Email: "bad@example.net"},
		{Email: "worse@example.net"},
	}, func(recipient outbound.Address) error {
		return &SMTPStatusError{Op: "rcpt", Code: 550, Message: recipient.Email + " rejected"}
	})

	if len(failures) != 2 {
		t.Fatalf("recipient failures = %+v, want 2", failures)
	}
	if len(accepted) != 0 {
		t.Fatalf("accepted recipients = %+v, want none", accepted)
	}
	joined := errors.Join(recipientFailureErrors(failures)...)
	if !strings.Contains(joined.Error(), "bad@example.net") || !strings.Contains(joined.Error(), "worse@example.net") {
		t.Fatalf("error = %v, want recipient context", joined)
	}
}

func TestDeliveryDeadlineUsesTimeout(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 5, 3, 12, 0, 0, 0, time.UTC)
	deadline := deliveryDeadline(context.Background(), 30*time.Second, now)
	if !deadline.Equal(now.Add(30 * time.Second)) {
		t.Fatalf("deadline = %s, want timeout deadline", deadline)
	}
}

func TestDeliveryDeadlinePrefersEarlierContextDeadline(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 5, 3, 12, 0, 0, 0, time.UTC)
	ctx, cancel := context.WithDeadline(context.Background(), now.Add(5*time.Second))
	defer cancel()

	deadline := deliveryDeadline(ctx, 30*time.Second, now)
	if !deadline.Equal(now.Add(5 * time.Second)) {
		t.Fatalf("deadline = %s, want earlier context deadline", deadline)
	}
}

func TestDeliveryDeadlineCanBeDisabled(t *testing.T) {
	t.Parallel()

	deadline := deliveryDeadline(context.Background(), 0, time.Now())
	if !deadline.IsZero() {
		t.Fatalf("deadline = %s, want zero deadline", deadline)
	}
}

type staticMXResolver struct {
	records []*net.MX
	err     error
}

type staticRouter struct {
	route Route
	err   error
}

func (r staticRouter) Route(context.Context, Job, string) (Route, error) {
	if r.err != nil {
		return Route{}, r.err
	}
	return r.route, nil
}

func (r staticMXResolver) LookupMX(context.Context, string) ([]*net.MX, error) {
	if r.err != nil {
		return nil, r.err
	}
	return r.records, nil
}
