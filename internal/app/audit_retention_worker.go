package app

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/gogomail/gogomail/internal/auditretention"
	"github.com/gogomail/gogomail/internal/config"
)

// auditRetentionPurger is the subset of the auditretention repository the purge
// worker depends on. It is an interface so the worker can be unit-tested with a
// fake.
type auditRetentionPurger interface {
	ListPolicies(ctx context.Context) ([]auditretention.Policy, error)
	PurgeExpired(ctx context.Context, companyID string, cutoff time.Time, batchSize int) (int64, error)
	CountExpired(ctx context.Context, companyID string, cutoff time.Time) (int64, error)
	RecordRun(ctx context.Context, record auditretention.RunRecord) (auditretention.RunRecord, error)
}

// auditRetentionPurgeResult summarizes one purge sweep across all companies
// that have a retention policy.
type auditRetentionPurgeResult struct {
	CompaniesScanned int
	RowsDeleted      int64
	Runs             []auditretention.RunRecord
}

// runAuditRetentionPurgeOnce enforces every company's audit retention policy in
// a single sweep. Companies without a policy row are never touched (they keep
// audit logs forever). For each company with a policy, audit_logs older than
// retention_days are deleted in bounded batches (or counted, when dry-run is
// enabled), and the outcome is recorded as a run.
func runAuditRetentionPurgeOnce(ctx context.Context, repo auditRetentionPurger, now func() time.Time, cfg config.Config, logger *slog.Logger) (auditRetentionPurgeResult, error) {
	if repo == nil {
		return auditRetentionPurgeResult{}, fmt.Errorf("audit retention repository is required")
	}
	if now == nil {
		now = time.Now
	}
	batchSize := cfg.AuditRetentionBatchSize
	if batchSize <= 0 {
		batchSize = auditretention.DefaultBatchSize
	}

	policies, err := repo.ListPolicies(ctx)
	if err != nil {
		return auditRetentionPurgeResult{}, err
	}

	result := auditRetentionPurgeResult{CompaniesScanned: len(policies)}
	for _, policy := range policies {
		if policy.RetentionDays <= 0 {
			// Defensive: a stored policy always has a positive window, but never
			// purge if that invariant is somehow violated.
			continue
		}
		started := now().UTC()
		cutoff := started.Add(-time.Duration(policy.RetentionDays) * 24 * time.Hour)

		var (
			rowsDeleted int64
			purgeErr    error
		)
		if cfg.AuditRetentionDryRun {
			rowsDeleted, purgeErr = repo.CountExpired(ctx, policy.CompanyID, cutoff)
		} else {
			rowsDeleted, purgeErr = repo.PurgeExpired(ctx, policy.CompanyID, cutoff, batchSize)
		}

		record := auditretention.RunRecord{
			CompanyID:     policy.CompanyID,
			RetentionDays: policy.RetentionDays,
			Cutoff:        cutoff,
			StartedAt:     started,
			FinishedAt:    now().UTC(),
			DryRun:        cfg.AuditRetentionDryRun,
			RowsDeleted:   rowsDeleted,
			Status:        auditretention.RunStatusCompleted,
		}
		if purgeErr != nil {
			record.Status = auditretention.RunStatusFailed
			record.ErrorMessage = purgeErr.Error()
			record.RowsDeleted = 0
		}

		saved, recErr := repo.RecordRun(ctx, record)
		if recErr != nil {
			return result, fmt.Errorf("record audit retention run for company %s: %w", policy.CompanyID, recErr)
		}
		result.Runs = append(result.Runs, saved)

		if purgeErr != nil {
			if logger != nil {
				logger.Error("audit retention purge failed for company",
					"company_id", policy.CompanyID,
					"retention_days", policy.RetentionDays,
					"cutoff", cutoff.Format(time.RFC3339),
					"dry_run", cfg.AuditRetentionDryRun,
					"error", purgeErr,
				)
			}
			return result, purgeErr
		}

		result.RowsDeleted += rowsDeleted
		if logger != nil {
			logger.Info("audit retention purge for company",
				"run_id", saved.ID,
				"company_id", policy.CompanyID,
				"retention_days", policy.RetentionDays,
				"cutoff", cutoff.Format(time.RFC3339),
				"dry_run", cfg.AuditRetentionDryRun,
				"rows_deleted", rowsDeleted,
			)
		}
	}
	return result, nil
}
