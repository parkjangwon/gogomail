package maildb

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/gogomail/gogomail/internal/audit"
)

type AuditLogListRequest struct {
	Limit        int
	Category     string
	Action       string
	ActionPrefix string
	Result       string
	TargetType   string
	CompanyID    string
	DomainID     string
	UserID       string
	ActorID      string
	TargetID     string
	Since        time.Time
	Before       time.Time // when set, filters created_at < Before (cursor pagination)
	ProbeMore    bool      // when true the query fetches Limit+1 to detect has_more
}

type AuditLogView struct {
	ID         string          `json:"id"`
	CompanyID  string          `json:"company_id,omitempty"`
	DomainID   string          `json:"domain_id,omitempty"`
	UserID     string          `json:"user_id,omitempty"`
	ActorID    string          `json:"actor_id,omitempty"`
	Category   string          `json:"category"`
	Action     string          `json:"action"`
	TargetType string          `json:"target_type,omitempty"`
	TargetID   string          `json:"target_id,omitempty"`
	IPAddress  string          `json:"ip_address,omitempty"`
	UserAgent  string          `json:"user_agent,omitempty"`
	Result     string          `json:"result"`
	Detail     json.RawMessage `json:"detail"`
	PrevHash   string          `json:"prev_hash"`
	Hash       string          `json:"hash"`
	CreatedAt  time.Time       `json:"created_at"`
}

type AuditLogIntegrityRequest struct {
	Limit int
	Since time.Time
}

type AuditLogIntegrityView struct {
	CheckedCount int                      `json:"checked_count"`
	Valid        bool                     `json:"valid"`
	FirstID      string                   `json:"first_id,omitempty"`
	LastID       string                   `json:"last_id,omitempty"`
	Breaks       []AuditLogIntegrityBreak `json:"breaks"`
}

type AuditLogIntegrityBreak struct {
	ID               string    `json:"id"`
	CreatedAt        time.Time `json:"created_at"`
	Reason           string    `json:"reason"`
	ExpectedPrevHash string    `json:"expected_prev_hash,omitempty"`
	ActualPrevHash   string    `json:"actual_prev_hash,omitempty"`
	ExpectedHash     string    `json:"expected_hash,omitempty"`
	ActualHash       string    `json:"actual_hash,omitempty"`
}

func (r *Repository) ListAuditLogs(ctx context.Context, req AuditLogListRequest) ([]AuditLogView, bool, error) {
	if r.db == nil {
		return nil, false, fmt.Errorf("database handle is required")
	}
	req = normalizeAuditLogListRequest(req)

	query := `
SELECT
  id::text,
  COALESCE(company_id::text, ''),
  COALESCE(domain_id::text, ''),
  COALESCE(user_id::text, ''),
  COALESCE(actor_id::text, ''),
  category,
  action,
  target_type,
  COALESCE(target_id::text, ''),
  COALESCE(ip_address::text, ''),
  user_agent,
  result,
  detail,
  prev_hash,
  hash,
  created_at
FROM audit_logs`
	var conditions []string
	var args []any
	if req.Category != "" {
		args = append(args, req.Category)
		conditions = append(conditions, fmt.Sprintf("category = $%d", len(args)))
	}
	if req.Action != "" {
		args = append(args, req.Action)
		conditions = append(conditions, fmt.Sprintf("action = $%d", len(args)))
	}
	if req.ActionPrefix != "" {
		args = append(args, req.ActionPrefix)
		conditions = append(conditions, fmt.Sprintf("LEFT(action, LENGTH($%d)) = $%d", len(args), len(args)))
	}
	if req.Result != "" {
		args = append(args, req.Result)
		conditions = append(conditions, fmt.Sprintf("result = $%d", len(args)))
	}
	if req.TargetType != "" {
		args = append(args, req.TargetType)
		conditions = append(conditions, fmt.Sprintf("target_type = $%d", len(args)))
	}
	if req.CompanyID != "" {
		args = append(args, req.CompanyID)
		conditions = append(conditions, fmt.Sprintf("company_id::text = $%d", len(args)))
	}
	if req.DomainID != "" {
		args = append(args, req.DomainID)
		conditions = append(conditions, fmt.Sprintf("domain_id::text = $%d", len(args)))
	}
	if req.UserID != "" {
		args = append(args, req.UserID)
		conditions = append(conditions, fmt.Sprintf("user_id::text = $%d", len(args)))
	}
	if req.ActorID != "" {
		args = append(args, req.ActorID)
		conditions = append(conditions, fmt.Sprintf("actor_id::text = $%d", len(args)))
	}
	if req.TargetID != "" {
		args = append(args, req.TargetID)
		conditions = append(conditions, fmt.Sprintf("target_id::text = $%d", len(args)))
	}
	if !req.Since.IsZero() {
		args = append(args, req.Since.UTC())
		conditions = append(conditions, fmt.Sprintf("created_at >= $%d", len(args)))
	}
	if !req.Before.IsZero() {
		args = append(args, req.Before.UTC())
		conditions = append(conditions, fmt.Sprintf("created_at < $%d", len(args)))
	}
	if len(conditions) > 0 {
		query += "\nWHERE " + strings.Join(conditions, "\n  AND ")
	}
	limit := req.Limit
	queryLimit := limit
	if req.ProbeMore {
		queryLimit = limit + 1
	}
	args = append(args, queryLimit)
	query += fmt.Sprintf(`
ORDER BY created_at DESC, id DESC
LIMIT $%d`, len(args))

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, false, fmt.Errorf("list audit logs: %w", err)
	}
	defer rows.Close()

	var logs []AuditLogView
	for rows.Next() {
		log, err := scanAuditLog(rows)
		if err != nil {
			return nil, false, err
		}
		logs = append(logs, log)
	}
	if err := rows.Err(); err != nil {
		return nil, false, fmt.Errorf("iterate audit logs: %w", err)
	}
	hasMore := req.ProbeMore && len(logs) > limit
	if hasMore {
		logs = logs[:limit]
	}
	return logs, hasMore, nil
}

func (r *Repository) GetAuditLog(ctx context.Context, id string) (AuditLogView, error) {
	if r.db == nil {
		return AuditLogView{}, fmt.Errorf("database handle is required")
	}
	id = strings.TrimSpace(id)
	if id == "" {
		return AuditLogView{}, fmt.Errorf("audit log id is required")
	}
	const query = `
SELECT
  id::text,
  COALESCE(company_id::text, ''),
  COALESCE(domain_id::text, ''),
  COALESCE(user_id::text, ''),
  COALESCE(actor_id::text, ''),
  category,
  action,
  target_type,
  COALESCE(target_id::text, ''),
  COALESCE(ip_address::text, ''),
  user_agent,
  result,
  detail,
  prev_hash,
  hash,
  created_at
FROM audit_logs
WHERE id = $1`
	log, err := scanAuditLog(r.db.QueryRowContext(ctx, query, id))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return AuditLogView{}, fmt.Errorf("audit log not found")
		}
		return AuditLogView{}, fmt.Errorf("get audit log: %w", err)
	}
	return log, nil
}

func (r *Repository) CheckAuditLogIntegrity(ctx context.Context, req AuditLogIntegrityRequest) (AuditLogIntegrityView, error) {
	if r.db == nil {
		return AuditLogIntegrityView{}, fmt.Errorf("database handle is required")
	}
	req = normalizeAuditLogIntegrityRequest(req)

	query, args := auditLogIntegrityQuery(req)
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return AuditLogIntegrityView{}, fmt.Errorf("check audit log integrity: %w", err)
	}
	defer rows.Close()

	var logs []AuditLogView
	for rows.Next() {
		log, err := scanAuditLog(rows)
		if err != nil {
			return AuditLogIntegrityView{}, err
		}
		logs = append(logs, log)
	}
	if err := rows.Err(); err != nil {
		return AuditLogIntegrityView{}, fmt.Errorf("iterate audit log integrity rows: %w", err)
	}

	view := AuditLogIntegrityView{
		CheckedCount: len(logs),
		Valid:        true,
		Breaks:       []AuditLogIntegrityBreak{},
	}
	if len(logs) == 0 {
		return view, nil
	}
	view.FirstID = logs[0].ID
	view.LastID = logs[len(logs)-1].ID

	for i, log := range logs {
		expectedHash, err := audit.ComputeHash(log.PrevHash, audit.Log{
			CompanyID:  log.CompanyID,
			DomainID:   log.DomainID,
			UserID:     log.UserID,
			ActorID:    log.ActorID,
			Category:   log.Category,
			Action:     log.Action,
			TargetType: log.TargetType,
			TargetID:   log.TargetID,
			IPAddress:  log.IPAddress,
			UserAgent:  log.UserAgent,
			Result:     log.Result,
			Detail:     log.Detail,
			CreatedAt:  log.CreatedAt,
		})
		if err != nil {
			return AuditLogIntegrityView{}, fmt.Errorf("compute audit hash for %s: %w", log.ID, err)
		}
		if log.Hash != expectedHash {
			view.Valid = false
			view.Breaks = append(view.Breaks, AuditLogIntegrityBreak{
				ID:           log.ID,
				CreatedAt:    log.CreatedAt,
				Reason:       "hash_mismatch",
				ExpectedHash: expectedHash,
				ActualHash:   log.Hash,
			})
		}
		if i > 0 && log.PrevHash != logs[i-1].Hash {
			view.Valid = false
			view.Breaks = append(view.Breaks, AuditLogIntegrityBreak{
				ID:               log.ID,
				CreatedAt:        log.CreatedAt,
				Reason:           "prev_hash_mismatch",
				ExpectedPrevHash: logs[i-1].Hash,
				ActualPrevHash:   log.PrevHash,
			})
		}
	}
	return view, nil
}

func auditLogIntegrityQuery(req AuditLogIntegrityRequest) (string, []any) {
	query := `
SELECT
  id::text,
  COALESCE(company_id::text, ''),
  COALESCE(domain_id::text, ''),
  COALESCE(user_id::text, ''),
  COALESCE(actor_id::text, ''),
  category,
  action,
  target_type,
  COALESCE(target_id::text, ''),
  COALESCE(ip_address::text, ''),
  user_agent,
  result,
  detail,
  prev_hash,
  hash,
  created_at
FROM (
  SELECT
    id,
    company_id,
    domain_id,
    user_id,
    actor_id,
    category,
    action,
    target_type,
    target_id,
    ip_address,
    user_agent,
    result,
    detail,
    prev_hash,
    hash,
    created_at
  FROM audit_logs`
	var args []any
	if !req.Since.IsZero() {
		args = append(args, req.Since.UTC())
		query += fmt.Sprintf(`
  WHERE created_at >= $%d`, len(args))
	}
	args = append(args, req.Limit)
	query += fmt.Sprintf(`
  ORDER BY created_at DESC, id DESC
  LIMIT $%d
) recent
ORDER BY created_at ASC, id ASC`, len(args))
	return query, args
}

func normalizeAuditLogListRequest(req AuditLogListRequest) AuditLogListRequest {
	req.Limit = normalizeLimit(req.Limit)
	req.Category = strings.TrimSpace(req.Category)
	req.Action = strings.TrimSpace(req.Action)
	req.ActionPrefix = strings.TrimSpace(req.ActionPrefix)
	req.Result = strings.TrimSpace(req.Result)
	req.TargetType = strings.TrimSpace(req.TargetType)
	req.CompanyID = strings.TrimSpace(req.CompanyID)
	req.DomainID = strings.TrimSpace(req.DomainID)
	req.UserID = strings.TrimSpace(req.UserID)
	req.ActorID = strings.TrimSpace(req.ActorID)
	req.TargetID = strings.TrimSpace(req.TargetID)
	return req
}

func normalizeAuditLogIntegrityRequest(req AuditLogIntegrityRequest) AuditLogIntegrityRequest {
	req.Limit = normalizeLimit(req.Limit)
	return req
}

type auditLogScanner interface {
	Scan(...any) error
}

func scanAuditLog(scanner auditLogScanner) (AuditLogView, error) {
	var log AuditLogView
	if err := scanner.Scan(
		&log.ID,
		&log.CompanyID,
		&log.DomainID,
		&log.UserID,
		&log.ActorID,
		&log.Category,
		&log.Action,
		&log.TargetType,
		&log.TargetID,
		&log.IPAddress,
		&log.UserAgent,
		&log.Result,
		&log.Detail,
		&log.PrevHash,
		&log.Hash,
		&log.CreatedAt,
	); err != nil {
		return AuditLogView{}, fmt.Errorf("scan audit log: %w", err)
	}
	if len(log.Detail) == 0 {
		log.Detail = json.RawMessage(`{}`)
	}
	log.CreatedAt = log.CreatedAt.UTC()
	return log, nil
}
