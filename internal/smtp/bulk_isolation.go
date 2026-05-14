package smtpd

import (
	"sync"
	"time"
)

// BulkSenderLimiter isolates bulk senders from regular users by rate limiting.
// Regular users get priority; bulk senders are capped at lower rate.
type BulkSenderLimiter struct {
	mu                    sync.RWMutex
	regularUserRateLimit  int                      // msg/sec for regular users (unlimited by default)
	bulkUserRateLimit     int                      // msg/sec for bulk users (e.g., 100)
	bulkUserRole          string                   // role name that identifies bulk users
	userRateLimiters      map[string]*TokenBucket
}

type TokenBucket struct {
	mu           sync.Mutex
	lastRefresh  time.Time
	tokensLeft   int
	maxTokens    int
	refillRate   int // tokens per second
}

// NewBulkSenderLimiter creates a bulk sender rate limiter.
// bulkRate: max messages/sec for bulk users (e.g., 100)
// bulkRole: user role that identifies bulk senders (e.g., "bulk_user")
func NewBulkSenderLimiter(bulkRate int, bulkRole string) *BulkSenderLimiter {
	if bulkRate <= 0 {
		bulkRate = 100 // default
	}
	if bulkRole == "" {
		bulkRole = "bulk_user"
	}
	return &BulkSenderLimiter{
		bulkUserRateLimit: bulkRate,
		bulkUserRole:      bulkRole,
		userRateLimiters:  make(map[string]*TokenBucket),
	}
}

// AllowSubmission checks if a user can submit a message.
// Returns true if within rate limit, false if rate limited.
func (bsl *BulkSenderLimiter) AllowSubmission(userID, userRole string) bool {
	// Regular users always allowed (no rate limit)
	if userRole != bsl.bulkUserRole {
		return true
	}

	// Bulk users are rate limited
	bsl.mu.Lock()
	limiter := bsl.userRateLimiters[userID]
	if limiter == nil {
		limiter = NewTokenBucket(bsl.bulkUserRateLimit)
		bsl.userRateLimiters[userID] = limiter
	}
	bsl.mu.Unlock()

	return limiter.Allow()
}

// NewTokenBucket creates a token bucket rate limiter.
// maxRate: tokens per second
func NewTokenBucket(maxRate int) *TokenBucket {
	return &TokenBucket{
		lastRefresh: time.Now(),
		tokensLeft:  maxRate,
		maxTokens:   maxRate,
		refillRate:  maxRate,
	}
}

// Allow attempts to consume one token.
// Returns true if a token was available, false otherwise.
func (rl *TokenBucket) Allow() bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	elapsed := now.Sub(rl.lastRefresh).Seconds()
	rl.lastRefresh = now

	// Refill tokens based on elapsed time
	tokensToAdd := int(elapsed * float64(rl.refillRate))
	rl.tokensLeft += tokensToAdd
	if rl.tokensLeft > rl.maxTokens {
		rl.tokensLeft = rl.maxTokens
	}

	// Try to consume one token
	if rl.tokensLeft > 0 {
		rl.tokensLeft--
		return true
	}

	return false
}

// IsBulkUser returns true if the user role indicates a bulk sender.
func (bsl *BulkSenderLimiter) IsBulkUser(userRole string) bool {
	return userRole == bsl.bulkUserRole
}

// GetStats returns rate limiter statistics for a user (for monitoring).
func (bsl *BulkSenderLimiter) GetStats(userID string) map[string]interface{} {
	bsl.mu.RLock()
	limiter := bsl.userRateLimiters[userID]
	bsl.mu.RUnlock()

	if limiter == nil {
		return map[string]interface{}{"status": "not_limited"}
	}

	limiter.mu.Lock()
	defer limiter.mu.Unlock()

	return map[string]interface{}{
		"max_tokens":   limiter.maxTokens,
		"tokens_left":  limiter.tokensLeft,
		"refill_rate":  limiter.refillRate,
		"last_refresh": limiter.lastRefresh.Format(time.RFC3339),
	}
}
