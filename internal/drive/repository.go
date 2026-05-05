package drive

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/gogomail/gogomail/internal/mail"
	"github.com/gogomail/gogomail/internal/storage"
)

type Repository struct {
	db *sql.DB
}

func NewRepository(db *sql.DB) *Repository {
	return &Repository{db: db}
}

type Node struct {
	ID             string
	CompanyID      string
	DomainID       string
	UserID         string
	ParentID       string
	Type           string
	Name           string
	NormalizedName string
	MIMEType       string
	Size           int64
	StorageBackend string
	StoragePath    string
	ChecksumSHA256 string
	Status         string
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

type CreateFolderRequest struct {
	UserID   string
	ParentID string
	Name     string
}

type CreateFileFromObjectRequest struct {
	UserID         string
	ParentID       string
	Name           string
	StorageBackend string
	StoragePath    string
	MIMEType       string
	ChecksumSHA256 string
}

func ValidateCreateFolderRequest(req CreateFolderRequest) (CreateFolderRequest, string, error) {
	userID, err := validateDriveID("user_id", req.UserID, true)
	if err != nil {
		return CreateFolderRequest{}, "", err
	}
	parentID, err := validateDriveID("parent_id", req.ParentID, false)
	if err != nil {
		return CreateFolderRequest{}, "", err
	}
	name, err := ValidateNodeName(req.Name)
	if err != nil {
		return CreateFolderRequest{}, "", err
	}
	normalizedName, err := NormalizeNodeName(name)
	if err != nil {
		return CreateFolderRequest{}, "", err
	}
	return CreateFolderRequest{
		UserID:   userID,
		ParentID: parentID,
		Name:     name,
	}, normalizedName, nil
}

func ValidateCreateFileFromObjectRequest(req CreateFileFromObjectRequest) (CreateFileFromObjectRequest, string, error) {
	userID, err := validateDriveID("user_id", req.UserID, true)
	if err != nil {
		return CreateFileFromObjectRequest{}, "", err
	}
	parentID, err := validateDriveID("parent_id", req.ParentID, false)
	if err != nil {
		return CreateFileFromObjectRequest{}, "", err
	}
	name, err := ValidateNodeName(req.Name)
	if err != nil {
		return CreateFileFromObjectRequest{}, "", err
	}
	normalizedName, err := NormalizeNodeName(name)
	if err != nil {
		return CreateFileFromObjectRequest{}, "", err
	}
	storageBackend, err := validateStorageBackend(req.StorageBackend)
	if err != nil {
		return CreateFileFromObjectRequest{}, "", err
	}
	storagePath, err := storage.ValidateObjectPath(req.StoragePath)
	if err != nil {
		return CreateFileFromObjectRequest{}, "", fmt.Errorf("unsafe storage path %q: %w", req.StoragePath, err)
	}
	mimeType, err := validateDriveMIMEType(req.MIMEType)
	if err != nil {
		return CreateFileFromObjectRequest{}, "", err
	}
	checksum, err := validateDriveChecksum(req.ChecksumSHA256)
	if err != nil {
		return CreateFileFromObjectRequest{}, "", err
	}
	return CreateFileFromObjectRequest{
		UserID:         userID,
		ParentID:       parentID,
		Name:           name,
		StorageBackend: storageBackend,
		StoragePath:    storagePath,
		MIMEType:       mimeType,
		ChecksumSHA256: checksum,
	}, normalizedName, nil
}

func (r *Repository) CreateFolder(ctx context.Context, req CreateFolderRequest) (Node, error) {
	if r == nil || r.db == nil {
		return Node{}, fmt.Errorf("database handle is required")
	}
	req, normalizedName, err := ValidateCreateFolderRequest(req)
	if err != nil {
		return Node{}, err
	}

	const query = `
WITH owner AS (
  SELECT u.id AS user_id, d.id AS domain_id, d.company_id
  FROM users u
  JOIN domains d ON d.id = u.domain_id
  WHERE u.id = $1::uuid
    AND u.status = 'active'
    AND d.status = 'active'
),
parent AS (
  SELECT n.id
  FROM drive_nodes n
  JOIN owner ON owner.user_id = n.user_id
  WHERE n.id = NULLIF($2, '')::uuid
    AND n.node_type = 'folder'
    AND n.status = 'active'
)
INSERT INTO drive_nodes (
  company_id,
  domain_id,
  user_id,
  parent_id,
  node_type,
  name,
  normalized_name,
  status
)
SELECT
  owner.company_id,
  owner.domain_id,
  owner.user_id,
  CASE WHEN NULLIF($2, '') IS NULL THEN NULL ELSE (SELECT id FROM parent) END,
  'folder',
  $3,
  $4,
  'active'
FROM owner
WHERE NULLIF($2, '') IS NULL OR EXISTS (SELECT 1 FROM parent)
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
  updated_at`

	var node Node
	err = r.db.QueryRowContext(ctx, query, req.UserID, req.ParentID, req.Name, normalizedName).Scan(
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
		if err == sql.ErrNoRows {
			return Node{}, fmt.Errorf("active user or parent folder not found")
		}
		return Node{}, fmt.Errorf("create drive folder: %w", err)
	}
	return node, nil
}

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
		return Node{}, err
	}
	if err := tx.Commit(); err != nil {
		return Node{}, fmt.Errorf("commit create drive file transaction: %w", err)
	}
	return node, nil
}

func insertDriveFileNode(ctx context.Context, tx *sql.Tx, req CreateFileFromObjectRequest, normalizedName string, size int64) (Node, error) {
	const query = `
WITH owner AS (
  SELECT u.id AS user_id, d.id AS domain_id, d.company_id
  FROM users u
  JOIN domains d ON d.id = u.domain_id
  WHERE u.id = $1::uuid
    AND u.status = 'active'
    AND d.status = 'active'
),
parent AS (
  SELECT n.id
  FROM drive_nodes n
  JOIN owner ON owner.user_id = n.user_id
  WHERE n.id = NULLIF($2, '')::uuid
    AND n.node_type = 'folder'
    AND n.status = 'active'
)
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
  CASE WHEN NULLIF($2, '') IS NULL THEN NULL ELSE (SELECT id FROM parent) END,
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
WHERE NULLIF($2, '') IS NULL OR EXISTS (SELECT 1 FROM parent)
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
  updated_at`
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
		return Node{}, fmt.Errorf("create drive file: %w", err)
	}
	return node, nil
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

func validateDriveID(field string, value string, required bool) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		if required {
			return "", fmt.Errorf("%s is required", field)
		}
		return "", nil
	}
	if len(value) > 128 {
		return "", fmt.Errorf("%s is too long", field)
	}
	if strings.ContainsAny(value, "\r\n") {
		return "", fmt.Errorf("%s must not contain line breaks", field)
	}
	return value, nil
}

func validateStorageBackend(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", fmt.Errorf("storage_backend is required")
	}
	if len(value) > 64 {
		return "", fmt.Errorf("storage_backend is too long")
	}
	if strings.ContainsAny(value, "\r\n") {
		return "", fmt.Errorf("storage_backend must not contain line breaks")
	}
	return value, nil
}

func validateDriveMIMEType(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "application/octet-stream", nil
	}
	if len(value) > 255 {
		return "", fmt.Errorf("mime_type is too long")
	}
	if strings.ContainsAny(value, "\r\n") {
		return "", fmt.Errorf("mime_type must not contain line breaks")
	}
	return value, nil
}

func validateDriveChecksum(value string) (string, error) {
	value = strings.TrimSpace(strings.ToLower(value))
	if value == "" {
		return "", nil
	}
	if len(value) != 64 {
		return "", fmt.Errorf("checksum_sha256 must be 64 lowercase hex characters")
	}
	for _, r := range value {
		if (r < '0' || r > '9') && (r < 'a' || r > 'f') {
			return "", fmt.Errorf("checksum_sha256 must be 64 lowercase hex characters")
		}
	}
	return value, nil
}
