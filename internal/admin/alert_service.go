package admin

import (
	"context"
	"fmt"
	"time"
)

// CreateAlertRule creates a new alert rule.
func (s *Service) CreateAlertRule(ctx context.Context, rule *AlertRule) error {
	if rule.CompanyID == "" {
		return fmt.Errorf("%w: company_id", ErrMissingRequiredField)
	}
	if rule.Name == "" {
		return fmt.Errorf("%w: name", ErrMissingRequiredField)
	}
	if rule.AlertType == "" {
		return fmt.Errorf("%w: alert_type", ErrMissingRequiredField)
	}
	if rule.Threshold <= 0 {
		return fmt.Errorf("threshold must be greater than 0")
	}
	if rule.CheckIntervalMinutes <= 0 {
		return fmt.Errorf("check_interval_minutes must be greater than 0")
	}

	rule.CreatedAt = time.Now()
	return s.repo.CreateAlertRule(ctx, rule)
}

// GetAlertRule retrieves an alert rule by ID.
func (s *Service) GetAlertRule(ctx context.Context, ruleID string) (*AlertRule, error) {
	return s.repo.GetAlertRule(ctx, ruleID)
}

// ListAlertRules lists all alert rules for a company.
func (s *Service) ListAlertRules(ctx context.Context, companyID string) ([]AlertRule, error) {
	return s.repo.ListAlertRules(ctx, companyID)
}

// UpdateAlertRule updates an existing alert rule.
func (s *Service) UpdateAlertRule(ctx context.Context, rule *AlertRule) error {
	if rule.ID == "" {
		return fmt.Errorf("%w: id", ErrMissingRequiredField)
	}
	if rule.Name == "" {
		return fmt.Errorf("%w: name", ErrMissingRequiredField)
	}
	if rule.Threshold <= 0 {
		return fmt.Errorf("threshold must be greater than 0")
	}
	if rule.CheckIntervalMinutes <= 0 {
		return fmt.Errorf("check_interval_minutes must be greater than 0")
	}

	return s.repo.UpdateAlertRule(ctx, rule)
}

// DeleteAlertRule deletes an alert rule.
func (s *Service) DeleteAlertRule(ctx context.Context, ruleID string) error {
	return s.repo.DeleteAlertRule(ctx, ruleID)
}

// CreateAlertChannel creates a new alert channel.
func (s *Service) CreateAlertChannel(ctx context.Context, channel *AlertChannel) error {
	if channel.CompanyID == "" {
		return fmt.Errorf("%w: company_id", ErrMissingRequiredField)
	}
	if channel.Name == "" {
		return fmt.Errorf("%w: name", ErrMissingRequiredField)
	}
	if channel.ChannelType == "" {
		return fmt.Errorf("%w: channel_type", ErrMissingRequiredField)
	}

	// Validate channel type
	validChannelTypes := map[string]bool{
		"email":     true,
		"webhook":   true,
		"dashboard": true,
	}
	if !validChannelTypes[channel.ChannelType] {
		return fmt.Errorf("invalid channel_type: %s", channel.ChannelType)
	}

	// Validate channel-specific config
	if channel.ChannelType == "email" && len(channel.Config.Recipients) == 0 {
		return fmt.Errorf("email channel must have recipients")
	}
	if channel.ChannelType == "webhook" && channel.Config.URL == "" {
		return fmt.Errorf("webhook channel must have URL")
	}

	channel.CreatedAt = time.Now()
	return s.repo.CreateAlertChannel(ctx, channel)
}

// GetAlertChannel retrieves an alert channel by ID.
func (s *Service) GetAlertChannel(ctx context.Context, channelID string) (*AlertChannel, error) {
	return s.repo.GetAlertChannel(ctx, channelID)
}

// ListAlertChannels lists all alert channels for a company.
func (s *Service) ListAlertChannels(ctx context.Context, companyID string) ([]AlertChannel, error) {
	return s.repo.ListAlertChannels(ctx, companyID)
}

// UpdateAlertChannel updates an existing alert channel.
func (s *Service) UpdateAlertChannel(ctx context.Context, channel *AlertChannel) error {
	if channel.ID == "" {
		return fmt.Errorf("%w: id", ErrMissingRequiredField)
	}
	if channel.Name == "" {
		return fmt.Errorf("%w: name", ErrMissingRequiredField)
	}

	// Validate channel-specific config
	if channel.ChannelType == "email" && len(channel.Config.Recipients) == 0 {
		return fmt.Errorf("email channel must have recipients")
	}
	if channel.ChannelType == "webhook" && channel.Config.URL == "" {
		return fmt.Errorf("webhook channel must have URL")
	}

	return s.repo.UpdateAlertChannel(ctx, channel)
}

// DeleteAlertChannel deletes an alert channel.
func (s *Service) DeleteAlertChannel(ctx context.Context, channelID string) error {
	return s.repo.DeleteAlertChannel(ctx, channelID)
}

// ListAlertEvents lists alert events.
func (s *Service) ListAlertEvents(ctx context.Context, filter AlertEventFilter) ([]AlertEvent, bool, error) {
	return s.repo.ListAlertEvents(ctx, filter)
}

// LogAlertEvent logs an alert event.
func (s *Service) LogAlertEvent(ctx context.Context, event *AlertEvent) error {
	if event.CompanyID == "" {
		return fmt.Errorf("%w: company_id", ErrMissingRequiredField)
	}
	if event.AlertRuleID == "" {
		return fmt.Errorf("%w: alert_rule_id", ErrMissingRequiredField)
	}

	event.TriggeredAt = time.Now()
	return s.repo.LogAlertEvent(ctx, event)
}
