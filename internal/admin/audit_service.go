package admin

import (
	"context"
	"fmt"
	"strings"
	"time"
)

var (
	ErrInvalidAuditAction = fmt.Errorf("invalid audit action")
)

// AuditService handles audit logging operations.
type AuditService struct {
	repo RepositoryInterface
}

// NewAuditService creates a new audit service.
func NewAuditService(repo RepositoryInterface) *AuditService {
	return &AuditService{
		repo: repo,
	}
}

// LogAdminAction logs an admin action for Level 1 audit.
func (s *AuditService) LogAdminAction(ctx context.Context, adminID, companyID, action, resourceType, resourceID string, changes *AuditChanges) error {
	if adminID == "" {
		return fmt.Errorf("%w: adminID", ErrMissingRequiredField)
	}
	if companyID == "" {
		return fmt.Errorf("%w: companyID", ErrMissingRequiredField)
	}
	if action == "" {
		return fmt.Errorf("%w: action", ErrMissingRequiredField)
	}

	log := &AuditLog{
		CompanyID:   companyID,
		AdminUserID: adminID,
		Action:      action,
		ResourceType: resourceType,
		ResourceID:  resourceID,
		Timestamp:   time.Now(),
	}
	if changes != nil {
		log.Changes = *changes
	}

	return s.repo.LogAuditEvent(ctx, log)
}

// LogSecurityEvent logs a security event for Level 2 audit.
func (s *AuditService) LogSecurityEvent(ctx context.Context, adminID, companyID, event string, details *AuditChanges) error {
	if adminID == "" {
		return fmt.Errorf("%w: adminID", ErrMissingRequiredField)
	}
	if companyID == "" {
		return fmt.Errorf("%w: companyID", ErrMissingRequiredField)
	}
	if event == "" {
		return fmt.Errorf("%w: event", ErrMissingRequiredField)
	}

	log := &AuditLog{
		CompanyID:   companyID,
		AdminUserID: adminID,
		Action:      event,
		ResourceType: "security",
		Timestamp:   time.Now(),
	}
	if details != nil {
		log.Changes = *details
	}

	return s.repo.LogAuditEvent(ctx, log)
}

// LogLoginAttempt logs a user login attempt.
func (s *AuditService) LogLoginAttempt(ctx context.Context, userID, companyID, ipAddress, userAgent string, success bool, failureReason string) error {
	if userID == "" {
		return fmt.Errorf("%w: userID", ErrMissingRequiredField)
	}
	if companyID == "" {
		return fmt.Errorf("%w: companyID", ErrMissingRequiredField)
	}

	log := &LoginAuditLog{
		UserID:        userID,
		CompanyID:     companyID,
		IPAddress:     ipAddress,
		UserAgent:     userAgent,
		Success:       success,
		FailureReason: failureReason,
		Timestamp:     time.Now(),
	}

	return s.repo.LogLoginAttempt(ctx, log)
}

// QueryAuditLogs retrieves audit logs with filtering.
func (s *AuditService) QueryAuditLogs(ctx context.Context, filter *AuditLogFilter) ([]AuditLog, int64, error) {
	if filter.CompanyID == "" {
		return nil, 0, fmt.Errorf("%w: companyID", ErrMissingRequiredField)
	}

	logs, count, err := s.repo.ListAuditLogs(ctx, *filter)
	if err != nil {
		return nil, 0, err
	}

	return logs, count, nil
}

// MaskEmail masks an email address for privacy.
func (s *AuditService) MaskEmail(email string) string {
	if email == "" {
		return ""
	}

	parts := strings.Split(email, "@")
	if len(parts) != 2 {
		return email
	}

	local := parts[0]
	domain := parts[1]

	// Mask local part: keep first char if > 1 char, else mask all
	if len(local) <= 1 {
		// Mask completely for single character
		return "*@" + domain
	}

	masked := string(local[0])
	for i := 1; i < len(local); i++ {
		masked += "*"
	}

	return masked + "@" + domain
}

// MaskContent masks sensitive content (mail body, etc).
func (s *AuditService) MaskContent(content string, maxLength int) string {
	if content == "" {
		return content
	}

	// Just truncate and mask
	if len(content) > maxLength {
		return "[masked - " + fmt.Sprintf("%d", len(content)) + " chars]"
	}

	return "[masked]"
}

// ApplyRetentionPolicy deletes audit logs older than retention period.
func (s *AuditService) ApplyRetentionPolicy(ctx context.Context, companyID string, retentionDays int) error {
	if companyID == "" {
		return fmt.Errorf("%w: companyID", ErrMissingRequiredField)
	}

	if retentionDays <= 0 {
		return fmt.Errorf("retention days must be positive")
	}

	cutoff := time.Now().AddDate(0, 0, -retentionDays)
	_, err := s.repo.DeleteAuditLogsBefore(ctx, companyID, cutoff)
	return err
}

// GetAuditLog retrieves a single audit log entry.
func (s *AuditService) GetAuditLog(ctx context.Context, logID string) (*AuditLog, error) {
	if logID == "" {
		return nil, fmt.Errorf("%w: logID", ErrMissingRequiredField)
	}
	return s.repo.GetAuditLog(ctx, logID)
}

// GetAuditStats returns statistics about audit logs.
func (s *AuditService) GetAuditStats(ctx context.Context, companyID string) (map[string]interface{}, error) {
	if companyID == "" {
		return nil, fmt.Errorf("%w: companyID", ErrMissingRequiredField)
	}

	filter := AuditLogFilter{
		CompanyID: companyID,
		Limit:     1000,
	}

	logs, count, err := s.repo.ListAuditLogs(ctx, filter)
	if err != nil {
		return nil, err
	}

	// Count actions by type
	actionCounts := make(map[string]int)
	adminCounts := make(map[string]int)

	for _, log := range logs {
		actionCounts[log.Action]++
		adminCounts[log.AdminUserID]++
	}

	stats := map[string]interface{}{
		"total_logs": count,
		"actions":    actionCounts,
		"admins":     adminCounts,
	}

	return stats, nil
}
