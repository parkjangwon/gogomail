package delivery

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/smtp"
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
		{Email: "d@Example.NET."},
	})
	if len(groups["example.com"]) != 2 {
		t.Fatalf("example.com recipients = %d, want 2", len(groups["example.com"]))
	}
	if len(groups["example.net"]) != 2 {
		t.Fatalf("example.net recipients = %d, want 2", len(groups["example.net"]))
	}
}

func TestPlanRecipientBatchesPreservesFirstSeenDomainOrder(t *testing.T) {
	t.Parallel()

	batches := PlanRecipientBatches([]outbound.Address{
		{Email: "a@example.net"},
		{Email: "b@example.com"},
		{Email: "c@Example.NET."},
		{Email: "d@example.org"},
		{Email: "e@example.com"},
	})

	wantDomains := []string{"example.net", "example.com", "example.org"}
	if len(batches) != len(wantDomains) {
		t.Fatalf("batches = %+v, want %d batches", batches, len(wantDomains))
	}
	for i, want := range wantDomains {
		if batches[i].Domain != want {
			t.Fatalf("batch %d domain = %q, want %q", i, batches[i].Domain, want)
		}
	}
	if got := []string{batches[0].Recipients[0].Email, batches[0].Recipients[1].Email}; got[0] != "a@example.net" || got[1] != "c@Example.NET." {
		t.Fatalf("example.net recipients = %+v, want original order", got)
	}
	if got := []string{batches[1].Recipients[0].Email, batches[1].Recipients[1].Email}; got[0] != "b@example.com" || got[1] != "e@example.com" {
		t.Fatalf("example.com recipients = %+v, want original order", got)
	}
}

func TestPlanRecipientBatchesSkipsInvalidRecipients(t *testing.T) {
	t.Parallel()

	batches := PlanRecipientBatches([]outbound.Address{
		{Email: "missing-domain"},
		{Email: "valid@example.com"},
		{Email: "empty@"},
		{Email: "trailing@example.com."},
	})
	if len(batches) != 1 {
		t.Fatalf("batches = %+v, want one deliverable domain batch", batches)
	}
	if batches[0].Domain != "example.com" {
		t.Fatalf("domain = %q, want example.com", batches[0].Domain)
	}
	if len(batches[0].Recipients) != 2 {
		t.Fatalf("recipients = %+v, want two valid recipients", batches[0].Recipients)
	}
}

func TestPlanRecipientBatchesWithLimitSplitsLargeDomainGroups(t *testing.T) {
	t.Parallel()

	batches := PlanRecipientBatchesWithLimit([]outbound.Address{
		{Email: "a@example.net"},
		{Email: "b@example.com"},
		{Email: "c@example.net"},
		{Email: "d@example.net"},
		{Email: "e@example.com"},
	}, 2)

	wantDomains := []string{"example.net", "example.net", "example.com"}
	if len(batches) != len(wantDomains) {
		t.Fatalf("batches = %+v, want %d batches", batches, len(wantDomains))
	}
	for i, want := range wantDomains {
		if batches[i].Domain != want {
			t.Fatalf("batch %d domain = %q, want %q", i, batches[i].Domain, want)
		}
		if len(batches[i].Recipients) > 2 {
			t.Fatalf("batch %d recipients = %d, want <= 2", i, len(batches[i].Recipients))
		}
	}
	wantEmails := [][]string{
		{"a@example.net", "c@example.net"},
		{"d@example.net"},
		{"b@example.com", "e@example.com"},
	}
	for i := range wantEmails {
		if len(batches[i].Recipients) != len(wantEmails[i]) {
			t.Fatalf("batch %d recipients = %+v, want %+v", i, batches[i].Recipients, wantEmails[i])
		}
		for j, want := range wantEmails[i] {
			if batches[i].Recipients[j].Email != want {
				t.Fatalf("batch %d recipient %d = %q, want %q", i, j, batches[i].Recipients[j].Email, want)
			}
		}
	}
}

func TestDirectSMTPTransportTLSReportDomainCanBeConfigured(t *testing.T) {
	t.Parallel()

	transport := NewDirectSMTPTransport()
	transport.TLSReportDomain = "mail.example.com"
	transport.recordTLSResult("mx.remote.example", "TLSv1.3", "TLS_AES_128_GCM_SHA256", nil)

	report := transport.tlsrptCollector.GenerateReport("Example Corp", "postmaster@example.com")
	if report.DomainName != "mail.example.com" {
		t.Fatalf("TLS-RPT report domain = %q, want configured SMTP domain", report.DomainName)
	}
}

func TestDirectSMTPTransportHonorsMaxRecipientsPerBatch(t *testing.T) {
	t.Parallel()

	var deliveredCounts []int
	transport := DirectSMTPTransport{
		MaxRecipientsPerBatch: 2,
		Router:                staticRouter{route: Route{Hosts: []string{"mx.example.test"}}},
		deliverHost: func(_ context.Context, _ Job, _ Route, _ string, recipients []outbound.Address) error {
			deliveredCounts = append(deliveredCounts, len(recipients))
			return nil
		},
	}

	err := transport.Deliver(context.Background(), Job{
		QueuedMessage: QueuedMessage{
			To: []outbound.Address{
				{Email: "a@example.net"},
				{Email: "b@example.net"},
				{Email: "c@example.net"},
			},
		},
	})
	if err != nil {
		t.Fatalf("Deliver returned error: %v", err)
	}

	wantCounts := []int{2, 1}
	if fmt.Sprint(deliveredCounts) != fmt.Sprint(wantCounts) {
		t.Fatalf("delivered counts = %+v, want %+v", deliveredCounts, wantCounts)
	}
}

func TestDirectSMTPTransportDeliversRecipientBatchesInPlannedOrder(t *testing.T) {
	t.Parallel()

	var deliveredDomains []string
	var deliveredCounts []int
	transport := DirectSMTPTransport{
		Router: staticRouter{route: Route{Hosts: []string{"mx.example.test"}}},
		deliverHost: func(_ context.Context, _ Job, _ Route, _ string, recipients []outbound.Address) error {
			if len(recipients) == 0 {
				t.Fatal("deliverHost received empty recipient batch")
			}
			domain := normalizedRecipientDomain(recipients[0].Email)
			deliveredDomains = append(deliveredDomains, domain)
			deliveredCounts = append(deliveredCounts, len(recipients))
			return nil
		},
	}

	err := transport.Deliver(context.Background(), Job{
		QueuedMessage: QueuedMessage{
			To: []outbound.Address{
				{Email: "a@example.net"},
				{Email: "b@example.com"},
				{Email: "c@example.net"},
				{Email: "d@example.org"},
			},
		},
	})
	if err != nil {
		t.Fatalf("Deliver returned error: %v", err)
	}

	wantDomains := []string{"example.net", "example.com", "example.org"}
	if fmt.Sprint(deliveredDomains) != fmt.Sprint(wantDomains) {
		t.Fatalf("delivered domains = %+v, want %+v", deliveredDomains, wantDomains)
	}
	wantCounts := []int{2, 1, 1}
	if fmt.Sprint(deliveredCounts) != fmt.Sprint(wantCounts) {
		t.Fatalf("delivered counts = %+v, want %+v", deliveredCounts, wantCounts)
	}
}

func TestDirectSMTPTransportRejectsNoDeliverableRecipients(t *testing.T) {
	t.Parallel()

	transport := NewDirectSMTPTransport()
	err := transport.Deliver(context.Background(), Job{
		QueuedMessage: QueuedMessage{
			To: []outbound.Address{{Email: "missing-domain"}},
		},
	})
	if err == nil {
		t.Fatal("Deliver accepted job with no deliverable recipients")
	}
	if !IsPermanentFailure(err) {
		t.Fatalf("err = %v, want permanent recipient failure", err)
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

func TestOrderedMXHostsLowercasesAndDeduplicates(t *testing.T) {
	t.Parallel()

	hosts := orderedMXHosts([]*net.MX{
		{Host: "MX.Example.NET.", Pref: 10},
		{Host: "mx.example.net", Pref: 10},
		{Host: "backup.example.net.", Pref: 20},
	})
	want := []string{"mx.example.net", "backup.example.net"}
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

func TestMXHostsTempfailsTemporaryDNSLookupErrors(t *testing.T) {
	t.Parallel()

	transport := DirectSMTPTransport{Resolver: staticMXResolver{err: &net.DNSError{
		Err:         "timeout",
		Name:        "example.net",
		IsTimeout:   true,
		IsTemporary: true,
	}}}
	_, err := transport.mxHosts(context.Background(), "example.net")
	if err == nil {
		t.Fatal("mxHosts accepted temporary DNS failure")
	}
	if !IsTemporaryFailure(err) {
		t.Fatalf("err = %v, want temporary SMTP failure", err)
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

func TestDirectSMTPTransportRouteInheritsGlobalTLSMode(t *testing.T) {
	t.Parallel()

	transport := DirectSMTPTransport{
		TLSMode: DeliveryTLSRequire,
		Router:  staticRouter{route: Route{Hosts: []string{"mx-route.example.net"}}},
	}
	route, err := transport.route(context.Background(), Job{QueuedMessage: QueuedMessage{Farm: "general"}}, "example.net")
	if err != nil {
		t.Fatalf("route returned error: %v", err)
	}
	if route.TLSMode != DeliveryTLSRequire {
		t.Fatalf("route TLSMode = %q, want inherited global require", route.TLSMode)
	}
}

func TestDirectSMTPTransportRouteOverrideTLSModeWins(t *testing.T) {
	t.Parallel()

	transport := DirectSMTPTransport{
		TLSMode: DeliveryTLSRequire,
		Router:  staticRouter{route: Route{Hosts: []string{"mx-route.example.net"}, TLSMode: DeliveryTLSDisable}},
	}
	route, err := transport.route(context.Background(), Job{QueuedMessage: QueuedMessage{Farm: "general"}}, "example.net")
	if err != nil {
		t.Fatalf("route returned error: %v", err)
	}
	if route.TLSMode != DeliveryTLSDisable {
		t.Fatalf("route TLSMode = %q, want route override disable", route.TLSMode)
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
		Router:  staticRouter{route: Route{Hosts: []string{"127.0.0.1", "127.0.0.2"}}},
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

func TestTemporaryAllRecipientFailureCanFailOverToNextMX(t *testing.T) {
	t.Parallel()

	hosts := []string{}
	transport := DirectSMTPTransport{
		Router: staticRouter{route: Route{Hosts: []string{"mx1.example.net", "mx2.example.net"}}},
		deliverHost: func(_ context.Context, _ Job, _ Route, host string, recipients []outbound.Address) error {
			hosts = append(hosts, host)
			if host == "mx1.example.net" {
				failed := make([]RecipientDeliveryError, 0, len(recipients))
				for _, recipient := range recipients {
					failed = append(failed, RecipientDeliveryError{
						Recipient: recipient,
						Err:       &SMTPStatusError{Op: "rcpt", Code: 451, Message: "4.7.1 try later"},
					})
				}
				return &PartialDeliveryError{Failed: failed}
			}
			return nil
		},
	}

	err := transport.deliverDomain(context.Background(), Job{QueuedMessage: QueuedMessage{Farm: "general"}}, "example.net", []outbound.Address{
		{Email: "one@example.net"},
		{Email: "two@example.net"},
	})
	if err != nil {
		t.Fatalf("deliverDomain returned error: %v", err)
	}
	if strings.Join(hosts, ",") != "mx1.example.net,mx2.example.net" {
		t.Fatalf("hosts = %+v, want failover to second MX", hosts)
	}
}

func TestDirectSMTPTransportAggregatesDomainPartialFailures(t *testing.T) {
	t.Parallel()

	transport := DirectSMTPTransport{
		Router: staticRouter{route: Route{Hosts: []string{"mx.example.net"}}},
		deliverHost: func(_ context.Context, _ Job, _ Route, _ string, recipients []outbound.Address) error {
			switch recipients[0].Email {
			case "ok@example.com":
				return nil
			case "temp@example.net":
				return &SMTPStatusError{Op: "data", Code: 451, Message: "try later"}
			case "good@example.org":
				return &PartialDeliveryError{
					Delivered: []outbound.Address{{Email: "good@example.org"}},
					Failed: []RecipientDeliveryError{{
						Recipient: outbound.Address{Email: "bad@example.org"},
						Err:       &SMTPStatusError{Op: "rcpt", Code: 550, Message: "no such user"},
					}},
				}
			default:
				return nil
			}
		},
	}
	err := transport.Deliver(context.Background(), Job{QueuedMessage: QueuedMessage{
		To: []outbound.Address{
			{Email: "ok@example.com"},
			{Email: "temp@example.net"},
			{Email: "good@example.org"},
			{Email: "bad@example.org"},
		},
	}})
	var partial *PartialDeliveryError
	if !errors.As(err, &partial) {
		t.Fatalf("Deliver error = %v, want PartialDeliveryError", err)
	}
	if len(partial.Delivered) != 2 {
		t.Fatalf("delivered = %+v, want 2 recipients", partial.Delivered)
	}
	if len(partial.Failed) != 2 {
		t.Fatalf("failed = %+v, want 2 recipients", partial.Failed)
	}
	if got := partial.TemporaryFailures(); len(got) != 1 || got[0].Email != "temp@example.net" {
		t.Fatalf("temporary failures = %+v, want temp@example.net only", got)
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

func TestRecipientsRejectedResultKeepsPerRecipientFailureClasses(t *testing.T) {
	t.Parallel()

	err := recipientsRejectedResult([]RecipientDeliveryError{
		{Recipient: outbound.Address{Email: "gone@example.net"}, Err: &SMTPStatusError{Op: "rcpt", Code: 550, Message: "5.1.1 gone"}},
		{Recipient: outbound.Address{Email: "temp@example.net"}, Err: &SMTPStatusError{Op: "rcpt", Code: 451, Message: "4.7.1 try later"}},
	})
	var partial *PartialDeliveryError
	if !errors.As(err, &partial) {
		t.Fatalf("recipientsRejectedResult error = %v, want PartialDeliveryError", err)
	}
	if len(partial.Delivered) != 0 {
		t.Fatalf("delivered = %+v, want none", partial.Delivered)
	}
	if len(partial.Failed) != 2 {
		t.Fatalf("failed = %+v, want both rejected recipients", partial.Failed)
	}
	if got := partial.TemporaryFailures(); len(got) != 1 || got[0].Email != "temp@example.net" {
		t.Fatalf("temporary failures = %+v, want temp@example.net only", got)
	}
}

func TestDataAcceptedResultKeepsPartialRecipientFailures(t *testing.T) {
	t.Parallel()

	err := dataAcceptedResult(
		[]outbound.Address{{Email: "ok@example.net"}},
		[]RecipientDeliveryError{{
			Recipient: outbound.Address{Email: "bad@example.net"},
			Err:       &SMTPStatusError{Op: "rcpt", Code: 550, Message: "no such user"},
		}},
	)
	var partial *PartialDeliveryError
	if !errors.As(err, &partial) {
		t.Fatalf("dataAcceptedResult error = %v, want PartialDeliveryError", err)
	}
	if len(partial.Delivered) != 1 || partial.Delivered[0].Email != "ok@example.net" {
		t.Fatalf("delivered = %+v, want accepted DATA recipients", partial.Delivered)
	}
	if len(partial.Failed) != 1 || partial.Failed[0].Recipient.Email != "bad@example.net" {
		t.Fatalf("failed = %+v, want rejected RCPT recipient", partial.Failed)
	}
}

func TestDirectSMTPTransportClassifiesRecipientFailuresFromSMTPSink(t *testing.T) {
	t.Parallel()

	addr, shutdown := startRecipientFailureSMTPSink(t)
	defer shutdown()
	host, portValue, err := net.SplitHostPort(addr)
	if err != nil {
		t.Fatalf("SplitHostPort returned error: %v", err)
	}
	var port int
	if _, err := fmt.Sscanf(portValue, "%d", &port); err != nil {
		t.Fatalf("parse sink port %q: %v", portValue, err)
	}
	transport := DirectSMTPTransport{
		Router: staticRouter{route: Route{
			Domain:  "example.net",
			Hosts:   []string{host},
			Port:    port,
			TLSMode: DeliveryTLSDisable,
		}},
		Hello:   "sender.example.com",
		Timeout: 5 * time.Second,
	}
	err = transport.Deliver(context.Background(), Job{QueuedMessage: QueuedMessage{
		From:        outbound.Address{Email: "sender@example.com"},
		To:          []outbound.Address{{Email: "ok@example.net"}, {Email: "gone@example.net"}, {Email: "temp@example.net"}},
		StoragePath: "mailstore/msg.eml",
	}, OpenMessage: func(context.Context) (io.ReadCloser, error) {
		return io.NopCloser(strings.NewReader("Subject: sink\r\n\r\nbody\r\n")), nil
	}})
	var partial *PartialDeliveryError
	if !errors.As(err, &partial) {
		t.Fatalf("Deliver error = %v, want PartialDeliveryError", err)
	}
	if len(partial.Delivered) != 1 || partial.Delivered[0].Email != "ok@example.net" {
		t.Fatalf("delivered = %+v, want accepted recipient", partial.Delivered)
	}
	if len(partial.Failed) != 2 {
		t.Fatalf("failed = %+v, want permanent and temporary RCPT failures", partial.Failed)
	}
	if got := partial.TemporaryFailures(); len(got) != 1 || got[0].Email != "temp@example.net" {
		t.Fatalf("temporary failures = %+v, want temp@example.net only", got)
	}
}

func TestDataAcceptedResultSucceedsWhenAllAccepted(t *testing.T) {
	t.Parallel()

	if err := dataAcceptedResult([]outbound.Address{{Email: "ok@example.net"}}, nil); err != nil {
		t.Fatalf("dataAcceptedResult returned error after accepted DATA: %v", err)
	}
}

func TestDSNOptionsForRecipientBuildsRCPTParameters(t *testing.T) {
	t.Parallel()

	options := dsnOptionsForRecipient([]DSNRecipientOptions{{
		Address:           "user@example.net",
		Notify:            []string{"FAILURE", "DELAY"},
		OriginalRecipient: "rfc822;user+40example.net",
	}}, "USER@example.net")

	if strings.Join(options, " ") != "NOTIFY=FAILURE,DELAY ORCPT=rfc822;user+40example.net" {
		t.Fatalf("options = %+v, want DSN RCPT parameters", options)
	}
}

func TestDSNOptionsForRecipientSkipsUnmatchedRecipient(t *testing.T) {
	t.Parallel()

	options := dsnOptionsForRecipient([]DSNRecipientOptions{{
		Address: "other@example.net",
		Notify:  []string{"FAILURE"},
	}}, "user@example.net")
	if len(options) != 0 {
		t.Fatalf("options = %+v, want no DSN parameters for unmatched recipient", options)
	}
}

func TestDSNRCPTOptionsByAddressPreservesFirstNormalizedMatch(t *testing.T) {
	t.Parallel()

	optionsByAddress := dsnRCPTOptionsByAddress([]DSNRecipientOptions{
		{
			Address:           " User@Example.NET ",
			Notify:            []string{"SUCCESS"},
			OriginalRecipient: "rfc822;first@example.net",
		},
		{
			Address:           "user@example.net",
			Notify:            []string{"FAILURE"},
			OriginalRecipient: "rfc822;second@example.net",
		},
	})
	options := dsnOptionsForRecipientMap(optionsByAddress, "USER@example.net")
	if strings.Join(options, " ") != "NOTIFY=SUCCESS ORCPT=rfc822;first@example.net" {
		t.Fatalf("options = %+v, want first normalized match", options)
	}
}

func TestNullReversePathSuppressesOutboundDSNMailOptions(t *testing.T) {
	t.Parallel()

	job := Job{QueuedMessage: QueuedMessage{
		From: outbound.Address{Email: ""},
		DSN:  DSNOptions{Return: "FULL", EnvelopeID: "env-1"},
	}}
	if shouldSendOutboundDSNMailOptions(job) {
		t.Fatal("null reverse-path should not request DSN options")
	}
	if shouldSendOutboundDSNRcptOptions(job) {
		t.Fatal("null reverse-path should not request recipient DSN options")
	}
}

func TestSMTPDSNOptionsAreSentOnWireWhenAdvertised(t *testing.T) {
	t.Parallel()

	clientConn, serverConn := net.Pipe()
	defer clientConn.Close()
	defer serverConn.Close()

	lines := make(chan string, 2)
	errs := make(chan error, 1)
	go func() {
		reader := bufio.NewReader(serverConn)
		if _, err := fmt.Fprintf(serverConn, "220 mx.example.net ESMTP\r\n"); err != nil {
			errs <- err
			return
		}
		if line, err := reader.ReadString('\n'); err != nil || !strings.HasPrefix(line, "EHLO ") {
			errs <- fmt.Errorf("EHLO line = %q, err = %v", line, err)
			return
		}
		if _, err := fmt.Fprintf(serverConn, "250-mx.example.net\r\n250-DSN\r\n250 8BITMIME\r\n"); err != nil {
			errs <- err
			return
		}
		for i := 0; i < 2; i++ {
			line, err := reader.ReadString('\n')
			if err != nil {
				errs <- err
				return
			}
			lines <- strings.TrimRight(line, "\r\n")
			if _, err := fmt.Fprintf(serverConn, "250 ok\r\n"); err != nil {
				errs <- err
				return
			}
		}
		errs <- nil
	}()

	client, err := smtp.NewClient(clientConn, "mx.example.net")
	if err != nil {
		t.Fatalf("NewClient returned error: %v", err)
	}
	defer client.Close()
	if err := client.Hello("sender.example.com"); err != nil {
		t.Fatalf("Hello returned error: %v", err)
	}
	job := Job{QueuedMessage: QueuedMessage{
		From: outbound.Address{Email: "sender@example.com"},
		DSN: DSNOptions{
			Return:     "FULL",
			EnvelopeID: "env+2D1",
			Recipients: []DSNRecipientOptions{{
				Address:           "user@example.net",
				Notify:            []string{"FAILURE", "DELAY"},
				OriginalRecipient: "rfc822;user+40example.net",
			}},
		},
	}}
	if err := smtpMail(client, job); err != nil {
		t.Fatalf("smtpMail returned error: %v", err)
	}
	if err := smtpRcpt(client, job, outbound.Address{Email: "user@example.net"}); err != nil {
		t.Fatalf("smtpRcpt returned error: %v", err)
	}
	if err := <-errs; err != nil {
		t.Fatalf("fake SMTP server error: %v", err)
	}

	mailLine := <-lines
	rcptLine := <-lines
	if mailLine != "MAIL FROM:<sender@example.com> BODY=8BITMIME RET=FULL ENVID=env+2D1" {
		t.Fatalf("MAIL line = %q", mailLine)
	}
	if rcptLine != "RCPT TO:<user@example.net> NOTIFY=FAILURE,DELAY ORCPT=rfc822;user+40example.net" {
		t.Fatalf("RCPT line = %q", rcptLine)
	}
}

func TestSMTPDSNOptionsAreSuppressedWhenNotAdvertised(t *testing.T) {
	t.Parallel()

	clientConn, serverConn := net.Pipe()
	defer clientConn.Close()
	defer serverConn.Close()

	lines := make(chan string, 2)
	errs := make(chan error, 1)
	go func() {
		reader := bufio.NewReader(serverConn)
		if _, err := fmt.Fprintf(serverConn, "220 mx.example.net ESMTP\r\n"); err != nil {
			errs <- err
			return
		}
		if line, err := reader.ReadString('\n'); err != nil || !strings.HasPrefix(line, "EHLO ") {
			errs <- fmt.Errorf("EHLO line = %q, err = %v", line, err)
			return
		}
		if _, err := fmt.Fprintf(serverConn, "250-mx.example.net\r\n250 8BITMIME\r\n"); err != nil {
			errs <- err
			return
		}
		for i := 0; i < 2; i++ {
			line, err := reader.ReadString('\n')
			if err != nil {
				errs <- err
				return
			}
			lines <- strings.TrimRight(line, "\r\n")
			if _, err := fmt.Fprintf(serverConn, "250 ok\r\n"); err != nil {
				errs <- err
				return
			}
		}
		errs <- nil
	}()

	client, err := smtp.NewClient(clientConn, "mx.example.net")
	if err != nil {
		t.Fatalf("NewClient returned error: %v", err)
	}
	defer client.Close()
	if err := client.Hello("sender.example.com"); err != nil {
		t.Fatalf("Hello returned error: %v", err)
	}
	job := Job{QueuedMessage: QueuedMessage{
		From: outbound.Address{Email: "sender@example.com"},
		DSN: DSNOptions{
			Return:     "FULL",
			EnvelopeID: "env+2D1",
			Recipients: []DSNRecipientOptions{{
				Address:           "user@example.net",
				Notify:            []string{"FAILURE", "DELAY"},
				OriginalRecipient: "rfc822;user+40example.net",
			}},
		},
	}}
	if err := smtpMail(client, job); err != nil {
		t.Fatalf("smtpMail returned error: %v", err)
	}
	if err := smtpRcpt(client, job, outbound.Address{Email: "user@example.net"}); err != nil {
		t.Fatalf("smtpRcpt returned error: %v", err)
	}
	if err := <-errs; err != nil {
		t.Fatalf("fake SMTP server error: %v", err)
	}

	mailLine := <-lines
	rcptLine := <-lines
	if strings.Contains(mailLine, "RET=") || strings.Contains(mailLine, "ENVID=") {
		t.Fatalf("MAIL line = %q, want no DSN MAIL parameters", mailLine)
	}
	if rcptLine != "RCPT TO:<user@example.net>" {
		t.Fatalf("RCPT line = %q, want DSN-free RCPT", rcptLine)
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

func startRecipientFailureSMTPSink(t *testing.T) (string, func()) {
	t.Helper()

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Listen returned error: %v", err)
	}
	errCh := make(chan error, 1)
	go func() {
		conn, err := listener.Accept()
		if err != nil {
			errCh <- err
			return
		}
		defer conn.Close()
		reader := bufio.NewReader(conn)
		if _, err := fmt.Fprintf(conn, "220 mx.example.net ESMTP\r\n"); err != nil {
			errCh <- err
			return
		}
		if line, err := reader.ReadString('\n'); err != nil || !strings.HasPrefix(line, "EHLO ") {
			errCh <- fmt.Errorf("EHLO line = %q, err = %v", line, err)
			return
		}
		if _, err := fmt.Fprintf(conn, "250 mx.example.net\r\n"); err != nil {
			errCh <- err
			return
		}
		if line, err := reader.ReadString('\n'); err != nil || !strings.HasPrefix(line, "MAIL FROM:") {
			errCh <- fmt.Errorf("MAIL line = %q, err = %v", line, err)
			return
		}
		if _, err := fmt.Fprintf(conn, "250 sender ok\r\n"); err != nil {
			errCh <- err
			return
		}
		rcptReplies := []string{
			"250 recipient ok\r\n",
			"550 5.1.1 no such user\r\n",
			"451 4.7.1 try later\r\n",
		}
		for _, reply := range rcptReplies {
			if line, err := reader.ReadString('\n'); err != nil || !strings.HasPrefix(line, "RCPT TO:") {
				errCh <- fmt.Errorf("RCPT line = %q, err = %v", line, err)
				return
			}
			if _, err := fmt.Fprint(conn, reply); err != nil {
				errCh <- err
				return
			}
		}
		if line, err := reader.ReadString('\n'); err != nil || strings.TrimSpace(line) != "DATA" {
			errCh <- fmt.Errorf("DATA line = %q, err = %v", line, err)
			return
		}
		if _, err := fmt.Fprintf(conn, "354 end with dot\r\n"); err != nil {
			errCh <- err
			return
		}
		for {
			line, err := reader.ReadString('\n')
			if err != nil {
				errCh <- err
				return
			}
			if strings.TrimRight(line, "\r\n") == "." {
				break
			}
		}
		if _, err := fmt.Fprintf(conn, "250 accepted\r\n"); err != nil {
			errCh <- err
			return
		}
		// The transport may not send QUIT if it pools the connection for reuse.
		// Use a short deadline so the goroutine exits cleanly either way.
		_ = conn.SetDeadline(time.Now().Add(500 * time.Millisecond))
		if line, err := reader.ReadString('\n'); err == nil && strings.HasPrefix(line, "QUIT") {
			_, _ = fmt.Fprintf(conn, "221 bye\r\n")
		}
		errCh <- nil
	}()
	return listener.Addr().String(), func() {
		_ = listener.Close()
		if err := <-errCh; err != nil && !strings.Contains(err.Error(), "use of closed network connection") {
			t.Fatalf("SMTP sink returned error: %v", err)
		}
	}
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
