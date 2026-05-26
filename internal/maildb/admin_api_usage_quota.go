package maildb

import (
	"context"
	"fmt"
	"strings"
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
