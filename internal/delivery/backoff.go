package delivery

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/gogomail/gogomail/internal/outbound"
	"github.com/redis/go-redis/v9"
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

type redisDomainBackoffClient interface {
	Eval(ctx context.Context, script string, keys []string, args ...interface{}) *redis.Cmd
}

type RedisDomainBackoff struct {
	client redisDomainBackoffClient
	prefix string
	policy DomainBackoffPolicy
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

func NewRedisDomainBackoff(client redisDomainBackoffClient, prefix string, policy DomainBackoffPolicy) *RedisDomainBackoff {
	prefix = strings.TrimSpace(prefix)
	if prefix == "" {
		prefix = "gogomail:delivery:domain_backoff"
	}
	return &RedisDomainBackoff{
		client: client,
		prefix: strings.TrimRight(prefix, ":"),
		policy: normalizeDomainBackoffPolicy(policy),
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

const redisDomainBackoffCheckScript = `
for i = 1, #KEYS do
  local ttl = redis.call('PTTL', KEYS[i])
  if ttl > 0 then
    return {0, KEYS[i], ttl}
  end
end
return {1, '', 0}
`

const redisDomainBackoffObserveScript = `
local base = tonumber(ARGV[1])
local max = tonumber(ARGV[2])
local selected = 0
for i = 1, #KEYS do
  local failures = tonumber(redis.call('INCR', KEYS[i]))
  local delay = base
  for n = 2, failures do
    delay = delay * 2
    if max > 0 and delay >= max then
      delay = max
      break
    end
  end
  if max > 0 and delay > max then
    delay = max
  end
  redis.call('PEXPIRE', KEYS[i], delay)
  if delay > selected then
    selected = delay
  end
end
return selected
`

func (b *RedisDomainBackoff) Check(ctx context.Context, job Job) error {
	if b == nil {
		return nil
	}
	if b.client == nil {
		return fmt.Errorf("redis domain backoff client is required")
	}
	keys := b.keysForRecipients(job.Recipients())
	if len(keys) == 0 {
		return nil
	}
	result, err := b.client.Eval(ctx, redisDomainBackoffCheckScript, keys).Result()
	if err != nil {
		return err
	}
	allowed, key, _, err := parseRedisThrottleAcquireResult(result)
	if err != nil {
		return err
	}
	if allowed {
		return nil
	}
	return &DomainBackoffError{Domain: strings.TrimPrefix(key, b.prefix+":")}
}

func (b *RedisDomainBackoff) ObserveTemporaryFailure(ctx context.Context, _ Job, recipients []outbound.Address, _ error) {
	if b == nil || b.client == nil {
		return
	}
	keys := b.keysForRecipients(recipients)
	if len(keys) == 0 {
		return
	}
	_, _ = b.client.Eval(ctx, redisDomainBackoffObserveScript, keys, int(b.policy.BaseDelay/time.Millisecond), int(b.policy.MaxDelay/time.Millisecond)).Result()
}

func (b *RedisDomainBackoff) keysForRecipients(recipients []outbound.Address) []string {
	domains := domainsForRecipients(recipients)
	keys := make([]string, 0, len(domains))
	for _, domain := range domains {
		keys = append(keys, b.prefix+":"+domain)
	}
	return keys
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
