package maildb

import (
	"context"
	"fmt"
	"strings"
	"time"
)

type QueueStat struct {
	Topic  string `json:"topic"`
	Status string `json:"status"`
	Count  int64  `json:"count"`
}

type DeliveryAttemptView struct {
	ID              string    `json:"id"`
	MessageID       string    `json:"message_id"`
	RFCMessageID    string    `json:"rfc_message_id"`
	Farm            string    `json:"farm"`
	Recipient       string    `json:"recipient"`
	RecipientDomain string    `json:"recipient_domain"`
	Status          string    `json:"status"`
	ErrorMessage    string    `json:"error_message"`
	AttemptedAt     time.Time `json:"attempted_at"`
}

type SuppressionEntry struct {
	ID              string    `json:"id"`
	DomainID        string    `json:"domain_id"`
	Email           string    `json:"email"`
	Reason          string    `json:"reason"`
	SourceMessageID string    `json:"source_message_id"`
	CreatedAt       time.Time `json:"created_at"`
}

type DomainView struct {
	ID         string    `json:"id"`
	CompanyID  string    `json:"company_id"`
	Name       string    `json:"name"`
	NameACE    string    `json:"name_ace"`
	Status     string    `json:"status"`
	QuotaUsed  int64     `json:"quota_used"`
	QuotaLimit int64     `json:"quota_limit,omitempty"`
	CreatedAt  time.Time `json:"created_at"`
}

type UserView struct {
	ID          string    `json:"id"`
	DomainID    string    `json:"domain_id"`
	Username    string    `json:"username"`
	DisplayName string    `json:"display_name"`
	Role        string    `json:"role"`
	Status      string    `json:"status"`
	QuotaUsed   int64     `json:"quota_used"`
	QuotaLimit  int64     `json:"quota_limit,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
}

type UpdateDomainStatusRequest struct {
	ID     string `json:"id"`
	Status string `json:"status"`
}

type UpdateUserStatusRequest struct {
	ID     string `json:"id"`
	Status string `json:"status"`
}

func ValidateUpdateDomainStatusRequest(req UpdateDomainStatusRequest) error {
	if strings.TrimSpace(req.ID) == "" {
		return fmt.Errorf("domain id is required")
	}
	switch strings.TrimSpace(req.Status) {
	case "active", "suspended", "disabled":
		return nil
	default:
		return fmt.Errorf("unsupported domain status %q", req.Status)
	}
}

func ValidateUpdateUserStatusRequest(req UpdateUserStatusRequest) error {
	if strings.TrimSpace(req.ID) == "" {
		return fmt.Errorf("user id is required")
	}
	switch strings.TrimSpace(req.Status) {
	case "active", "suspended", "disabled":
		return nil
	default:
		return fmt.Errorf("unsupported user status %q", req.Status)
	}
}

func (r *Repository) UpdateDomainStatus(ctx context.Context, req UpdateDomainStatusRequest) error {
	if r.db == nil {
		return fmt.Errorf("database handle is required")
	}
	if err := ValidateUpdateDomainStatusRequest(req); err != nil {
		return err
	}
	result, err := r.db.ExecContext(ctx, `
UPDATE domains
SET status = $2,
    updated_at = now()
WHERE id = $1`, strings.TrimSpace(req.ID), strings.TrimSpace(req.Status))
	if err != nil {
		return fmt.Errorf("update domain status: %w", err)
	}
	affected, err := result.RowsAffected()
	if err == nil && affected == 0 {
		return fmt.Errorf("domain %q not found", req.ID)
	}
	return nil
}

func (r *Repository) UpdateUserStatus(ctx context.Context, req UpdateUserStatusRequest) error {
	if r.db == nil {
		return fmt.Errorf("database handle is required")
	}
	if err := ValidateUpdateUserStatusRequest(req); err != nil {
		return err
	}
	result, err := r.db.ExecContext(ctx, `
UPDATE users
SET status = $2,
    updated_at = now()
WHERE id = $1`, strings.TrimSpace(req.ID), strings.TrimSpace(req.Status))
	if err != nil {
		return fmt.Errorf("update user status: %w", err)
	}
	affected, err := result.RowsAffected()
	if err == nil && affected == 0 {
		return fmt.Errorf("user %q not found", req.ID)
	}
	return nil
}

func (r *Repository) ListUsers(ctx context.Context, domainID string, limit int) ([]UserView, error) {
	if r.db == nil {
		return nil, fmt.Errorf("database handle is required")
	}
	limit = normalizeLimit(limit)

	const query = `
SELECT
  id::text,
  domain_id::text,
  username,
  display_name,
  role,
  status,
  quota_used,
  COALESCE(quota_limit, 0),
  created_at
FROM users
WHERE ($1 = '' OR domain_id::text = $1)
ORDER BY created_at DESC
LIMIT $2`

	rows, err := r.db.QueryContext(ctx, query, domainID, limit)
	if err != nil {
		return nil, fmt.Errorf("list users: %w", err)
	}
	defer rows.Close()

	var users []UserView
	for rows.Next() {
		var user UserView
		if err := rows.Scan(
			&user.ID,
			&user.DomainID,
			&user.Username,
			&user.DisplayName,
			&user.Role,
			&user.Status,
			&user.QuotaUsed,
			&user.QuotaLimit,
			&user.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan user: %w", err)
		}
		users = append(users, user)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate users: %w", err)
	}
	return users, nil
}

func (r *Repository) ListDomains(ctx context.Context, limit int) ([]DomainView, error) {
	if r.db == nil {
		return nil, fmt.Errorf("database handle is required")
	}
	limit = normalizeLimit(limit)

	const query = `
SELECT
  id::text,
  company_id::text,
  name,
  name_ace,
  status,
  quota_used,
  COALESCE(quota_limit, 0),
  created_at
FROM domains
ORDER BY created_at DESC
LIMIT $1`

	rows, err := r.db.QueryContext(ctx, query, limit)
	if err != nil {
		return nil, fmt.Errorf("list domains: %w", err)
	}
	defer rows.Close()

	var domains []DomainView
	for rows.Next() {
		var domain DomainView
		if err := rows.Scan(
			&domain.ID,
			&domain.CompanyID,
			&domain.Name,
			&domain.NameACE,
			&domain.Status,
			&domain.QuotaUsed,
			&domain.QuotaLimit,
			&domain.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan domain: %w", err)
		}
		domains = append(domains, domain)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate domains: %w", err)
	}
	return domains, nil
}

func (r *Repository) ListQueueStats(ctx context.Context) ([]QueueStat, error) {
	if r.db == nil {
		return nil, fmt.Errorf("database handle is required")
	}

	const query = `
SELECT topic, status, count(*)
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
		if err := rows.Scan(&stat.Topic, &stat.Status, &stat.Count); err != nil {
			return nil, fmt.Errorf("scan queue stat: %w", err)
		}
		stats = append(stats, stat)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate queue stats: %w", err)
	}
	return stats, nil
}

func (r *Repository) ListDeliveryAttempts(ctx context.Context, limit int) ([]DeliveryAttemptView, error) {
	if r.db == nil {
		return nil, fmt.Errorf("database handle is required")
	}
	limit = normalizeLimit(limit)

	const query = `
SELECT
  id::text,
  message_id::text,
  rfc_message_id,
  farm,
  recipient,
  recipient_domain,
  status,
  error_message,
  attempted_at
FROM delivery_attempts
ORDER BY attempted_at DESC
LIMIT $1`

	rows, err := r.db.QueryContext(ctx, query, limit)
	if err != nil {
		return nil, fmt.Errorf("list delivery attempts: %w", err)
	}
	defer rows.Close()

	var attempts []DeliveryAttemptView
	for rows.Next() {
		var attempt DeliveryAttemptView
		if err := rows.Scan(
			&attempt.ID,
			&attempt.MessageID,
			&attempt.RFCMessageID,
			&attempt.Farm,
			&attempt.Recipient,
			&attempt.RecipientDomain,
			&attempt.Status,
			&attempt.ErrorMessage,
			&attempt.AttemptedAt,
		); err != nil {
			return nil, fmt.Errorf("scan delivery attempt: %w", err)
		}
		attempts = append(attempts, attempt)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate delivery attempts: %w", err)
	}
	return attempts, nil
}

func (r *Repository) ListSuppressionEntries(ctx context.Context, limit int) ([]SuppressionEntry, error) {
	if r.db == nil {
		return nil, fmt.Errorf("database handle is required")
	}
	limit = normalizeLimit(limit)

	const query = `
SELECT
  id::text,
  COALESCE(domain_id::text, ''),
  email,
  reason,
  COALESCE(source_message_id::text, ''),
  created_at
FROM suppression_list
ORDER BY created_at DESC
LIMIT $1`

	rows, err := r.db.QueryContext(ctx, query, limit)
	if err != nil {
		return nil, fmt.Errorf("list suppression entries: %w", err)
	}
	defer rows.Close()

	var entries []SuppressionEntry
	for rows.Next() {
		var entry SuppressionEntry
		if err := rows.Scan(
			&entry.ID,
			&entry.DomainID,
			&entry.Email,
			&entry.Reason,
			&entry.SourceMessageID,
			&entry.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan suppression entry: %w", err)
		}
		entries = append(entries, entry)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate suppression entries: %w", err)
	}
	return entries, nil
}

func (r *Repository) RetryOutbox(ctx context.Context, id string) error {
	if r.db == nil {
		return fmt.Errorf("database handle is required")
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

	result, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("retry outbox event: %w", err)
	}
	affected, err := result.RowsAffected()
	if err == nil && affected == 0 {
		return fmt.Errorf("outbox event %q not found", id)
	}
	return nil
}

func (r *Repository) DeleteSuppressionEntry(ctx context.Context, id string) error {
	if r.db == nil {
		return fmt.Errorf("database handle is required")
	}

	const query = `DELETE FROM suppression_list WHERE id = $1`
	result, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("delete suppression entry: %w", err)
	}
	affected, err := result.RowsAffected()
	if err == nil && affected == 0 {
		return fmt.Errorf("suppression entry %q not found", id)
	}
	return nil
}
