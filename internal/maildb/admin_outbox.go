package maildb

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/gogomail/gogomail/internal/audit"
)

func (r *Repository) ListQueueStats(ctx context.Context) ([]QueueStat, error) {
	if r.db == nil {
		return nil, fmt.Errorf("database handle is required")
	}

	const query = `
SELECT
  topic,
  status,
  count(*)::bigint,
  count(*) FILTER (WHERE status = 'pending' AND available_at <= now())::bigint,
  count(*) FILTER (WHERE status = 'pending' AND available_at > now())::bigint,
  count(*) FILTER (WHERE status = 'processing' AND locked_at < now() - interval '5 minutes')::bigint,
  min(created_at) FILTER (WHERE status = 'pending' AND available_at <= now()),
  min(available_at) FILTER (WHERE status = 'pending' AND available_at > now())
FROM outbox
GROUP BY topic, status
ORDER BY topic, status`

	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("list queue stats: %w", err)
	}
	defer rows.Close()

	var stats []QueueStat
	for rows.Next() {
		var stat QueueStat
		var oldestReadyAt sql.NullTime
		var nextAvailableAt sql.NullTime
		if err := rows.Scan(
			&stat.Topic,
			&stat.Status,
			&stat.Count,
			&stat.ReadyCount,
			&stat.DelayedCount,
			&stat.StaleProcessingCount,
			&oldestReadyAt,
			&nextAvailableAt,
		); err != nil {
			return nil, fmt.Errorf("scan queue stat: %w", err)
		}
		if oldestReadyAt.Valid {
			stat.OldestReadyAt = &oldestReadyAt.Time
		}
		if nextAvailableAt.Valid {
			stat.NextAvailableAt = &nextAvailableAt.Time
		}
		stats = append(stats, stat)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate queue stats: %w", err)
	}
	return stats, nil
}

func (r *Repository) ListOutboxEvents(ctx context.Context, req OutboxEventListRequest) ([]OutboxEventView, bool, error) {
	if r.db == nil {
		return nil, false, fmt.Errorf("database handle is required")
	}
	req.Limit = normalizeLimit(req.Limit)
	req.Topic = strings.TrimSpace(req.Topic)
	req.PartitionKey = strings.TrimSpace(req.PartitionKey)
	req.Status = strings.ToLower(strings.TrimSpace(req.Status))
	if !req.Since.IsZero() {
		req.Since = req.Since.UTC()
	}
	if req.Status != "" && !allowedOutboxStatus(req.Status) {
		return nil, false, fmt.Errorf("unsupported outbox status")
	}
	queryLimit := req.Limit
	if req.ProbeMore {
		queryLimit = req.Limit + 1
	}

	query, args := buildListOutboxEventsQuery(req, queryLimit)
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, false, fmt.Errorf("list outbox events: %w", err)
	}
	defer rows.Close()

	var events []OutboxEventView
	for rows.Next() {
		var event OutboxEventView
		var lockedAt sql.NullTime
		var processedAt sql.NullTime
		if err := rows.Scan(
			&event.ID,
			&event.Topic,
			&event.PartitionKey,
			&event.Status,
			&event.Attempts,
			&event.LastError,
			&event.CreatedAt,
			&event.AvailableAt,
			&lockedAt,
			&processedAt,
		); err != nil {
			return nil, false, fmt.Errorf("scan outbox event: %w", err)
		}
		event.LastError = truncateUTF8Bytes(event.LastError, outboxEventListErrorPreviewBytes)
		if lockedAt.Valid {
			event.LockedAt = &lockedAt.Time
		}
		if processedAt.Valid {
			event.ProcessedAt = &processedAt.Time
		}
		events = append(events, event)
	}
	if err := rows.Err(); err != nil {
		return nil, false, fmt.Errorf("iterate outbox events: %w", err)
	}
	hasMore := req.ProbeMore && len(events) > req.Limit
	if hasMore {
		events = events[:req.Limit]
	}
	return events, hasMore, nil
}

func buildListOutboxEventsQuery(req OutboxEventListRequest, queryLimit int) (string, []any) {
	query := listOutboxEventsBaseSQL
	var conditions []string
	var args []any

	if req.Topic != "" {
		args = append(args, req.Topic)
		conditions = append(conditions, fmt.Sprintf("topic = $%d", len(args)))
	}
	if req.PartitionKey != "" {
		args = append(args, req.PartitionKey)
		conditions = append(conditions, fmt.Sprintf("partition_key = $%d", len(args)))
	}
	if req.Status != "" {
		args = append(args, req.Status)
		conditions = append(conditions, fmt.Sprintf("status = $%d", len(args)))
	}
	if !req.Since.IsZero() {
		args = append(args, req.Since.UTC())
		conditions = append(conditions, fmt.Sprintf("created_at >= $%d", len(args)))
	}
	if len(conditions) > 0 {
		query += "\nWHERE " + strings.Join(conditions, "\n  AND ")
	}

	args = append(args, queryLimit)
	query += fmt.Sprintf(`
ORDER BY created_at DESC, id DESC
LIMIT $%d`, len(args))
	return query, args
}

func (r *Repository) GetOutboxEvent(ctx context.Context, id string) (OutboxEventView, error) {
	if r.db == nil {
		return OutboxEventView{}, fmt.Errorf("database handle is required")
	}
	id = strings.TrimSpace(id)
	if id == "" {
		return OutboxEventView{}, fmt.Errorf("outbox event id is required")
	}

	const query = `
SELECT
  id::text,
  topic,
  partition_key,
  status,
  attempts,
  COALESCE(last_error, ''),
  created_at,
  available_at,
  locked_at,
  processed_at
FROM outbox
WHERE id = $1`

	var event OutboxEventView
	var lockedAt sql.NullTime
	var processedAt sql.NullTime
	if err := r.db.QueryRowContext(ctx, query, id).Scan(
		&event.ID,
		&event.Topic,
		&event.PartitionKey,
		&event.Status,
		&event.Attempts,
		&event.LastError,
		&event.CreatedAt,
		&event.AvailableAt,
		&lockedAt,
		&processedAt,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return OutboxEventView{}, fmt.Errorf("outbox event %q not found", id)
		}
		return OutboxEventView{}, fmt.Errorf("get outbox event: %w", err)
	}
	if lockedAt.Valid {
		event.LockedAt = &lockedAt.Time
	}
	if processedAt.Valid {
		event.ProcessedAt = &processedAt.Time
	}
	return event, nil
}

func allowedOutboxStatus(status string) bool {
	switch status {
	case "pending", "processing", "done", "failed":
		return true
	default:
		return false
	}
}
func (r *Repository) RetryOutbox(ctx context.Context, id string) error {
	if r.db == nil {
		return fmt.Errorf("database handle is required")
	}
	id = strings.TrimSpace(id)
	if id == "" {
		return fmt.Errorf("outbox event id is required")
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin outbox retry transaction: %w", err)
	}
	defer tx.Rollback()

	var event OutboxEventView
	var lockedAt sql.NullTime
	var processedAt sql.NullTime
	if err := tx.QueryRowContext(ctx, `
SELECT
  id::text,
  topic,
  partition_key,
  status,
  attempts,
  COALESCE(last_error, ''),
  created_at,
  available_at,
  locked_at,
  processed_at
FROM outbox
WHERE id = $1
FOR UPDATE`, id).Scan(
		&event.ID,
		&event.Topic,
		&event.PartitionKey,
		&event.Status,
		&event.Attempts,
		&event.LastError,
		&event.CreatedAt,
		&event.AvailableAt,
		&lockedAt,
		&processedAt,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("outbox event %q not found", id)
		}
		return fmt.Errorf("read outbox event for retry: %w", err)
	}
	if lockedAt.Valid {
		event.LockedAt = &lockedAt.Time
	}
	if processedAt.Valid {
		event.ProcessedAt = &processedAt.Time
	}

	const query = `
UPDATE outbox
SET status = 'pending',
    attempts = 0,
    last_error = NULL,
    locked_at = NULL,
    available_at = now(),
    processed_at = NULL
WHERE id = $1`

	result, err := tx.ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("retry outbox event: %w", err)
	}
	affected, err := result.RowsAffected()
	if err == nil && affected == 0 {
		return fmt.Errorf("outbox event %q not found", id)
	}
	detail, err := outboxRetryAuditDetail(event)
	if err != nil {
		return err
	}
	if err := audit.InsertTx(ctx, tx, audit.Log{
		Category:   "admin",
		Action:     "outbox.retry",
		TargetType: "outbox_event",
		TargetID:   event.ID,
		Result:     "retried",
		Detail:     detail,
	}); err != nil {
		return fmt.Errorf("record outbox retry audit: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit outbox retry transaction: %w", err)
	}
	return nil
}

func outboxRetryAuditDetail(event OutboxEventView) (json.RawMessage, error) {
	detail, err := json.Marshal(map[string]any{
		"outbox_event_id":   event.ID,
		"topic":             event.Topic,
		"partition_key":     event.PartitionKey,
		"previous_status":   event.Status,
		"previous_attempts": event.Attempts,
		"previous_last_error": truncateUTF8Bytes(
			event.LastError,
			outboxEventListErrorPreviewBytes,
		),
	})
	if err != nil {
		return nil, fmt.Errorf("marshal outbox retry audit detail: %w", err)
	}
	return detail, nil
}
