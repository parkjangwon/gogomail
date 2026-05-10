package admin

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"time"
)

// AlertRule represents an alert rule for threshold-based monitoring.
type AlertRule struct {
	ID                  string    `json:"id"`
	CompanyID           string    `json:"company_id"`
	AlertType           string    `json:"alert_type"` // 'storage', 'login_failures', 'api_errors'
	Name                string    `json:"name"`
	Description         string    `json:"description,omitempty"`
	Threshold           float64   `json:"threshold"`
	CheckIntervalMinutes int      `json:"check_interval_minutes"`
	IsEnabled           bool      `json:"is_enabled"`
	CreatedAt           time.Time `json:"created_at"`
	CreatedBy           string    `json:"created_by,omitempty"`
}

// AlertChannel represents a notification channel for alerts.
type AlertChannel struct {
	ID          string          `json:"id"`
	CompanyID   string          `json:"company_id"`
	ChannelType string          `json:"channel_type"` // 'email', 'webhook', 'dashboard'
	Name        string          `json:"name"`
	Config      AlertChannelConfig `json:"config"`
	IsEnabled   bool            `json:"is_enabled"`
	CreatedAt   time.Time       `json:"created_at"`
	CreatedBy   string          `json:"created_by,omitempty"`
}

// AlertChannelConfig holds channel-specific configuration.
type AlertChannelConfig struct {
	Recipients []string `json:"recipients,omitempty"` // email
	URL        string   `json:"url,omitempty"`        // webhook
	AuthHeader string   `json:"auth_header,omitempty"` // webhook
}

func (c AlertChannelConfig) Value() (driver.Value, error) {
	return json.Marshal(c)
}

func (c *AlertChannelConfig) Scan(value interface{}) error {
	b, ok := value.([]byte)
	if !ok {
		return fmt.Errorf("type assertion failed")
	}
	return json.Unmarshal(b, &c)
}

// AlertRuleChannel represents the mapping between an alert rule and its notification channels.
type AlertRuleChannel struct {
	ID            string `json:"id"`
	AlertRuleID   string `json:"alert_rule_id"`
	AlertChannelID string `json:"alert_channel_id"`
}

// AlertEvent represents a triggered alert event.
type AlertEvent struct {
	ID          string     `json:"id"`
	CompanyID   string     `json:"company_id"`
	AlertRuleID string     `json:"alert_rule_id"`
	CurrentValue float64   `json:"current_value"`
	Threshold   float64    `json:"threshold"`
	Message     string     `json:"message,omitempty"`
	TriggeredAt time.Time  `json:"triggered_at"`
	ResolvedAt  *time.Time `json:"resolved_at,omitempty"`
}

// AlertEventFilter for querying alert events.
type AlertEventFilter struct {
	CompanyID      string
	AlertRuleID    string
	OnlyUnresolved bool
	Limit          int
	Offset         int
}
