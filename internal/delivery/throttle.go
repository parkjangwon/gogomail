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

type InMemoryThrottler struct {
	mu     sync.Mutex
	policy ThrottlePolicy
	used   map[string]int
}

func NewInMemoryThrottler(policy ThrottlePolicy) *InMemoryThrottler {
	return &InMemoryThrottler{policy: normalizeThrottlePolicy(policy), used: make(map[string]int)}
}

func (t *InMemoryThrottler) Acquire(_ context.Context, job Job) (func(), error) {
	keys := throttleKeys(job)
	t.mu.Lock()
	defer t.mu.Unlock()
	for _, key := range keys {
		limit := t.limitFor(key)
		if limit > 0 && t.used[key] >= limit {
			return nil, &ThrottleError{Key: key, Limit: limit}
		}
	}
	for _, key := range keys {
		t.used[key]++
	}
	var once sync.Once
	return func() {
		once.Do(func() {
			t.mu.Lock()
			defer t.mu.Unlock()
			for _, key := range keys {
				if t.used[key] > 0 {
					t.used[key]--
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

func (t *InMemoryThrottler) limitFor(key string) int {
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
