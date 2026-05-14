package delivery

import (
	"context"
	"sync"
	"time"

	"github.com/gogomail/gogomail/internal/outbound"
)

type DomainBackoffPolicy struct {
	BaseDelay time.Duration
	MaxDelay  time.Duration
}

type InMemoryDomainBackoff struct {
	mu     sync.Mutex
	policy DomainBackoffPolicy
	now    func() time.Time
	state  map[string]domainBackoffState
}

type domainBackoffState struct {
	failures int
	until    time.Time
}

func NewInMemoryDomainBackoff(policy DomainBackoffPolicy) *InMemoryDomainBackoff {
	return &InMemoryDomainBackoff{
		policy: normalizeDomainBackoffPolicy(policy),
		now:    time.Now,
		state:  make(map[string]domainBackoffState),
	}
}

func (b *InMemoryDomainBackoff) Check(_ context.Context, job Job) error {
	if b == nil {
		return nil
	}
	now := b.now().UTC()
	b.mu.Lock()
	defer b.mu.Unlock()
	for _, domain := range domainsForRecipients(job.Recipients()) {
		state := b.state[domain]
		if !state.until.IsZero() && now.Before(state.until) {
			return &DomainBackoffError{Domain: domain}
		}
	}
	return nil
}

func (b *InMemoryDomainBackoff) ObserveTemporaryFailure(_ context.Context, _ Job, recipients []outbound.Address, _ error) {
	if b == nil {
		return
	}
	now := b.now().UTC()
	b.mu.Lock()
	defer b.mu.Unlock()
	for _, domain := range domainsForRecipients(recipients) {
		state := b.state[domain]
		state.failures++
		state.until = now.Add(b.delayForFailures(state.failures))
		b.state[domain] = state
	}
}

func (b *InMemoryDomainBackoff) delayForFailures(failures int) time.Duration {
	if failures <= 1 {
		return b.policy.BaseDelay
	}
	delay := b.policy.BaseDelay
	for i := 1; i < failures; i++ {
		if b.policy.MaxDelay > 0 && delay >= b.policy.MaxDelay/2 {
			return b.policy.MaxDelay
		}
		delay *= 2
	}
	if b.policy.MaxDelay > 0 && delay > b.policy.MaxDelay {
		return b.policy.MaxDelay
	}
	return delay
}

func normalizeDomainBackoffPolicy(policy DomainBackoffPolicy) DomainBackoffPolicy {
	if policy.BaseDelay <= 0 {
		policy.BaseDelay = time.Minute
	}
	if policy.MaxDelay > 0 && policy.MaxDelay < policy.BaseDelay {
		policy.MaxDelay = policy.BaseDelay
	}
	return policy
}

func domainsForRecipients(recipients []outbound.Address) []string {
	domains := make([]string, 0, len(recipients))
	seen := make(map[string]struct{}, len(recipients))
	for _, recipient := range recipients {
		domain := domainFromAddress(recipient.Email)
		if domain == "" {
			continue
		}
		if _, ok := seen[domain]; ok {
			continue
		}
		seen[domain] = struct{}{}
		domains = append(domains, domain)
	}
	return domains
}
