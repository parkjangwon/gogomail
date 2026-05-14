package delivery

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/gogomail/gogomail/internal/outbound"
)

type Throttler interface {
	Acquire(ctx context.Context, job Job) (release func(), err error)
}

type ThrottlePolicy struct {
	FarmMaxConcurrent   map[outbound.Farm]int
	DomainMaxConcurrent map[string]int
	DefaultConcurrent   int
}

type ThrottleLease struct {
	Key   string
	Limit int
}

type ThrottleCounter interface {
	Acquire(ctx context.Context, leases []ThrottleLease) (release func(), err error)
}

type CoordinatedThrottler struct {
	policy  ThrottlePolicy
	counter ThrottleCounter
}

type LocalThrottleCounter struct {
	mu   sync.Mutex
	used map[string]int
}

type InMemoryThrottler struct {
	*CoordinatedThrottler
}

func NewInMemoryThrottler(policy ThrottlePolicy) *InMemoryThrottler {
	return &InMemoryThrottler{CoordinatedThrottler: NewCoordinatedThrottler(policy, NewLocalThrottleCounter())}
}

func NewCoordinatedThrottler(policy ThrottlePolicy, counter ThrottleCounter) *CoordinatedThrottler {
	if counter == nil {
		counter = NewLocalThrottleCounter()
	}
	return &CoordinatedThrottler{policy: normalizeThrottlePolicy(policy), counter: counter}
}

func NewLocalThrottleCounter() *LocalThrottleCounter {
	return &LocalThrottleCounter{used: make(map[string]int)}
}

func (t *CoordinatedThrottler) Acquire(ctx context.Context, job Job) (func(), error) {
	leases := t.leasesFor(job)
	if len(leases) == 0 {
		return func() {}, nil
	}
	return t.counter.Acquire(ctx, leases)
}

func (c *LocalThrottleCounter) Acquire(_ context.Context, leases []ThrottleLease) (func(), error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	for _, lease := range leases {
		if lease.Limit > 0 && c.used[lease.Key] >= lease.Limit {
			return nil, &ThrottleError{Key: lease.Key, Limit: lease.Limit}
		}
	}
	for _, lease := range leases {
		c.used[lease.Key]++
	}
	var once sync.Once
	return func() {
		once.Do(func() {
			c.mu.Lock()
			defer c.mu.Unlock()
			for _, lease := range leases {
				if c.used[lease.Key] > 0 {
					c.used[lease.Key]--
				}
			}
		})
	}, nil
}

type ThrottleError struct {
	Key   string
	Limit int
}

func (e *ThrottleError) Error() string {
	return fmt.Sprintf("delivery throttled for %s at limit %d", e.Key, e.Limit)
}

func throttleKeys(job Job) []string {
	keys := []string{"farm:" + string(job.Farm)}
	for _, recipient := range job.Recipients() {
		if domain := domainFromAddress(recipient.Email); domain != "" {
			keys = append(keys, "domain:"+domain)
		}
	}
	return dedupeStrings(keys)
}

func (t *CoordinatedThrottler) leasesFor(job Job) []ThrottleLease {
	keys := throttleKeys(job)
	leases := make([]ThrottleLease, 0, len(keys))
	for _, key := range keys {
		limit := t.limitFor(key)
		if limit > 0 {
			leases = append(leases, ThrottleLease{Key: key, Limit: limit})
		}
	}
	return leases
}

func (t *CoordinatedThrottler) limitFor(key string) int {
	if farm, ok := strings.CutPrefix(key, "farm:"); ok {
		if limit := t.policy.FarmMaxConcurrent[outbound.Farm(farm)]; limit > 0 {
			return limit
		}
	}
	if domain, ok := strings.CutPrefix(key, "domain:"); ok {
		if limit := t.policy.DomainMaxConcurrent[domain]; limit > 0 {
			return limit
		}
	}
	return t.policy.DefaultConcurrent
}

func normalizeThrottlePolicy(policy ThrottlePolicy) ThrottlePolicy {
	if policy.DefaultConcurrent < 0 {
		policy.DefaultConcurrent = 0
	}
	policy.FarmMaxConcurrent = copyFarmLimits(policy.FarmMaxConcurrent)
	policy.DomainMaxConcurrent = copyDomainLimits(policy.DomainMaxConcurrent)
	return policy
}

func copyFarmLimits(in map[outbound.Farm]int) map[outbound.Farm]int {
	out := make(map[outbound.Farm]int, len(in))
	for farm, limit := range in {
		if limit > 0 {
			out[farm] = limit
		}
	}
	return out
}

func copyDomainLimits(in map[string]int) map[string]int {
	out := make(map[string]int, len(in))
	for domain, limit := range in {
		domain = strings.ToLower(strings.TrimSpace(domain))
		if domain != "" && limit > 0 {
			out[domain] = limit
		}
	}
	return out
}

func domainFromAddress(address string) string {
	_, domain, ok := strings.Cut(strings.TrimSpace(address), "@")
	if !ok {
		return ""
	}
	return strings.ToLower(domain)
}

func dedupeStrings(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	out := values[:0]
	for _, value := range values {
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	return out
}
