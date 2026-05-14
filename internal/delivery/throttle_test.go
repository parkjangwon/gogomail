package delivery

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/gogomail/gogomail/internal/outbound"
	"github.com/redis/go-redis/v9"
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

func TestRedisThrottleCounterAcquireIsAtomicAcrossKeys(t *testing.T) {
	client := newFakeThrottleRedis()
	counter := NewRedisThrottleCounter(client, "test")

	release, err := counter.Acquire(context.Background(), []ThrottleLease{
		{Key: "farm:bulk", Limit: 2},
		{Key: "domain:example.net", Limit: 1},
	})
	if err != nil {
		t.Fatalf("first Acquire() error = %v", err)
	}
	if client.used["test:farm:bulk"] != 1 || client.used["test:domain:example.net"] != 1 {
		t.Fatalf("used after first acquire = %+v, want both keys incremented", client.used)
	}

	if _, err := counter.Acquire(context.Background(), []ThrottleLease{
		{Key: "farm:bulk", Limit: 2},
		{Key: "domain:example.net", Limit: 1},
	}); err == nil {
		t.Fatal("second Acquire() error = nil, want domain throttle")
	}
	if client.used["test:farm:bulk"] != 1 || client.used["test:domain:example.net"] != 1 {
		t.Fatalf("used after failed acquire = %+v, want no partial increment", client.used)
	}

	release()
	if client.used["test:farm:bulk"] != 0 || client.used["test:domain:example.net"] != 0 {
		t.Fatalf("used after release = %+v, want both keys released", client.used)
	}
}

func TestRedisThrottleCounterReleaseIsIdempotent(t *testing.T) {
	client := newFakeThrottleRedis()
	counter := NewRedisThrottleCounter(client, "test")

	release, err := counter.Acquire(context.Background(), []ThrottleLease{{Key: "farm:general", Limit: 1}})
	if err != nil {
		t.Fatalf("Acquire() error = %v", err)
	}
	release()
	release()
	if client.used["test:farm:general"] != 0 {
		t.Fatalf("used after duplicate release = %+v, want zero", client.used)
	}
}

type fakeThrottleRedis struct {
	used map[string]int
}

func newFakeThrottleRedis() *fakeThrottleRedis {
	return &fakeThrottleRedis{used: make(map[string]int)}
}

func (f *fakeThrottleRedis) Eval(ctx context.Context, script string, keys []string, args ...interface{}) *redis.Cmd {
	if strings.Contains(script, "redis.call('INCR'") {
		for i, key := range keys {
			limit, ok := args[i].(int)
			if !ok {
				return redis.NewCmdResult(nil, errors.New("limit arg is not int"))
			}
			if limit > 0 && f.used[key] >= limit {
				return redis.NewCmdResult([]interface{}{int64(0), key, int64(limit)}, nil)
			}
		}
		for _, key := range keys {
			f.used[key]++
		}
		return redis.NewCmdResult([]interface{}{int64(1), "", int64(0)}, nil)
	}
	for _, key := range keys {
		if f.used[key] > 0 {
			f.used[key]--
		}
	}
	return redis.NewCmdResult(int64(1), nil)
}
