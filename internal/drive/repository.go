package drive

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"
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
