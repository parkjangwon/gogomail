package outbox

import (
	"context"
	"database/sql"
	"fmt"
)

// ShardedPostgresStore wraps PostgresStore to filter events by partition key
// shard.  Deploying N relay processes each with a different ShardIndex ensures
// that events with the same PartitionKey are always processed by the same
// process, preserving per-partition ordering.
//
// Shard assignment uses Postgres hashtext(partition_key) % TotalShards so the
// computation is deterministic and requires no extra columns or coordination.
//
// Example — 3 relay processes:
//
//	process 0: ShardedPostgresStore{TotalShards: 3, ShardIndex: 0}
//	process 1: ShardedPostgresStore{TotalShards: 3, ShardIndex: 1}
//	process 2: ShardedPostgresStore{TotalShards: 3, ShardIndex: 2}
type ShardedPostgresStore struct {
	*PostgresStore
	TotalShards int // total number of relay processes
	ShardIndex  int // 0-indexed shard owned by this process
}

// NewShardedPostgresStore creates a store that claims only events whose
// partition_key hashes to shardIndex (mod totalShards).
//
// With totalShards == 1 (or TotalShards == 0) the sharding filter is disabled
// and the store behaves identically to PostgresStore.
func NewShardedPostgresStore(db *sql.DB, maxAttempts, totalShards, shardIndex int) (*ShardedPostgresStore, error) {
	if totalShards < 1 {
		totalShards = 1
	}
	if shardIndex < 0 || shardIndex >= totalShards {
		return nil, fmt.Errorf("outbox: shardIndex %d out of range [0, %d)", shardIndex, totalShards)
	}
	return &ShardedPostgresStore{
		PostgresStore: NewPostgresStore(db, maxAttempts),
		TotalShards:   totalShards,
		ShardIndex:    shardIndex,
	}, nil
}

// FetchPending overrides PostgresStore.FetchPending to add a shard filter.
// When TotalShards == 1 the filter is omitted and the query is identical to
// the base PostgresStore (no performance overhead).
func (s *ShardedPostgresStore) FetchPending(ctx context.Context, limit int) ([]Event, error) {
	if s.TotalShards <= 1 {
		return s.PostgresStore.FetchPending(ctx, limit)
	}
	if s.db == nil {
		return nil, fmt.Errorf("database handle is required")
	}
	if limit <= 0 {
		limit = 100
	}

	// The shard filter is: abs(hashtext(partition_key)) % totalShards = shardIndex.
	// hashtext is Postgres-native, deterministic, and indexed-friendly when combined
	// with a partial index on (status, available_at).
	const query = `
WITH candidate AS (
  SELECT id, created_at
  FROM (
    SELECT id, created_at
    FROM outbox
    WHERE status = 'pending'
      AND available_at <= now()
      AND abs(hashtext(partition_key)) % $3 = $4
    UNION ALL
    SELECT id, created_at
    FROM outbox
    WHERE status = 'processing'
      AND locked_at < now() - $2::interval
      AND abs(hashtext(partition_key)) % $3 = $4
  ) AS candidates
  ORDER BY created_at, id
  LIMIT $1
),
picked AS (
  SELECT o.id
  FROM outbox AS o
  JOIN candidate ON candidate.id = o.id
  ORDER BY candidate.created_at, candidate.id
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

	rows, err := s.db.QueryContext(ctx, query, limit, s.lockTimeout, s.TotalShards, s.ShardIndex)
	if err != nil {
		return nil, fmt.Errorf("claim sharded outbox events (shard %d/%d): %w", s.ShardIndex, s.TotalShards, err)
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
