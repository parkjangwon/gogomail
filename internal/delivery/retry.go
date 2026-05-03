package delivery

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

var ErrRetryExhausted = errors.New("delivery retry attempts exhausted")

type RetryPolicy struct {
	Delays []time.Duration
}

func DefaultRetryPolicy() RetryPolicy {
	return RetryPolicy{Delays: []time.Duration{
		5 * time.Minute,
		30 * time.Minute,
		2 * time.Hour,
		8 * time.Hour,
		24 * time.Hour,
	}}
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
	delay, ok := s.policy.NextDelay(job.RetryAttempt)
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

	topic := "mail.outbound." + string(job.Farm)
	errorMessage := ""
	if cause != nil {
		errorMessage = cause.Error()
	}
	if len(errorMessage) > 2000 {
		errorMessage = errorMessage[:2000]
	}

	if _, err := s.db.ExecContext(ctx, query, topic, job.MessageID, string(payload), availableAt, errorMessage); err != nil {
		return fmt.Errorf("schedule delivery retry: %w", err)
	}
	return nil
}
