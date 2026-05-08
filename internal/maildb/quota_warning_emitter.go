package maildb

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

type QuotaWarningEmitterInterface interface {
	EmitIfNeeded(ctx context.Context, userID string) error
}

var _ QuotaWarningEmitterInterface = (*QuotaWarningEmitter)(nil)

const (
	QuotaWarningEventTopic  = "mail.quota_warning"
	QuotaWarningWindow     = 24 * time.Hour
)

type QuotaWarningEmitter struct {
	db          *sql.DB
	redisClient *redis.Client
	eventStream string
}

func NewQuotaWarningEmitter(db *sql.DB, redisClient *redis.Client, eventStream string) *QuotaWarningEmitter {
	if eventStream == "" {
		eventStream = "mail.event"
	}
	return &QuotaWarningEmitter{
		db:          db,
		redisClient: redisClient,
		eventStream: eventStream,
	}
}

type QuotaWarningPayload struct {
	Event       string  `json:"event"`
	SchemaVersion string `json:"schema_version"`
	CompanyID   string  `json:"company_id"`
	DomainID    string  `json:"domain_id,omitempty"`
	UserID      string  `json:"user_id,omitempty"`
	Scope       string  `json:"scope"`
	AlertType   string  `json:"alert_type"`
	QuotaUsed   int64   `json:"quota_used"`
	QuotaLimit  int64   `json:"quota_limit"`
	UsageRatio  float64 `json:"usage_ratio"`
}

func (e *QuotaWarningEmitter) EmitIfNeeded(ctx context.Context, userID string) error {
	if e.db == nil {
		return fmt.Errorf("database handle is required")
	}

	usage, err := e.getUserQuotaUsage(ctx, userID)
	if err != nil {
		return fmt.Errorf("get user quota usage: %w", err)
	}

	if err := e.evaluateAndEmit(ctx, usage); err != nil {
		return fmt.Errorf("evaluate and emit quota warnings: %w", err)
	}

	return nil
}

func (e *QuotaWarningEmitter) getUserQuotaUsage(ctx context.Context, userID string) (*quotaUsageInfo, error) {
	const query = `
SELECT
  u.id::text,
  u.domain_id::text,
  d.company_id::text,
  COALESCE(u.quota_used, 0),
  COALESCE(u.quota_limit, 0),
  COALESCE(d.quota_used, 0),
  COALESCE(d.quota_limit, 0),
  COALESCE(c.quota_used, 0),
  COALESCE(c.quota_limit, 0)
FROM users u
JOIN domains d ON d.id = u.domain_id
JOIN companies c ON c.id = d.company_id
WHERE u.id = $1`

	var info quotaUsageInfo
	var userUsed, userLimit, domainUsed, domainLimit, companyUsed, companyLimit int64
	var domainID, companyID string

	err := e.db.QueryRowContext(ctx, query, userID).Scan(
		&info.UserID,
		&domainID,
		&companyID,
		&userUsed,
		&userLimit,
		&domainUsed,
		&domainLimit,
		&companyUsed,
		&companyLimit,
	)
	if err != nil {
		return nil, err
	}

	info.DomainID = domainID
	info.CompanyID = companyID
	info.UserUsed = userUsed
	info.UserLimit = userLimit
	info.DomainUsed = domainUsed
	info.DomainLimit = domainLimit
	info.CompanyUsed = companyUsed
	info.CompanyLimit = companyLimit

	return &info, nil
}

type quotaUsageInfo struct {
	UserID       string
	DomainID     string
	CompanyID    string
	UserUsed     int64
	UserLimit    int64
	DomainUsed   int64
	DomainLimit  int64
	CompanyUsed  int64
	CompanyLimit int64
}

func (e *QuotaWarningEmitter) evaluateAndEmit(ctx context.Context, usage *quotaUsageInfo) error {
	scopes := []struct {
		scope       QuotaAlertScope
		scopeID     string
		quotaUsed   int64
		quotaLimit  int64
	}{
		{QuotaAlertScopeUser, usage.UserID, usage.UserUsed, usage.UserLimit},
		{QuotaAlertScopeDomain, usage.DomainID, usage.DomainUsed, usage.DomainLimit},
		{QuotaAlertScopeCompany, "", usage.CompanyUsed, usage.CompanyLimit},
	}

	for _, s := range scopes {
		if s.quotaLimit <= 0 {
			continue
		}

		thresholds, err := e.getThresholdsForScope(ctx, usage.CompanyID, s.scope, s.scopeID)
		if err != nil {
			return err
		}

		for _, threshold := range thresholds {
			ratio := float64(s.quotaUsed) / float64(s.quotaLimit)
			if ratio < threshold.WarningRatio {
				continue
			}

			alertType := QuotaAlertTypeWarning
			if ratio >= threshold.CriticalRatio {
				alertType = QuotaAlertTypeCritical
			}

			sent, err := e.checkAndRecordAlert(ctx, usage.CompanyID, s.scope, s.scopeID, alertType, s.quotaUsed, s.quotaLimit, ratio)
			if err != nil {
				return err
			}
			if !sent {
				continue
			}

			if err := e.emitEvent(ctx, usage, s.scope, alertType, s.quotaUsed, s.quotaLimit, ratio); err != nil {
				return err
			}
		}
	}

	return nil
}

func (e *QuotaWarningEmitter) getThresholdsForScope(ctx context.Context, companyID string, scope QuotaAlertScope, scopeID string) ([]QuotaAlertThresholdView, error) {
	var args []any
	argIdx := 1

	query := `
SELECT id::text, scope, COALESCE(scope_id::text, ''), company_id::text, warning_ratio, critical_ratio, created_at, updated_at
FROM quota_alert_thresholds
WHERE company_id = $1`

	args = append(args, companyID)
	argIdx++

	query += fmt.Sprintf(" AND scope = $%d", argIdx)
	args = append(args, scope)
	argIdx++

	if scope == QuotaAlertScopeUser || scope == QuotaAlertScopeDomain {
		query += fmt.Sprintf(" AND scope_id::text = $%d", argIdx)
		args = append(args, scopeID)
	} else {
		query += " AND scope_id IS NULL"
	}

	rows, err := e.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var thresholds []QuotaAlertThresholdView
	for rows.Next() {
		var t QuotaAlertThresholdView
		if err := rows.Scan(&t.ID, &t.Scope, &t.ScopeID, &t.CompanyID, &t.WarningRatio, &t.CriticalRatio, &t.CreatedAt, &t.UpdatedAt); err != nil {
			return nil, err
		}
		thresholds = append(thresholds, t)
	}
	return thresholds, rows.Err()
}

func (e *QuotaWarningEmitter) checkAndRecordAlert(ctx context.Context, companyID string, scope QuotaAlertScope, scopeID string, alertType QuotaAlertType, quotaUsed, quotaLimit int64, usageRatio float64) (bool, error) {
	eventID := uuid.New().String()

	query := `
INSERT INTO quota_alerts (company_id, domain_id, user_id, scope, alert_type, quota_used, quota_limit, usage_ratio, event_id)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
RETURNING id`

	var domainID, userID *string
	if scope == QuotaAlertScopeDomain {
		domainID = &scopeID
	} else if scope == QuotaAlertScopeUser {
		userID = &scopeID
	}

	var id string
	err := e.db.QueryRowContext(ctx, query,
		companyID,
		domainID,
		userID,
		scope,
		alertType,
		quotaUsed,
		quotaLimit,
		usageRatio,
		eventID,
	).Scan(&id)

	if err != nil {
		return false, err
	}

	return true, nil
}

func (e *QuotaWarningEmitter) emitEvent(ctx context.Context, usage *quotaUsageInfo, scope QuotaAlertScope, alertType QuotaAlertType, quotaUsed, quotaLimit int64, usageRatio float64) error {
	if e.redisClient == nil {
		return nil
	}

	payload := QuotaWarningPayload{
		Event:        "mail.quota_warning",
		SchemaVersion: "2026-05-08.quota-warning.v1",
		CompanyID:    usage.CompanyID,
		Scope:        string(scope),
		AlertType:    string(alertType),
		QuotaUsed:    quotaUsed,
		QuotaLimit:   quotaLimit,
		UsageRatio:   usageRatio,
	}

	switch scope {
	case QuotaAlertScopeUser:
		payload.UserID = usage.UserID
		payload.DomainID = usage.DomainID
	case QuotaAlertScopeDomain:
		payload.DomainID = usage.DomainID
	}

	if alertType == QuotaAlertTypeCritical && quotaUsed >= quotaLimit {
		payload.AlertType = string(QuotaAlertTypeExhausted)
	}

	rawPayload, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal quota warning payload: %w", err)
	}

	partitionKey := usage.CompanyID
	if scope == QuotaAlertScopeUser {
		partitionKey = usage.UserID
	} else if scope == QuotaAlertScopeDomain {
		partitionKey = usage.DomainID
	}

	values := map[string]any{
		"outbox_id":      uuid.New().String(),
		"partition_key": partitionKey,
		"payload":        string(rawPayload),
	}

	if err := e.redisClient.XAdd(ctx, &redis.XAddArgs{
		Stream: e.eventStream,
		Values: values,
	}).Err(); err != nil {
		return fmt.Errorf("publish quota warning to redis: %w", err)
	}

	return nil
}