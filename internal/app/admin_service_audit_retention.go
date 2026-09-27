package app

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/gogomail/gogomail/internal/audit"
	"github.com/gogomail/gogomail/internal/auditretention"
)

// GetAuditRetentionPolicy returns the audit log retention policy for a company,
// or nil when the company keeps audit logs forever (no policy row).
func (s adminService) GetAuditRetentionPolicy(ctx context.Context, companyID string) (*auditretention.Policy, error) {
	if s.auditRetention == nil {
		return nil, fmt.Errorf("audit retention repository is not configured")
	}
	companyID = strings.TrimSpace(companyID)
	if companyID == "" {
		return nil, fmt.Errorf("company id is required")
	}
	return s.auditRetention.GetPolicy(ctx, companyID)
}

// SetAuditRetentionPolicy sets (or, for retentionDays == 0, removes) the audit
// log retention policy for a company and records an audit log for the change.
// retentionDays of 0 means "keep forever" and deletes any stored policy.
func (s adminService) SetAuditRetentionPolicy(ctx context.Context, companyID string, retentionDays int) (*auditretention.Policy, error) {
	if s.auditRetention == nil {
		return nil, fmt.Errorf("audit retention repository is not configured")
	}
	companyID = strings.TrimSpace(companyID)
	if companyID == "" {
		return nil, fmt.Errorf("company id is required")
	}
	if err := auditretention.ValidateRetentionDays(retentionDays); err != nil {
		return nil, err
	}

	var (
		policy *auditretention.Policy
		err    error
	)
	if retentionDays == auditretention.RetentionForever {
		err = s.auditRetention.DeletePolicy(ctx, companyID)
	} else {
		policy, err = s.auditRetention.SetPolicy(ctx, companyID, retentionDays, "")
	}
	if err != nil {
		return nil, err
	}

	if s.audit != nil {
		detail, derr := auditRetentionPolicyAuditDetail(companyID, retentionDays)
		if derr != nil {
			return nil, derr
		}
		action := "audit_retention.policy_set"
		if retentionDays == auditretention.RetentionForever {
			action = "audit_retention.policy_cleared"
		}
		if aerr := s.audit.Insert(ctx, audit.Log{
			CompanyID:  companyID,
			Category:   "admin",
			Action:     action,
			TargetType: "audit_retention_policy",
			TargetID:   companyID,
			Result:     "completed",
			Detail:     detail,
		}); aerr != nil {
			return nil, fmt.Errorf("record audit retention policy audit: %w", aerr)
		}
	}
	return policy, nil
}

// ListAuditRetentionRuns returns recent purge runs for a company.
func (s adminService) ListAuditRetentionRuns(ctx context.Context, req auditretention.RunListRequest) ([]auditretention.RunRecord, error) {
	if s.auditRetention == nil {
		return nil, fmt.Errorf("audit retention repository is not configured")
	}
	return s.auditRetention.ListRuns(ctx, req)
}

func auditRetentionPolicyAuditDetail(companyID string, retentionDays int) (json.RawMessage, error) {
	detail := struct {
		CompanyID     string `json:"company_id"`
		RetentionDays int    `json:"retention_days"`
		KeepForever   bool   `json:"keep_forever"`
	}{
		CompanyID:     companyID,
		RetentionDays: retentionDays,
		KeepForever:   retentionDays == auditretention.RetentionForever,
	}
	raw, err := json.Marshal(detail)
	if err != nil {
		return nil, fmt.Errorf("marshal audit retention policy detail: %w", err)
	}
	return raw, nil
}
