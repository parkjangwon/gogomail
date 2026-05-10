package admin

import (
	"context"
	"fmt"
	"time"
)

// MailLogEntry represents a mail operation log entry
type MailLogEntry struct {
	ID        string            `json:"id"`
	CompanyID string            `json:"company_id"`
	UserID    string            `json:"user_id"`
	Action    string            `json:"action"`
	MessageID string            `json:"message_id"`
	Details   map[string]string `json:"details"`
	Timestamp time.Time         `json:"timestamp"`
}

// MailLogQuery represents a query for mail logs
type MailLogQuery struct {
	UserID    string
	Action    string
	StartTime time.Time
	EndTime   time.Time
	Limit     int
	Offset    int
}

// MailLogRepository interface for mail log persistence
type MailLogRepository interface {
	CreateMailLog(ctx context.Context, log *MailLogEntry) error
	GetMailLog(ctx context.Context, logID string) (*MailLogEntry, error)
	QueryMailLogs(ctx context.Context, query *MailLogQuery) ([]*MailLogEntry, int64, error)
	DeleteMailLogsBefore(ctx context.Context, timestamp time.Time) (int64, error)
	CountMailLogsByAction(ctx context.Context, action string) (int64, error)
}

// MailLogService manages mail operation logging
type MailLogService struct {
	repo MailLogRepository
}

// NewMailLogService creates a new mail log service
func NewMailLogService(repo MailLogRepository) *MailLogService {
	return &MailLogService{
		repo: repo,
	}
}

// LogOperation logs a mail operation
func (mls *MailLogService) LogOperation(ctx context.Context, userID, action, messageID string, details map[string]string) (string, error) {
	if userID == "" {
		return "", fmt.Errorf("%w: userID", ErrMissingRequiredField)
	}
	if action == "" {
		return "", fmt.Errorf("%w: action", ErrMissingRequiredField)
	}
	if messageID == "" {
		return "", fmt.Errorf("%w: messageID", ErrMissingRequiredField)
	}

	logID := fmt.Sprintf("maillog-%d", time.Now().UnixNano())

	entry := &MailLogEntry{
		ID:        logID,
		UserID:    userID,
		Action:    action,
		MessageID: messageID,
		Details:   details,
		Timestamp: time.Now(),
	}

	if err := mls.repo.CreateMailLog(ctx, entry); err != nil {
		return "", fmt.Errorf("failed to create log: %w", err)
	}

	return logID, nil
}

// QueryLogs queries mail logs based on criteria
func (mls *MailLogService) QueryLogs(ctx context.Context, query *MailLogQuery) ([]*MailLogEntry, int64, error) {
	if query == nil {
		return nil, 0, fmt.Errorf("query required")
	}

	if query.Limit == 0 {
		query.Limit = 100
	}
	if query.Limit > 1000 {
		query.Limit = 1000
	}

	return mls.repo.QueryMailLogs(ctx, query)
}

// ApplyRetentionPolicy deletes logs older than the retention period (in months)
func (mls *MailLogService) ApplyRetentionPolicy(ctx context.Context, retentionMonths int) (int64, error) {
	if retentionMonths <= 0 {
		retentionMonths = 12 // Default 12 months
	}

	cutoffTime := time.Now().AddDate(0, -retentionMonths, 0)
	deleted, err := mls.repo.DeleteMailLogsBefore(ctx, cutoffTime)
	if err != nil {
		return 0, fmt.Errorf("failed to delete logs: %w", err)
	}

	return deleted, nil
}

// GetActionCount returns the count of logs for a specific action
func (mls *MailLogService) GetActionCount(ctx context.Context, action string) (int64, error) {
	if action == "" {
		return 0, fmt.Errorf("%w: action", ErrMissingRequiredField)
	}

	return mls.repo.CountMailLogsByAction(ctx, action)
}
