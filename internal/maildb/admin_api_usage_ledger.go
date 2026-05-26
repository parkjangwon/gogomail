package maildb

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
)

func (r *Repository) ListAPIUsageLedger(ctx context.Context, req APIUsageLedgerListRequest) ([]APIUsageLedgerView, error) {
	var usages []APIUsageLedgerView
	if err := r.StreamAPIUsageLedger(ctx, req, func(usage APIUsageLedgerView) error {
		usages = append(usages, usage)
		return nil
	}); err != nil {
		return nil, err
	}
	return usages, nil
}

func (r *Repository) StreamAPIUsageLedger(ctx context.Context, req APIUsageLedgerListRequest, yield func(APIUsageLedgerView) error) error {
	if r.db == nil {
		return fmt.Errorf("database handle is required")
	}
	if yield == nil {
		return fmt.Errorf("api usage ledger yield function is required")
	}
	limit, unbounded := apiUsageLedgerStreamLimit(req.Limit)

	query := `
SELECT
  event_id,
  schema_version,
  event_timestamp,
  recorded_at,
  method,
  route,
  status,
  tenant_id,
  company_id,
  domain_id,
  user_id,
  api_key_id,
  principal_id,
  auth_source,
  request_count,
  request_bytes,
  response_bytes,
  latency_ms,
  payload
FROM api_usage_ledger`
	var conditions []string
	var args []any
	if tenantID := strings.TrimSpace(req.TenantID); tenantID != "" {
		args = append(args, tenantID)
		conditions = append(conditions, fmt.Sprintf("tenant_id = $%d", len(args)))
	}
	if principalID := strings.TrimSpace(req.PrincipalID); principalID != "" {
		args = append(args, principalID)
		conditions = append(conditions, fmt.Sprintf("principal_id = $%d", len(args)))
	}
	if !req.From.IsZero() {
		args = append(args, req.From.UTC())
		conditions = append(conditions, fmt.Sprintf("event_timestamp >= $%d", len(args)))
	}
	if !req.To.IsZero() {
		args = append(args, req.To.UTC())
		conditions = append(conditions, fmt.Sprintf("event_timestamp < $%d", len(args)))
	}
	if len(conditions) > 0 {
		query += "\nWHERE " + strings.Join(conditions, "\n  AND ")
	}
	if unbounded {
		query += `
ORDER BY event_timestamp DESC, event_id DESC
`
	} else {
		args = append(args, limit)
		query += fmt.Sprintf(`
ORDER BY event_timestamp DESC, event_id DESC
LIMIT $%d`, len(args))
	}

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("stream api usage ledger: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var usage APIUsageLedgerView
		if err := rows.Scan(
			&usage.EventID,
			&usage.SchemaVersion,
			&usage.EventTime,
			&usage.RecordedAt,
			&usage.Method,
			&usage.Route,
			&usage.Status,
			&usage.TenantID,
			&usage.CompanyID,
			&usage.DomainID,
			&usage.UserID,
			&usage.APIKeyID,
			&usage.PrincipalID,
			&usage.AuthSource,
			&usage.RequestCount,
			&usage.RequestBytes,
			&usage.ResponseBytes,
			&usage.LatencyMS,
			&usage.Payload,
		); err != nil {
			return fmt.Errorf("scan api usage ledger: %w", err)
		}
		if err := yield(usage); err != nil {
			return fmt.Errorf("yield api usage ledger: %w", err)
		}
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate api usage ledger: %w", err)
	}
	return nil
}

func apiUsageLedgerStreamLimit(limit int) (int, bool) {
	if limit == APIUsageLedgerNoLimit {
		return 0, true
	}
	return normalizeLimit(limit), false
}

func (r *Repository) GetAPIUsageLedgerStats(ctx context.Context, req APIUsageLedgerListRequest) (APIUsageLedgerStatsView, error) {
	if r.db == nil {
		return APIUsageLedgerStatsView{}, fmt.Errorf("database handle is required")
	}
	query := `
SELECT
  count(*)::bigint,
  COALESCE(sum(request_count), 0)::bigint,
  COALESCE(sum(request_bytes), 0)::bigint,
  COALESCE(sum(response_bytes), 0)::bigint,
  COALESCE(sum(latency_ms), 0)::bigint,
  COALESCE(max(latency_ms), 0)::bigint,
  min(event_timestamp),
  max(event_timestamp)
FROM api_usage_ledger`
	var conditions []string
	var args []any
	if tenantID := strings.TrimSpace(req.TenantID); tenantID != "" {
		args = append(args, tenantID)
		conditions = append(conditions, fmt.Sprintf("tenant_id = $%d", len(args)))
	}
	if principalID := strings.TrimSpace(req.PrincipalID); principalID != "" {
		args = append(args, principalID)
		conditions = append(conditions, fmt.Sprintf("principal_id = $%d", len(args)))
	}
	if !req.From.IsZero() {
		args = append(args, req.From.UTC())
		conditions = append(conditions, fmt.Sprintf("event_timestamp >= $%d", len(args)))
	}
	if !req.To.IsZero() {
		args = append(args, req.To.UTC())
		conditions = append(conditions, fmt.Sprintf("event_timestamp < $%d", len(args)))
	}
	if len(conditions) > 0 {
		query += "\nWHERE " + strings.Join(conditions, "\n  AND ")
	}

	var stats APIUsageLedgerStatsView
	var firstEventAt sql.NullTime
	var lastEventAt sql.NullTime
	if err := r.db.QueryRowContext(ctx, query, args...).Scan(
		&stats.EventCount,
		&stats.RequestCount,
		&stats.RequestBytes,
		&stats.ResponseBytes,
		&stats.LatencyMSTotal,
		&stats.LatencyMSMax,
		&firstEventAt,
		&lastEventAt,
	); err != nil {
		return APIUsageLedgerStatsView{}, fmt.Errorf("get api usage ledger stats: %w", err)
	}
	if firstEventAt.Valid {
		stats.FirstEventAt = &firstEventAt.Time
	}
	if lastEventAt.Valid {
		stats.LastEventAt = &lastEventAt.Time
	}
	if stats.RequestCount > 0 {
		stats.LatencyMSAverage = float64(stats.LatencyMSTotal) / float64(stats.RequestCount)
	}
	return stats, nil
}
