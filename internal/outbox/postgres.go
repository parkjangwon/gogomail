package outbox

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
)

type PostgresStore struct {
	db          *sql.DB
	maxAttempts int
	lockTimeout string
}

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

	const query = `
WITH picked AS (
  SELECT id
  FROM outbox
  WHERE (status = 'pending' AND available_at <= now())
     OR (status = 'processing' AND locked_at < now() - $2::interval)
  ORDER BY created_at
  LIMIT $1
  FOR UPDATE SKIP LOCKED
)
UPDATE outbox AS o
SET status = 'processing',
    attempts = attempts + 1,
    last_error = NULL,
    locked_at = now()
FROM picked
WHERE o.id = picked.id
RETURNING o.id::text, o.topic, o.partition_key, o.payload`

	rows, err := s.db.QueryContext(ctx, query, limit, s.lockTimeout)
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

func (s *PostgresStore) MarkFailed(ctx context.Context, id string, cause error) error {
	if s.db == nil {
		return fmt.Errorf("database handle is required")
	}

	message := "publish failed"
	if cause != nil {
		message = cause.Error()
	}
	message = strings.TrimSpace(message)
	if len(message) > 2000 {
		message = message[:2000]
	}

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
