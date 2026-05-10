package admin

import (
	"context"
	"fmt"
)

// AuditPolicyRepository interface for audit policy persistence
type AuditPolicyRepository interface {
	GetPolicy(ctx context.Context, companyID string) (*AuditPolicyConfig, error)
	SavePolicy(ctx context.Context, policy *AuditPolicyConfig) error
	DeletePolicy(ctx context.Context, companyID string) error
}

// AuditPolicyService manages audit policies and enforcement levels
type AuditPolicyService struct {
	repo AuditPolicyRepository
}

// NewAuditPolicyService creates a new audit policy service
func NewAuditPolicyService(repo AuditPolicyRepository) *AuditPolicyService {
	return &AuditPolicyService{
		repo: repo,
	}
}

// GetPolicy retrieves the audit policy for a company
func (aps *AuditPolicyService) GetPolicy(ctx context.Context, companyID string) (*AuditPolicyConfig, error) {
	if companyID == "" {
		return nil, fmt.Errorf("%w: companyID", ErrMissingRequiredField)
	}

	return aps.repo.GetPolicy(ctx, companyID)
}

// SetPolicy sets the audit policy for a company
func (aps *AuditPolicyService) SetPolicy(ctx context.Context, companyID string, level string) error {
	if companyID == "" {
		return fmt.Errorf("%w: companyID", ErrMissingRequiredField)
	}

	if err := aps.ValidatePolicy(level); err != nil {
		return err
	}

	policy := &AuditPolicyConfig{
		CompanyID:  companyID,
		AuditLevel: level,
	}

	return aps.repo.SavePolicy(ctx, policy)
}

// ValidatePolicy validates audit policy parameters
func (aps *AuditPolicyService) ValidatePolicy(level string) error {
	// Validate audit level
	switch level {
	case "level_1", "level_2", "level_3":
		return nil
	default:
		return fmt.Errorf("invalid audit level: %s (must be level_1, level_2, or level_3)", level)
	}
}

// IsLevelEnabled checks if a specific audit level is enabled for a company
func (aps *AuditPolicyService) IsLevelEnabled(ctx context.Context, companyID string, checkLevel string) bool {
	policy, err := aps.repo.GetPolicy(ctx, companyID)
	if err != nil {
		return false
	}

	// Parse levels to integers for comparison
	policyLevelNum := aps.levelToNum(policy.AuditLevel)
	checkLevelNum := aps.levelToNum(checkLevel)

	// Higher levels include all lower levels
	return policyLevelNum >= checkLevelNum
}

// levelToNum converts audit level string to numeric value
func (aps *AuditPolicyService) levelToNum(level string) int {
	switch level {
	case "level_1":
		return 1
	case "level_2":
		return 2
	case "level_3":
		return 3
	default:
		return 0
	}
}

// DeletePolicy removes the audit policy for a company
func (aps *AuditPolicyService) DeletePolicy(ctx context.Context, companyID string) error {
	if companyID == "" {
		return fmt.Errorf("%w: companyID", ErrMissingRequiredField)
	}

	return aps.repo.DeletePolicy(ctx, companyID)
}
