package maildb

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/gogomail/gogomail/internal/apimeter"
	"github.com/gogomail/gogomail/internal/audit"
)

func (r *Repository) ListQuotaUsage(ctx context.Context, req QuotaUsageListRequest) ([]QuotaUsageView, error) {
	if r.db == nil {
		return nil, fmt.Errorf("database handle is required")
	}
	req, err := normalizeQuotaUsageListRequest(req)
	if err != nil {
		return nil, err
	}

	query, args := buildListQuotaUsageQuery(req)
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list quota usage: %w", err)
	}
	defer rows.Close()

	var usages []QuotaUsageView
	for rows.Next() {
		var usage QuotaUsageView
		if err := rows.Scan(
			&usage.Scope,
			&usage.ID,
			&usage.DomainID,
			&usage.Name,
			&usage.QuotaUsed,
			&usage.QuotaLimit,
			&usage.AllocatedQuota,
			&usage.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan quota usage: %w", err)
		}
		usage.QuotaRemaining = quotaRemaining(usage.QuotaUsed, usage.QuotaLimit)
		usage.AllocatableQuota = quotaRemaining(usage.AllocatedQuota, usage.QuotaLimit)
		usage.UsageRatio = quotaUsageRatio(usage.QuotaUsed, usage.QuotaLimit)
		usage.AllocationRatio = quotaUsageRatio(usage.AllocatedQuota, usage.QuotaLimit)
		usage.OverLimit = usage.QuotaLimit > 0 && usage.QuotaUsed >= usage.QuotaLimit
		usage.OverAllocated = usage.QuotaLimit > 0 && usage.AllocatedQuota > usage.QuotaLimit
		usages = append(usages, usage)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate quota usage: %w", err)
	}
	return usages, nil
}

func buildListQuotaUsageQuery(req QuotaUsageListRequest) (string, []any) {
	args := []any{req.Limit}
	where := make([]string, 0, 4)
	if req.Scope != "" {
		args = append(args, req.Scope)
		where = append(where, fmt.Sprintf("scope = $%d", len(args)))
	}
	if req.DomainID != "" {
		args = append(args, req.DomainID)
		where = append(where, fmt.Sprintf("domain_id = $%d", len(args)))
	}
	if req.OverLimit != nil {
		args = append(args, *req.OverLimit)
		where = append(where, fmt.Sprintf("(quota_used >= quota_limit) = $%d::bool", len(args)))
	}
	if req.OverAllocated != nil {
		args = append(args, *req.OverAllocated)
		where = append(where, fmt.Sprintf("(allocated_quota > quota_limit) = $%d::bool", len(args)))
	}

	var builder strings.Builder
	builder.WriteString(`
SELECT scope, id, domain_id, name, quota_used, quota_limit, allocated_quota, updated_at
FROM (
  SELECT
    'company' AS scope,
    id::text AS id,
    '' AS domain_id,
    name AS name,
    quota_used,
    quota_limit,
    COALESCE((
      SELECT SUM(child.quota_limit)
      FROM domains child
      WHERE child.company_id = companies.id
        AND child.quota_limit IS NOT NULL
        AND child.quota_limit > 0
    ), 0) AS allocated_quota,
    updated_at
  FROM companies
  WHERE quota_limit IS NOT NULL AND quota_limit > 0
  UNION ALL
  SELECT
    'domain' AS scope,
    id::text AS id,
    id::text AS domain_id,
    name AS name,
    quota_used,
    quota_limit,
    COALESCE((
      SELECT SUM(child.quota_limit)
      FROM users child
      WHERE child.domain_id = domains.id
        AND child.quota_limit IS NOT NULL
        AND child.quota_limit > 0
    ), 0) AS allocated_quota,
    updated_at
  FROM domains
  WHERE quota_limit IS NOT NULL AND quota_limit > 0
  UNION ALL
  SELECT
    'user' AS scope,
    users.id::text AS id,
    users.domain_id::text AS domain_id,
    users.username || '@' || domains.name_ace AS name,
    users.quota_used,
    users.quota_limit,
    0::bigint AS allocated_quota,
    users.updated_at
  FROM users
  JOIN domains ON domains.id = users.domain_id
  WHERE users.quota_limit IS NOT NULL AND users.quota_limit > 0
) usage
`)
	if len(where) > 0 {
		builder.WriteString("WHERE ")
		builder.WriteString(strings.Join(where, "\n  AND "))
		builder.WriteByte('\n')
	}
	builder.WriteString("ORDER BY (quota_used::double precision / quota_limit::double precision) DESC, updated_at DESC, id DESC\nLIMIT $1")
	return builder.String(), args
}

func normalizeQuotaUsageListRequest(req QuotaUsageListRequest) (QuotaUsageListRequest, error) {
	req.Limit = normalizeLimit(req.Limit)
	req.Scope = strings.ToLower(strings.TrimSpace(req.Scope))
	if req.Scope != "" {
		switch req.Scope {
		case "company", "domain", "user":
		default:
			return QuotaUsageListRequest{}, fmt.Errorf("unsupported quota usage scope %q", req.Scope)
		}
	}
	var err error
	if req.DomainID, err = normalizeAPIUsageAggregateFilter("domain_id", req.DomainID, false); err != nil {
		return QuotaUsageListRequest{}, err
	}
	return req, nil
}

func (r *Repository) ListAPIUsageDaily(ctx context.Context, req APIUsageAggregateListRequest) ([]APIUsageDailyView, error) {
	if r.db == nil {
		return nil, fmt.Errorf("database handle is required")
	}
	filters, err := normalizeAPIUsageAggregateListRequest(req)
	if err != nil {
		return nil, err
	}

	query, args := buildAPIUsageAggregateQuery("api_usage_daily", "day", filters)
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list api usage daily: %w", err)
	}
	defer rows.Close()

	var usages []APIUsageDailyView
	for rows.Next() {
		var usage APIUsageDailyView
		if err := rows.Scan(
			&usage.Day,
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
			&usage.LatencyMSTotal,
			&usage.LatencyMSMax,
			&usage.FirstSeenAt,
			&usage.LastSeenAt,
		); err != nil {
			return nil, fmt.Errorf("scan api usage daily: %w", err)
		}
		if usage.RequestCount > 0 {
			usage.LatencyMSAverage = float64(usage.LatencyMSTotal) / float64(usage.RequestCount)
		}
		usages = append(usages, usage)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate api usage daily: %w", err)
	}
	return usages, nil
}

func (r *Repository) ListAPIUsageMonthly(ctx context.Context, req APIUsageAggregateListRequest) ([]APIUsageMonthlyView, error) {
	if r.db == nil {
		return nil, fmt.Errorf("database handle is required")
	}
	filters, err := normalizeAPIUsageAggregateListRequest(req)
	if err != nil {
		return nil, err
	}

	query, args := buildAPIUsageAggregateQuery("api_usage_monthly", "month", filters)
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list api usage monthly: %w", err)
	}
	defer rows.Close()

	var usages []APIUsageMonthlyView
	for rows.Next() {
		var usage APIUsageMonthlyView
		if err := rows.Scan(
			&usage.Month,
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
			&usage.LatencyMSTotal,
			&usage.LatencyMSMax,
			&usage.FirstSeenAt,
			&usage.LastSeenAt,
		); err != nil {
			return nil, fmt.Errorf("scan api usage monthly: %w", err)
		}
		if usage.RequestCount > 0 {
			usage.LatencyMSAverage = float64(usage.LatencyMSTotal) / float64(usage.RequestCount)
		}
		usages = append(usages, usage)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate api usage monthly: %w", err)
	}
	return usages, nil
}

func buildAPIUsageAggregateQuery(tableName string, periodColumn string, filters APIUsageAggregateListRequest) (string, []any) {
	query := fmt.Sprintf("SELECT\n  %s,%s\nFROM %s", periodColumn, apiUsageAggregateProjectionSQL, tableName)
	var conditions []string
	var args []any

	for _, filter := range []struct {
		column string
		value  string
	}{
		{"tenant_id", filters.TenantID},
		{"company_id", filters.CompanyID},
		{"domain_id", filters.DomainID},
		{"user_id", filters.UserID},
		{"api_key_id", filters.APIKeyID},
		{"principal_id", filters.PrincipalID},
		{"auth_source", filters.AuthSource},
		{"method", filters.Method},
		{"route", filters.Route},
	} {
		if filter.value == "" {
			continue
		}
		args = append(args, filter.value)
		conditions = append(conditions, fmt.Sprintf("%s = $%d", filter.column, len(args)))
	}
	if filters.Status != 0 {
		args = append(args, filters.Status)
		conditions = append(conditions, fmt.Sprintf("status = $%d", len(args)))
	}
	if !filters.From.IsZero() {
		args = append(args, filters.From.UTC())
		conditions = append(conditions, fmt.Sprintf("%s >= $%d", periodColumn, len(args)))
	}
	if !filters.To.IsZero() {
		args = append(args, filters.To.UTC())
		conditions = append(conditions, fmt.Sprintf("%s < $%d", periodColumn, len(args)))
	}
	if len(conditions) > 0 {
		query += "\nWHERE " + strings.Join(conditions, "\n  AND ")
	}
	args = append(args, filters.Limit)
	query += fmt.Sprintf(`
ORDER BY %s DESC, request_count DESC, route, status
LIMIT $%d`, periodColumn, len(args))
	return query, args
}

func normalizeAPIUsageAggregateListRequest(req APIUsageAggregateListRequest) (APIUsageAggregateListRequest, error) {
	req.Limit = normalizeLimit(req.Limit)
	var err error
	if req.TenantID, err = normalizeAPIUsageAggregateFilter("tenant_id", req.TenantID, false); err != nil {
		return APIUsageAggregateListRequest{}, err
	}
	if req.CompanyID, err = normalizeAPIUsageAggregateFilter("company_id", req.CompanyID, false); err != nil {
		return APIUsageAggregateListRequest{}, err
	}
	if req.DomainID, err = normalizeAPIUsageAggregateFilter("domain_id", req.DomainID, false); err != nil {
		return APIUsageAggregateListRequest{}, err
	}
	if req.UserID, err = normalizeAPIUsageAggregateFilter("user_id", req.UserID, false); err != nil {
		return APIUsageAggregateListRequest{}, err
	}
	if req.APIKeyID, err = normalizeAPIUsageAggregateFilter("api_key_id", req.APIKeyID, false); err != nil {
		return APIUsageAggregateListRequest{}, err
	}
	if req.PrincipalID, err = normalizeAPIUsageAggregateFilter("principal_id", req.PrincipalID, false); err != nil {
		return APIUsageAggregateListRequest{}, err
	}
	if req.AuthSource, err = normalizeAPIUsageAggregateFilter("auth_source", req.AuthSource, true); err != nil {
		return APIUsageAggregateListRequest{}, err
	}
	if req.Method, err = normalizeAPIUsageAggregateFilter("method", req.Method, false); err != nil {
		return APIUsageAggregateListRequest{}, err
	}
	if req.Route, err = normalizeAPIUsageAggregateFilter("route", req.Route, false); err != nil {
		return APIUsageAggregateListRequest{}, err
	}
	if req.Status < 0 || req.Status > 599 || (req.Status > 0 && req.Status < 100) {
		return APIUsageAggregateListRequest{}, fmt.Errorf("status must be an HTTP-like status code")
	}
	if !req.From.IsZero() {
		req.From = req.From.UTC()
	}
	if !req.To.IsZero() {
		req.To = req.To.UTC()
	}
	if !req.From.IsZero() && !req.To.IsZero() && !req.From.Before(req.To) {
		return APIUsageAggregateListRequest{}, fmt.Errorf("from must be before to")
	}
	return req, nil
}

func normalizeAPIUsageAggregateFilter(name, value string, lower bool) (string, error) {
	value = strings.TrimSpace(value)
	if lower {
		value = strings.ToLower(value)
	}
	if err := validatePushNotificationFilter(name, value); err != nil {
		return "", err
	}
	return value, nil
}

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

func (r *Repository) GetAPIUsageLedgerRetentionReadiness(ctx context.Context, req APIUsageLedgerRetentionRequest) (APIUsageLedgerRetentionReadinessView, error) {
	if r.db == nil {
		return APIUsageLedgerRetentionReadinessView{}, fmt.Errorf("database handle is required")
	}
	req.TenantID = strings.TrimSpace(req.TenantID)
	req.PrincipalID = strings.TrimSpace(req.PrincipalID)
	if req.Cutoff.IsZero() {
		return APIUsageLedgerRetentionReadinessView{}, fmt.Errorf("cutoff is required")
	}
	req.Cutoff = req.Cutoff.UTC()
	if req.Cutoff.After(time.Now().UTC()) {
		return APIUsageLedgerRetentionReadinessView{}, fmt.Errorf("cutoff must not be in the future")
	}

	view := APIUsageLedgerRetentionReadinessView{
		Cutoff:      req.Cutoff,
		TenantID:    req.TenantID,
		PrincipalID: req.PrincipalID,
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
  max(event_timestamp),
  max(recorded_at)
FROM api_usage_ledger`
	var conditions []string
	var args []any
	args = append(args, req.Cutoff)
	conditions = append(conditions, fmt.Sprintf("event_timestamp < $%d", len(args)))
	if req.TenantID != "" {
		args = append(args, req.TenantID)
		conditions = append(conditions, fmt.Sprintf("tenant_id = $%d", len(args)))
	}
	if req.PrincipalID != "" {
		args = append(args, req.PrincipalID)
		conditions = append(conditions, fmt.Sprintf("principal_id = $%d", len(args)))
	}
	query += "\nWHERE " + strings.Join(conditions, "\n  AND ")

	var firstCandidateAt sql.NullTime
	var lastCandidateAt sql.NullTime
	var latestRecordedAt sql.NullTime
	if err := r.db.QueryRowContext(ctx, query, args...).Scan(
		&view.CandidateEventCount,
		&view.CandidateRequestCount,
		&view.CandidateRequestBytes,
		&view.CandidateResponseBytes,
		&view.CandidateLatencyMSTotal,
		&view.CandidateLatencyMSMax,
		&firstCandidateAt,
		&lastCandidateAt,
		&latestRecordedAt,
	); err != nil {
		return APIUsageLedgerRetentionReadinessView{}, fmt.Errorf("get api usage ledger retention candidates: %w", err)
	}
	if firstCandidateAt.Valid {
		view.FirstCandidateEventAt = &firstCandidateAt.Time
	}
	if lastCandidateAt.Valid {
		view.LastCandidateEventAt = &lastCandidateAt.Time
	}
	if latestRecordedAt.Valid {
		view.LatestCandidateRecordedAt = &latestRecordedAt.Time
	}
	if view.CandidateEventCount > 0 && view.FirstCandidateEventAt != nil {
		if err := r.findAPIUsageLedgerRetentionCoveringBatch(ctx, req, view.FirstCandidateEventAt, &view); err != nil {
			return APIUsageLedgerRetentionReadinessView{}, err
		}
	}
	applyAPIUsageLedgerRetentionReadiness(&view)
	return view, nil
}

func NormalizeAPIUsageLedgerRetentionLimit(limit int) int {
	if limit <= 0 {
		return APIUsageLedgerRetentionDefaultLimit
	}
	if limit > APIUsageLedgerRetentionMaxLimit {
		return APIUsageLedgerRetentionMaxLimit
	}
	return limit
}

func (r *Repository) RunAPIUsageLedgerRetention(ctx context.Context, req APIUsageLedgerRetentionRunRequest) (APIUsageLedgerRetentionRunView, error) {
	if r.db == nil {
		return APIUsageLedgerRetentionRunView{}, fmt.Errorf("database handle is required")
	}
	req.TenantID = strings.TrimSpace(req.TenantID)
	req.PrincipalID = strings.TrimSpace(req.PrincipalID)
	if req.Cutoff.IsZero() {
		return APIUsageLedgerRetentionRunView{}, fmt.Errorf("cutoff is required")
	}
	req.Cutoff = req.Cutoff.UTC()
	if req.Cutoff.After(time.Now().UTC()) {
		return APIUsageLedgerRetentionRunView{}, fmt.Errorf("cutoff must not be in the future")
	}
	if req.Limit < 0 {
		return APIUsageLedgerRetentionRunView{}, fmt.Errorf("limit must not be negative")
	}
	if !req.DryRun && !req.ConfirmReady {
		return APIUsageLedgerRetentionRunView{}, fmt.Errorf("confirm_ready is required for destructive retention runs")
	}
	limit := NormalizeAPIUsageLedgerRetentionLimit(req.Limit)
	id, err := newAPIUsageLedgerRetentionRunID()
	if err != nil {
		return APIUsageLedgerRetentionRunView{}, err
	}

	readiness, err := r.GetAPIUsageLedgerRetentionReadiness(ctx, APIUsageLedgerRetentionRequest{
		Cutoff:      req.Cutoff,
		TenantID:    req.TenantID,
		PrincipalID: req.PrincipalID,
	})
	if err != nil {
		return APIUsageLedgerRetentionRunView{}, err
	}
	limited := readiness.CandidateEventCount
	if limited > int64(limit) {
		limited = int64(limit)
	}
	view := APIUsageLedgerRetentionRunView{
		ID:             id,
		Cutoff:         req.Cutoff,
		TenantID:       req.TenantID,
		PrincipalID:    req.PrincipalID,
		Limit:          limit,
		DryRun:         req.DryRun,
		ConfirmReady:   req.ConfirmReady,
		Ready:          readiness.Ready,
		CandidateCount: readiness.CandidateEventCount,
		LimitedCount:   limited,
		Readiness:      readiness,
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return APIUsageLedgerRetentionRunView{}, fmt.Errorf("begin api usage ledger retention transaction: %w", err)
	}
	defer tx.Rollback()

	if req.DryRun || !readiness.Ready || limited == 0 {
		if err := r.insertAPIUsageLedgerRetentionRun(ctx, tx, &view); err != nil {
			return APIUsageLedgerRetentionRunView{}, err
		}
		if err := recordAPIUsageLedgerRetentionRunAudit(ctx, tx, view); err != nil {
			return APIUsageLedgerRetentionRunView{}, err
		}
		if err := tx.Commit(); err != nil {
			return APIUsageLedgerRetentionRunView{}, fmt.Errorf("commit api usage ledger retention transaction: %w", err)
		}
		return view, nil
	}

	var conditions []string
	var args []any
	args = append(args, req.Cutoff)
	conditions = append(conditions, fmt.Sprintf("event_timestamp < $%d", len(args)))
	if req.TenantID != "" {
		args = append(args, req.TenantID)
		conditions = append(conditions, fmt.Sprintf("tenant_id = $%d", len(args)))
	}
	if req.PrincipalID != "" {
		args = append(args, req.PrincipalID)
		conditions = append(conditions, fmt.Sprintf("principal_id = $%d", len(args)))
	}
	if readiness.CoveringExportBatchCompletedAt != nil {
		args = append(args, readiness.CoveringExportBatchCompletedAt.UTC())
		conditions = append(conditions, fmt.Sprintf("recorded_at <= $%d", len(args)))
	}
	args = append(args, limit)
	limitPlaceholder := fmt.Sprintf("$%d", len(args))
	query := fmt.Sprintf(`
DELETE FROM api_usage_ledger
WHERE event_id IN (
  SELECT event_id
  FROM api_usage_ledger
  WHERE %s
  ORDER BY event_timestamp ASC, event_id ASC
  LIMIT %s
)`, strings.Join(conditions, "\n    AND "), limitPlaceholder)

	result, err := tx.ExecContext(ctx, query, args...)
	if err != nil {
		return APIUsageLedgerRetentionRunView{}, fmt.Errorf("run api usage ledger retention: %w", err)
	}
	view.DeletedCount, _ = result.RowsAffected()
	if err := r.insertAPIUsageLedgerRetentionRun(ctx, tx, &view); err != nil {
		return APIUsageLedgerRetentionRunView{}, err
	}
	if err := recordAPIUsageLedgerRetentionRunAudit(ctx, tx, view); err != nil {
		return APIUsageLedgerRetentionRunView{}, err
	}
	if err := tx.Commit(); err != nil {
		return APIUsageLedgerRetentionRunView{}, fmt.Errorf("commit api usage ledger retention transaction: %w", err)
	}
	return view, nil
}

func recordAPIUsageLedgerRetentionRunAudit(ctx context.Context, tx *sql.Tx, view APIUsageLedgerRetentionRunView) error {
	detail, err := apiUsageLedgerRetentionRunAuditDetail(view)
	if err != nil {
		return err
	}
	if err := audit.InsertTx(ctx, tx, audit.Log{
		Category:   "admin",
		Action:     "api_usage.retention_run",
		TargetType: "api_usage_ledger_retention_run",
		TargetID:   view.ID,
		Result:     apiUsageLedgerRetentionRunAuditResult(view),
		Detail:     detail,
	}); err != nil {
		return fmt.Errorf("record api usage ledger retention run audit: %w", err)
	}
	return nil
}

func apiUsageLedgerRetentionRunAuditResult(view APIUsageLedgerRetentionRunView) string {
	switch {
	case view.DryRun:
		return "dry_run"
	case !view.Ready:
		return "blocked"
	case view.DeletedCount == 0:
		return "no_op"
	default:
		return "completed"
	}
}

func apiUsageLedgerRetentionRunAuditDetail(view APIUsageLedgerRetentionRunView) (json.RawMessage, error) {
	detail, err := json.Marshal(map[string]any{
		"run_id":                             view.ID,
		"cutoff":                             view.Cutoff.UTC().Format(time.RFC3339),
		"tenant_id":                          view.TenantID,
		"principal_id":                       view.PrincipalID,
		"limit":                              view.Limit,
		"dry_run":                            view.DryRun,
		"confirm_ready":                      view.ConfirmReady,
		"ready":                              view.Ready,
		"candidate_count":                    view.CandidateCount,
		"limited_count":                      view.LimitedCount,
		"deleted_count":                      view.DeletedCount,
		"blocking_reasons":                   view.Readiness.BlockingReasons,
		"covering_export_batch_id":           view.Readiness.CoveringExportBatchID,
		"covering_artifact_count":            view.Readiness.CoveringArtifactCount,
		"covering_manifest_digest_count":     view.Readiness.CoveringManifestDigestCount,
		"covering_manifest_signature_count":  view.Readiness.CoveringManifestSignatureCount,
		"covering_export_batch_completed_at": optionalTimeStringPtr(view.Readiness.CoveringExportBatchCompletedAt),
	})
	if err != nil {
		return nil, fmt.Errorf("marshal api usage ledger retention run audit detail: %w", err)
	}
	return detail, nil
}

func (r *Repository) ListAPIUsageLedgerRetentionRuns(ctx context.Context, req APIUsageLedgerRetentionRunListRequest) ([]APIUsageLedgerRetentionRunView, error) {
	if r.db == nil {
		return nil, fmt.Errorf("database handle is required")
	}
	req.Limit = normalizeLimit(req.Limit)
	req.TenantID = strings.TrimSpace(req.TenantID)
	req.PrincipalID = strings.TrimSpace(req.PrincipalID)

	query := `
SELECT id, created_at, cutoff, tenant_id, principal_id, limit_count, dry_run,
  confirm_ready, ready, candidate_count, limited_count, deleted_count, readiness
FROM api_usage_ledger_retention_runs`
	var conditions []string
	var args []any
	if req.TenantID != "" {
		args = append(args, req.TenantID)
		conditions = append(conditions, fmt.Sprintf("tenant_id = $%d", len(args)))
	}
	if req.PrincipalID != "" {
		args = append(args, req.PrincipalID)
		conditions = append(conditions, fmt.Sprintf("principal_id = $%d", len(args)))
	}
	if !req.CreatedFrom.IsZero() {
		args = append(args, req.CreatedFrom.UTC())
		conditions = append(conditions, fmt.Sprintf("created_at >= $%d", len(args)))
	}
	if !req.CreatedTo.IsZero() {
		args = append(args, req.CreatedTo.UTC())
		conditions = append(conditions, fmt.Sprintf("created_at < $%d", len(args)))
	}
	if len(conditions) > 0 {
		query += "\nWHERE " + strings.Join(conditions, "\n  AND ")
	}
	args = append(args, req.Limit)
	query += fmt.Sprintf(`
ORDER BY created_at DESC, id DESC
LIMIT $%d`, len(args))

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list api usage ledger retention runs: %w", err)
	}
	defer rows.Close()

	var runs []APIUsageLedgerRetentionRunView
	for rows.Next() {
		run, err := scanAPIUsageLedgerRetentionRun(rows)
		if err != nil {
			return nil, err
		}
		runs = append(runs, run)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate api usage ledger retention runs: %w", err)
	}
	return runs, nil
}

func (r *Repository) GetAPIUsageLedgerRetentionRun(ctx context.Context, id string) (APIUsageLedgerRetentionRunView, error) {
	if r.db == nil {
		return APIUsageLedgerRetentionRunView{}, fmt.Errorf("database handle is required")
	}
	id = strings.TrimSpace(id)
	if id == "" {
		return APIUsageLedgerRetentionRunView{}, fmt.Errorf("api usage ledger retention run id is required")
	}
	const query = `
SELECT id, created_at, cutoff, tenant_id, principal_id, limit_count, dry_run,
  confirm_ready, ready, candidate_count, limited_count, deleted_count, readiness
FROM api_usage_ledger_retention_runs
WHERE id = $1`
	run, err := scanAPIUsageLedgerRetentionRun(r.db.QueryRowContext(ctx, query, id))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return APIUsageLedgerRetentionRunView{}, fmt.Errorf("api usage ledger retention run not found")
		}
		return APIUsageLedgerRetentionRunView{}, fmt.Errorf("get api usage ledger retention run: %w", err)
	}
	return run, nil
}

type apiUsageLedgerRetentionRunScanner interface {
	Scan(...any) error
}

func scanAPIUsageLedgerRetentionRun(scanner apiUsageLedgerRetentionRunScanner) (APIUsageLedgerRetentionRunView, error) {
	var run APIUsageLedgerRetentionRunView
	var readiness json.RawMessage
	if err := scanner.Scan(
		&run.ID,
		&run.CreatedAt,
		&run.Cutoff,
		&run.TenantID,
		&run.PrincipalID,
		&run.Limit,
		&run.DryRun,
		&run.ConfirmReady,
		&run.Ready,
		&run.CandidateCount,
		&run.LimitedCount,
		&run.DeletedCount,
		&readiness,
	); err != nil {
		return APIUsageLedgerRetentionRunView{}, fmt.Errorf("scan api usage ledger retention run: %w", err)
	}
	if len(readiness) > 0 {
		if err := json.Unmarshal(readiness, &run.Readiness); err != nil {
			return APIUsageLedgerRetentionRunView{}, fmt.Errorf("decode api usage ledger retention run readiness: %w", err)
		}
	}
	run.CreatedAt = run.CreatedAt.UTC()
	run.Cutoff = run.Cutoff.UTC()
	return run, nil
}

func (r *Repository) insertAPIUsageLedgerRetentionRun(ctx context.Context, execer interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
}, view *APIUsageLedgerRetentionRunView) error {
	readiness, err := json.Marshal(view.Readiness)
	if err != nil {
		return fmt.Errorf("marshal api usage ledger retention readiness: %w", err)
	}
	const query = `
INSERT INTO api_usage_ledger_retention_runs (
  id,
  cutoff,
  tenant_id,
  principal_id,
  limit_count,
  dry_run,
  confirm_ready,
  ready,
  candidate_count,
  limited_count,
  deleted_count,
  readiness
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12::jsonb)
RETURNING created_at`
	if err := execer.QueryRowContext(
		ctx,
		query,
		view.ID,
		view.Cutoff,
		view.TenantID,
		view.PrincipalID,
		view.Limit,
		view.DryRun,
		view.ConfirmReady,
		view.Ready,
		view.CandidateCount,
		view.LimitedCount,
		view.DeletedCount,
		string(readiness),
	).Scan(&view.CreatedAt); err != nil {
		return fmt.Errorf("record api usage ledger retention run: %w", err)
	}
	view.CreatedAt = view.CreatedAt.UTC()
	return nil
}

func (r *Repository) findAPIUsageLedgerRetentionCoveringBatch(ctx context.Context, req APIUsageLedgerRetentionRequest, firstCandidateAt *time.Time, view *APIUsageLedgerRetentionReadinessView) error {
	var completedAt sql.NullTime
	var windowStart sql.NullTime
	var windowEnd sql.NullTime
	err := r.db.QueryRowContext(ctx, apiUsageLedgerRetentionCoveringBatchSQL, req.TenantID, req.PrincipalID, firstCandidateAt.UTC(), req.Cutoff).Scan(
		&view.CoveringExportBatchID,
		&completedAt,
		&windowStart,
		&windowEnd,
		&view.CoveringExportBatchEventCount,
		&view.CoveringArtifactCount,
		&view.CoveringArtifactEventCount,
		&view.CoveringManifestDigestCount,
		&view.CoveringManifestSignatureCount,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil
		}
		return fmt.Errorf("get api usage ledger retention covering export batch: %w", err)
	}
	if completedAt.Valid {
		view.CoveringExportBatchCompletedAt = &completedAt.Time
	}
	if windowStart.Valid {
		view.CoveringExportBatchWindowStart = &windowStart.Time
	}
	if windowEnd.Valid {
		view.CoveringExportBatchWindowEnd = &windowEnd.Time
	}
	return nil
}

const apiUsageLedgerRetentionCoveringBatchSQL = `
SELECT
  b.id,
  b.completed_at,
  b.window_start,
  b.window_end,
  b.event_count,
  COALESCE(a.artifact_count, 0)::bigint,
  COALESCE(a.artifact_event_count, 0)::bigint,
  COALESCE(d.digest_count, 0)::bigint,
  COALESCE(s.signature_count, 0)::bigint
FROM api_usage_export_batches b
LEFT JOIN LATERAL (
  SELECT count(*)::bigint AS artifact_count, COALESCE(sum(event_count), 0)::bigint AS artifact_event_count
  FROM api_usage_export_artifacts
  WHERE batch_id = b.id
) a ON true
LEFT JOIN LATERAL (
  SELECT count(*)::bigint AS digest_count
  FROM api_usage_export_manifest_digests
  WHERE batch_id = b.id
) d ON true
LEFT JOIN LATERAL (
  SELECT count(*)::bigint AS signature_count
  FROM api_usage_export_manifest_signatures
  WHERE batch_id = b.id
) s ON true
WHERE b.status = 'completed'
  AND b.completed_at IS NOT NULL
  AND b.tenant_id = $1
  AND b.principal_id = $2
  AND COALESCE(b.window_start, '-infinity'::timestamptz) <= $3
  AND b.window_end IS NOT NULL
  AND b.window_end >= $4
ORDER BY b.completed_at DESC, b.id DESC
LIMIT 1`

func applyAPIUsageLedgerRetentionReadiness(view *APIUsageLedgerRetentionReadinessView) {
	var blocking []string
	if view.CandidateEventCount > 0 {
		if view.CoveringExportBatchID == "" {
			blocking = append(blocking, "covering_export_batch_required")
		}
		if view.CoveringExportBatchCompletedAt != nil && view.LatestCandidateRecordedAt != nil && view.CoveringExportBatchCompletedAt.Before(*view.LatestCandidateRecordedAt) {
			blocking = append(blocking, "covering_export_batch_stale")
		}
		if view.CoveringExportBatchID != "" && (view.CoveringArtifactCount == 0 || view.CoveringArtifactEventCount < view.CoveringExportBatchEventCount) {
			blocking = append(blocking, "covering_export_artifact_required")
		}
		if view.CoveringExportBatchID != "" && view.CoveringManifestDigestCount == 0 {
			blocking = append(blocking, "covering_manifest_digest_required")
		}
		if view.CoveringExportBatchID != "" && view.CoveringManifestSignatureCount == 0 {
			blocking = append(blocking, "covering_manifest_signature_required")
		}
	}
	view.BlockingReasons = blocking
	view.Ready = len(blocking) == 0
}

func (r *Repository) CreateAPIUsageExportBatch(ctx context.Context, req APIUsageLedgerListRequest) (APIUsageExportBatchView, error) {
	if r.db == nil {
		return APIUsageExportBatchView{}, fmt.Errorf("database handle is required")
	}
	stats, err := r.GetAPIUsageLedgerStats(ctx, req)
	if err != nil {
		return APIUsageExportBatchView{}, err
	}
	id, err := newAPIUsageExportBatchID()
	if err != nil {
		return APIUsageExportBatchView{}, err
	}
	manifest, err := json.Marshal(map[string]any{
		"version":      "2026-05-04.api-usage-export.v1",
		"tenant_id":    strings.TrimSpace(req.TenantID),
		"principal_id": strings.TrimSpace(req.PrincipalID),
		"from":         optionalTimeString(req.From),
		"to":           optionalTimeString(req.To),
		"format":       "ndjson",
	})
	if err != nil {
		return APIUsageExportBatchView{}, fmt.Errorf("marshal api usage export manifest: %w", err)
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return APIUsageExportBatchView{}, fmt.Errorf("begin api usage export batch transaction: %w", err)
	}
	defer tx.Rollback()

	var batch APIUsageExportBatchView
	var completedAt sql.NullTime
	var windowStart sql.NullTime
	var windowEnd sql.NullTime
	var firstEventAt sql.NullTime
	var lastEventAt sql.NullTime
	const query = `
INSERT INTO api_usage_export_batches (
  id,
  completed_at,
  status,
  export_format,
  tenant_id,
  principal_id,
  window_start,
  window_end,
  event_count,
  request_count,
  request_bytes,
  response_bytes,
  latency_ms_total,
  latency_ms_max,
  first_event_at,
  last_event_at,
  manifest
) VALUES ($1, now(), 'completed', 'ndjson', $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)
RETURNING id, created_at, completed_at, status, export_format, tenant_id, principal_id,
  window_start, window_end, event_count, request_count, request_bytes, response_bytes,
  latency_ms_total, latency_ms_max, first_event_at, last_event_at, manifest`
	if err := tx.QueryRowContext(
		ctx,
		query,
		id,
		strings.TrimSpace(req.TenantID),
		strings.TrimSpace(req.PrincipalID),
		nullableTime(req.From),
		nullableTime(req.To),
		stats.EventCount,
		stats.RequestCount,
		stats.RequestBytes,
		stats.ResponseBytes,
		stats.LatencyMSTotal,
		stats.LatencyMSMax,
		nullableTimePtr(stats.FirstEventAt),
		nullableTimePtr(stats.LastEventAt),
		manifest,
	).Scan(
		&batch.ID,
		&batch.CreatedAt,
		&completedAt,
		&batch.Status,
		&batch.ExportFormat,
		&batch.TenantID,
		&batch.PrincipalID,
		&windowStart,
		&windowEnd,
		&batch.EventCount,
		&batch.RequestCount,
		&batch.RequestBytes,
		&batch.ResponseBytes,
		&batch.LatencyMSTotal,
		&batch.LatencyMSMax,
		&firstEventAt,
		&lastEventAt,
		&batch.Manifest,
	); err != nil {
		return APIUsageExportBatchView{}, fmt.Errorf("create api usage export batch: %w", err)
	}
	applyExportBatchNullableTimes(&batch, completedAt, windowStart, windowEnd, firstEventAt, lastEventAt)
	detail, err := apiUsageExportBatchAuditDetail(batch)
	if err != nil {
		return APIUsageExportBatchView{}, err
	}
	if err := audit.InsertTx(ctx, tx, audit.Log{
		Category:   "admin",
		Action:     "api_usage_export.batch_create",
		TargetType: "api_usage_export_batch",
		TargetID:   batch.ID,
		Result:     "created",
		Detail:     detail,
	}); err != nil {
		return APIUsageExportBatchView{}, fmt.Errorf("record api usage export batch audit: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return APIUsageExportBatchView{}, fmt.Errorf("commit api usage export batch transaction: %w", err)
	}
	return batch, nil
}

func apiUsageExportBatchAuditDetail(batch APIUsageExportBatchView) (json.RawMessage, error) {
	detail, err := json.Marshal(map[string]any{
		"batch_id":         batch.ID,
		"tenant_id":        batch.TenantID,
		"principal_id":     batch.PrincipalID,
		"status":           batch.Status,
		"export_format":    batch.ExportFormat,
		"window_start":     optionalTimeStringPtr(batch.WindowStart),
		"window_end":       optionalTimeStringPtr(batch.WindowEnd),
		"event_count":      batch.EventCount,
		"request_count":    batch.RequestCount,
		"request_bytes":    batch.RequestBytes,
		"response_bytes":   batch.ResponseBytes,
		"latency_ms_total": batch.LatencyMSTotal,
		"latency_ms_max":   batch.LatencyMSMax,
		"first_event_at":   optionalTimeStringPtr(batch.FirstEventAt),
		"last_event_at":    optionalTimeStringPtr(batch.LastEventAt),
	})
	if err != nil {
		return nil, fmt.Errorf("marshal api usage export batch audit detail: %w", err)
	}
	return detail, nil
}

func optionalTimeStringPtr(value *time.Time) string {
	if value == nil || value.IsZero() {
		return ""
	}
	return value.UTC().Format(time.RFC3339)
}

func ValidateAPIUsageExportBatchListRequest(req APIUsageExportBatchListRequest) error {
	for field, value := range map[string]string{
		"tenant_id":    strings.TrimSpace(req.TenantID),
		"principal_id": strings.TrimSpace(req.PrincipalID),
	} {
		if err := validatePushNotificationFilter(field, value); err != nil {
			return err
		}
	}
	status := strings.ToLower(strings.TrimSpace(req.Status))
	if status != "" && !isAPIUsageExportBatchStatus(status) {
		return fmt.Errorf("unsupported api usage export batch status %q", req.Status)
	}
	if !req.From.IsZero() && !req.To.IsZero() && !req.From.Before(req.To) {
		return fmt.Errorf("from must be before to")
	}
	return nil
}

func isAPIUsageExportBatchStatus(status string) bool {
	switch status {
	case "pending", "completed", "failed":
		return true
	default:
		return false
	}
}

func buildListAPIUsageExportBatchesQuery(req APIUsageExportBatchListRequest) (string, []any) {
	query := listAPIUsageExportBatchesBaseSQL
	var conditions []string
	var args []any

	tenantID := strings.TrimSpace(req.TenantID)
	if tenantID != "" {
		args = append(args, tenantID)
		conditions = append(conditions, fmt.Sprintf("tenant_id = $%d", len(args)))
	}
	principalID := strings.TrimSpace(req.PrincipalID)
	if principalID != "" {
		args = append(args, principalID)
		conditions = append(conditions, fmt.Sprintf("principal_id = $%d", len(args)))
	}
	status := strings.ToLower(strings.TrimSpace(req.Status))
	if status != "" {
		args = append(args, status)
		conditions = append(conditions, fmt.Sprintf("status = $%d", len(args)))
	}
	if !req.From.IsZero() {
		args = append(args, req.From.UTC())
		conditions = append(conditions, fmt.Sprintf("window_start >= $%d", len(args)))
	}
	if !req.To.IsZero() {
		args = append(args, req.To.UTC())
		conditions = append(conditions, fmt.Sprintf("window_end < $%d", len(args)))
	}
	if len(conditions) > 0 {
		query += "\nWHERE " + strings.Join(conditions, "\n  AND ")
	}
	args = append(args, normalizeLimit(req.Limit))
	query += fmt.Sprintf(`
ORDER BY created_at DESC, id DESC
LIMIT $%d`, len(args))
	return query, args
}

func (r *Repository) ListAPIUsageExportBatches(ctx context.Context, req APIUsageExportBatchListRequest) ([]APIUsageExportBatchView, error) {
	if r.db == nil {
		return nil, fmt.Errorf("database handle is required")
	}
	if err := ValidateAPIUsageExportBatchListRequest(req); err != nil {
		return nil, err
	}
	query, args := buildListAPIUsageExportBatchesQuery(req)
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list api usage export batches: %w", err)
	}
	defer rows.Close()

	var batches []APIUsageExportBatchView
	for rows.Next() {
		var batch APIUsageExportBatchView
		var completedAt sql.NullTime
		var windowStart sql.NullTime
		var windowEnd sql.NullTime
		var firstEventAt sql.NullTime
		var lastEventAt sql.NullTime
		if err := rows.Scan(
			&batch.ID,
			&batch.CreatedAt,
			&completedAt,
			&batch.Status,
			&batch.ExportFormat,
			&batch.TenantID,
			&batch.PrincipalID,
			&windowStart,
			&windowEnd,
			&batch.EventCount,
			&batch.RequestCount,
			&batch.RequestBytes,
			&batch.ResponseBytes,
			&batch.LatencyMSTotal,
			&batch.LatencyMSMax,
			&firstEventAt,
			&lastEventAt,
			&batch.Manifest,
		); err != nil {
			return nil, fmt.Errorf("scan api usage export batch: %w", err)
		}
		applyExportBatchNullableTimes(&batch, completedAt, windowStart, windowEnd, firstEventAt, lastEventAt)
		batches = append(batches, batch)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate api usage export batches: %w", err)
	}
	return batches, nil
}

func (r *Repository) GetAPIUsageExportBatch(ctx context.Context, id string) (APIUsageExportBatchView, error) {
	if r.db == nil {
		return APIUsageExportBatchView{}, fmt.Errorf("database handle is required")
	}
	id = strings.TrimSpace(id)
	if id == "" {
		return APIUsageExportBatchView{}, fmt.Errorf("api usage export batch id is required")
	}
	const query = `
SELECT id, created_at, completed_at, status, export_format, tenant_id, principal_id,
  window_start, window_end, event_count, request_count, request_bytes, response_bytes,
  latency_ms_total, latency_ms_max, first_event_at, last_event_at, manifest
FROM api_usage_export_batches
WHERE id = $1`
	var batch APIUsageExportBatchView
	var completedAt sql.NullTime
	var windowStart sql.NullTime
	var windowEnd sql.NullTime
	var firstEventAt sql.NullTime
	var lastEventAt sql.NullTime
	if err := r.db.QueryRowContext(ctx, query, id).Scan(
		&batch.ID,
		&batch.CreatedAt,
		&completedAt,
		&batch.Status,
		&batch.ExportFormat,
		&batch.TenantID,
		&batch.PrincipalID,
		&windowStart,
		&windowEnd,
		&batch.EventCount,
		&batch.RequestCount,
		&batch.RequestBytes,
		&batch.ResponseBytes,
		&batch.LatencyMSTotal,
		&batch.LatencyMSMax,
		&firstEventAt,
		&lastEventAt,
		&batch.Manifest,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return APIUsageExportBatchView{}, fmt.Errorf("api usage export batch not found")
		}
		return APIUsageExportBatchView{}, fmt.Errorf("get api usage export batch: %w", err)
	}
	applyExportBatchNullableTimes(&batch, completedAt, windowStart, windowEnd, firstEventAt, lastEventAt)
	return batch, nil
}

func (r *Repository) GetAPIUsageExportHandoff(ctx context.Context, batchID string) (APIUsageExportHandoffView, error) {
	if r.db == nil {
		return APIUsageExportHandoffView{}, fmt.Errorf("database handle is required")
	}
	batch, err := r.GetAPIUsageExportBatch(ctx, batchID)
	if err != nil {
		return APIUsageExportHandoffView{}, err
	}
	view := APIUsageExportHandoffView{
		BatchID:        batch.ID,
		BatchStatus:    batch.Status,
		BatchCompleted: batch.Status == "completed" && batch.CompletedAt != nil,
		EventCount:     batch.EventCount,
	}
	const artifactQuery = `
SELECT count(*), coalesce(sum(event_count), 0), coalesce(sum(byte_count), 0)
FROM api_usage_export_artifacts
WHERE batch_id = $1`
	if err := r.db.QueryRowContext(ctx, artifactQuery, batch.ID).Scan(
		&view.ArtifactCount,
		&view.ArtifactEventCount,
		&view.ArtifactByteCount,
	); err != nil {
		return APIUsageExportHandoffView{}, fmt.Errorf("get api usage export artifact handoff stats: %w", err)
	}

	var latestDigestAt sql.NullTime
	const digestQuery = `
SELECT count(*),
  coalesce((array_agg(id ORDER BY created_at DESC, id DESC))[1], ''),
  coalesce((array_agg(digest_hex ORDER BY created_at DESC, id DESC))[1], ''),
  (array_agg(created_at ORDER BY created_at DESC, id DESC))[1]
FROM api_usage_export_manifest_digests
WHERE batch_id = $1`
	if err := r.db.QueryRowContext(ctx, digestQuery, batch.ID).Scan(
		&view.ManifestDigestCount,
		&view.LatestManifestDigestID,
		&view.LatestManifestDigestHex,
		&latestDigestAt,
	); err != nil {
		return APIUsageExportHandoffView{}, fmt.Errorf("get api usage export manifest digest handoff stats: %w", err)
	}
	if latestDigestAt.Valid {
		view.LatestManifestDigestAt = &latestDigestAt.Time
	}

	if view.LatestManifestDigestID != "" {
		var latestSignatureAt sql.NullTime
		const signatureQuery = `
SELECT count(*),
  coalesce((array_agg(id ORDER BY created_at DESC, id DESC))[1], ''),
  coalesce((array_agg(signer_backend ORDER BY created_at DESC, id DESC))[1], ''),
  coalesce((array_agg(key_id ORDER BY created_at DESC, id DESC))[1], ''),
  (array_agg(created_at ORDER BY created_at DESC, id DESC))[1]
FROM api_usage_export_manifest_signatures
WHERE batch_id = $1
  AND digest_id = $2`
		if err := r.db.QueryRowContext(ctx, signatureQuery, batch.ID, view.LatestManifestDigestID).Scan(
			&view.LatestDigestSignatureCount,
			&view.LatestSignatureID,
			&view.LatestSignatureSigner,
			&view.LatestSignatureKeyID,
			&latestSignatureAt,
		); err != nil {
			return APIUsageExportHandoffView{}, fmt.Errorf("get api usage export manifest signature handoff stats: %w", err)
		}
		if latestSignatureAt.Valid {
			view.LatestSignatureAt = &latestSignatureAt.Time
		}
	}

	applyAPIUsageExportHandoffReadiness(&view)
	return view, nil
}

func (r *Repository) CreateAPIUsageExportArtifact(ctx context.Context, req CreateAPIUsageExportArtifactRequest) (APIUsageExportArtifactView, error) {
	if r.db == nil {
		return APIUsageExportArtifactView{}, fmt.Errorf("database handle is required")
	}
	if err := ValidateCreateAPIUsageExportArtifactRequest(&req); err != nil {
		return APIUsageExportArtifactView{}, err
	}
	id, err := newAPIUsageExportArtifactID()
	if err != nil {
		return APIUsageExportArtifactView{}, err
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return APIUsageExportArtifactView{}, fmt.Errorf("begin api usage export artifact transaction: %w", err)
	}
	defer tx.Rollback()
	const query = `
INSERT INTO api_usage_export_artifacts (
  id,
  batch_id,
  storage_backend,
  object_key,
  content_type,
  byte_count,
  sha256_hex,
  event_count,
  metadata
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
ON CONFLICT (batch_id, object_key) DO UPDATE SET
  metadata = EXCLUDED.metadata
WHERE api_usage_export_artifacts.sha256_hex = EXCLUDED.sha256_hex
RETURNING id, batch_id, created_at, storage_backend, object_key, content_type,
  byte_count, sha256_hex, event_count, metadata`
	var artifact APIUsageExportArtifactView
	if err := tx.QueryRowContext(
		ctx,
		query,
		id,
		req.BatchID,
		req.StorageBackend,
		req.ObjectKey,
		req.ContentType,
		req.ByteCount,
		req.SHA256Hex,
		req.EventCount,
		req.Metadata,
	).Scan(
		&artifact.ID,
		&artifact.BatchID,
		&artifact.CreatedAt,
		&artifact.StorageBackend,
		&artifact.ObjectKey,
		&artifact.ContentType,
		&artifact.ByteCount,
		&artifact.SHA256Hex,
		&artifact.EventCount,
		&artifact.Metadata,
	); err != nil {
		return APIUsageExportArtifactView{}, fmt.Errorf("create api usage export artifact: %w", err)
	}
	detail, err := apiUsageExportArtifactAuditDetail(artifact)
	if err != nil {
		return APIUsageExportArtifactView{}, err
	}
	if err := audit.InsertTx(ctx, tx, audit.Log{
		Category:   "admin",
		Action:     "api_usage_export.artifact_create",
		TargetType: "api_usage_export_artifact",
		TargetID:   artifact.ID,
		Result:     "created",
		Detail:     detail,
	}); err != nil {
		return APIUsageExportArtifactView{}, fmt.Errorf("record api usage export artifact audit: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return APIUsageExportArtifactView{}, fmt.Errorf("commit api usage export artifact transaction: %w", err)
	}
	return artifact, nil
}

func apiUsageExportArtifactAuditDetail(artifact APIUsageExportArtifactView) (json.RawMessage, error) {
	detail, err := json.Marshal(map[string]any{
		"artifact_id":     artifact.ID,
		"batch_id":        artifact.BatchID,
		"storage_backend": artifact.StorageBackend,
		"object_key":      artifact.ObjectKey,
		"content_type":    artifact.ContentType,
		"byte_count":      artifact.ByteCount,
		"sha256_hex":      artifact.SHA256Hex,
		"event_count":     artifact.EventCount,
	})
	if err != nil {
		return nil, fmt.Errorf("marshal api usage export artifact audit detail: %w", err)
	}
	return detail, nil
}

func (r *Repository) ListAPIUsageExportArtifacts(ctx context.Context, batchID string, limit int) ([]APIUsageExportArtifactView, error) {
	return r.listAPIUsageExportArtifacts(ctx, batchID, limit, false)
}

func (r *Repository) ListAllAPIUsageExportArtifacts(ctx context.Context, batchID string) ([]APIUsageExportArtifactView, error) {
	return r.listAllAPIUsageExportArtifacts(ctx, batchID)
}

func (r *Repository) listAllAPIUsageExportArtifacts(ctx context.Context, batchID string) ([]APIUsageExportArtifactView, error) {
	return r.listAPIUsageExportArtifacts(ctx, batchID, 0, true)
}

func (r *Repository) listAPIUsageExportArtifacts(ctx context.Context, batchID string, limit int, unbounded bool) ([]APIUsageExportArtifactView, error) {
	if r.db == nil {
		return nil, fmt.Errorf("database handle is required")
	}
	batchID = strings.TrimSpace(batchID)
	if batchID == "" {
		return nil, fmt.Errorf("batch_id is required")
	}
	query := apiUsageExportArtifactsQuery(unbounded)
	args := []any{batchID}
	if !unbounded {
		limit = normalizeLimit(limit)
		args = append(args, limit)
	}
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list api usage export artifacts: %w", err)
	}
	defer rows.Close()

	var artifacts []APIUsageExportArtifactView
	for rows.Next() {
		var artifact APIUsageExportArtifactView
		if err := rows.Scan(
			&artifact.ID,
			&artifact.BatchID,
			&artifact.CreatedAt,
			&artifact.StorageBackend,
			&artifact.ObjectKey,
			&artifact.ContentType,
			&artifact.ByteCount,
			&artifact.SHA256Hex,
			&artifact.EventCount,
			&artifact.Metadata,
		); err != nil {
			return nil, fmt.Errorf("scan api usage export artifact: %w", err)
		}
		artifacts = append(artifacts, artifact)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate api usage export artifacts: %w", err)
	}
	return artifacts, nil
}

func apiUsageExportArtifactsQuery(unbounded bool) string {
	query := `
SELECT id, batch_id, created_at, storage_backend, object_key, content_type,
  byte_count, sha256_hex, event_count, metadata
FROM api_usage_export_artifacts
WHERE batch_id = $1
ORDER BY created_at DESC, id DESC
`
	if !unbounded {
		query += `LIMIT $2`
	}
	return query
}

func (r *Repository) GetAPIUsageExportArtifact(ctx context.Context, batchID string, artifactID string) (APIUsageExportArtifactView, error) {
	if r.db == nil {
		return APIUsageExportArtifactView{}, fmt.Errorf("database handle is required")
	}
	batchID = strings.TrimSpace(batchID)
	artifactID = strings.TrimSpace(artifactID)
	if batchID == "" {
		return APIUsageExportArtifactView{}, fmt.Errorf("batch_id is required")
	}
	if artifactID == "" {
		return APIUsageExportArtifactView{}, fmt.Errorf("artifact_id is required")
	}
	const query = `
SELECT id, batch_id, created_at, storage_backend, object_key, content_type,
  byte_count, sha256_hex, event_count, metadata
FROM api_usage_export_artifacts
WHERE batch_id = $1
  AND id = $2`
	var artifact APIUsageExportArtifactView
	if err := r.db.QueryRowContext(ctx, query, batchID, artifactID).Scan(
		&artifact.ID,
		&artifact.BatchID,
		&artifact.CreatedAt,
		&artifact.StorageBackend,
		&artifact.ObjectKey,
		&artifact.ContentType,
		&artifact.ByteCount,
		&artifact.SHA256Hex,
		&artifact.EventCount,
		&artifact.Metadata,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return APIUsageExportArtifactView{}, fmt.Errorf("api usage export artifact not found")
		}
		return APIUsageExportArtifactView{}, fmt.Errorf("get api usage export artifact: %w", err)
	}
	return artifact, nil
}

func (r *Repository) CreateAPIUsageExportManifestDigest(ctx context.Context, batchID string) (APIUsageExportManifestDigestView, error) {
	if r.db == nil {
		return APIUsageExportManifestDigestView{}, fmt.Errorf("database handle is required")
	}
	batch, err := r.GetAPIUsageExportBatch(ctx, batchID)
	if err != nil {
		return APIUsageExportManifestDigestView{}, err
	}
	artifacts, err := r.listAllAPIUsageExportArtifacts(ctx, batch.ID)
	if err != nil {
		return APIUsageExportManifestDigestView{}, err
	}
	manifest := apiUsageExportManifest(batch, artifacts)
	digest, raw, err := apimeter.DigestExportManifest(manifest)
	if err != nil {
		return APIUsageExportManifestDigestView{}, err
	}
	id, err := newAPIUsageExportManifestDigestID()
	if err != nil {
		return APIUsageExportManifestDigestView{}, err
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return APIUsageExportManifestDigestView{}, fmt.Errorf("begin api usage export manifest digest transaction: %w", err)
	}
	defer tx.Rollback()
	const query = `
INSERT INTO api_usage_export_manifest_digests (
  id,
  batch_id,
  schema_version,
  digest_algorithm,
  digest_hex,
  manifest
) VALUES ($1, $2, $3, 'sha256', $4, $5)
ON CONFLICT (batch_id, digest_algorithm, digest_hex) DO UPDATE SET
  manifest = EXCLUDED.manifest
RETURNING id, batch_id, created_at, schema_version, digest_algorithm, digest_hex, manifest`
	var view APIUsageExportManifestDigestView
	if err := tx.QueryRowContext(ctx, query, id, batch.ID, manifest.SchemaVersion, digest, raw).Scan(
		&view.ID,
		&view.BatchID,
		&view.CreatedAt,
		&view.SchemaVersion,
		&view.DigestAlgorithm,
		&view.DigestHex,
		&view.Manifest,
	); err != nil {
		return APIUsageExportManifestDigestView{}, fmt.Errorf("create api usage export manifest digest: %w", err)
	}
	detail, err := apiUsageExportManifestDigestAuditDetail(view, len(manifest.Artifacts))
	if err != nil {
		return APIUsageExportManifestDigestView{}, err
	}
	if err := audit.InsertTx(ctx, tx, audit.Log{
		Category:   "admin",
		Action:     "api_usage_export.manifest_digest_create",
		TargetType: "api_usage_export_manifest_digest",
		TargetID:   view.ID,
		Result:     "created",
		Detail:     detail,
	}); err != nil {
		return APIUsageExportManifestDigestView{}, fmt.Errorf("record api usage export manifest digest audit: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return APIUsageExportManifestDigestView{}, fmt.Errorf("commit api usage export manifest digest transaction: %w", err)
	}
	return view, nil
}

func apiUsageExportManifestDigestAuditDetail(digest APIUsageExportManifestDigestView, artifactCount int) (json.RawMessage, error) {
	detail, err := json.Marshal(map[string]any{
		"digest_id":        digest.ID,
		"batch_id":         digest.BatchID,
		"schema_version":   digest.SchemaVersion,
		"digest_algorithm": digest.DigestAlgorithm,
		"digest_hex":       digest.DigestHex,
		"manifest_bytes":   len(digest.Manifest),
		"artifact_count":   artifactCount,
	})
	if err != nil {
		return nil, fmt.Errorf("marshal api usage export manifest digest audit detail: %w", err)
	}
	return detail, nil
}

func (r *Repository) ListAPIUsageExportManifestDigests(ctx context.Context, batchID string, limit int) ([]APIUsageExportManifestDigestView, error) {
	if r.db == nil {
		return nil, fmt.Errorf("database handle is required")
	}
	batchID = strings.TrimSpace(batchID)
	if batchID == "" {
		return nil, fmt.Errorf("batch_id is required")
	}
	limit = normalizeLimit(limit)
	const query = `
SELECT id, batch_id, created_at, schema_version, digest_algorithm, digest_hex, manifest
FROM api_usage_export_manifest_digests
WHERE batch_id = $1
ORDER BY created_at DESC, id DESC
LIMIT $2`
	rows, err := r.db.QueryContext(ctx, query, batchID, limit)
	if err != nil {
		return nil, fmt.Errorf("list api usage export manifest digests: %w", err)
	}
	defer rows.Close()

	var digests []APIUsageExportManifestDigestView
	for rows.Next() {
		var digest APIUsageExportManifestDigestView
		if err := rows.Scan(
			&digest.ID,
			&digest.BatchID,
			&digest.CreatedAt,
			&digest.SchemaVersion,
			&digest.DigestAlgorithm,
			&digest.DigestHex,
			&digest.Manifest,
		); err != nil {
			return nil, fmt.Errorf("scan api usage export manifest digest: %w", err)
		}
		digests = append(digests, digest)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate api usage export manifest digests: %w", err)
	}
	return digests, nil
}

func (r *Repository) GetAPIUsageExportManifestDigest(ctx context.Context, batchID string, digestID string) (APIUsageExportManifestDigestView, error) {
	if r.db == nil {
		return APIUsageExportManifestDigestView{}, fmt.Errorf("database handle is required")
	}
	batchID = strings.TrimSpace(batchID)
	digestID = strings.TrimSpace(digestID)
	if batchID == "" {
		return APIUsageExportManifestDigestView{}, fmt.Errorf("batch_id is required")
	}
	if digestID == "" {
		return APIUsageExportManifestDigestView{}, fmt.Errorf("digest_id is required")
	}
	const query = `
SELECT id, batch_id, created_at, schema_version, digest_algorithm, digest_hex, manifest
FROM api_usage_export_manifest_digests
WHERE batch_id = $1
  AND id = $2`
	var digest APIUsageExportManifestDigestView
	if err := r.db.QueryRowContext(ctx, query, batchID, digestID).Scan(
		&digest.ID,
		&digest.BatchID,
		&digest.CreatedAt,
		&digest.SchemaVersion,
		&digest.DigestAlgorithm,
		&digest.DigestHex,
		&digest.Manifest,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return APIUsageExportManifestDigestView{}, fmt.Errorf("api usage export manifest digest not found")
		}
		return APIUsageExportManifestDigestView{}, fmt.Errorf("get api usage export manifest digest: %w", err)
	}
	return digest, nil
}

func (r *Repository) VerifyAPIUsageExportManifestDigest(ctx context.Context, batchID string, digestID string) (APIUsageExportManifestDigestVerificationView, error) {
	digest, err := r.GetAPIUsageExportManifestDigest(ctx, batchID, digestID)
	if err != nil {
		return APIUsageExportManifestDigestVerificationView{}, err
	}
	return apiUsageExportManifestDigestVerification(digest)
}

func apiUsageExportManifestDigestVerification(digest APIUsageExportManifestDigestView) (APIUsageExportManifestDigestVerificationView, error) {
	actual, canonical, err := apimeter.DigestExportManifestJSON(digest.Manifest)
	if err != nil {
		return APIUsageExportManifestDigestVerificationView{}, err
	}
	return APIUsageExportManifestDigestVerificationView{
		BatchID:           digest.BatchID,
		DigestID:          digest.ID,
		SchemaVersion:     digest.SchemaVersion,
		DigestAlgorithm:   digest.DigestAlgorithm,
		ExpectedDigestHex: digest.DigestHex,
		ActualDigestHex:   actual,
		Valid:             digest.DigestAlgorithm == "sha256" && digest.DigestHex == actual,
		CanonicalManifest: canonical,
	}, nil
}

func applyAPIUsageExportHandoffReadiness(view *APIUsageExportHandoffView) {
	view.EventsCovered = view.ArtifactEventCount >= view.EventCount
	var missing []string
	if !view.BatchCompleted {
		missing = append(missing, "batch_completed")
	}
	if view.ArtifactCount == 0 {
		missing = append(missing, "export_artifact")
	}
	if !view.EventsCovered {
		missing = append(missing, "event_coverage")
	}
	if view.ManifestDigestCount == 0 || view.LatestManifestDigestID == "" {
		missing = append(missing, "manifest_digest")
	}
	if view.LatestDigestSignatureCount == 0 {
		missing = append(missing, "manifest_signature")
	}
	view.MissingRequirements = missing
	view.Ready = len(missing) == 0
	view.ReadinessGrade = "billing_blocked"
	if !view.Ready {
		view.BillingBlockingReasons = []string{"handoff_not_ready"}
		return
	}
	if apiUsageExportManifestSignerNeedsProductionBackend(view.LatestSignatureSigner) {
		view.ReadinessGrade = "operational"
		view.BillingBlockingReasons = []string{"production_manifest_signer_required"}
		return
	}
	view.ReadinessGrade = "billing_candidate"
	view.BillingReady = true
}

func apiUsageExportManifestSignerNeedsProductionBackend(backend string) bool {
	switch strings.ToLower(strings.TrimSpace(backend)) {
	case "", "local-hmac", "local-ed25519":
		return true
	default:
		return false
	}
}

func (r *Repository) CreateAPIUsageExportManifestSignature(ctx context.Context, req CreateAPIUsageExportManifestSignatureRequest) (APIUsageExportManifestSignatureView, error) {
	if r.db == nil {
		return APIUsageExportManifestSignatureView{}, fmt.Errorf("database handle is required")
	}
	if err := ValidateCreateAPIUsageExportManifestSignatureRequest(&req); err != nil {
		return APIUsageExportManifestSignatureView{}, err
	}
	digest, err := r.GetAPIUsageExportManifestDigest(ctx, req.BatchID, req.DigestID)
	if err != nil {
		return APIUsageExportManifestSignatureView{}, err
	}
	if digest.DigestHex != req.Signature.SignedDigestHex {
		return APIUsageExportManifestSignatureView{}, fmt.Errorf("signed_digest_hex must match manifest digest")
	}
	id, err := newAPIUsageExportManifestSignatureID()
	if err != nil {
		return APIUsageExportManifestSignatureView{}, err
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return APIUsageExportManifestSignatureView{}, fmt.Errorf("begin api usage export manifest signature transaction: %w", err)
	}
	defer tx.Rollback()
	const query = `
INSERT INTO api_usage_export_manifest_signatures (
  id,
  digest_id,
  batch_id,
  signer_backend,
  key_id,
  signature_algorithm,
  signed_digest_hex,
  signature_hex,
  metadata
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
ON CONFLICT (digest_id, signature_algorithm, key_id, signature_hex) DO UPDATE SET
  metadata = EXCLUDED.metadata
RETURNING id, digest_id, batch_id, created_at, signer_backend, key_id,
  signature_algorithm, signed_digest_hex, signature_hex, metadata`
	var view APIUsageExportManifestSignatureView
	if err := tx.QueryRowContext(
		ctx,
		query,
		id,
		req.DigestID,
		req.BatchID,
		req.SignerBackend,
		req.Signature.KeyID,
		req.Signature.Algorithm,
		req.Signature.SignedDigestHex,
		req.Signature.SignatureHex,
		req.Metadata,
	).Scan(
		&view.ID,
		&view.DigestID,
		&view.BatchID,
		&view.CreatedAt,
		&view.SignerBackend,
		&view.KeyID,
		&view.SignatureAlgorithm,
		&view.SignedDigestHex,
		&view.SignatureHex,
		&view.Metadata,
	); err != nil {
		return APIUsageExportManifestSignatureView{}, fmt.Errorf("create api usage export manifest signature: %w", err)
	}
	detail, err := apiUsageExportManifestSignatureAuditDetail(view)
	if err != nil {
		return APIUsageExportManifestSignatureView{}, err
	}
	if err := audit.InsertTx(ctx, tx, audit.Log{
		Category:   "admin",
		Action:     "api_usage_export.manifest_signature_create",
		TargetType: "api_usage_export_manifest_signature",
		TargetID:   view.ID,
		Result:     "created",
		Detail:     detail,
	}); err != nil {
		return APIUsageExportManifestSignatureView{}, fmt.Errorf("record api usage export manifest signature audit: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return APIUsageExportManifestSignatureView{}, fmt.Errorf("commit api usage export manifest signature transaction: %w", err)
	}
	return view, nil
}

func apiUsageExportManifestSignatureAuditDetail(signature APIUsageExportManifestSignatureView) (json.RawMessage, error) {
	detail, err := json.Marshal(map[string]any{
		"signature_id":        signature.ID,
		"digest_id":           signature.DigestID,
		"batch_id":            signature.BatchID,
		"signer_backend":      signature.SignerBackend,
		"key_id":              signature.KeyID,
		"signature_algorithm": signature.SignatureAlgorithm,
		"signed_digest_hex":   signature.SignedDigestHex,
		"signature_hex_len":   len(signature.SignatureHex),
	})
	if err != nil {
		return nil, fmt.Errorf("marshal api usage export manifest signature audit detail: %w", err)
	}
	return detail, nil
}

func (r *Repository) ListAPIUsageExportManifestSignatures(ctx context.Context, batchID string, digestID string, limit int) ([]APIUsageExportManifestSignatureView, error) {
	if r.db == nil {
		return nil, fmt.Errorf("database handle is required")
	}
	batchID = strings.TrimSpace(batchID)
	digestID = strings.TrimSpace(digestID)
	if batchID == "" {
		return nil, fmt.Errorf("batch_id is required")
	}
	if digestID == "" {
		return nil, fmt.Errorf("digest_id is required")
	}
	limit = normalizeLimit(limit)
	const query = `
SELECT id, digest_id, batch_id, created_at, signer_backend, key_id,
  signature_algorithm, signed_digest_hex, signature_hex, metadata
FROM api_usage_export_manifest_signatures
WHERE batch_id = $1
  AND digest_id = $2
ORDER BY created_at DESC, id DESC
LIMIT $3`
	rows, err := r.db.QueryContext(ctx, query, batchID, digestID, limit)
	if err != nil {
		return nil, fmt.Errorf("list api usage export manifest signatures: %w", err)
	}
	defer rows.Close()

	var signatures []APIUsageExportManifestSignatureView
	for rows.Next() {
		var signature APIUsageExportManifestSignatureView
		if err := scanAPIUsageExportManifestSignature(rows, &signature); err != nil {
			return nil, fmt.Errorf("scan api usage export manifest signature: %w", err)
		}
		signatures = append(signatures, signature)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate api usage export manifest signatures: %w", err)
	}
	return signatures, nil
}

func (r *Repository) GetAPIUsageExportManifestSignature(ctx context.Context, batchID string, digestID string, signatureID string) (APIUsageExportManifestSignatureView, error) {
	if r.db == nil {
		return APIUsageExportManifestSignatureView{}, fmt.Errorf("database handle is required")
	}
	batchID = strings.TrimSpace(batchID)
	digestID = strings.TrimSpace(digestID)
	signatureID = strings.TrimSpace(signatureID)
	if batchID == "" {
		return APIUsageExportManifestSignatureView{}, fmt.Errorf("batch_id is required")
	}
	if digestID == "" {
		return APIUsageExportManifestSignatureView{}, fmt.Errorf("digest_id is required")
	}
	if signatureID == "" {
		return APIUsageExportManifestSignatureView{}, fmt.Errorf("signature_id is required")
	}
	const query = `
SELECT id, digest_id, batch_id, created_at, signer_backend, key_id,
  signature_algorithm, signed_digest_hex, signature_hex, metadata
FROM api_usage_export_manifest_signatures
WHERE batch_id = $1
  AND digest_id = $2
  AND id = $3`
	var signature APIUsageExportManifestSignatureView
	if err := scanAPIUsageExportManifestSignature(r.db.QueryRowContext(ctx, query, batchID, digestID, signatureID), &signature); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return APIUsageExportManifestSignatureView{}, fmt.Errorf("api usage export manifest signature not found")
		}
		return APIUsageExportManifestSignatureView{}, fmt.Errorf("get api usage export manifest signature: %w", err)
	}
	return signature, nil
}

type apiUsageExportManifestSignatureScanner interface {
	Scan(dest ...any) error
}

func scanAPIUsageExportManifestSignature(scanner apiUsageExportManifestSignatureScanner, signature *APIUsageExportManifestSignatureView) error {
	return scanner.Scan(
		&signature.ID,
		&signature.DigestID,
		&signature.BatchID,
		&signature.CreatedAt,
		&signature.SignerBackend,
		&signature.KeyID,
		&signature.SignatureAlgorithm,
		&signature.SignedDigestHex,
		&signature.SignatureHex,
		&signature.Metadata,
	)
}

func ValidateCreateAPIUsageExportManifestSignatureRequest(req *CreateAPIUsageExportManifestSignatureRequest) error {
	req.BatchID = strings.TrimSpace(req.BatchID)
	req.DigestID = strings.TrimSpace(req.DigestID)
	req.SignerBackend = strings.TrimSpace(req.SignerBackend)
	req.Signature.Algorithm = strings.TrimSpace(req.Signature.Algorithm)
	req.Signature.KeyID = strings.TrimSpace(req.Signature.KeyID)
	req.Signature.SignedDigestHex = strings.ToLower(strings.TrimSpace(req.Signature.SignedDigestHex))
	req.Signature.SignatureHex = strings.ToLower(strings.TrimSpace(req.Signature.SignatureHex))
	if req.BatchID == "" {
		return fmt.Errorf("batch_id is required")
	}
	if req.DigestID == "" {
		return fmt.Errorf("digest_id is required")
	}
	if req.SignerBackend == "" {
		return fmt.Errorf("signer_backend is required")
	}
	switch req.Signature.Algorithm {
	case apimeter.ExportManifestSignatureAlgorithmHMACSHA256, apimeter.ExportManifestSignatureAlgorithmEd25519:
	default:
		return fmt.Errorf("signature_algorithm must be hmac-sha256 or ed25519")
	}
	if !apiUsageExportManifestSignatureBackendMatchesAlgorithm(req.SignerBackend, req.Signature.Algorithm) {
		return fmt.Errorf("signer_backend %q is not compatible with signature_algorithm %q", req.SignerBackend, req.Signature.Algorithm)
	}
	if req.Signature.KeyID == "" {
		return fmt.Errorf("key_id is required")
	}
	if !isLowerHexSHA256(req.Signature.SignedDigestHex) {
		return fmt.Errorf("signed_digest_hex must be 64 lowercase hex characters")
	}
	if req.Signature.Algorithm == apimeter.ExportManifestSignatureAlgorithmHMACSHA256 && !isLowerHexBytes(req.Signature.SignatureHex, 32) {
		return fmt.Errorf("signature_hex must be 64 lowercase hex characters")
	}
	if req.Signature.Algorithm == apimeter.ExportManifestSignatureAlgorithmEd25519 && !isLowerHexBytes(req.Signature.SignatureHex, 64) {
		return fmt.Errorf("signature_hex must be 128 lowercase hex characters")
	}
	if len(req.Metadata) == 0 {
		req.Metadata = json.RawMessage(`{}`)
	}
	var metadata map[string]any
	if err := json.Unmarshal(req.Metadata, &metadata); err != nil {
		return fmt.Errorf("metadata must be a JSON object: %w", err)
	}
	return nil
}

func apiUsageExportManifestSignatureBackendMatchesAlgorithm(backend string, algorithm string) bool {
	switch strings.ToLower(strings.TrimSpace(backend)) {
	case "local-hmac":
		return algorithm == apimeter.ExportManifestSignatureAlgorithmHMACSHA256
	case "local-ed25519":
		return algorithm == apimeter.ExportManifestSignatureAlgorithmEd25519
	default:
		return true
	}
}

func apiUsageExportManifest(batch APIUsageExportBatchView, artifacts []APIUsageExportArtifactView) apimeter.ExportManifest {
	ordered := append([]APIUsageExportArtifactView(nil), artifacts...)
	sort.Slice(ordered, func(i, j int) bool {
		return ordered[i].ID < ordered[j].ID
	})
	manifest := apimeter.ExportManifest{
		SchemaVersion: apimeter.ExportManifestSchemaV1,
		Batch: apimeter.ExportManifestBatch{
			ID:             batch.ID,
			TenantID:       batch.TenantID,
			PrincipalID:    batch.PrincipalID,
			WindowStart:    apimeter.FormatManifestTime(batch.WindowStart),
			WindowEnd:      apimeter.FormatManifestTime(batch.WindowEnd),
			EventCount:     batch.EventCount,
			RequestCount:   batch.RequestCount,
			RequestBytes:   batch.RequestBytes,
			ResponseBytes:  batch.ResponseBytes,
			LatencyMSTotal: batch.LatencyMSTotal,
			LatencyMSMax:   batch.LatencyMSMax,
		},
		Artifacts: make([]apimeter.ExportManifestArtifact, 0, len(ordered)),
	}
	for _, artifact := range ordered {
		manifest.Artifacts = append(manifest.Artifacts, apimeter.ExportManifestArtifact{
			ID:             artifact.ID,
			StorageBackend: artifact.StorageBackend,
			ObjectKey:      artifact.ObjectKey,
			ContentType:    artifact.ContentType,
			ByteCount:      artifact.ByteCount,
			SHA256Hex:      artifact.SHA256Hex,
			EventCount:     artifact.EventCount,
		})
	}
	return manifest
}

func ValidateCreateAPIUsageExportArtifactRequest(req *CreateAPIUsageExportArtifactRequest) error {
	req.BatchID = strings.TrimSpace(req.BatchID)
	req.StorageBackend = strings.TrimSpace(req.StorageBackend)
	if strings.ContainsAny(req.ObjectKey, "\r\n") {
		return fmt.Errorf("object_key cannot contain line breaks")
	}
	req.ObjectKey = strings.TrimSpace(req.ObjectKey)
	req.ContentType = strings.TrimSpace(req.ContentType)
	req.SHA256Hex = strings.ToLower(strings.TrimSpace(req.SHA256Hex))
	if req.BatchID == "" {
		return fmt.Errorf("batch_id is required")
	}
	if req.StorageBackend == "" {
		req.StorageBackend = "external"
	}
	if req.ObjectKey == "" {
		return fmt.Errorf("object_key is required")
	}
	if req.ContentType == "" {
		req.ContentType = "application/x-ndjson"
	}
	if req.ContentType != "application/x-ndjson" {
		return fmt.Errorf("content_type must be application/x-ndjson")
	}
	if req.ByteCount < 0 {
		return fmt.Errorf("byte_count must be nonnegative")
	}
	if req.EventCount < 0 {
		return fmt.Errorf("event_count must be nonnegative")
	}
	if !isLowerHexSHA256(req.SHA256Hex) {
		return fmt.Errorf("sha256_hex must be 64 lowercase hex characters")
	}
	if len(req.Metadata) == 0 {
		req.Metadata = json.RawMessage(`{}`)
	}
	var metadata map[string]any
	if err := json.Unmarshal(req.Metadata, &metadata); err != nil {
		return fmt.Errorf("metadata must be a JSON object: %w", err)
	}
	return nil
}

func isLowerHexSHA256(value string) bool {
	return isLowerHexBytes(value, 32)
}

func isLowerHexBytes(value string, bytes int) bool {
	if len(value) != bytes*2 {
		return false
	}
	for _, r := range value {
		if (r < '0' || r > '9') && (r < 'a' || r > 'f') {
			return false
		}
	}
	return true
}

func newAPIUsageExportBatchID() (string, error) {
	var random [16]byte
	if _, err := rand.Read(random[:]); err != nil {
		return "", fmt.Errorf("generate api usage export batch id: %w", err)
	}
	return "api-usage-export-" + hex.EncodeToString(random[:]), nil
}

func newAPIUsageLedgerRetentionRunID() (string, error) {
	var random [16]byte
	if _, err := rand.Read(random[:]); err != nil {
		return "", fmt.Errorf("generate api usage ledger retention run id: %w", err)
	}
	return "api-usage-retention-" + hex.EncodeToString(random[:]), nil
}

func newAPIUsageExportArtifactID() (string, error) {
	var random [16]byte
	if _, err := rand.Read(random[:]); err != nil {
		return "", fmt.Errorf("generate api usage export artifact id: %w", err)
	}
	return "api-usage-artifact-" + hex.EncodeToString(random[:]), nil
}

func newAPIUsageExportManifestDigestID() (string, error) {
	var random [16]byte
	if _, err := rand.Read(random[:]); err != nil {
		return "", fmt.Errorf("generate api usage export manifest digest id: %w", err)
	}
	return "api-usage-manifest-" + hex.EncodeToString(random[:]), nil
}

func newAPIUsageExportManifestSignatureID() (string, error) {
	var random [16]byte
	if _, err := rand.Read(random[:]); err != nil {
		return "", fmt.Errorf("generate api usage export manifest signature id: %w", err)
	}
	return "api-usage-signature-" + hex.EncodeToString(random[:]), nil
}

func optionalTimeString(value time.Time) string {
	if value.IsZero() {
		return ""
	}
	return value.UTC().Format(time.RFC3339Nano)
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

func nullableTime(value time.Time) any {
	if value.IsZero() {
		return nil
	}
	return value.UTC()
}

func nullableTimePtr(value *time.Time) any {
	if value == nil {
		return nil
	}
	return value.UTC()
}

func applyExportBatchNullableTimes(batch *APIUsageExportBatchView, completedAt sql.NullTime, windowStart sql.NullTime, windowEnd sql.NullTime, firstEventAt sql.NullTime, lastEventAt sql.NullTime) {
	if completedAt.Valid {
		batch.CompletedAt = &completedAt.Time
	}
	if windowStart.Valid {
		batch.WindowStart = &windowStart.Time
	}
	if windowEnd.Valid {
		batch.WindowEnd = &windowEnd.Time
	}
	if firstEventAt.Valid {
		batch.FirstEventAt = &firstEventAt.Time
	}
	if lastEventAt.Valid {
		batch.LastEventAt = &lastEventAt.Time
	}
}

func quotaUsageRatio(used int64, limit int64) float64 {
	if limit <= 0 {
		return 0
	}
	if used <= 0 {
		return 0
	}
	return float64(used) / float64(limit)
}

func quotaRemaining(used int64, limit int64) int64 {
	if limit <= 0 {
		return 0
	}
	remaining := limit - used
	if remaining < 0 {
		return 0
	}
	return remaining
}
