package maildb

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/gogomail/gogomail/internal/mail"
	"github.com/lib/pq"
)

// checkAndIncrementUserQuota checks whether the company -> domain -> user quota
// hierarchy has room for size bytes and atomically increments every ledger when
// the limits permit it. It must be called inside an open transaction; the
// row-level locks prevent concurrent over-quota writes. Returns
// mail.ErrMailboxFull when any tier would be exceeded.
func checkAndIncrementUserQuota(ctx context.Context, tx *sql.Tx, userID string, size int64) error {
	if size <= 0 {
		return nil
	}

	const selectQ = `
SELECT
  u.quota_used,
  COALESCE(u.quota_limit, 0),
  d.id::text,
  d.quota_used,
  COALESCE(d.quota_limit, 0),
  c.id::text,
  c.quota_used,
  COALESCE(c.quota_limit, 0)
FROM users u
JOIN domains d ON d.id = u.domain_id
JOIN companies c ON c.id = d.company_id
WHERE u.id = $1
FOR UPDATE OF u, d, c`

	var userUsed, userLimit int64
	var domainID string
	var domainUsed, domainLimit int64
	var companyID string
	var companyUsed, companyLimit int64
	if err := tx.QueryRowContext(ctx, selectQ, userID).Scan(
		&userUsed,
		&userLimit,
		&domainID,
		&domainUsed,
		&domainLimit,
		&companyID,
		&companyUsed,
		&companyLimit,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("user %q not found for quota check", userID)
		}
		return fmt.Errorf("read quota ledger: %w", err)
	}
	if userLimit > 0 && userUsed+size > userLimit {
		return fmt.Errorf("%w: user used %d, limit %d, write %d bytes", mail.ErrMailboxFull, userUsed, userLimit, size)
	}
	if domainLimit > 0 && domainUsed+size > domainLimit {
		return fmt.Errorf("%w: domain used %d, limit %d, write %d bytes", mail.ErrMailboxFull, domainUsed, domainLimit, size)
	}
	if companyLimit > 0 && companyUsed+size > companyLimit {
		return fmt.Errorf("%w: company used %d, limit %d, write %d bytes", mail.ErrMailboxFull, companyUsed, companyLimit, size)
	}

	const updateUserQ = `
UPDATE users
SET quota_used = quota_used + $2,
    updated_at = now()
WHERE id = $1`

	if _, err := tx.ExecContext(ctx, updateUserQ, userID, size); err != nil {
		return fmt.Errorf("increment user quota: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `
UPDATE domains
SET quota_used = quota_used + $2,
    updated_at = now()
WHERE id = $1`, domainID, size); err != nil {
		return fmt.Errorf("increment domain quota: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `
UPDATE companies
SET quota_used = quota_used + $2,
    updated_at = now()
WHERE id = $1`, companyID, size); err != nil {
		return fmt.Errorf("increment company quota: %w", err)
	}
	return nil
}

// decrementUserQuota subtracts size bytes from every quota ledger, clamping at
// zero so stale accounting cannot produce a negative balance.
func decrementUserQuota(ctx context.Context, tx *sql.Tx, userID string, size int64) error {
	if size <= 0 {
		return nil
	}

	var domainID, companyID string
	if err := tx.QueryRowContext(ctx, `
SELECT d.id::text, c.id::text
FROM users u
JOIN domains d ON d.id = u.domain_id
JOIN companies c ON c.id = d.company_id
WHERE u.id = $1
FOR UPDATE OF u, d, c`, userID).Scan(&domainID, &companyID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("user %q not found for quota decrement", userID)
		}
		return fmt.Errorf("read quota ledger for decrement: %w", err)
	}

	const q = `
UPDATE users
SET quota_used = GREATEST(0, quota_used - $2),
    updated_at = now()
WHERE id = $1`

	if _, err := tx.ExecContext(ctx, q, userID, size); err != nil {
		return fmt.Errorf("decrement user quota: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `
UPDATE domains
SET quota_used = GREATEST(0, quota_used - $2),
    updated_at = now()
WHERE id = $1`, domainID, size); err != nil {
		return fmt.Errorf("decrement domain quota: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `
UPDATE companies
SET quota_used = GREATEST(0, quota_used - $2),
    updated_at = now()
WHERE id = $1`, companyID, size); err != nil {
		return fmt.Errorf("decrement company quota: %w", err)
	}
	return nil
}

const decrementUserQuotasSQL = `
WITH input AS (
  SELECT user_id, bytes
  FROM unnest($1::uuid[], $2::bigint[]) AS input(user_id, bytes)
  WHERE bytes > 0
),
user_usage AS (
  SELECT user_id, SUM(bytes)::bigint AS bytes
  FROM input
  GROUP BY user_id
),
domain_usage AS (
  SELECT u.domain_id, SUM(user_usage.bytes)::bigint AS bytes
  FROM user_usage
  JOIN users u ON u.id = user_usage.user_id
  GROUP BY u.domain_id
),
company_usage AS (
  SELECT d.company_id, SUM(domain_usage.bytes)::bigint AS bytes
  FROM domain_usage
  JOIN domains d ON d.id = domain_usage.domain_id
  GROUP BY d.company_id
),
updated_users AS (
  UPDATE users u
  SET quota_used = GREATEST(0, quota_used - user_usage.bytes),
      updated_at = now()
  FROM user_usage
  WHERE u.id = user_usage.user_id
  RETURNING u.id
),
updated_domains AS (
  UPDATE domains d
  SET quota_used = GREATEST(0, quota_used - domain_usage.bytes),
      updated_at = now()
  FROM domain_usage
  WHERE d.id = domain_usage.domain_id
  RETURNING d.id
),
updated_companies AS (
  UPDATE companies c
  SET quota_used = GREATEST(0, quota_used - company_usage.bytes),
      updated_at = now()
  FROM company_usage
  WHERE c.id = company_usage.company_id
  RETURNING c.id
)
SELECT
  (SELECT COUNT(*) FROM updated_users),
  (SELECT COUNT(*) FROM updated_domains),
  (SELECT COUNT(*) FROM updated_companies)`

func decrementUserQuotas(ctx context.Context, tx *sql.Tx, userIDs []string, sizes []int64) error {
	if len(userIDs) == 0 || len(sizes) == 0 {
		return nil
	}
	if len(userIDs) != len(sizes) {
		return fmt.Errorf("quota decrement input length mismatch")
	}
	var updatedUsers, updatedDomains, updatedCompanies int64
	if err := tx.QueryRowContext(ctx, decrementUserQuotasSQL, pq.Array(userIDs), pq.Array(sizes)).Scan(&updatedUsers, &updatedDomains, &updatedCompanies); err != nil {
		return fmt.Errorf("decrement quota ledgers: %w", err)
	}
	return nil
}
