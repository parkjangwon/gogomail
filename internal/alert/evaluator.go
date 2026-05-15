package alert

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// DefaultEvaluator implements the Evaluator interface with threshold evaluation logic.
type DefaultEvaluator struct {
	repo Repository
}

// NewDefaultEvaluator creates a new evaluator with the given repository.
func NewDefaultEvaluator(repo Repository) *DefaultEvaluator {
	return &DefaultEvaluator{repo: repo}
}

// EvaluateStorage checks if storage usage exceeds any configured thresholds.
func (e *DefaultEvaluator) EvaluateStorage(ctx context.Context, companyID uuid.UUID, usagePercent float64) error {
	configs, err := e.repo.ListConfigs(ctx, companyID)
	if err != nil {
		return err
	}

	for _, cfg := range configs {
		if cfg.AlertType != AlertTypeStorage || !cfg.IsEnabled {
			continue
		}

		if usagePercent >= cfg.Threshold {
			notif := &Notification{
				CompanyID:     companyID,
				AlertConfigID: cfg.ID,
				AlertType:     AlertTypeStorage,
				Threshold:     cfg.Threshold,
				CurrentValue:  usagePercent,
				NotificationData: map[string]interface{}{
					"usage_percent": usagePercent,
					"threshold":     cfg.Threshold,
					"at":             time.Now().Unix(),
				},
			}

			if err := e.repo.CreateNotification(ctx, notif); err != nil {
				return fmt.Errorf("failed to create storage alert: %w", err)
			}
		}
	}

	return nil
}

// EvaluateLoginFailures checks if login failure count exceeds any configured thresholds.
func (e *DefaultEvaluator) EvaluateLoginFailures(ctx context.Context, companyID uuid.UUID, failureCount int) error {
	configs, err := e.repo.ListConfigs(ctx, companyID)
	if err != nil {
		return err
	}

	for _, cfg := range configs {
		if cfg.AlertType != AlertTypeLoginFailures || !cfg.IsEnabled {
			continue
		}

		if float64(failureCount) >= cfg.Threshold {
			notif := &Notification{
				CompanyID:     companyID,
				AlertConfigID: cfg.ID,
				AlertType:     AlertTypeLoginFailures,
				Threshold:     cfg.Threshold,
				CurrentValue:  float64(failureCount),
				NotificationData: map[string]interface{}{
					"failure_count": failureCount,
					"threshold":     cfg.Threshold,
					"at":             time.Now().Unix(),
				},
			}

			if err := e.repo.CreateNotification(ctx, notif); err != nil {
				return fmt.Errorf("failed to create login failure alert: %w", err)
			}
		}
	}

	return nil
}

// EvaluateAPIErrors checks if API error rate exceeds any configured thresholds.
func (e *DefaultEvaluator) EvaluateAPIErrors(ctx context.Context, companyID uuid.UUID, errorRate float64) error {
	configs, err := e.repo.ListConfigs(ctx, companyID)
	if err != nil {
		return err
	}

	for _, cfg := range configs {
		if cfg.AlertType != AlertTypeAPIErrors || !cfg.IsEnabled {
			continue
		}

		if errorRate >= cfg.Threshold {
			notif := &Notification{
				CompanyID:     companyID,
				AlertConfigID: cfg.ID,
				AlertType:     AlertTypeAPIErrors,
				Threshold:     cfg.Threshold,
				CurrentValue:  errorRate,
				NotificationData: map[string]interface{}{
					"error_rate": errorRate,
					"threshold":  cfg.Threshold,
					"at":          time.Now().Unix(),
				},
			}

			if err := e.repo.CreateNotification(ctx, notif); err != nil {
				return fmt.Errorf("failed to create API error alert: %w", err)
			}
		}
	}

	return nil
}
