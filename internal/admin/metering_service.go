package admin

import (
	"context"
	"fmt"
	"time"
)

// MeteringRepository interface for usage tracking
type MeteringRepository interface {
	RecordAPICall(ctx context.Context, userID string) error
	GetUsage(ctx context.Context, userID string, period time.Time) (int64, error)
	SetQuota(ctx context.Context, userID string, quota int64) error
	GetQuota(ctx context.Context, userID string) (int64, error)
	ResetUsage(ctx context.Context, userID string) error
}

// MeteringService manages API usage tracking and rate limiting
type MeteringService struct {
	repo MeteringRepository
}

// NewMeteringService creates a new metering service
func NewMeteringService(repo MeteringRepository) *MeteringService {
	return &MeteringService{
		repo: repo,
	}
}

// RecordAPICall records an API call for rate limiting
func (ms *MeteringService) RecordAPICall(ctx context.Context, userID string) error {
	if userID == "" {
		return fmt.Errorf("%w: userID", ErrMissingRequiredField)
	}

	return ms.repo.RecordAPICall(ctx, userID)
}

// CheckRateLimit checks if user has exceeded their quota
func (ms *MeteringService) CheckRateLimit(ctx context.Context, userID string) (bool, error) {
	if userID == "" {
		return false, fmt.Errorf("%w: userID", ErrMissingRequiredField)
	}

	// Get current usage
	usage, err := ms.repo.GetUsage(ctx, userID, time.Now())
	if err != nil {
		return true, fmt.Errorf("failed to get usage: %w", err)
	}

	// Get quota
	quota, err := ms.repo.GetQuota(ctx, userID)
	if err != nil {
		// No quota set, allow unlimited
		return true, nil
	}

	// Check if usage exceeds quota
	if usage >= quota {
		return false, nil
	}

	return true, nil
}

// GetUsage returns current usage for a user
func (ms *MeteringService) GetUsage(ctx context.Context, userID string) (int64, error) {
	if userID == "" {
		return 0, fmt.Errorf("%w: userID", ErrMissingRequiredField)
	}

	return ms.repo.GetUsage(ctx, userID, time.Now())
}

// SetQuota sets the API quota for a user
func (ms *MeteringService) SetQuota(ctx context.Context, userID string, quota int64) error {
	if userID == "" {
		return fmt.Errorf("%w: userID", ErrMissingRequiredField)
	}
	if quota <= 0 {
		return fmt.Errorf("%w: quota must be positive", ErrMissingRequiredField)
	}

	return ms.repo.SetQuota(ctx, userID, quota)
}

// GetQuota returns the API quota for a user
func (ms *MeteringService) GetQuota(ctx context.Context, userID string) (int64, error) {
	if userID == "" {
		return 0, fmt.Errorf("%w: userID", ErrMissingRequiredField)
	}

	return ms.repo.GetQuota(ctx, userID)
}

// ResetUsage resets the usage counter for a user
func (ms *MeteringService) ResetUsage(ctx context.Context, userID string) error {
	if userID == "" {
		return fmt.Errorf("%w: userID", ErrMissingRequiredField)
	}

	return ms.repo.ResetUsage(ctx, userID)
}
