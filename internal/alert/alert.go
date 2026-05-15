package alert

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// AlertType defines the kind of alert being monitored.
type AlertType string

const (
	AlertTypeStorage       AlertType = "storage"
	AlertTypeLoginFailures AlertType = "login_failures"
	AlertTypeAPIErrors     AlertType = "api_errors"
)

// ChannelType defines the delivery mechanism for alerts.
type ChannelType string

const (
	ChannelTypeEmail     ChannelType = "email"
	ChannelTypeWebhook   ChannelType = "webhook"
	ChannelTypeDashboard ChannelType = "dashboard"
)

// Config represents an alert configuration with thresholds and channels.
type Config struct {
	ID                   uuid.UUID
	CompanyID            uuid.UUID
	AlertType            AlertType
	Threshold            float64
	Name                 string
	Description          string
	CheckIntervalMinutes int
	IsEnabled            bool
	CreatedAt            time.Time
	UpdatedAt            time.Time
	CreatedByID          *uuid.UUID
	Channels             []Channel
}

// Channel represents a notification channel for an alert.
type Channel struct {
	ID          uuid.UUID
	ConfigID    uuid.UUID
	ChannelType ChannelType
	Config      map[string]interface{}
	IsEnabled   bool
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// Notification represents a triggered alert notification.
type Notification struct {
	ID                 uuid.UUID
	CompanyID          uuid.UUID
	AlertConfigID      uuid.UUID
	AlertType          AlertType
	Threshold          float64
	CurrentValue       float64
	EmailSent          bool
	WebhookSent        bool
	DashboardShown     bool
	NotificationData   map[string]interface{}
	CreatedAt          time.Time
	AcknowledgedAt     *time.Time
}

// Evaluator defines alert threshold evaluation logic.
type Evaluator interface {
	// EvaluateStorage checks storage usage against configured threshold.
	EvaluateStorage(ctx context.Context, companyID uuid.UUID, usagePercent float64) error

	// EvaluateLoginFailures checks login failure rate against configured threshold.
	EvaluateLoginFailures(ctx context.Context, companyID uuid.UUID, failureCount int) error

	// EvaluateAPIErrors checks API error rate against configured threshold.
	EvaluateAPIErrors(ctx context.Context, companyID uuid.UUID, errorRate float64) error
}

// Dispatcher handles alert notification delivery.
type Dispatcher interface {
	// DispatchNotification sends alerts through configured channels.
	DispatchNotification(ctx context.Context, notification *Notification, channels []Channel) error
}

// Repository provides data access for alert configurations and notifications.
type Repository interface {
	// CreateConfig creates a new alert configuration.
	CreateConfig(ctx context.Context, cfg *Config) error

	// GetConfig retrieves a single alert configuration.
	GetConfig(ctx context.Context, id uuid.UUID) (*Config, error)

	// ListConfigs lists all alert configurations for a company.
	ListConfigs(ctx context.Context, companyID uuid.UUID) ([]Config, error)

	// UpdateConfig updates an existing alert configuration.
	UpdateConfig(ctx context.Context, cfg *Config) error

	// DeleteConfig deletes an alert configuration.
	DeleteConfig(ctx context.Context, id uuid.UUID) error

	// CreateChannel creates a notification channel for a config.
	CreateChannel(ctx context.Context, channel *Channel) error

	// DeleteChannel deletes a notification channel.
	DeleteChannel(ctx context.Context, id uuid.UUID) error

	// CreateNotification records a triggered alert notification.
	CreateNotification(ctx context.Context, notif *Notification) error

	// ListNotifications lists recent notifications for a company.
	ListNotifications(ctx context.Context, companyID uuid.UUID, limit int) ([]Notification, error)

	// AcknowledgeNotification marks a notification as acknowledged.
	AcknowledgeNotification(ctx context.Context, id uuid.UUID) error
}
