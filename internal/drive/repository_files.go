package drive

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/gogomail/gogomail/internal/mail"
	"github.com/gogomail/gogomail/internal/storage"
)

func (r *Repository) CreateFileFromObject(ctx context.Context, store storage.Store, req CreateFileFromObjectRequest) (Node, error) {
	if r == nil || r.db == nil {
		return Node{}, fmt.Errorf("database handle is required")
	}
	if store == nil {
		return Node{}, fmt.Errorf("storage store is required")
	}
	req, normalizedName, err := ValidateCreateFileFromObjectRequest(req)
	if err != nil {
		return Node{}, err
	}
	info, err := store.Stat(ctx, req.StoragePath)
	if err != nil {
		return Node{}, fmt.Errorf("stat drive file object: %w", err)
	}
	if info.Size < 0 {
		return Node{}, fmt.Errorf("drive file object size is invalid")
	}
	if info.ContentType != "" && req.MIMEType == "application/octet-stream" {
		req.MIMEType, err = validateDriveMIMEType(info.ContentType)
		if err != nil {
			return Node{}, fmt.Errorf("storage object content type is invalid: %w", err)
		}
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return Node{}, fmt.Errorf("begin create drive file transaction: %w", err)
	}
	defer tx.Rollback()

	if err := incrementDriveQuota(ctx, tx, req.UserID, info.Size); err != nil {
		return Node{}, err
	}
	node, err := insertDriveFileNode(ctx, tx, req, normalizedName, info.Size)
	if err != nil {
		if errors.Is(err, ErrDriveNodeAlreadyExists) {
			return Node{}, err
		}
		return Node{}, err
	}
	if err := tx.Commit(); err != nil {
		return Node{}, fmt.Errorf("commit create drive file transaction: %w", err)
	}
	return node, nil
}

func insertDriveFileNode(ctx context.Context, tx *sql.Tx, req CreateFileFromObjectRequest, normalizedName string, size int64) (Node, error) {
	query := buildInsertDriveFileNodeQuery(req.ParentID)
	var node Node
	err := tx.QueryRowContext(
		ctx,
		query,
		req.UserID,
		req.ParentID,
		req.Name,
		normalizedName,
		req.MIMEType,
		size,
		req.StorageBackend,
		req.StoragePath,
		req.ChecksumSHA256,
	).Scan(
		&node.ID,
		&node.CompanyID,
		&node.DomainID,
		&node.UserID,
		&node.ParentID,
		&node.Type,
		&node.Name,
		&node.NormalizedName,
		&node.MIMEType,
		&node.Size,
		&node.StorageBackend,
		&node.StoragePath,
		&node.ChecksumSHA256,
		&node.Status,
		&node.CreatedAt,
		&node.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Node{}, fmt.Errorf("active user or parent folder not found")
		}
		return Node{}, mapDriveFileCreateError(err)
	}
	return node, nil
}

func buildInsertDriveFileNodeQuery(parentID string) string {
	parentCTE, parentIDExpr, parentWhere := driveParentFolderInsertFragments(parentID)
	return fmt.Sprintf(`
WITH owner AS (
  SELECT u.id AS user_id, d.id AS domain_id, d.company_id
  FROM users u
  JOIN domains d ON d.id = u.domain_id
  WHERE u.id = $1::uuid
    AND u.status = 'active'
    AND d.status = 'active'
)
%s
INSERT INTO drive_nodes (
  company_id,
  domain_id,
  user_id,
  parent_id,
  node_type,
  name,
  normalized_name,
  mime_type,
  size,
  storage_backend,
  storage_path,
  checksum_sha256,
  status
)
SELECT
  owner.company_id,
  owner.domain_id,
  owner.user_id,
  %s,
  'file',
  $3,
  $4,
  $5,
  $6,
  $7,
  $8,
  $9,
  'active'
FROM owner
%s
RETURNING
  id::text,
  company_id::text,
  domain_id::text,
  user_id::text,
  COALESCE(parent_id::text, ''),
  node_type,
  name,
  normalized_name,
  mime_type,
  size,
  storage_backend,
  storage_path,
  checksum_sha256,
  status,
  created_at,
  updated_at`, parentCTE, parentIDExpr, parentWhere)
}

func incrementDriveQuota(ctx context.Context, tx *sql.Tx, userID string, size int64) error {
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
  AND u.status = 'active'
  AND d.status = 'active'
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
			return fmt.Errorf("active user %q not found for drive quota check", userID)
		}
		return fmt.Errorf("read drive quota ledger: %w", err)
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

	if _, err := tx.ExecContext(ctx, `
UPDATE users
SET quota_used = quota_used + $2,
    updated_at = now()
WHERE id = $1`, userID, size); err != nil {
		return fmt.Errorf("increment user drive quota: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `
UPDATE domains
SET quota_used = quota_used + $2,
    updated_at = now()
WHERE id = $1`, domainID, size); err != nil {
		return fmt.Errorf("increment domain drive quota: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `
UPDATE companies
SET quota_used = quota_used + $2,
    updated_at = now()
WHERE id = $1`, companyID, size); err != nil {
		return fmt.Errorf("increment company drive quota: %w", err)
	}
	return nil
}

func decrementDriveQuota(ctx context.Context, tx *sql.Tx, userID string, size int64) error {
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
			return fmt.Errorf("user %q not found for drive quota decrement", userID)
		}
		return fmt.Errorf("read drive quota ledger for decrement: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `
UPDATE users
SET quota_used = GREATEST(0, quota_used - $2),
    updated_at = now()
WHERE id = $1`, userID, size); err != nil {
		return fmt.Errorf("decrement user drive quota: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `
UPDATE domains
SET quota_used = GREATEST(0, quota_used - $2),
    updated_at = now()
WHERE id = $1`, domainID, size); err != nil {
		return fmt.Errorf("decrement domain drive quota: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `
UPDATE companies
SET quota_used = GREATEST(0, quota_used - $2),
    updated_at = now()
WHERE id = $1`, companyID, size); err != nil {
		return fmt.Errorf("decrement company drive quota: %w", err)
	}
	return nil
}
