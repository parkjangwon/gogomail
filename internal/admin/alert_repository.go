package admin

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

// CreateAlertRule inserts a new alert rule.
func (r *Repository) CreateAlertRule(ctx context.Context, rule *AlertRule) error {
	err := r.db.QueryRowContext(ctx,
		`INSERT INTO alert_rules (company_id, alert_type, name, description, threshold, check_interval_minutes, is_enabled, created_by, created_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		 RETURNING id, created_at`,
		rule.CompanyID, rule.AlertType, rule.Name, rule.Description, rule.Threshold, rule.CheckIntervalMinutes, rule.IsEnabled, rule.CreatedBy, time.Now(),
	).Scan(&rule.ID, &rule.CreatedAt)
	if err != nil {
		return fmt.Errorf("insert alert rule: %w", err)
	}
	return nil
}

// GetAlertRule retrieves an alert rule by ID.
func (r *Repository) GetAlertRule(ctx context.Context, ruleID string) (*AlertRule, error) {
	rule := &AlertRule{}
	err := r.db.QueryRowContext(ctx,
		`SELECT id, company_id, alert_type, name, description, threshold, check_interval_minutes, is_enabled, created_at, created_by
		 FROM alert_rules WHERE id = $1`,
		ruleID,
	).Scan(&rule.ID, &rule.CompanyID, &rule.AlertType, &rule.Name, &rule.Description, &rule.Threshold, &rule.CheckIntervalMinutes, &rule.IsEnabled, &rule.CreatedAt, &rule.CreatedBy)
	if err == sql.ErrNoRows {
		return nil, ErrRoleNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get alert rule: %w", err)
	}
	return rule, nil
}

// ListAlertRules lists all alert rules for a company.
func (r *Repository) ListAlertRules(ctx context.Context, companyID string) ([]AlertRule, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, company_id, alert_type, name, description, threshold, check_interval_minutes, is_enabled, created_at, created_by
		 FROM alert_rules WHERE company_id = $1 ORDER BY created_at DESC`,
		companyID,
	)
	if err != nil {
		return nil, fmt.Errorf("list alert rules: %w", err)
	}
	defer rows.Close()

	var rules []AlertRule
	for rows.Next() {
		rule := AlertRule{}
		if err := rows.Scan(&rule.ID, &rule.CompanyID, &rule.AlertType, &rule.Name, &rule.Description, &rule.Threshold, &rule.CheckIntervalMinutes, &rule.IsEnabled, &rule.CreatedAt, &rule.CreatedBy); err != nil {
			return nil, fmt.Errorf("scan alert rule: %w", err)
		}
		rules = append(rules, rule)
	}
	return rules, rows.Err()
}

// UpdateAlertRule updates an existing alert rule.
func (r *Repository) UpdateAlertRule(ctx context.Context, rule *AlertRule) error {
	result, err := r.db.ExecContext(ctx,
		`UPDATE alert_rules SET name = $1, description = $2, threshold = $3, check_interval_minutes = $4, is_enabled = $5
		 WHERE id = $6`,
		rule.Name, rule.Description, rule.Threshold, rule.CheckIntervalMinutes, rule.IsEnabled, rule.ID,
	)
	if err != nil {
		return fmt.Errorf("update alert rule: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("rows affected: %w", err)
	}
	if rows == 0 {
		return ErrRoleNotFound
	}
	return nil
}

// DeleteAlertRule deletes an alert rule.
func (r *Repository) DeleteAlertRule(ctx context.Context, ruleID string) error {
	result, err := r.db.ExecContext(ctx, "DELETE FROM alert_rules WHERE id = $1", ruleID)
	if err != nil {
		return fmt.Errorf("delete alert rule: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("rows affected: %w", err)
	}
	if rows == 0 {
		return ErrRoleNotFound
	}
	return nil
}

// CreateAlertChannel inserts a new alert channel.
func (r *Repository) CreateAlertChannel(ctx context.Context, channel *AlertChannel) error {
	b, err := channel.Config.Value()
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}

	err = r.db.QueryRowContext(ctx,
		`INSERT INTO alert_channels (company_id, channel_type, name, config, is_enabled, created_by, created_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7)
		 RETURNING id, created_at`,
		channel.CompanyID, channel.ChannelType, channel.Name, b, channel.IsEnabled, channel.CreatedBy, time.Now(),
	).Scan(&channel.ID, &channel.CreatedAt)
	if err != nil {
		return fmt.Errorf("insert alert channel: %w", err)
	}
	return nil
}

// GetAlertChannel retrieves an alert channel by ID.
func (r *Repository) GetAlertChannel(ctx context.Context, channelID string) (*AlertChannel, error) {
	channel := &AlertChannel{}
	var configJSON []byte

	err := r.db.QueryRowContext(ctx,
		`SELECT id, company_id, channel_type, name, config, is_enabled, created_at, created_by
		 FROM alert_channels WHERE id = $1`,
		channelID,
	).Scan(&channel.ID, &channel.CompanyID, &channel.ChannelType, &channel.Name, &configJSON, &channel.IsEnabled, &channel.CreatedAt, &channel.CreatedBy)
	if err == sql.ErrNoRows {
		return nil, ErrPermissionNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get alert channel: %w", err)
	}

	if err := channel.Config.Scan(configJSON); err != nil {
		return nil, fmt.Errorf("unmarshal config: %w", err)
	}
	return channel, nil
}

// ListAlertChannels lists all alert channels for a company.
func (r *Repository) ListAlertChannels(ctx context.Context, companyID string) ([]AlertChannel, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, company_id, channel_type, name, config, is_enabled, created_at, created_by
		 FROM alert_channels WHERE company_id = $1 ORDER BY created_at DESC`,
		companyID,
	)
	if err != nil {
		return nil, fmt.Errorf("list alert channels: %w", err)
	}
	defer rows.Close()

	var channels []AlertChannel
	for rows.Next() {
		channel := AlertChannel{}
		var configJSON []byte
		if err := rows.Scan(&channel.ID, &channel.CompanyID, &channel.ChannelType, &channel.Name, &configJSON, &channel.IsEnabled, &channel.CreatedAt, &channel.CreatedBy); err != nil {
			return nil, fmt.Errorf("scan alert channel: %w", err)
		}
		if err := channel.Config.Scan(configJSON); err != nil {
			return nil, fmt.Errorf("unmarshal config: %w", err)
		}
		channels = append(channels, channel)
	}
	return channels, rows.Err()
}

// UpdateAlertChannel updates an existing alert channel.
func (r *Repository) UpdateAlertChannel(ctx context.Context, channel *AlertChannel) error {
	b, err := channel.Config.Value()
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}

	result, err := r.db.ExecContext(ctx,
		`UPDATE alert_channels SET name = $1, config = $2, is_enabled = $3
		 WHERE id = $4`,
		channel.Name, b, channel.IsEnabled, channel.ID,
	)
	if err != nil {
		return fmt.Errorf("update alert channel: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("rows affected: %w", err)
	}
	if rows == 0 {
		return ErrPermissionNotFound
	}
	return nil
}

// DeleteAlertChannel deletes an alert channel.
func (r *Repository) DeleteAlertChannel(ctx context.Context, channelID string) error {
	result, err := r.db.ExecContext(ctx, "DELETE FROM alert_channels WHERE id = $1", channelID)
	if err != nil {
		return fmt.Errorf("delete alert channel: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("rows affected: %w", err)
	}
	if rows == 0 {
		return ErrPermissionNotFound
	}
	return nil
}

// CreateAlertRuleChannel creates a mapping between an alert rule and channel.
func (r *Repository) CreateAlertRuleChannel(ctx context.Context, mapping *AlertRuleChannel) error {
	err := r.db.QueryRowContext(ctx,
		`INSERT INTO alert_rule_channels (id, alert_rule_id, alert_channel_id) VALUES ($1, $2, $3)
		 RETURNING id`,
		mapping.ID, mapping.AlertRuleID, mapping.AlertChannelID,
	).Scan(&mapping.ID)
	if err != nil {
		return fmt.Errorf("insert alert rule channel: %w", err)
	}
	return nil
}

// ListAlertRuleChannels lists all channels for an alert rule.
func (r *Repository) ListAlertRuleChannels(ctx context.Context, ruleID string) ([]AlertChannel, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT c.id, c.company_id, c.channel_type, c.name, c.config, c.is_enabled, c.created_at, c.created_by
		 FROM alert_channels c
		 INNER JOIN alert_rule_channels rc ON c.id = rc.alert_channel_id
		 WHERE rc.alert_rule_id = $1
		 ORDER BY c.created_at DESC`,
		ruleID,
	)
	if err != nil {
		return nil, fmt.Errorf("list alert rule channels: %w", err)
	}
	defer rows.Close()

	var channels []AlertChannel
	for rows.Next() {
		channel := AlertChannel{}
		var configJSON []byte
		if err := rows.Scan(&channel.ID, &channel.CompanyID, &channel.ChannelType, &channel.Name, &configJSON, &channel.IsEnabled, &channel.CreatedAt, &channel.CreatedBy); err != nil {
			return nil, fmt.Errorf("scan alert channel: %w", err)
		}
		if err := channel.Config.Scan(configJSON); err != nil {
			return nil, fmt.Errorf("unmarshal config: %w", err)
		}
		channels = append(channels, channel)
	}
	return channels, rows.Err()
}

// DeleteAlertRuleChannel removes a mapping between an alert rule and channel.
func (r *Repository) DeleteAlertRuleChannel(ctx context.Context, ruleID, channelID string) error {
	result, err := r.db.ExecContext(ctx,
		"DELETE FROM alert_rule_channels WHERE alert_rule_id = $1 AND alert_channel_id = $2",
		ruleID, channelID,
	)
	if err != nil {
		return fmt.Errorf("delete alert rule channel: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("rows affected: %w", err)
	}
	if rows == 0 {
		return ErrPermissionNotFound
	}
	return nil
}

// LogAlertEvent logs an alert event.
func (r *Repository) LogAlertEvent(ctx context.Context, event *AlertEvent) error {
	err := r.db.QueryRowContext(ctx,
		`INSERT INTO alert_events (id, company_id, alert_rule_id, current_value, threshold, message, triggered_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7)
		 RETURNING id, triggered_at`,
		event.ID, event.CompanyID, event.AlertRuleID, event.CurrentValue, event.Threshold, event.Message, time.Now(),
	).Scan(&event.ID, &event.TriggeredAt)
	if err != nil {
		return fmt.Errorf("insert alert event: %w", err)
	}
	return nil
}

// ListAlertEvents lists alert events with filtering.
func (r *Repository) ListAlertEvents(ctx context.Context, filter AlertEventFilter) ([]AlertEvent, error) {
	query := `SELECT id, company_id, alert_rule_id, current_value, threshold, message, triggered_at, resolved_at
		 FROM alert_events WHERE company_id = $1`
	args := []interface{}{filter.CompanyID}

	if filter.AlertRuleID != "" {
		query += " AND alert_rule_id = $" + fmt.Sprintf("%d", len(args)+1)
		args = append(args, filter.AlertRuleID)
	}

	if filter.OnlyUnresolved {
		query += " AND resolved_at IS NULL"
	}

	query += " ORDER BY triggered_at DESC"

	if filter.Limit > 0 {
		query += " LIMIT $" + fmt.Sprintf("%d", len(args)+1)
		args = append(args, filter.Limit)
	}

	if filter.Offset > 0 {
		query += " OFFSET $" + fmt.Sprintf("%d", len(args)+1)
		args = append(args, filter.Offset)
	}

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list alert events: %w", err)
	}
	defer rows.Close()

	var events []AlertEvent
	for rows.Next() {
		event := AlertEvent{}
		if err := rows.Scan(&event.ID, &event.CompanyID, &event.AlertRuleID, &event.CurrentValue, &event.Threshold, &event.Message, &event.TriggeredAt, &event.ResolvedAt); err != nil {
			return nil, fmt.Errorf("scan alert event: %w", err)
		}
		events = append(events, event)
	}
	return events, rows.Err()
}

// ResolveAlertEvent marks an alert event as resolved.
func (r *Repository) ResolveAlertEvent(ctx context.Context, eventID string) error {
	result, err := r.db.ExecContext(ctx,
		"UPDATE alert_events SET resolved_at = $1 WHERE id = $2 AND resolved_at IS NULL",
		time.Now(), eventID,
	)
	if err != nil {
		return fmt.Errorf("resolve alert event: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("rows affected: %w", err)
	}
	if rows == 0 {
		return ErrAuditLogNotFound
	}
	return nil
}
