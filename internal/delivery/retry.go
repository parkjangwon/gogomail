package delivery

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"hash/fnv"
	"math"
	"time"

	"github.com/gogomail/gogomail/internal/outbound"
)

var ErrRetryExhausted = errors.New("delivery retry attempts exhausted")

type RetryPolicy struct {
	Delays      []time.Duration
	JitterRatio float64
	MaxDelay    time.Duration
}

func DefaultRetryPolicy() RetryPolicy {
	return RetryPolicy{Delays: []time.Duration{
		5 * time.Minute,
		30 * time.Minute,
		2 * time.Hour,
		8 * time.Hour,
		24 * time.Hour,
	}, JitterRatio: 0.20, MaxDelay: 24 * time.Hour}
}

func (p RetryPolicy) NextDelay(currentAttempt int) (time.Duration, bool) {
	if len(p.Delays) == 0 {
		return 0, false
	}
	if currentAttempt < 0 {
		currentAttempt = 0
	}
	if currentAttempt >= len(p.Delays) {
		return 0, false
	}
	return p.Delays[currentAttempt], true
}

func (p RetryPolicy) NextScheduledDelay(key string, currentAttempt int) (time.Duration, bool) {
	delay, ok := p.NextDelay(currentAttempt)
	if !ok {
		return 0, false
	}
	delay = deterministicJitter(delay, p.JitterRatio, key, currentAttempt)
	if p.MaxDelay > 0 && delay > p.MaxDelay {
		return p.MaxDelay, true
	}
	return delay, true
}

type RetryScheduler interface {
	ScheduleRetry(ctx context.Context, job Job, cause error) error
}

type PostgresRetryScheduler struct {
	db     *sql.DB
	policy RetryPolicy
	now    func() time.Time
}

func NewPostgresRetryScheduler(db *sql.DB, policy RetryPolicy) *PostgresRetryScheduler {
	return &PostgresRetryScheduler{db: db, policy: policy, now: time.Now}
}

func (s *PostgresRetryScheduler) ScheduleRetry(ctx context.Context, job Job, cause error) error {
	if s.db == nil {
		return fmt.Errorf("database handle is required")
	}
	delay, ok := s.policy.NextScheduledDelay(job.MessageID, job.RetryAttempt)
	if !ok {
		return fmt.Errorf("%w for message %s", ErrRetryExhausted, job.MessageID)
	}

	next := job.QueuedMessage
	next.RetryAttempt = job.RetryAttempt + 1
	payload, err := json.Marshal(next)
	if err != nil {
		return fmt.Errorf("marshal delivery retry payload: %w", err)
	}

	availableAt := s.now().UTC().Add(delay)
	const query = `
INSERT INTO outbox (topic, partition_key, payload, status, available_at, last_error)
VALUES ($1, $2, $3::jsonb, 'pending', $4, $5)`

	topic := "mail.outbound." + string(normalizeRetryFarm(job.Farm))
	errorMessage := retryErrorMessage(cause)

	if _, err := s.db.ExecContext(ctx, query, topic, job.MessageID, string(payload), availableAt, errorMessage); err != nil {
		return fmt.Errorf("schedule delivery retry: %w", err)
	}
	return nil
}

func normalizeRetryFarm(farm outbound.Farm) outbound.Farm {
	return outbound.NormalizeFarm(farm)
}

func retryErrorMessage(cause error) string {
	if cause == nil {
		return ""
	}
	return truncateUTF8Bytes(cause.Error(), 2000)
}

func deterministicJitter(base time.Duration, ratio float64, key string, attempt int) time.Duration {
	if base <= 0 || ratio <= 0 || math.IsNaN(ratio) {
		return base
	}
	ratio = math.Min(ratio, 1)
	hash := fnv.New64a()
	_, _ = fmt.Fprintf(hash, "%s:%d", key, attempt)
	unit := float64(hash.Sum64()%1_000_000) / 999_999
	factor := 1 - ratio + unit*(2*ratio)
	delay := time.Duration(float64(base) * factor)
	if delay <= 0 {
		return time.Millisecond
	}
	return delay
}
