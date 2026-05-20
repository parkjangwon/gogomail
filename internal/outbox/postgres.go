package outbox

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/lib/pq"
)

type PostgresStore struct {
	db          *sql.DB
	maxAttempts int
	lockTimeout string
}

const fetchPendingSQL = `
WITH candidate AS (
  SELECT id, created_at
  FROM (
    SELECT id, created_at
    FROM outbox
    WHERE status = 'pending'
      AND available_at <= now()
    UNION ALL
    SELECT id, created_at
    FROM outbox
    WHERE status = 'processing'
      AND locked_at < now() - $2::interval
  ) AS candidates
  ORDER BY created_at
  LIMIT $1
),
picked AS (
  SELECT o.id
  FROM outbox AS o
  JOIN candidate ON candidate.id = o.id
  ORDER BY candidate.created_at
  FOR UPDATE OF o SKIP LOCKED
)
UPDATE outbox AS o
SET status = 'processing',
    attempts = attempts + 1,
    last_error = NULL,
    locked_at = now()
FROM picked
WHERE o.id = picked.id
RETURNING o.id::text, o.topic, o.partition_key, o.payload`

func NewPostgresStore(db *sql.DB, maxAttempts int) *PostgresStore {
	if maxAttempts <= 0 {
		maxAttempts = 10
	}
	return &PostgresStore{db: db, maxAttempts: maxAttempts, lockTimeout: "5 minutes"}
}

func (s *PostgresStore) FetchPending(ctx context.Context, limit int) ([]Event, error) {
	if s.db == nil {
		return nil, fmt.Errorf("database handle is required")
	}
	if limit <= 0 {
		limit = 100
	}

	rows, err := s.db.QueryContext(ctx, fetchPendingSQL, limit, s.lockTimeout)
	if err != nil {
		return nil, fmt.Errorf("claim pending outbox events: %w", err)
	}
	defer rows.Close()

	var events []Event
	for rows.Next() {
		var event Event
		if err := rows.Scan(&event.ID, &event.Topic, &event.PartitionKey, &event.Payload); err != nil {
			return nil, fmt.Errorf("scan outbox event: %w", err)
		}
		events = append(events, event)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate outbox events: %w", err)
	}
	return events, nil
}

func (s *PostgresStore) MarkDone(ctx context.Context, id string) error {
	if s.db == nil {
		return fmt.Errorf("database handle is required")
	}

	const query = `
UPDATE outbox
SET status = 'done',
    last_error = NULL,
    locked_at = NULL,
    processed_at = now()
WHERE id = $1`

	if _, err := s.db.ExecContext(ctx, query, id); err != nil {
		return fmt.Errorf("mark outbox event done: %w", err)
	}
	return nil
}

func (s *PostgresStore) MarkDoneBatch(ctx context.Context, ids []string) error {
	if s.db == nil {
		return fmt.Errorf("database handle is required")
	}
	if len(ids) == 0 {
		return nil
	}

	const query = `
UPDATE outbox
SET status = 'done',
    last_error = NULL,
    locked_at = NULL,
    processed_at = now()
WHERE id = ANY($1::uuid[])`

	if _, err := s.db.ExecContext(ctx, query, pq.Array(ids)); err != nil {
		return fmt.Errorf("mark outbox events done: %w", err)
	}
	return nil
}

func (s *PostgresStore) MarkFailed(ctx context.Context, id string, cause error) error {
	if s.db == nil {
		return fmt.Errorf("database handle is required")
	}

	message := publishFailureMessage(cause)

	const query = `
UPDATE outbox
SET status = CASE WHEN attempts >= $2 THEN 'failed' ELSE 'pending' END,
    last_error = $3,
    locked_at = NULL,
    processed_at = CASE WHEN attempts >= $2 THEN now() ELSE processed_at END
WHERE id = $1`

	if _, err := s.db.ExecContext(ctx, query, id, s.maxAttempts, message); err != nil {
		return fmt.Errorf("mark outbox event failed: %w", err)
	}
	return nil
}

func (s *PostgresStore) MarkFailedBatch(ctx context.Context, failures []FailedEvent) error {
	if s.db == nil {
		return fmt.Errorf("database handle is required")
	}
	if len(failures) == 0 {
		return nil
	}

	ids := make([]string, 0, len(failures))
	messages := make([]string, 0, len(failures))
	for _, failure := range failures {
		ids = append(ids, failure.ID)
		messages = append(messages, publishFailureMessage(failure.Cause))
	}

	if _, err := s.db.ExecContext(ctx, markFailedBatchSQL, pq.Array(ids), s.maxAttempts, pq.Array(messages)); err != nil {
		return fmt.Errorf("mark outbox events failed: %w", err)
	}
	return nil
}

const markFailedBatchSQL = `
WITH failed(id, last_error) AS (
  SELECT id, last_error
  FROM unnest($1::uuid[], $3::text[]) AS input(id, last_error)
)
UPDATE outbox AS o
SET status = CASE WHEN attempts >= $2 THEN 'failed' ELSE 'pending' END,
    last_error = failed.last_error,
    locked_at = NULL,
    processed_at = CASE WHEN attempts >= $2 THEN now() ELSE processed_at END
FROM failed
WHERE o.id = failed.id`

func publishFailureMessage(cause error) string {
	message := "publish failed"
	if cause != nil {
		message = cause.Error()
	}
	message = strings.TrimSpace(message)
	return truncateUTF8Bytes(message, 2000)
}

func truncateUTF8Bytes(value string, maxBytes int) string {
	if maxBytes <= 0 || len(value) <= maxBytes {
		return value
	}
	value = value[:maxBytes]
	for !utf8.ValidString(value) && len(value) > 0 {
		value = value[:len(value)-1]
	}
	return value
}
