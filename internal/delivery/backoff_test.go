package delivery

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/gogomail/gogomail/internal/outbound"
)

func TestInMemoryDomainBackoffDefersOnlyFailedDomain(t *testing.T) {
	now := time.Date(2026, 5, 14, 12, 0, 0, 0, time.UTC)
	backoff := NewInMemoryDomainBackoff(DomainBackoffPolicy{BaseDelay: time.Minute})
	backoff.now = func() time.Time { return now }

	backoff.ObserveTemporaryFailure(context.Background(), Job{}, []outbound.Address{{Email: "user@example.net"}}, errors.New("tempfail"))
	if err := backoff.Check(context.Background(), Job{QueuedMessage: QueuedMessage{
		To: []outbound.Address{{Email: "other@example.net"}},
	}}); err == nil {
		t.Fatal("Check() error = nil, want backed off domain")
	}
	if err := backoff.Check(context.Background(), Job{QueuedMessage: QueuedMessage{
		Farm: outbound.FarmTransactional,
		To:   []outbound.Address{{Email: "user@example.org"}},
	}}); err != nil {
		t.Fatalf("Check() unrelated domain error = %v, want nil", err)
	}
}

func TestInMemoryDomainBackoffExpiresAndCapsDelay(t *testing.T) {
	now := time.Date(2026, 5, 14, 12, 0, 0, 0, time.UTC)
	backoff := NewInMemoryDomainBackoff(DomainBackoffPolicy{
		BaseDelay: time.Minute,
		MaxDelay:  2 * time.Minute,
	})
	backoff.now = func() time.Time { return now }
	recipient := []outbound.Address{{Email: "user@example.net"}}

	backoff.ObserveTemporaryFailure(context.Background(), Job{}, recipient, errors.New("first"))
	backoff.ObserveTemporaryFailure(context.Background(), Job{}, recipient, errors.New("second"))
	backoff.ObserveTemporaryFailure(context.Background(), Job{}, recipient, errors.New("third"))

	now = now.Add(2*time.Minute - time.Second)
	if err := backoff.Check(context.Background(), Job{QueuedMessage: QueuedMessage{To: recipient}}); err == nil {
		t.Fatal("Check() error = nil before capped delay expires")
	}
	now = now.Add(time.Second)
	if err := backoff.Check(context.Background(), Job{QueuedMessage: QueuedMessage{To: recipient}}); err != nil {
		t.Fatalf("Check() after capped delay = %v, want nil", err)
	}
}

func TestDomainsForRecipientsDeduplicatesAndNormalizes(t *testing.T) {
	got := domainsForRecipients([]outbound.Address{
		{Email: "one@Example.NET"},
		{Email: "two@example.net"},
		{Email: "missing-at"},
		{Email: "user@example.org"},
	})
	if len(got) != 2 || got[0] != "example.net" || got[1] != "example.org" {
		t.Fatalf("domainsForRecipients = %v, want normalized unique domains", got)
	}
}
