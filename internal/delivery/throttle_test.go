package delivery

import (
	"context"
	"reflect"
	"testing"

	"github.com/gogomail/gogomail/internal/outbound"
)

func TestInMemoryThrottlerLimitsFarmConcurrency(t *testing.T) {
	throttler := NewInMemoryThrottler(ThrottlePolicy{
		FarmMaxConcurrent: map[outbound.Farm]int{outbound.FarmBulk: 1},
	})
	job := Job{QueuedMessage: QueuedMessage{Farm: outbound.FarmBulk, To: []outbound.Address{{Email: "a@example.com"}}}}

	release, err := throttler.Acquire(context.Background(), job)
	if err != nil {
		t.Fatalf("first Acquire() error = %v", err)
	}
	if _, err := throttler.Acquire(context.Background(), job); err == nil {
		t.Fatal("second Acquire() error = nil, want throttle")
	}
	release()
	if release2, err := throttler.Acquire(context.Background(), job); err != nil {
		t.Fatalf("Acquire() after release error = %v", err)
	} else {
		release2()
	}
}

func TestInMemoryThrottlerLimitsRecipientDomain(t *testing.T) {
	throttler := NewInMemoryThrottler(ThrottlePolicy{
		DomainMaxConcurrent: map[string]int{"example.net": 1},
	})
	first := Job{QueuedMessage: QueuedMessage{Farm: outbound.FarmGeneral, To: []outbound.Address{{Email: "a@example.net"}}}}
	second := Job{QueuedMessage: QueuedMessage{Farm: outbound.FarmGeneral, To: []outbound.Address{{Email: "b@example.net"}}}}

	release, err := throttler.Acquire(context.Background(), first)
	if err != nil {
		t.Fatalf("first Acquire() error = %v", err)
	}
	defer release()
	if _, err := throttler.Acquire(context.Background(), second); err == nil {
		t.Fatal("second Acquire() error = nil, want domain throttle")
	}
}

func TestInMemoryThrottlerReleaseIsIdempotent(t *testing.T) {
	throttler := NewInMemoryThrottler(ThrottlePolicy{DefaultConcurrent: 1})
	job := Job{QueuedMessage: QueuedMessage{Farm: outbound.FarmGeneral, To: []outbound.Address{{Email: "a@example.com"}}}}

	release, err := throttler.Acquire(context.Background(), job)
	if err != nil {
		t.Fatalf("Acquire() error = %v", err)
	}
	release()
	release()
	if release2, err := throttler.Acquire(context.Background(), job); err != nil {
		t.Fatalf("Acquire() after duplicate release error = %v", err)
	} else {
		release2()
	}
}

func TestCoordinatedThrottlerSharesCounterAcrossWorkers(t *testing.T) {
	counter := NewLocalThrottleCounter()
	policy := ThrottlePolicy{
		FarmMaxConcurrent:   map[outbound.Farm]int{outbound.FarmBulk: 1},
		DomainMaxConcurrent: map[string]int{"example.net": 1},
	}
	workerA := NewCoordinatedThrottler(policy, counter)
	workerB := NewCoordinatedThrottler(policy, counter)
	bulk := Job{QueuedMessage: QueuedMessage{Farm: outbound.FarmBulk, To: []outbound.Address{{Email: "first@example.org"}}}}

	release, err := workerA.Acquire(context.Background(), bulk)
	if err != nil {
		t.Fatalf("workerA Acquire() error = %v", err)
	}
	if _, err := workerB.Acquire(context.Background(), bulk); err == nil {
		t.Fatal("workerB Acquire() error = nil, want shared farm throttle")
	}
	release()
	if release2, err := workerB.Acquire(context.Background(), bulk); err != nil {
		t.Fatalf("workerB Acquire() after release error = %v", err)
	} else {
		release2()
	}

	transactional := Job{QueuedMessage: QueuedMessage{Farm: outbound.FarmTransactional, To: []outbound.Address{{Email: "one@example.net"}}}}
	general := Job{QueuedMessage: QueuedMessage{Farm: outbound.FarmGeneral, To: []outbound.Address{{Email: "two@example.net"}}}}
	releaseDomain, err := workerA.Acquire(context.Background(), transactional)
	if err != nil {
		t.Fatalf("transactional Acquire() error = %v", err)
	}
	defer releaseDomain()
	if _, err := workerB.Acquire(context.Background(), general); err == nil {
		t.Fatal("general Acquire() error = nil, want shared domain throttle across farms")
	}
}

func TestThrottleKeysAreDeterministicAcrossFarmsAndDomains(t *testing.T) {
	tests := []struct {
		name string
		farm outbound.Farm
		to   []outbound.Address
		want []string
	}{
		{
			name: "transactional domain dedupe",
			farm: outbound.FarmTransactional,
			to: []outbound.Address{
				{Email: "one@Example.NET"},
				{Email: "two@example.net"},
			},
			want: []string{"farm:transactional", "domain:example.net"},
		},
		{
			name: "general mixed domains",
			farm: outbound.FarmGeneral,
			to: []outbound.Address{
				{Email: "one@example.org"},
				{Email: "two@example.net"},
			},
			want: []string{"farm:general", "domain:example.org", "domain:example.net"},
		},
		{
			name: "bulk invalid recipient skipped",
			farm: outbound.FarmBulk,
			to: []outbound.Address{
				{Email: "no-at-symbol"},
				{Email: "bulk@example.com"},
			},
			want: []string{"farm:bulk", "domain:example.com"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			job := Job{QueuedMessage: QueuedMessage{Farm: tt.farm, To: tt.to}}
			if got := throttleKeys(job); !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("throttleKeys() = %v, want %v", got, tt.want)
			}
		})
	}
}
