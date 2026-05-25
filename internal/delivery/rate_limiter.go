package delivery

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"
)

// RateLimiter controls the rate of outbound message delivery per recipient domain.
// It is checked before acquiring a concurrency slot from the Throttler.
type RateLimiter interface {
	// Allow returns nil if the job can proceed, or *RateLimitError if any
	// recipient domain has exceeded its configured per-minute message cap.
	Allow(ctx context.Context, job Job) error
}

// DomainRateLimitPolicy configures per-minute message delivery caps per recipient domain.
type DomainRateLimitPolicy struct {
	// DomainMessagesPerMinute maps recipient domain (e.g. "gmail.com") to its
	// per-minute delivery cap. 0 or absent means the domain uses DefaultMessagesPerMinute.
	DomainMessagesPerMinute map[string]int

	// DefaultMessagesPerMinute is the cap applied to any domain not listed in
	// DomainMessagesPerMinute. 0 means unlimited for unspecified domains.
	DefaultMessagesPerMinute int
}

// RateLimitError is returned by RateLimiter.Allow when a recipient domain has
// exceeded its configured per-minute delivery cap.
type RateLimitError struct {
	Domain         string
	LimitPerMinute int
}

func (e *RateLimitError) Error() string {
	return fmt.Sprintf("rate limit exceeded for domain %s (%d/min)", e.Domain, e.LimitPerMinute)
}

// --- In-memory implementation ---

// domainWindow tracks message count within a one-minute fixed window.
type domainWindow struct {
	windowStart time.Time
	count       int
}

// InMemoryDomainRateLimiter enforces per-domain rate limits in a single process.
// It uses a fixed one-minute window per domain, protected by a mutex.
// For multi-node deployments, use RedisDomainRateLimiter instead.
type InMemoryDomainRateLimiter struct {
	policy  DomainRateLimitPolicy
	mu      sync.Mutex
	windows map[string]*domainWindow
	now     func() time.Time
}

// NewInMemoryDomainRateLimiter creates a rate limiter using the given policy.
func NewInMemoryDomainRateLimiter(policy DomainRateLimitPolicy) *InMemoryDomainRateLimiter {
	return newInMemoryDomainRateLimiterWithClock(policy, time.Now)
}

// newInMemoryDomainRateLimiterWithClock creates a rate limiter with an injectable clock
// for deterministic unit testing.
func newInMemoryDomainRateLimiterWithClock(policy DomainRateLimitPolicy, now func() time.Time) *InMemoryDomainRateLimiter {
	return &InMemoryDomainRateLimiter{
		policy:  normalizeDomainRateLimitPolicy(policy),
		windows: make(map[string]*domainWindow),
		now:     now,
	}
}

// Allow checks each recipient domain in the job against the configured limits.
// On the first domain that would exceed its limit it returns *RateLimitError
// without incrementing any counter (fail-fast, atomic-per-domain).
func (r *InMemoryDomainRateLimiter) Allow(_ context.Context, job Job) error {
	domains := recipientDomainsForJob(job)
	if len(domains) == 0 {
		return nil
	}

	t := r.now()
	windowStart := t.Truncate(time.Minute)

	r.mu.Lock()
	defer r.mu.Unlock()

	// First pass: check whether any domain would exceed its limit.
	for _, domain := range domains {
		limit := r.limitFor(domain)
		if limit <= 0 {
			continue // unlimited
		}
		w := r.windowFor(domain, windowStart)
		if w.count >= limit {
			return &RateLimitError{Domain: domain, LimitPerMinute: limit}
		}
	}

	// Second pass: increment counters only after all checks pass.
	for _, domain := range domains {
		limit := r.limitFor(domain)
		if limit <= 0 {
			continue
		}
		r.windowFor(domain, windowStart).count++
	}
	return nil
}

// windowFor returns the current window for a domain, resetting it if the
// window has advanced.
func (r *InMemoryDomainRateLimiter) windowFor(domain string, windowStart time.Time) *domainWindow {
	w, ok := r.windows[domain]
	if !ok || !w.windowStart.Equal(windowStart) {
		w = &domainWindow{windowStart: windowStart}
		r.windows[domain] = w
	}
	return w
}

// limitFor returns the per-minute cap for a domain (0 = unlimited).
func (r *InMemoryDomainRateLimiter) limitFor(domain string) int {
	if r.policy.DomainMessagesPerMinute != nil {
		if limit, ok := r.policy.DomainMessagesPerMinute[domain]; ok {
			return limit
		}
	}
	return r.policy.DefaultMessagesPerMinute
}

// normalizeDomainRateLimitPolicy canonicalises domain keys to lowercase.
func normalizeDomainRateLimitPolicy(p DomainRateLimitPolicy) DomainRateLimitPolicy {
	if p.DefaultMessagesPerMinute < 0 {
		p.DefaultMessagesPerMinute = 0
	}
	out := make(map[string]int, len(p.DomainMessagesPerMinute))
	for domain, limit := range p.DomainMessagesPerMinute {
		domain = strings.ToLower(strings.TrimSpace(domain))
		if domain != "" && limit > 0 {
			out[domain] = limit
		}
	}
	p.DomainMessagesPerMinute = out
	return p
}

// recipientDomainsForJob extracts unique lowercase recipient domains from a job.
func recipientDomainsForJob(job Job) []string {
	seen := make(map[string]struct{})
	var domains []string
	for _, addr := range job.Recipients() {
		domain := domainFromAddress(addr.Email)
		if domain == "" {
			continue
		}
		if _, ok := seen[domain]; !ok {
			seen[domain] = struct{}{}
			domains = append(domains, domain)
		}
	}
	return domains
}
