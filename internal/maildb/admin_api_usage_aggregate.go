package maildb

import (
	"context"
	"fmt"
	"strings"
)

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
