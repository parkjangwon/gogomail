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
	"github.com/gogomail/gogomail/internal/dkim"
	"github.com/gogomail/gogomail/internal/dnscheck"
)

type DKIMKey struct {
	ID            string
	DomainID      string
	DomainName    string
	Selector      string
	PrivateKeyPEM string
	PublicKeyDNS  string
	Status        string
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

type DKIMKeyView struct {
	ID            string     `json:"id"`
	DomainID      string     `json:"domain_id"`
	Selector      string     `json:"selector"`
	PublicKeyDNS  string     `json:"public_key_dns"`
	Status        string     `json:"status"`
	DNSVerifiedAt *time.Time `json:"dns_verified_at,omitempty"`
	CreatedAt     time.Time  `json:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at"`
}

type DKIMKeyListRequest struct {
	DomainID string
	Status   string
	Limit    int
}

type CreateDKIMKeyInput struct {
	DomainID      string `json:"domain_id"`
	Selector      string `json:"selector"`
	PrivateKeyPEM string `json:"private_key_pem"`
	PublicKeyDNS  string `json:"public_key_dns"`
}

func (r *Repository) ActiveDKIMKey(ctx context.Context, domainID string) (DKIMKey, error) {
	if r.db == nil {
		return DKIMKey{}, fmt.Errorf("database handle is required")
	}

	const query = `
SELECT
  id::text,
  domain_id::text,
  COALESCE(NULLIF(d.name_ace, ''), d.name),
  selector,
  private_key_pem,
  public_key_dns,
  status,
  created_at,
  updated_at
FROM dkim_keys
JOIN domains d ON d.id = dkim_keys.domain_id
WHERE dkim_keys.domain_id::text = $1
  AND dkim_keys.status = 'active'
ORDER BY updated_at DESC
LIMIT 1`

	var key DKIMKey
	if err := r.db.QueryRowContext(ctx, query, strings.TrimSpace(domainID)).Scan(
		&key.ID,
		&key.DomainID,
		&key.DomainName,
		&key.Selector,
		&key.PrivateKeyPEM,
		&key.PublicKeyDNS,
		&key.Status,
		&key.CreatedAt,
		&key.UpdatedAt,
	); err != nil {
		if err == sql.ErrNoRows {
			return DKIMKey{}, fmt.Errorf("active dkim key for domain %q not found", domainID)
		}
		return DKIMKey{}, fmt.Errorf("lookup active dkim key: %w", err)
	}
	return key, nil
}

func (r *Repository) ListDKIMKeys(ctx context.Context, req DKIMKeyListRequest) ([]DKIMKeyView, error) {
	if r.db == nil {
		return nil, fmt.Errorf("database handle is required")
	}
	if err := ValidateDKIMKeyListRequest(req); err != nil {
		return nil, err
	}
	limit := normalizeLimit(req.Limit)
	status := normalizeDKIMKeyStatus(req.Status)

	const query = `
SELECT
  id::text,
  domain_id::text,
  selector,
  public_key_dns,
  status,
  dns_verified_at,
  created_at,
  updated_at
FROM dkim_keys
WHERE ($1 = '' OR domain_id::text = $1)
  AND ($2 = '' OR status = $2)
ORDER BY updated_at DESC
LIMIT $3`

	rows, err := r.db.QueryContext(ctx, query, strings.TrimSpace(req.DomainID), status, limit)
	if err != nil {
		return nil, fmt.Errorf("list dkim keys: %w", err)
	}
	defer rows.Close()

	var keys []DKIMKeyView
	for rows.Next() {
		var key DKIMKeyView
		if err := rows.Scan(
			&key.ID,
			&key.DomainID,
			&key.Selector,
			&key.PublicKeyDNS,
			&key.Status,
			&key.DNSVerifiedAt,
			&key.CreatedAt,
			&key.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan dkim key: %w", err)
		}
		keys = append(keys, key)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate dkim keys: %w", err)
	}
	return keys, nil
}

func ValidateDKIMKeyListRequest(req DKIMKeyListRequest) error {
	status := normalizeDKIMKeyStatus(req.Status)
	if status == "" || status == "active" || status == "inactive" {
		return nil
	}
	return fmt.Errorf("unsupported dkim key status %q", req.Status)
}

func normalizeDKIMKeyStatus(status string) string {
	return strings.ToLower(strings.TrimSpace(status))
}

func (r *Repository) CreateDKIMKey(ctx context.Context, input CreateDKIMKeyInput) (string, error) {
	if r.db == nil {
		return "", fmt.Errorf("database handle is required")
	}
	if strings.TrimSpace(input.DomainID) == "" {
		return "", fmt.Errorf("domain_id is required")
	}
	if strings.TrimSpace(input.Selector) == "" {
		return "", fmt.Errorf("selector is required")
	}
	if strings.TrimSpace(input.PrivateKeyPEM) == "" {
		return "", fmt.Errorf("private_key_pem is required")
	}
	publicKeyDNS := strings.TrimSpace(input.PublicKeyDNS)
	if publicKeyDNS == "" {
		derived, err := dkim.PublicKeyDNSFromPrivateKeyPEM(input.PrivateKeyPEM)
		if err != nil {
			return "", err
		}
		publicKeyDNS = derived
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return "", fmt.Errorf("begin dkim key create transaction: %w", err)
	}
	defer tx.Rollback()

	const query = `
INSERT INTO dkim_keys (domain_id, selector, private_key_pem, public_key_dns, status)
VALUES ($1, $2, $3, $4, 'active')
ON CONFLICT (domain_id, selector)
DO UPDATE SET
  private_key_pem = EXCLUDED.private_key_pem,
  public_key_dns = EXCLUDED.public_key_dns,
  status = 'active',
  updated_at = now()
RETURNING id::text`

	var id string
	if err := tx.QueryRowContext(
		ctx,
		query,
		strings.TrimSpace(input.DomainID),
		strings.TrimSpace(input.Selector),
		input.PrivateKeyPEM,
		publicKeyDNS,
	).Scan(&id); err != nil {
		return "", fmt.Errorf("create dkim key: %w", err)
	}
	detail, err := dkimKeyAuditDetail(dkimKeyAuditView{
		ID:                     id,
		DomainID:               strings.TrimSpace(input.DomainID),
		Selector:               strings.TrimSpace(input.Selector),
		Status:                 "active",
		PublicKeyDNSConfigured: publicKeyDNS != "",
	})
	if err != nil {
		return "", err
	}
	if err := audit.InsertTx(ctx, tx, audit.Log{
		DomainID:   strings.TrimSpace(input.DomainID),
		Category:   "admin",
		Action:     "dkim_key.create",
		TargetType: "dkim_key",
		TargetID:   id,
		Result:     "active",
		Detail:     detail,
	}); err != nil {
		return "", fmt.Errorf("record dkim key create audit: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return "", fmt.Errorf("commit dkim key create transaction: %w", err)
	}
	return id, nil
}

// DKIMKeyDNSVerificationResult carries the outcome of a targeted DNS TXT
// record check for a single DKIM key.
type DKIMKeyDNSVerificationResult struct {
	KeyID      string               `json:"key_id"`
	DomainID   string               `json:"domain_id"`
	Selector   string               `json:"selector"`
	Check      dnscheck.RecordCheck `json:"check"`
	VerifiedAt *time.Time           `json:"verified_at,omitempty"`
}

// VerifyDKIMKeyDNS looks up the DKIM key by id, queries DNS for its expected
// TXT record, persists the result to domain_dns_checks, and when the record
// matches marks the key with dns_verified_at.
func (r *Repository) VerifyDKIMKeyDNS(ctx context.Context, keyID string) (DKIMKeyDNSVerificationResult, error) {
	if r.db == nil {
		return DKIMKeyDNSVerificationResult{}, fmt.Errorf("database handle is required")
	}
	keyID = strings.TrimSpace(keyID)
	if keyID == "" {
		return DKIMKeyDNSVerificationResult{}, fmt.Errorf("key id is required")
	}

	// Load the key with its domain info.
	const keyQuery = `
SELECT
  dk.id::text,
  dk.domain_id::text,
  dk.selector,
  dk.public_key_dns,
  d.company_id::text,
  COALESCE(NULLIF(d.name_ace, ''), d.name),
  d.id::text
FROM dkim_keys dk
JOIN domains d ON d.id = dk.domain_id
WHERE dk.id = $1`

	var (
		id, domainID, selector, publicKeyDNS string
		companyID, domainName, domainDBID    string
	)
	if err := r.db.QueryRowContext(ctx, keyQuery, keyID).Scan(
		&id, &domainID, &selector, &publicKeyDNS,
		&companyID, &domainName, &domainDBID,
	); err != nil {
		if err == sql.ErrNoRows {
			return DKIMKeyDNSVerificationResult{}, fmt.Errorf("dkim key %q not found", keyID)
		}
		return DKIMKeyDNSVerificationResult{}, fmt.Errorf("lookup dkim key: %w", err)
	}

	// Run the DNS check for just this key.
	check := dnscheck.Verifier{}.VerifyDKIMRecord(ctx, domainName, dnscheck.DKIMExpectation{
		Selector:     selector,
		PublicKeyDNS: publicKeyDNS,
	})

	result := DKIMKeyDNSVerificationResult{
		KeyID:    id,
		DomainID: domainID,
		Selector: selector,
		Check:    check,
	}

	// Persist a targeted dns check for this key.
	summaryStatus := string(check.Status)
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return DKIMKeyDNSVerificationResult{}, fmt.Errorf("begin dkim dns verification transaction: %w", err)
	}
	defer tx.Rollback()

	var dnsCheckID string
	if err := tx.QueryRowContext(ctx, `
INSERT INTO domain_dns_checks (domain_id, status, report)
VALUES ($1, $2, $3::jsonb)
RETURNING id::text`,
		domainDBID, summaryStatus,
		fmt.Sprintf(`{"domain":%q,"dkim_key_id":%q,"selector":%q,"check":%q}`,
			domainName, id, selector, summaryStatus),
	).Scan(&dnsCheckID); err != nil {
		return DKIMKeyDNSVerificationResult{}, fmt.Errorf("record dkim dns check: %w", err)
	}

	// On success mark the key as DNS-verified.
	if check.Status == dnscheck.StatusOK {
		now := time.Now().UTC()
		if _, err := tx.ExecContext(ctx, `
UPDATE dkim_keys
SET dns_verified_at = $2, updated_at = now()
WHERE id = $1`, id, now); err != nil {
			return DKIMKeyDNSVerificationResult{}, fmt.Errorf("mark dkim key dns verified: %w", err)
		}
		result.VerifiedAt = &now
	}
	detail, err := dkimKeyAuditDetail(dkimKeyAuditView{
		ID:                     id,
		DomainID:               domainID,
		Selector:               selector,
		Status:                 "active",
		PublicKeyDNSConfigured: publicKeyDNS != "",
		DNSCheckID:             dnsCheckID,
		DNSStatus:              summaryStatus,
		DNSVerified:            result.VerifiedAt != nil,
	})
	if err != nil {
		return DKIMKeyDNSVerificationResult{}, err
	}
	if err := audit.InsertTx(ctx, tx, audit.Log{
		CompanyID:  companyID,
		DomainID:   domainID,
		Category:   "admin",
		Action:     "dkim_key.verify_dns",
		TargetType: "dkim_key",
		TargetID:   id,
		Result:     summaryStatus,
		Detail:     detail,
	}); err != nil {
		return DKIMKeyDNSVerificationResult{}, fmt.Errorf("record dkim dns verification audit: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return DKIMKeyDNSVerificationResult{}, fmt.Errorf("commit dkim dns verification transaction: %w", err)
	}
	return result, nil
}

func (r *Repository) DeactivateDKIMKey(ctx context.Context, id string) error {
	if r.db == nil {
		return fmt.Errorf("database handle is required")
	}
	id = strings.TrimSpace(id)
	if id == "" {
		return fmt.Errorf("dkim key id is required")
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin dkim key deactivate transaction: %w", err)
	}
	defer tx.Rollback()

	const query = `
UPDATE dkim_keys
SET status = 'inactive',
    updated_at = now()
WHERE id = $1
RETURNING id::text, domain_id::text, selector, status, dns_verified_at IS NOT NULL`

	var view dkimKeyAuditView
	if err := tx.QueryRowContext(ctx, query, id).Scan(
		&view.ID,
		&view.DomainID,
		&view.Selector,
		&view.Status,
		&view.DNSVerified,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("dkim key %q not found", id)
		}
		return fmt.Errorf("deactivate dkim key: %w", err)
	}
	detail, err := dkimKeyAuditDetail(view)
	if err != nil {
		return err
	}
	if err := audit.InsertTx(ctx, tx, audit.Log{
		DomainID:   view.DomainID,
		Category:   "admin",
		Action:     "dkim_key.deactivate",
		TargetType: "dkim_key",
		TargetID:   view.ID,
		Result:     view.Status,
		Detail:     detail,
	}); err != nil {
		return fmt.Errorf("record dkim key deactivate audit: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit dkim key deactivate transaction: %w", err)
	}
	return nil
}

type dkimKeyAuditView struct {
	ID                     string
	DomainID               string
	Selector               string
	Status                 string
	PublicKeyDNSConfigured bool
	DNSCheckID             string
	DNSStatus              string
	DNSVerified            bool
}

func dkimKeyAuditDetail(view dkimKeyAuditView) (json.RawMessage, error) {
	detail, err := json.Marshal(map[string]any{
		"dkim_key_id":               view.ID,
		"domain_id":                 view.DomainID,
		"selector":                  view.Selector,
		"status":                    view.Status,
		"public_key_dns_configured": view.PublicKeyDNSConfigured,
		"dns_check_id":              view.DNSCheckID,
		"dns_status":                view.DNSStatus,
		"dns_verified":              view.DNSVerified,
	})
	if err != nil {
		return nil, fmt.Errorf("marshal dkim key audit detail: %w", err)
	}
	return detail, nil
}
