package maildb

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/gogomail/gogomail/internal/audit"
)

func (r *Repository) ListCompanies(ctx context.Context, req CompanyListRequest) ([]CompanyView, error) {
	if r.db == nil {
		return nil, fmt.Errorf("database handle is required")
	}
	if err := ValidateCompanyListRequest(req); err != nil {
		return nil, err
	}
	limit := normalizeLimit(req.Limit)
	status := normalizeAdminStatus(req.Status)

	rows, err := r.db.QueryContext(ctx, `
SELECT
  id::text,
  name,
  status,
  quota_used,
  COALESCE(quota_limit, 0),
  COALESCE((
    SELECT SUM(child.quota_limit)
    FROM domains child
    WHERE child.company_id = companies.id
      AND child.quota_limit IS NOT NULL
      AND child.quota_limit > 0
  ), 0) AS allocated_domain_quota,
  created_at
FROM companies
WHERE ($1 = '' OR status = $1)
ORDER BY created_at DESC
LIMIT $2`, status, limit)
	if err != nil {
		return nil, fmt.Errorf("list companies: %w", err)
	}
	defer rows.Close()

	var companies []CompanyView
	for rows.Next() {
		var company CompanyView
		if err := rows.Scan(
			&company.ID,
			&company.Name,
			&company.Status,
			&company.QuotaUsed,
			&company.QuotaLimit,
			&company.AllocatedDomainQuota,
			&company.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan company: %w", err)
		}
		company.QuotaRemaining = quotaRemaining(company.QuotaUsed, company.QuotaLimit)
		company.AllocatableDomainQuota = quotaRemaining(company.AllocatedDomainQuota, company.QuotaLimit)
		company.OverAllocated = company.QuotaLimit > 0 && company.AllocatedDomainQuota > company.QuotaLimit
		companies = append(companies, company)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate companies: %w", err)
	}
	return companies, nil
}

func (r *Repository) GetCompany(ctx context.Context, id string) (CompanyView, error) {
	if r.db == nil {
		return CompanyView{}, fmt.Errorf("database handle is required")
	}
	id = strings.TrimSpace(id)
	if id == "" {
		return CompanyView{}, fmt.Errorf("company id is required")
	}

	var company CompanyView
	if err := r.db.QueryRowContext(ctx, `
SELECT
  id::text,
  name,
  status,
  quota_used,
  COALESCE(quota_limit, 0),
  COALESCE((
    SELECT SUM(child.quota_limit)
    FROM domains child
    WHERE child.company_id = companies.id
      AND child.quota_limit IS NOT NULL
      AND child.quota_limit > 0
  ), 0) AS allocated_domain_quota,
  created_at
FROM companies
WHERE id = $1`, id).Scan(
		&company.ID,
		&company.Name,
		&company.Status,
		&company.QuotaUsed,
		&company.QuotaLimit,
		&company.AllocatedDomainQuota,
		&company.CreatedAt,
	); err != nil {
		if err == sql.ErrNoRows {
			return CompanyView{}, fmt.Errorf("company %q not found", id)
		}
		return CompanyView{}, fmt.Errorf("get company: %w", err)
	}
	company.QuotaRemaining = quotaRemaining(company.QuotaUsed, company.QuotaLimit)
	company.AllocatableDomainQuota = quotaRemaining(company.AllocatedDomainQuota, company.QuotaLimit)
	company.OverAllocated = company.QuotaLimit > 0 && company.AllocatedDomainQuota > company.QuotaLimit
	return company, nil
}

func (r *Repository) CreateCompany(ctx context.Context, req CreateCompanyRequest) (CompanyView, error) {
	if r.db == nil {
		return CompanyView{}, fmt.Errorf("database handle is required")
	}
	req.Name = strings.TrimSpace(req.Name)
	if req.Name == "" {
		return CompanyView{}, fmt.Errorf("company name is required")
	}
	if req.QuotaLimit < 0 {
		return CompanyView{}, fmt.Errorf("quota limit must be non-negative")
	}

	var company CompanyView
	if err := r.db.QueryRowContext(ctx, `
INSERT INTO companies (name, status, quota_limit, created_at)
VALUES ($1, 'active', $2, NOW())
RETURNING id::text, name, status, quota_used, COALESCE(quota_limit, 0), 0, created_at
	`, req.Name, req.QuotaLimit).Scan(
		&company.ID,
		&company.Name,
		&company.Status,
		&company.QuotaUsed,
		&company.QuotaLimit,
		&company.AllocatedDomainQuota,
		&company.CreatedAt,
	); err != nil {
		return CompanyView{}, fmt.Errorf("create company: %w", err)
	}
	company.QuotaRemaining = quotaRemaining(company.QuotaUsed, company.QuotaLimit)
	company.AllocatableDomainQuota = quotaRemaining(company.AllocatedDomainQuota, company.QuotaLimit)
	company.OverAllocated = company.QuotaLimit > 0 && company.AllocatedDomainQuota > company.QuotaLimit
	return company, nil
}

func (r *Repository) UpdateCompanyQuota(ctx context.Context, req UpdateCompanyQuotaRequest) error {
	if r.db == nil {
		return fmt.Errorf("database handle is required")
	}
	if err := ValidateUpdateCompanyQuotaRequest(req); err != nil {
		return err
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin company quota transaction: %w", err)
	}
	defer tx.Rollback()

	var view companyQuotaAuditView
	if err := tx.QueryRowContext(ctx, `
UPDATE companies
SET quota_limit = NULLIF($2::bigint, 0),
    updated_at = now()
WHERE id = $1
RETURNING id::text, name, status, COALESCE(quota_limit, 0)`, strings.TrimSpace(req.ID), req.QuotaLimit).Scan(
		&view.ID,
		&view.Name,
		&view.Status,
		&view.QuotaLimit,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("company %q not found", req.ID)
		}
		return fmt.Errorf("update company quota: %w", err)
	}
	detail, err := companyQuotaAuditDetail(view)
	if err != nil {
		return err
	}
	if err := audit.InsertTx(ctx, tx, audit.Log{
		CompanyID:  view.ID,
		Category:   "admin",
		Action:     "company.quota_update",
		TargetType: "company",
		TargetID:   view.ID,
		Result:     "updated",
		Detail:     detail,
	}); err != nil {
		return fmt.Errorf("record company quota audit: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit company quota transaction: %w", err)
	}
	return nil
}

func (r *Repository) UpdateCompany(ctx context.Context, req UpdateCompanyRequest) (CompanyView, error) {
	if r.db == nil {
		return CompanyView{}, fmt.Errorf("database handle is required")
	}
	if strings.TrimSpace(req.ID) == "" {
		return CompanyView{}, fmt.Errorf("id is required")
	}
	if req.Name != "" && strings.TrimSpace(req.Name) == "" {
		return CompanyView{}, fmt.Errorf("name must not be blank")
	}
	if req.QuotaLimit < 0 {
		return CompanyView{}, fmt.Errorf("quota_limit must not be negative")
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return CompanyView{}, fmt.Errorf("begin update company transaction: %w", err)
	}
	defer tx.Rollback()

	var view CompanyView
	if err := tx.QueryRowContext(ctx, `
UPDATE companies
SET name        = CASE WHEN $2 <> '' THEN $2 ELSE name END,
    quota_limit = CASE WHEN $3::bigint >= 0 THEN NULLIF($3::bigint, 0) ELSE quota_limit END,
    updated_at  = now()
WHERE id = $1
RETURNING id::text, name, status, quota_used, COALESCE(quota_limit, 0), created_at`,
		strings.TrimSpace(req.ID), strings.TrimSpace(req.Name), req.QuotaLimit,
	).Scan(&view.ID, &view.Name, &view.Status, &view.QuotaUsed, &view.QuotaLimit, &view.CreatedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return CompanyView{}, fmt.Errorf("company %q not found", req.ID)
		}
		return CompanyView{}, fmt.Errorf("update company: %w", err)
	}
	if err := audit.InsertTx(ctx, tx, audit.Log{
		CompanyID:  view.ID,
		Category:   "admin",
		Action:     "company.update",
		TargetType: "company",
		TargetID:   view.ID,
		Result:     "updated",
	}); err != nil {
		return CompanyView{}, fmt.Errorf("record company update audit: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return CompanyView{}, fmt.Errorf("commit update company transaction: %w", err)
	}
	return view, nil
}

func (r *Repository) DeleteCompany(ctx context.Context, id string) error {
	if r.db == nil {
		return fmt.Errorf("database handle is required")
	}
	id = strings.TrimSpace(id)
	if id == "" {
		return fmt.Errorf("id is required")
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin delete company transaction: %w", err)
	}
	defer tx.Rollback()

	var domainCount int
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM domains WHERE company_id = $1`, id).Scan(&domainCount); err != nil {
		return fmt.Errorf("check company domains: %w", err)
	}
	if domainCount > 0 {
		return fmt.Errorf("cannot delete company with %d domain(s); remove all domains first", domainCount)
	}

	var name string
	if err := tx.QueryRowContext(ctx, `DELETE FROM companies WHERE id = $1 RETURNING name`, id).Scan(&name); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("company %q not found", id)
		}
		return fmt.Errorf("delete company: %w", err)
	}
	if err := audit.InsertTx(ctx, tx, audit.Log{
		CompanyID:  id,
		Category:   "admin",
		Action:     "company.delete",
		TargetType: "company",
		TargetID:   id,
		Result:     "deleted",
	}); err != nil {
		return fmt.Errorf("record company delete audit: %w", err)
	}
	return tx.Commit()
}
