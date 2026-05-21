package maildb

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
)

type QuotaAlertScope string

const (
	QuotaAlertScopeUser    QuotaAlertScope = "user"
	QuotaAlertScopeDomain  QuotaAlertScope = "domain"
	QuotaAlertScopeCompany QuotaAlertScope = "company"
)

type QuotaAlertType string

const (
	QuotaAlertTypeWarning   QuotaAlertType = "warning"
	QuotaAlertTypeCritical  QuotaAlertType = "critical"
	QuotaAlertTypeExhausted QuotaAlertType = "exhausted"
)

type QuotaAlertThresholdView struct {
	ID            string          `json:"id"`
	Scope         QuotaAlertScope `json:"scope"`
	ScopeID       string          `json:"scope_id,omitempty"`
	CompanyID     string          `json:"company_id"`
	WarningRatio  float64         `json:"warning_ratio"`
	CriticalRatio float64         `json:"critical_ratio"`
	CreatedAt     time.Time       `json:"created_at"`
	UpdatedAt     time.Time       `json:"updated_at"`
}

type QuotaAlertView struct {
	ID         string          `json:"id"`
	CompanyID  string          `json:"company_id"`
	DomainID   string          `json:"domain_id,omitempty"`
	UserID     string          `json:"user_id,omitempty"`
	Scope      QuotaAlertScope `json:"scope"`
	AlertType  QuotaAlertType  `json:"alert_type"`
	QuotaUsed  int64           `json:"quota_used"`
	QuotaLimit int64           `json:"quota_limit"`
	UsageRatio float64         `json:"usage_ratio"`
	EventID    string          `json:"event_id"`
	CreatedAt  time.Time       `json:"created_at"`
}

type QuotaAlertListRequest struct {
	Limit     int
	CompanyID string
	DomainID  string
	UserID    string
	Scope     string
	AlertType string
	Since     time.Time
	Until     time.Time
}

type CreateQuotaAlertThresholdRequest struct {
	Scope         QuotaAlertScope `json:"scope"`
	ScopeID       string          `json:"scope_id,omitempty"`
	CompanyID     string          `json:"company_id"`
	WarningRatio  float64         `json:"warning_ratio"`
	CriticalRatio float64         `json:"critical_ratio"`
}

type UpdateQuotaAlertThresholdRequest struct {
	ID            string  `json:"id"`
	WarningRatio  float64 `json:"warning_ratio"`
	CriticalRatio float64 `json:"critical_ratio"`
}

type QuotaAlertThresholdListRequest struct {
	Limit     int
	CompanyID string
	Scope     string
}

func ValidateCreateQuotaAlertThresholdRequest(req CreateQuotaAlertThresholdRequest) error {
	req.CompanyID = strings.TrimSpace(req.CompanyID)
	if req.CompanyID == "" {
		return fmt.Errorf("company_id is required")
	}
	if req.Scope != QuotaAlertScopeUser && req.Scope != QuotaAlertScopeDomain && req.Scope != QuotaAlertScopeCompany {
		return fmt.Errorf("scope must be user, domain, or company")
	}
	if req.Scope == QuotaAlertScopeUser || req.Scope == QuotaAlertScopeDomain {
		req.ScopeID = strings.TrimSpace(req.ScopeID)
		if req.ScopeID == "" {
			return fmt.Errorf("scope_id is required for user and domain scopes")
		}
	}
	if req.WarningRatio <= 0 || req.WarningRatio > 1 {
		return fmt.Errorf("warning_ratio must be between 0 and 1")
	}
	if req.CriticalRatio <= 0 || req.CriticalRatio > 1 {
		return fmt.Errorf("critical_ratio must be between 0 and 1")
	}
	if req.CriticalRatio < req.WarningRatio {
		return fmt.Errorf("critical_ratio must be greater than or equal to warning_ratio")
	}
	return nil
}

func ValidateUpdateQuotaAlertThresholdRequest(req UpdateQuotaAlertThresholdRequest) error {
	req.ID = strings.TrimSpace(req.ID)
	if req.ID == "" {
		return fmt.Errorf("id is required")
	}
	if req.WarningRatio <= 0 || req.WarningRatio > 1 {
		return fmt.Errorf("warning_ratio must be between 0 and 1")
	}
	if req.CriticalRatio <= 0 || req.CriticalRatio > 1 {
		return fmt.Errorf("critical_ratio must be between 0 and 1")
	}
	if req.CriticalRatio < req.WarningRatio {
		return fmt.Errorf("critical_ratio must be greater than or equal to warning_ratio")
	}
	return nil
}

func (r *Repository) CreateQuotaAlertThreshold(ctx context.Context, req CreateQuotaAlertThresholdRequest) (QuotaAlertThresholdView, error) {
	if r.db == nil {
		return QuotaAlertThresholdView{}, fmt.Errorf("database handle is required")
	}
	if err := ValidateCreateQuotaAlertThresholdRequest(req); err != nil {
		return QuotaAlertThresholdView{}, err
	}

	var scopeID *string
	if req.ScopeID != "" {
		scopeID = &req.ScopeID
	}

	const query = `
INSERT INTO quota_alert_thresholds (scope, scope_id, company_id, warning_ratio, critical_ratio)
VALUES ($1, $2, $3, $4, $5)
RETURNING id::text, scope, COALESCE(scope_id::text, ''), company_id::text, warning_ratio, critical_ratio, created_at, updated_at`

	var view QuotaAlertThresholdView
	err := r.db.QueryRowContext(ctx, query,
		req.Scope,
		scopeID,
		req.CompanyID,
		req.WarningRatio,
		req.CriticalRatio,
	).Scan(
		&view.ID,
		&view.Scope,
		&view.ScopeID,
		&view.CompanyID,
		&view.WarningRatio,
		&view.CriticalRatio,
		&view.CreatedAt,
		&view.UpdatedAt,
	)
	if err != nil {
		return QuotaAlertThresholdView{}, fmt.Errorf("create quota alert threshold: %w", err)
	}
	return view, nil
}

func (r *Repository) UpdateQuotaAlertThreshold(ctx context.Context, req UpdateQuotaAlertThresholdRequest) (QuotaAlertThresholdView, error) {
	if r.db == nil {
		return QuotaAlertThresholdView{}, fmt.Errorf("database handle is required")
	}
	if err := ValidateUpdateQuotaAlertThresholdRequest(req); err != nil {
		return QuotaAlertThresholdView{}, err
	}

	const query = `
UPDATE quota_alert_thresholds
SET warning_ratio = $2,
    critical_ratio = $3,
    updated_at = now()
WHERE id = $1
RETURNING id::text, scope, COALESCE(scope_id::text, ''), company_id::text, warning_ratio, critical_ratio, created_at, updated_at`

	var view QuotaAlertThresholdView
	err := r.db.QueryRowContext(ctx, query,
		req.ID,
		req.WarningRatio,
		req.CriticalRatio,
	).Scan(
		&view.ID,
		&view.Scope,
		&view.ScopeID,
		&view.CompanyID,
		&view.WarningRatio,
		&view.CriticalRatio,
		&view.CreatedAt,
		&view.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return QuotaAlertThresholdView{}, fmt.Errorf("quota alert threshold not found")
		}
		return QuotaAlertThresholdView{}, fmt.Errorf("update quota alert threshold: %w", err)
	}
	return view, nil
}

func (r *Repository) DeleteQuotaAlertThreshold(ctx context.Context, id string) error {
	if r.db == nil {
		return fmt.Errorf("database handle is required")
	}
	id = strings.TrimSpace(id)
	if id == "" {
		return fmt.Errorf("id is required")
	}

	const query = `DELETE FROM quota_alert_thresholds WHERE id = $1`
	result, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("delete quota alert threshold: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("check rows affected: %w", err)
	}
	if rows == 0 {
		return fmt.Errorf("quota alert threshold not found")
	}
	return nil
}

func (r *Repository) GetQuotaAlertThreshold(ctx context.Context, id string) (QuotaAlertThresholdView, error) {
	if r.db == nil {
		return QuotaAlertThresholdView{}, fmt.Errorf("database handle is required")
	}
	id = strings.TrimSpace(id)
	if id == "" {
		return QuotaAlertThresholdView{}, fmt.Errorf("id is required")
	}

	const query = `
SELECT id::text, scope, COALESCE(scope_id::text, ''), company_id::text, warning_ratio, critical_ratio, created_at, updated_at
FROM quota_alert_thresholds
WHERE id = $1`

	var view QuotaAlertThresholdView
	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&view.ID,
		&view.Scope,
		&view.ScopeID,
		&view.CompanyID,
		&view.WarningRatio,
		&view.CriticalRatio,
		&view.CreatedAt,
		&view.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return QuotaAlertThresholdView{}, fmt.Errorf("quota alert threshold not found")
		}
		return QuotaAlertThresholdView{}, fmt.Errorf("get quota alert threshold: %w", err)
	}
	return view, nil
}

func (r *Repository) ListQuotaAlertThresholds(ctx context.Context, req QuotaAlertThresholdListRequest) ([]QuotaAlertThresholdView, error) {
	if r.db == nil {
		return nil, fmt.Errorf("database handle is required")
	}
	req = normalizeQuotaAlertThresholdListRequest(req)

	query, args := buildQuotaAlertThresholdListSQL(req)
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list quota alert thresholds: %w", err)
	}
	defer rows.Close()

	var thresholds []QuotaAlertThresholdView
	for rows.Next() {
		var view QuotaAlertThresholdView
		if err := rows.Scan(
			&view.ID,
			&view.Scope,
			&view.ScopeID,
			&view.CompanyID,
			&view.WarningRatio,
			&view.CriticalRatio,
			&view.CreatedAt,
			&view.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan quota alert threshold: %w", err)
		}
		thresholds = append(thresholds, view)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate quota alert thresholds: %w", err)
	}
	return thresholds, nil
}

func buildQuotaAlertThresholdListSQL(req QuotaAlertThresholdListRequest) (string, []any) {
	var conditions []string
	var args []any

	if req.CompanyID != "" {
		args = append(args, req.CompanyID)
		conditions = append(conditions, fmt.Sprintf("company_id = $%d::uuid", len(args)))
	}
	if req.Scope != "" {
		args = append(args, req.Scope)
		conditions = append(conditions, fmt.Sprintf("scope = $%d", len(args)))
	}

	query := `
SELECT id::text, scope, COALESCE(scope_id::text, ''), company_id::text, warning_ratio, critical_ratio, created_at, updated_at
FROM quota_alert_thresholds`
	if len(conditions) > 0 {
		query += "\nWHERE " + strings.Join(conditions, "\n  AND ")
	}
	query += "\nORDER BY created_at DESC, id DESC"

	args = append(args, req.Limit)
	query += fmt.Sprintf("\nLIMIT $%d", len(args))

	return query, args
}

func (r *Repository) ListQuotaAlerts(ctx context.Context, req QuotaAlertListRequest) ([]QuotaAlertView, error) {
	if r.db == nil {
		return nil, fmt.Errorf("database handle is required")
	}
	req = normalizeQuotaAlertListRequest(req)

	query, args := buildQuotaAlertListSQL(req)
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list quota alerts: %w", err)
	}
	defer rows.Close()

	var alerts []QuotaAlertView
	for rows.Next() {
		var view QuotaAlertView
		if err := rows.Scan(
			&view.ID,
			&view.CompanyID,
			&view.DomainID,
			&view.UserID,
			&view.Scope,
			&view.AlertType,
			&view.QuotaUsed,
			&view.QuotaLimit,
			&view.UsageRatio,
			&view.EventID,
			&view.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan quota alert: %w", err)
		}
		alerts = append(alerts, view)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate quota alerts: %w", err)
	}
	return alerts, nil
}

func buildQuotaAlertListSQL(req QuotaAlertListRequest) (string, []any) {
	var conditions []string
	var args []any

	if req.CompanyID != "" {
		args = append(args, req.CompanyID)
		conditions = append(conditions, fmt.Sprintf("company_id = $%d::uuid", len(args)))
	}
	if req.DomainID != "" {
		args = append(args, req.DomainID)
		conditions = append(conditions, fmt.Sprintf("domain_id = $%d::uuid", len(args)))
	}
	if req.UserID != "" {
		args = append(args, req.UserID)
		conditions = append(conditions, fmt.Sprintf("user_id = $%d::uuid", len(args)))
	}
	if req.Scope != "" {
		args = append(args, req.Scope)
		conditions = append(conditions, fmt.Sprintf("scope = $%d", len(args)))
	}
	if req.AlertType != "" {
		args = append(args, req.AlertType)
		conditions = append(conditions, fmt.Sprintf("alert_type = $%d", len(args)))
	}
	if !req.Since.IsZero() {
		args = append(args, req.Since.UTC())
		conditions = append(conditions, fmt.Sprintf("created_at >= $%d", len(args)))
	}
	if !req.Until.IsZero() {
		args = append(args, req.Until.UTC())
		conditions = append(conditions, fmt.Sprintf("created_at <= $%d", len(args)))
	}

	query := `
SELECT id::text, company_id::text, COALESCE(domain_id::text, ''), COALESCE(user_id::text, ''), scope, alert_type, quota_used, quota_limit, usage_ratio, event_id::text, created_at
FROM quota_alerts`
	if len(conditions) > 0 {
		query += "\nWHERE " + strings.Join(conditions, "\n  AND ")
	}
	query += "\nORDER BY created_at DESC, id DESC"

	args = append(args, req.Limit)
	query += fmt.Sprintf("\nLIMIT $%d", len(args))

	return query, args
}

func (r *Repository) GetQuotaAlert(ctx context.Context, id string) (QuotaAlertView, error) {
	if r.db == nil {
		return QuotaAlertView{}, fmt.Errorf("database handle is required")
	}
	id = strings.TrimSpace(id)
	if id == "" {
		return QuotaAlertView{}, fmt.Errorf("id is required")
	}

	const query = `
SELECT id::text, company_id::text, COALESCE(domain_id::text, ''), COALESCE(user_id::text, ''), scope, alert_type, quota_used, quota_limit, usage_ratio, event_id::text, created_at
FROM quota_alerts
WHERE id = $1`

	var view QuotaAlertView
	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&view.ID,
		&view.CompanyID,
		&view.DomainID,
		&view.UserID,
		&view.Scope,
		&view.AlertType,
		&view.QuotaUsed,
		&view.QuotaLimit,
		&view.UsageRatio,
		&view.EventID,
		&view.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return QuotaAlertView{}, fmt.Errorf("quota alert not found")
		}
		return QuotaAlertView{}, fmt.Errorf("get quota alert: %w", err)
	}
	return view, nil
}

type QuotaAlertRecord struct {
	CompanyID  string
	DomainID   string
	UserID     string
	Scope      QuotaAlertScope
	AlertType  QuotaAlertType
	QuotaUsed  int64
	QuotaLimit int64
	UsageRatio float64
	EventID    string
}

func (r *Repository) RecordQuotaAlert(ctx context.Context, record QuotaAlertRecord) error {
	if r.db == nil {
		return fmt.Errorf("database handle is required")
	}

	var domainID, userID *string
	if record.DomainID != "" {
		domainID = &record.DomainID
	}
	if record.UserID != "" {
		userID = &record.UserID
	}

	const query = `
INSERT INTO quota_alerts (company_id, domain_id, user_id, scope, alert_type, quota_used, quota_limit, usage_ratio, event_id)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`

	_, err := r.db.ExecContext(ctx, query,
		record.CompanyID,
		domainID,
		userID,
		record.Scope,
		record.AlertType,
		record.QuotaUsed,
		record.QuotaLimit,
		record.UsageRatio,
		record.EventID,
	)
	if err != nil {
		return fmt.Errorf("record quota alert: %w", err)
	}
	return nil
}

func (r *Repository) GetQuotaAlertThresholdsForScope(ctx context.Context, companyID string, scope QuotaAlertScope, scopeID string) ([]QuotaAlertThresholdView, error) {
	if r.db == nil {
		return nil, fmt.Errorf("database handle is required")
	}

	var thresholds []QuotaAlertThresholdView
	query, args := buildQuotaAlertThresholdsForScopeSQL(companyID, scope, scopeID)
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("get quota alert thresholds for scope: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var view QuotaAlertThresholdView
		if err := rows.Scan(
			&view.ID,
			&view.Scope,
			&view.ScopeID,
			&view.CompanyID,
			&view.WarningRatio,
			&view.CriticalRatio,
			&view.CreatedAt,
			&view.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan quota alert threshold: %w", err)
		}
		thresholds = append(thresholds, view)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate quota alert thresholds: %w", err)
	}
	return thresholds, nil
}

func buildQuotaAlertThresholdsForScopeSQL(companyID string, scope QuotaAlertScope, scopeID string) (string, []any) {
	var args []any
	var conditions []string

	args = append(args, companyID)
	conditions = append(conditions, fmt.Sprintf("company_id = $%d::uuid", len(args)))

	args = append(args, scope)
	conditions = append(conditions, fmt.Sprintf("scope = $%d", len(args)))

	if scope == QuotaAlertScopeUser || scope == QuotaAlertScopeDomain {
		args = append(args, scopeID)
		conditions = append(conditions, fmt.Sprintf("scope_id = $%d::uuid", len(args)))
	} else {
		conditions = append(conditions, "scope_id IS NULL")
	}

	query := `
SELECT id::text, scope, COALESCE(scope_id::text, ''), company_id::text, warning_ratio, critical_ratio, created_at, updated_at
FROM quota_alert_thresholds
WHERE ` + strings.Join(conditions, " AND ") + `
ORDER BY created_at DESC, id DESC`

	return query, args
}

func (r *Repository) CheckQuotaAlertSent(ctx context.Context, companyID string, scope QuotaAlertScope, scopeID string, alertType QuotaAlertType, since time.Duration) (bool, error) {
	if r.db == nil {
		return false, fmt.Errorf("database handle is required")
	}

	query, args := buildQuotaAlertSentSQL(companyID, scope, scopeID, alertType, since)
	var exists bool
	err := r.db.QueryRowContext(ctx, query, args...).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("check quota alert sent: %w", err)
	}
	return exists, nil
}

func buildQuotaAlertSentSQL(companyID string, scope QuotaAlertScope, scopeID string, alertType QuotaAlertType, since time.Duration) (string, []any) {
	var scopeCondition string
	var args []any

	args = append(args, companyID)
	args = append(args, string(scope))
	args = append(args, string(alertType))
	args = append(args, time.Now().UTC().Add(-since))

	if scopeID != "" {
		args = append(args, scopeID)
		switch scope {
		case QuotaAlertScopeUser:
			scopeCondition = fmt.Sprintf("AND user_id = $%d::uuid", len(args))
		case QuotaAlertScopeDomain:
			scopeCondition = fmt.Sprintf("AND domain_id = $%d::uuid", len(args))
		default:
			scopeCondition = fmt.Sprintf("AND company_id = $%d::uuid", len(args))
		}
	} else {
		if scope == QuotaAlertScopeUser || scope == QuotaAlertScopeDomain {
			scopeCondition = "AND user_id IS NULL AND domain_id IS NULL"
		} else {
			scopeCondition = "AND user_id IS NULL AND domain_id IS NULL"
		}
	}

	query := fmt.Sprintf(`
SELECT EXISTS(
  SELECT 1 FROM quota_alerts
  WHERE company_id = $1::uuid
    AND scope = $2
    AND alert_type = $3
    AND created_at >= $4
    %s
)`, scopeCondition)

	return query, args
}

func normalizeQuotaAlertThresholdListRequest(req QuotaAlertThresholdListRequest) QuotaAlertThresholdListRequest {
	req.Limit = normalizeLimit(req.Limit)
	if req.Limit <= 0 {
		req.Limit = 50
	}
	if req.Limit > 500 {
		req.Limit = 500
	}
	req.CompanyID = strings.TrimSpace(req.CompanyID)
	req.Scope = strings.TrimSpace(req.Scope)
	return req
}

func normalizeQuotaAlertListRequest(req QuotaAlertListRequest) QuotaAlertListRequest {
	req.Limit = normalizeLimit(req.Limit)
	if req.Limit <= 0 {
		req.Limit = 50
	}
	if req.Limit > 500 {
		req.Limit = 500
	}
	req.CompanyID = strings.TrimSpace(req.CompanyID)
	req.DomainID = strings.TrimSpace(req.DomainID)
	req.UserID = strings.TrimSpace(req.UserID)
	req.Scope = strings.TrimSpace(req.Scope)
	req.AlertType = strings.TrimSpace(req.AlertType)
	return req
}
