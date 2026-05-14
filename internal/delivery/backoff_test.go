package delivery

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/gogomail/gogomail/internal/outbound"
	"github.com/redis/go-redis/v9"
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

func TestInMemoryDomainBackoffFarmDomainScopeIsolatesFarms(t *testing.T) {
	now := time.Date(2026, 5, 14, 12, 0, 0, 0, time.UTC)
	backoff := NewInMemoryDomainBackoff(DomainBackoffPolicy{
		BaseDelay: time.Minute,
		Scope:     DomainBackoffScopeFarmDomain,
	})
	backoff.now = func() time.Time { return now }
	recipient := []outbound.Address{{Email: "user@example.net"}}

	backoff.ObserveTemporaryFailure(context.Background(), Job{QueuedMessage: QueuedMessage{
		Farm: outbound.FarmBulk,
	}}, recipient, errors.New("tempfail"))
	if err := backoff.Check(context.Background(), Job{QueuedMessage: QueuedMessage{
		Farm: outbound.FarmBulk,
		To:   recipient,
	}}); err == nil {
		t.Fatal("Check() bulk farm error = nil, want backed off farm/domain")
	}
	if err := backoff.Check(context.Background(), Job{QueuedMessage: QueuedMessage{
		Farm: outbound.FarmTransactional,
		To:   recipient,
	}}); err != nil {
		t.Fatalf("Check() transactional farm error = %v, want nil", err)
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

func TestDomainBackoffKeysFarmDomainDeduplicatesAndNormalizes(t *testing.T) {
	got := domainBackoffKeys(Job{QueuedMessage: QueuedMessage{
		Farm: outbound.Farm(" BULK "),
	}}, []outbound.Address{
		{Email: "one@Example.NET"},
		{Email: "two@example.net"},
		{Email: "user@example.org"},
	}, DomainBackoffScopeFarmDomain)
	want := []string{"farm:bulk:domain:example.net", "farm:bulk:domain:example.org"}
	if len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
		t.Fatalf("domainBackoffKeys = %v, want %v", got, want)
	}
}

func TestRedisDomainBackoffDefersOnlyActiveDomain(t *testing.T) {
	client := newFakeDomainBackoffRedis()
	backoff := NewRedisDomainBackoff(client, "test", DomainBackoffPolicy{BaseDelay: time.Minute, MaxDelay: 10 * time.Minute})
	recipient := []outbound.Address{{Email: "user@example.net"}}

	backoff.ObserveTemporaryFailure(context.Background(), Job{}, recipient, errors.New("tempfail"))
	if err := backoff.Check(context.Background(), Job{QueuedMessage: QueuedMessage{To: recipient}}); err == nil {
		t.Fatal("Check() error = nil, want redis-backed domain backoff")
	}
	if err := backoff.Check(context.Background(), Job{QueuedMessage: QueuedMessage{
		To: []outbound.Address{{Email: "user@example.org"}},
	}}); err != nil {
		t.Fatalf("Check() unrelated domain error = %v, want nil", err)
	}
}

func TestRedisDomainBackoffFarmDomainScopeIsolatesFarms(t *testing.T) {
	client := newFakeDomainBackoffRedis()
	backoff := NewRedisDomainBackoff(client, "test", DomainBackoffPolicy{
		BaseDelay: time.Minute,
		MaxDelay:  10 * time.Minute,
		Scope:     DomainBackoffScopeFarmDomain,
	})
	recipient := []outbound.Address{{Email: "user@example.net"}}

	backoff.ObserveTemporaryFailure(context.Background(), Job{QueuedMessage: QueuedMessage{
		Farm: outbound.FarmBulk,
	}}, recipient, errors.New("tempfail"))
	if client.ttl["test:farm:bulk:domain:example.net"] != int64(time.Minute/time.Millisecond) {
		t.Fatalf("bulk key ttl = %dms, want 1m", client.ttl["test:farm:bulk:domain:example.net"])
	}
	if err := backoff.Check(context.Background(), Job{QueuedMessage: QueuedMessage{
		Farm: outbound.FarmBulk,
		To:   recipient,
	}}); err == nil {
		t.Fatal("Check() bulk farm error = nil, want redis-backed farm/domain backoff")
	}
	if err := backoff.Check(context.Background(), Job{QueuedMessage: QueuedMessage{
		Farm: outbound.FarmTransactional,
		To:   recipient,
	}}); err != nil {
		t.Fatalf("Check() transactional farm error = %v, want nil", err)
	}
}

func TestRedisDomainBackoffExtendsAndCapsDelay(t *testing.T) {
	client := newFakeDomainBackoffRedis()
	backoff := NewRedisDomainBackoff(client, "test", DomainBackoffPolicy{BaseDelay: time.Minute, MaxDelay: 2 * time.Minute})
	recipient := []outbound.Address{{Email: "user@example.net"}}

	backoff.ObserveTemporaryFailure(context.Background(), Job{}, recipient, errors.New("first"))
	if client.ttl["test:example.net"] != int64(time.Minute/time.Millisecond) {
		t.Fatalf("ttl after first = %dms, want 1m", client.ttl["test:example.net"])
	}
	backoff.ObserveTemporaryFailure(context.Background(), Job{}, recipient, errors.New("second"))
	if client.ttl["test:example.net"] != int64((2*time.Minute)/time.Millisecond) {
		t.Fatalf("ttl after second = %dms, want capped 2m", client.ttl["test:example.net"])
	}
	backoff.ObserveTemporaryFailure(context.Background(), Job{}, recipient, errors.New("third"))
	if client.ttl["test:example.net"] != int64((2*time.Minute)/time.Millisecond) {
		t.Fatalf("ttl after third = %dms, want still capped 2m", client.ttl["test:example.net"])
	}
}

type fakeDomainBackoffRedis struct {
	failures map[string]int64
	ttl      map[string]int64
}

func newFakeDomainBackoffRedis() *fakeDomainBackoffRedis {
	return &fakeDomainBackoffRedis{
		failures: make(map[string]int64),
		ttl:      make(map[string]int64),
	}
}

func (f *fakeDomainBackoffRedis) Eval(ctx context.Context, script string, keys []string, args ...interface{}) *redis.Cmd {
	if strings.Contains(script, "PTTL") {
		for _, key := range keys {
			if f.ttl[key] > 0 {
				return redis.NewCmdResult([]interface{}{int64(0), key, f.ttl[key]}, nil)
			}
		}
		return redis.NewCmdResult([]interface{}{int64(1), "", int64(0)}, nil)
	}
	base := int64(args[0].(int))
	maxDelay := int64(args[1].(int))
	var selected int64
	for _, key := range keys {
		f.failures[key]++
		delay := base
		for i := int64(2); i <= f.failures[key]; i++ {
			delay *= 2
			if maxDelay > 0 && delay >= maxDelay {
				delay = maxDelay
				break
			}
		}
		if maxDelay > 0 && delay > maxDelay {
			delay = maxDelay
		}
		f.ttl[key] = delay
		if delay > selected {
			selected = delay
		}
	}
	return redis.NewCmdResult(selected, nil)
}
