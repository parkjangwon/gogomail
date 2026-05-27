package maildb

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/gogomail/gogomail/internal/audit"
	"github.com/gogomail/gogomail/internal/dnscheck"
)

func (r *Repository) CreateDomain(ctx context.Context, req CreateDomainRequest) (DomainView, error) {
	if r.db == nil {
		return DomainView{}, fmt.Errorf("database handle is required")
	}
	if err := ValidateCreateDomainRequest(req); err != nil {
		return DomainView{}, err
	}
	name := strings.ToLower(strings.TrimSpace(req.Name))
	nameACE := strings.ToLower(strings.TrimSpace(req.NameACE))
	if nameACE == "" {
		nameACE = name
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return DomainView{}, fmt.Errorf("begin create domain transaction: %w", err)
	}
	defer tx.Rollback()

	const query = `
INSERT INTO domains (company_id, name, name_ace, quota_limit)
VALUES ($1, $2, $3, NULLIF($4::bigint, 0))
RETURNING id::text, company_id::text, name, name_ace, status, quota_used, COALESCE(quota_limit, 0), created_at`

	var domain DomainView
	if err := tx.QueryRowContext(ctx, query, strings.TrimSpace(req.CompanyID), name, nameACE, req.QuotaLimit).Scan(
		&domain.ID,
		&domain.CompanyID,
		&domain.Name,
		&domain.NameACE,
		&domain.Status,
		&domain.QuotaUsed,
		&domain.QuotaLimit,
		&domain.CreatedAt,
	); err != nil {
		return DomainView{}, fmt.Errorf("create domain: %w", err)
	}
	detail, err := domainCreateAuditDetail(domain)
	if err != nil {
		return DomainView{}, err
	}
	if err := audit.InsertTx(ctx, tx, audit.Log{
		CompanyID:  domain.CompanyID,
		DomainID:   domain.ID,
		Category:   "admin",
		Action:     "domain.create",
		TargetType: "domain",
		TargetID:   domain.ID,
		Result:     "created",
		Detail:     detail,
	}); err != nil {
		return DomainView{}, fmt.Errorf("record domain create audit: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return DomainView{}, fmt.Errorf("commit create domain transaction: %w", err)
	}
	return domain, nil
}
func (r *Repository) UpdateDomainStatus(ctx context.Context, req UpdateDomainStatusRequest) error {
	if r.db == nil {
		return fmt.Errorf("database handle is required")
	}
	if err := ValidateUpdateDomainStatusRequest(req); err != nil {
		return err
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin domain status transaction: %w", err)
	}
	defer tx.Rollback()

	var view domainStatusAuditView
	if err := tx.QueryRowContext(ctx, `
UPDATE domains
SET status = $2,
    updated_at = now()
WHERE id = $1
RETURNING id::text, company_id::text, name, status`, strings.TrimSpace(req.ID), normalizeAdminStatus(req.Status)).Scan(
		&view.ID,
		&view.CompanyID,
		&view.Name,
		&view.Status,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("domain %q not found", req.ID)
		}
		return fmt.Errorf("update domain status: %w", err)
	}
	detail, err := domainStatusAuditDetail(view)
	if err != nil {
		return err
	}
	if err := audit.InsertTx(ctx, tx, audit.Log{
		CompanyID:  view.CompanyID,
		DomainID:   view.ID,
		Category:   "admin",
		Action:     "domain.status_update",
		TargetType: "domain",
		TargetID:   view.ID,
		Result:     view.Status,
		Detail:     detail,
	}); err != nil {
		return fmt.Errorf("record domain status audit: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit domain status transaction: %w", err)
	}
	return nil
}

func (r *Repository) UpdateDomainQuota(ctx context.Context, req UpdateDomainQuotaRequest) error {
	if r.db == nil {
		return fmt.Errorf("database handle is required")
	}
	if err := ValidateUpdateDomainQuotaRequest(req); err != nil {
		return err
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin update domain quota transaction: %w", err)
	}
	defer tx.Rollback()

	var view domainQuotaAuditView
	if err := tx.QueryRowContext(ctx, `
UPDATE domains
SET quota_limit = NULLIF($2::bigint, 0),
    updated_at = now()
WHERE id = $1
RETURNING id::text, company_id::text, name, COALESCE(quota_limit, 0)`, strings.TrimSpace(req.ID), req.QuotaLimit).Scan(
		&view.ID,
		&view.CompanyID,
		&view.Name,
		&view.QuotaLimit,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("domain %q not found", req.ID)
		}
		return fmt.Errorf("update domain quota: %w", err)
	}
	if req.DefaultUserQuota != nil {
		defaultQuota := *req.DefaultUserQuota
		view.DefaultUserQuotaSet = true
		view.DefaultUserQuota = defaultQuota
		if _, err := tx.ExecContext(ctx, `
UPDATE domains
SET settings = jsonb_set(settings, '{policy,default_user_quota}', to_jsonb($2::bigint), true),
    updated_at = now()
WHERE id = $1`, strings.TrimSpace(req.ID), defaultQuota); err != nil {
			return fmt.Errorf("update domain default user quota: %w", err)
		}
		result, err := tx.ExecContext(ctx, `
UPDATE users
SET quota_limit = NULLIF($2::bigint, 0),
    updated_at = now()
WHERE domain_id = $1
  AND quota_source = 'default'`, strings.TrimSpace(req.ID), defaultQuota)
		if err != nil {
			return fmt.Errorf("apply domain default user quota: %w", err)
		}
		if affected, err := result.RowsAffected(); err == nil {
			view.DefaultUserQuotaUserUpdates = affected
		}
	}
	detail, err := domainQuotaAuditDetail(view)
	if err != nil {
		return err
	}
	if err := audit.InsertTx(ctx, tx, audit.Log{
		CompanyID:  view.CompanyID,
		DomainID:   view.ID,
		Category:   "admin",
		Action:     "domain.quota_update",
		TargetType: "domain",
		TargetID:   view.ID,
		Result:     "updated",
		Detail:     detail,
	}); err != nil {
		return fmt.Errorf("record domain quota audit: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit update domain quota transaction: %w", err)
	}
	return nil
}
func (r *Repository) UpdateDomainPolicy(ctx context.Context, req UpdateDomainPolicyRequest) (DomainPolicyView, error) {
	if r.db == nil {
		return DomainPolicyView{}, fmt.Errorf("database handle is required")
	}
	if err := ValidateUpdateDomainPolicyRequest(req); err != nil {
		return DomainPolicyView{}, err
	}
	inboundMode, _ := normalizeDomainPolicyMode(req.InboundMode)
	outboundMode, _ := normalizeDomainPolicyMode(req.OutboundMode)
	policy := DomainPolicyView{
		DomainID:                strings.TrimSpace(req.ID),
		InboundMode:             inboundMode,
		OutboundMode:            outboundMode,
		MaxRecipientsPerMessage: req.MaxRecipientsPerMessage,
		MaxMessageBytes:         req.MaxMessageBytes,
		MaxAttachmentBytes:      req.MaxAttachmentBytes,
	}
	policyJSON, err := json.Marshal(policy)
	if err != nil {
		return DomainPolicyView{}, fmt.Errorf("marshal domain policy: %w", err)
	}

	const query = `
UPDATE domains
SET settings = jsonb_set(settings, '{policy}', COALESCE(settings->'policy', '{}'::jsonb) || $2::jsonb, true),
    updated_at = now()
WHERE id = $1
RETURNING id::text, company_id::text, name, updated_at`

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return DomainPolicyView{}, fmt.Errorf("begin domain policy transaction: %w", err)
	}
	defer tx.Rollback()

	var auditView domainPolicyAuditView
	if err := tx.QueryRowContext(ctx, query, policy.DomainID, policyJSON).Scan(
		&auditView.ID,
		&auditView.CompanyID,
		&auditView.Name,
		&policy.UpdatedAt,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return DomainPolicyView{}, fmt.Errorf("domain %q not found", req.ID)
		}
		return DomainPolicyView{}, fmt.Errorf("update domain policy: %w", err)
	}
	auditView.Policy = policy
	detail, err := domainPolicyAuditDetail(auditView)
	if err != nil {
		return DomainPolicyView{}, err
	}
	if err := audit.InsertTx(ctx, tx, audit.Log{
		CompanyID:  auditView.CompanyID,
		DomainID:   auditView.ID,
		Category:   "admin",
		Action:     "domain.policy_update",
		TargetType: "domain",
		TargetID:   auditView.ID,
		Result:     "updated",
		Detail:     detail,
	}); err != nil {
		return DomainPolicyView{}, fmt.Errorf("record domain policy audit: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return DomainPolicyView{}, fmt.Errorf("commit domain policy transaction: %w", err)
	}
	return policy, nil
}
func normalizeAdminStatus(status string) string {
	return strings.ToLower(strings.TrimSpace(status))
}

type domainStatusAuditView struct {
	ID        string
	CompanyID string
	Name      string
	Status    string
}

func domainStatusAuditDetail(view domainStatusAuditView) (json.RawMessage, error) {
	detail, err := json.Marshal(map[string]any{
		"domain_id":  view.ID,
		"company_id": view.CompanyID,
		"name":       view.Name,
		"status":     view.Status,
	})
	if err != nil {
		return nil, fmt.Errorf("marshal domain status audit detail: %w", err)
	}
	return detail, nil
}

func domainCreateAuditDetail(domain DomainView) (json.RawMessage, error) {
	detail, err := json.Marshal(map[string]any{
		"domain_id":   domain.ID,
		"company_id":  domain.CompanyID,
		"name":        domain.Name,
		"name_ace":    domain.NameACE,
		"status":      domain.Status,
		"quota_limit": domain.QuotaLimit,
	})
	if err != nil {
		return nil, fmt.Errorf("marshal domain create audit detail: %w", err)
	}
	return detail, nil
}
type companyQuotaAuditView struct {
	ID         string
	Name       string
	Status     string
	QuotaLimit int64
}

func companyQuotaAuditDetail(view companyQuotaAuditView) (json.RawMessage, error) {
	detail, err := json.Marshal(map[string]any{
		"company_id":  view.ID,
		"name":        view.Name,
		"status":      view.Status,
		"quota_limit": view.QuotaLimit,
	})
	if err != nil {
		return nil, fmt.Errorf("marshal company quota audit detail: %w", err)
	}
	return detail, nil
}

type domainQuotaAuditView struct {
	ID                          string
	CompanyID                   string
	Name                        string
	QuotaLimit                  int64
	DefaultUserQuotaSet         bool
	DefaultUserQuota            int64
	DefaultUserQuotaUserUpdates int64
}

func domainQuotaAuditDetail(view domainQuotaAuditView) (json.RawMessage, error) {
	detail, err := json.Marshal(map[string]any{
		"domain_id":                       view.ID,
		"company_id":                      view.CompanyID,
		"name":                            view.Name,
		"quota_limit":                     view.QuotaLimit,
		"default_user_quota_set":          view.DefaultUserQuotaSet,
		"default_user_quota":              view.DefaultUserQuota,
		"default_user_quota_user_updates": view.DefaultUserQuotaUserUpdates,
	})
	if err != nil {
		return nil, fmt.Errorf("marshal domain quota audit detail: %w", err)
	}
	return detail, nil
}
type domainPolicyAuditView struct {
	ID        string
	CompanyID string
	Name      string
	Policy    DomainPolicyView
}

func domainPolicyAuditDetail(view domainPolicyAuditView) (json.RawMessage, error) {
	detail, err := json.Marshal(map[string]any{
		"domain_id":                  view.ID,
		"company_id":                 view.CompanyID,
		"name":                       view.Name,
		"inbound_mode":               view.Policy.InboundMode,
		"outbound_mode":              view.Policy.OutboundMode,
		"max_recipients_per_message": view.Policy.MaxRecipientsPerMessage,
		"max_message_bytes":          view.Policy.MaxMessageBytes,
		"max_attachment_bytes":       view.Policy.MaxAttachmentBytes,
	})
	if err != nil {
		return nil, fmt.Errorf("marshal domain policy audit detail: %w", err)
	}
	return detail, nil
}
func (r *Repository) ListDomains(ctx context.Context, req DomainListRequest) ([]DomainView, bool, error) {
	if r.db == nil {
		return nil, false, fmt.Errorf("database handle is required")
	}
	if err := ValidateDomainListRequest(req); err != nil {
		return nil, false, err
	}
	limit := normalizeLimit(req.Limit)
	status := normalizeAdminStatus(req.Status)
	dnsStatus := normalizeDNSStatus(req.DNSStatus)
	queryLimit := limit
	if req.ProbeMore {
		queryLimit = limit + 1
	}

	query, args := buildListDomainsQuery(req, status, dnsStatus, queryLimit)
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, false, fmt.Errorf("list domains: %w", err)
	}
	defer rows.Close()

	var domains []DomainView
	for rows.Next() {
		var domain DomainView
		var lastDNSCheckedAt sql.NullTime
		if err := rows.Scan(
			&domain.ID,
			&domain.CompanyID,
			&domain.CompanyName,
			&domain.Name,
			&domain.NameACE,
			&domain.Status,
			&domain.QuotaUsed,
			&domain.QuotaLimit,
			&domain.DefaultUserQuota,
			&domain.AllocatedUserQuota,
			&domain.LastDNSCheckStatus,
			&lastDNSCheckedAt,
			&domain.CreatedAt,
		); err != nil {
			return nil, false, fmt.Errorf("scan domain: %w", err)
		}
		domain.QuotaRemaining = quotaRemaining(domain.QuotaUsed, domain.QuotaLimit)
		domain.AllocatableUserQuota = quotaRemaining(domain.AllocatedUserQuota, domain.QuotaLimit)
		domain.OverAllocated = domain.QuotaLimit > 0 && domain.AllocatedUserQuota > domain.QuotaLimit
		if lastDNSCheckedAt.Valid {
			domain.LastDNSCheckedAt = &lastDNSCheckedAt.Time
		}
		domains = append(domains, domain)
	}
	if err := rows.Err(); err != nil {
		return nil, false, fmt.Errorf("iterate domains: %w", err)
	}
	hasMore := req.ProbeMore && len(domains) > limit
	if hasMore {
		domains = domains[:limit]
	}
	return domains, hasMore, nil
}

func buildListDomainsQuery(req DomainListRequest, status string, dnsStatus string, queryLimit int) (string, []any) {
	args := make([]any, 0, 4)
	where := make([]string, 0, 3)
	if companyID := strings.TrimSpace(req.CompanyID); companyID != "" {
		args = append(args, companyID)
		where = append(where, fmt.Sprintf("d.company_id = $%d::uuid", len(args)))
	}
	if status != "" {
		args = append(args, status)
		where = append(where, fmt.Sprintf("d.status = $%d", len(args)))
	}
	if dnsStatus != "" {
		args = append(args, dnsStatus)
		where = append(where, fmt.Sprintf("COALESCE(latest.status, '') = $%d", len(args)))
	}
	args = append(args, queryLimit)

	var builder strings.Builder
	builder.WriteString(`
SELECT
  d.id::text,
  d.company_id::text,
  COALESCE(c.name, ''),
  d.name,
  d.name_ace,
  d.status,
  d.quota_used,
  COALESCE(d.quota_limit, 0),
  COALESCE((d.settings #>> '{policy,default_user_quota}')::bigint, 0),
  COALESCE((
    SELECT SUM(child.quota_limit)
    FROM users child
    WHERE child.domain_id = d.id
      AND child.quota_limit IS NOT NULL
      AND child.quota_limit > 0
  ), 0) AS allocated_user_quota,
  COALESCE(latest.status, ''),
  latest.checked_at,
  d.created_at
FROM domains d
LEFT JOIN companies c ON c.id = d.company_id
LEFT JOIN LATERAL (
  SELECT status, checked_at
  FROM domain_dns_checks
  WHERE domain_id = d.id
  ORDER BY checked_at DESC, id DESC
  LIMIT 1
) latest ON true
`)
	if len(where) > 0 {
		builder.WriteString("WHERE ")
		builder.WriteString(strings.Join(where, "\n  AND "))
		builder.WriteByte('\n')
	}
	builder.WriteString(fmt.Sprintf("ORDER BY d.created_at DESC, d.id DESC\nLIMIT $%d", len(args)))
	return builder.String(), args
}

func (r *Repository) GetDomain(ctx context.Context, id string) (DomainView, error) {
	if r.db == nil {
		return DomainView{}, fmt.Errorf("database handle is required")
	}
	id = strings.TrimSpace(id)
	if id == "" {
		return DomainView{}, fmt.Errorf("domain id is required")
	}

	const query = `
SELECT
  d.id::text,
  d.company_id::text,
  d.name,
  d.name_ace,
  d.status,
  d.quota_used,
  COALESCE(d.quota_limit, 0),
  COALESCE((d.settings #>> '{policy,default_user_quota}')::bigint, 0),
  COALESCE((
    SELECT SUM(child.quota_limit)
    FROM users child
    WHERE child.domain_id = d.id
      AND child.quota_limit IS NOT NULL
      AND child.quota_limit > 0
  ), 0) AS allocated_user_quota,
  COALESCE(latest.status, ''),
  latest.checked_at,
  d.created_at
FROM domains d
LEFT JOIN LATERAL (
  SELECT status, checked_at
  FROM domain_dns_checks
  WHERE domain_id = d.id
  ORDER BY checked_at DESC, id DESC
  LIMIT 1
) latest ON true
WHERE d.id = $1
LIMIT 1`

	var domain DomainView
	var lastDNSCheckedAt sql.NullTime
	if err := r.db.QueryRowContext(ctx, query, id).Scan(
		&domain.ID,
		&domain.CompanyID,
		&domain.Name,
		&domain.NameACE,
		&domain.Status,
		&domain.QuotaUsed,
		&domain.QuotaLimit,
		&domain.DefaultUserQuota,
		&domain.AllocatedUserQuota,
		&domain.LastDNSCheckStatus,
		&lastDNSCheckedAt,
		&domain.CreatedAt,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return DomainView{}, fmt.Errorf("domain %q not found", id)
		}
		return DomainView{}, fmt.Errorf("get domain: %w", err)
	}
	if lastDNSCheckedAt.Valid {
		domain.LastDNSCheckedAt = &lastDNSCheckedAt.Time
	}
	domain.QuotaRemaining = quotaRemaining(domain.QuotaUsed, domain.QuotaLimit)
	domain.AllocatableUserQuota = quotaRemaining(domain.AllocatedUserQuota, domain.QuotaLimit)
	domain.OverAllocated = domain.QuotaLimit > 0 && domain.AllocatedUserQuota > domain.QuotaLimit
	return domain, nil
}

func (r *Repository) GetDomainStats(ctx context.Context, id string) (DomainStatsView, error) {
	if r.db == nil {
		return DomainStatsView{}, fmt.Errorf("database handle is required")
	}
	id = strings.TrimSpace(id)
	if id == "" {
		return DomainStatsView{}, fmt.Errorf("domain id is required")
	}

	const query = `
SELECT
  d.id::text,
  (SELECT COUNT(*) FROM users WHERE domain_id = d.id AND status = 'active'),
  (SELECT COUNT(*) FROM users WHERE domain_id = d.id),
  (SELECT COUNT(*) FROM messages WHERE domain_id = d.id AND status = 'active'),
  (SELECT COUNT(*) FROM messages WHERE domain_id = d.id AND received_at IS NOT NULL AND sent_at IS NULL AND status = 'active'),
  (SELECT COUNT(*) FROM messages WHERE domain_id = d.id AND sent_at IS NOT NULL AND status = 'active'),
  d.quota_used,
  COALESCE(d.quota_limit, 0),
  (SELECT COUNT(*) FROM delivery_attempts da
   JOIN messages m ON m.id = da.message_id
   WHERE m.domain_id = d.id AND da.attempted_at > now() - INTERVAL '24 hours'
     AND da.status = 'delivered'),
  (SELECT COUNT(*) FROM delivery_attempts da
   JOIN messages m ON m.id = da.message_id
   WHERE m.domain_id = d.id AND da.attempted_at > now() - INTERVAL '24 hours'
     AND da.status IN ('failed', 'bounced', 'exhausted')),
  (SELECT COUNT(*) FROM suppression_list WHERE domain_id = d.id)
FROM domains d
WHERE d.id = $1
LIMIT 1`

	var stats DomainStatsView
	if err := r.db.QueryRowContext(ctx, query, id).Scan(
		&stats.DomainID,
		&stats.ActiveUsers,
		&stats.TotalUsers,
		&stats.ActiveMessages,
		&stats.InboundMessages,
		&stats.OutboundMessages,
		&stats.StorageUsedBytes,
		&stats.StorageLimitBytes,
		&stats.Delivered24h,
		&stats.Failed24h,
		&stats.SuppressionCount,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return DomainStatsView{}, fmt.Errorf("domain %q not found", id)
		}
		return DomainStatsView{}, fmt.Errorf("get domain stats: %w", err)
	}
	return stats, nil
}

func ValidateDomainDNSCheckListRequest(req DomainDNSCheckListRequest) error {
	domainID := strings.TrimSpace(req.DomainID)
	if domainID == "" {
		return fmt.Errorf("domain id is required")
	}
	if err := validatePushNotificationFilter("domain_id", domainID); err != nil {
		return err
	}
	status := strings.ToLower(strings.TrimSpace(req.Status))
	if status != "" && !isDomainDNSCheckStatus(status) {
		return fmt.Errorf("unsupported domain dns check status %q", req.Status)
	}
	return nil
}

func isDomainDNSCheckStatus(status string) bool {
	switch dnscheck.Status(status) {
	case dnscheck.StatusOK, dnscheck.StatusMissing, dnscheck.StatusMismatch, dnscheck.StatusError:
		return true
	default:
		return false
	}
}

func (r *Repository) ListDomainDNSChecks(ctx context.Context, req DomainDNSCheckListRequest) ([]DomainDNSCheckView, error) {
	if r.db == nil {
		return nil, fmt.Errorf("database handle is required")
	}
	if err := ValidateDomainDNSCheckListRequest(req); err != nil {
		return nil, err
	}
	domainID := strings.TrimSpace(req.DomainID)
	limit := normalizeLimit(req.Limit)
	status := strings.ToLower(strings.TrimSpace(req.Status))

	query, args := buildListDomainDNSChecksQuery(domainID, status, req.Since, limit)
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list domain dns checks: %w", err)
	}
	defer rows.Close()

	var checks []DomainDNSCheckView
	for rows.Next() {
		var check DomainDNSCheckView
		var rawReport []byte
		if err := rows.Scan(
			&check.ID,
			&check.DomainID,
			&check.Status,
			&rawReport,
			&check.CheckedAt,
		); err != nil {
			return nil, fmt.Errorf("scan domain dns check: %w", err)
		}
		if err := json.Unmarshal(rawReport, &check.Report); err != nil {
			return nil, fmt.Errorf("decode domain dns check report: %w", err)
		}
		checks = append(checks, check)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate domain dns checks: %w", err)
	}
	return checks, nil
}

func buildListDomainDNSChecksQuery(domainID string, status string, since time.Time, limit int) (string, []any) {
	query := listDomainDNSChecksBaseSQL
	args := []any{domainID}

	if status != "" {
		args = append(args, status)
		query += fmt.Sprintf("\n  AND status = $%d", len(args))
	}
	if !since.IsZero() {
		args = append(args, since.UTC())
		query += fmt.Sprintf("\n  AND checked_at >= $%d", len(args))
	}
	args = append(args, limit)
	query += fmt.Sprintf(`
ORDER BY checked_at DESC, id DESC
LIMIT $%d`, len(args))
	return query, args
}

func (r *Repository) VerifyDomainDNS(ctx context.Context, id string) (dnscheck.DomainReport, error) {
	if r.db == nil {
		return dnscheck.DomainReport{}, fmt.Errorf("database handle is required")
	}
	domain, err := r.GetDomain(ctx, id)
	if err != nil {
		return dnscheck.DomainReport{}, err
	}
	keys, err := r.ListDKIMKeys(ctx, DKIMKeyListRequest{DomainID: id, Limit: 200})
	if err != nil {
		return dnscheck.DomainReport{}, err
	}
	expectations := make([]dnscheck.DKIMExpectation, 0, len(keys))
	for _, key := range keys {
		if normalizeAdminStatus(key.Status) != "active" {
			continue
		}
		expectations = append(expectations, dnscheck.DKIMExpectation{
			Selector:     key.Selector,
			PublicKeyDNS: key.PublicKeyDNS,
		})
	}
	name := strings.TrimSpace(domain.NameACE)
	if name == "" {
		name = domain.Name
	}
	report := dnscheck.Verifier{}.VerifyDomain(ctx, name, expectations)
	if err := r.recordDomainDNSCheck(ctx, domain, report); err != nil {
		return dnscheck.DomainReport{}, err
	}
	return report, nil
}

func (r *Repository) recordDomainDNSCheck(ctx context.Context, domain DomainView, report dnscheck.DomainReport) error {
	reportJSON, err := json.Marshal(report)
	if err != nil {
		return fmt.Errorf("marshal domain dns check report: %w", err)
	}
	status := string(report.SummaryStatus())

	var checkID string
	if err := r.db.QueryRowContext(ctx, `
INSERT INTO domain_dns_checks (domain_id, status, report)
VALUES ($1, $2, $3)
RETURNING id::text`, domain.ID, status, reportJSON).Scan(&checkID); err != nil {
		return fmt.Errorf("record domain dns check: %w", err)
	}

	detailJSON, err := json.Marshal(map[string]any{
		"dns_check_id": checkID,
		"domain":       report.Domain,
		"status":       status,
	})
	if err != nil {
		return fmt.Errorf("marshal domain dns check audit detail: %w", err)
	}
	if err := audit.NewPostgresRepository(r.db).Insert(ctx, audit.Log{
		CompanyID:  domain.CompanyID,
		DomainID:   domain.ID,
		Category:   "admin",
		Action:     "domain.dns_check",
		TargetType: "domain",
		TargetID:   domain.ID,
		Result:     status,
		Detail:     detailJSON,
	}); err != nil {
		return fmt.Errorf("record domain dns check audit: %w", err)
	}
	return nil
}

func (r *Repository) DeleteDomain(ctx context.Context, id string) error {
	if r.db == nil {
		return fmt.Errorf("database handle is required")
	}
	id = strings.TrimSpace(id)
	if id == "" {
		return fmt.Errorf("id is required")
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin delete domain transaction: %w", err)
	}
	defer tx.Rollback()

	var userCount int
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM users WHERE domain_id = $1`, id).Scan(&userCount); err != nil {
		return fmt.Errorf("check domain users: %w", err)
	}
	if userCount > 0 {
		return fmt.Errorf("cannot delete domain with %d user(s); remove all users first", userCount)
	}

	var companyID, name string
	if err := tx.QueryRowContext(ctx, `DELETE FROM domains WHERE id = $1 RETURNING company_id::text, name`, id).Scan(&companyID, &name); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("domain %q not found", id)
		}
		return fmt.Errorf("delete domain: %w", err)
	}
	if err := audit.InsertTx(ctx, tx, audit.Log{
		CompanyID:  companyID,
		DomainID:   id,
		Category:   "admin",
		Action:     "domain.delete",
		TargetType: "domain",
		TargetID:   id,
		Result:     "deleted",
	}); err != nil {
		return fmt.Errorf("record domain delete audit: %w", err)
	}
	return tx.Commit()
}
