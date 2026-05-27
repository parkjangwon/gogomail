package drive

import (
	"errors"
	"context"
	"database/sql"
	"fmt"
	"time"
)

func (r *Repository) MoveNode(ctx context.Context, req MoveNodeRequest) (Node, error) {
	if r == nil || r.db == nil {
		return Node{}, fmt.Errorf("database handle is required")
	}
	req, err := ValidateMoveNodeRequest(req)
	if err != nil {
		return Node{}, err
	}
	const query = `
WITH RECURSIVE owner AS (
  SELECT u.id AS user_id
  FROM users u
  JOIN domains d ON d.id = u.domain_id
  WHERE u.id = $1::uuid
    AND u.status = 'active'
    AND d.status = 'active'
),
target AS (
  SELECT n.id
  FROM drive_nodes n
  JOIN owner ON owner.user_id = n.user_id
  WHERE n.id = $2::uuid
    AND n.status = 'active'
),
subtree AS (
  SELECT id FROM target
  UNION ALL
  SELECT child.id
  FROM drive_nodes child
  JOIN subtree ON subtree.id = child.parent_id
  JOIN owner ON owner.user_id = child.user_id
  WHERE child.status = 'active'
),
parent AS (
  SELECT NULL::uuid AS id
  WHERE NULLIF($3, '') IS NULL
  UNION ALL
  SELECT p.id
  FROM drive_nodes p
  JOIN owner ON owner.user_id = p.user_id
  WHERE p.id = NULLIF($3, '')::uuid
    AND p.node_type = 'folder'
    AND p.status = 'active'
),
updated AS (
  UPDATE drive_nodes n
  SET
    parent_id = (SELECT id FROM parent LIMIT 1),
    updated_at = now()
  FROM target
  WHERE n.id = target.id
    AND EXISTS (SELECT 1 FROM parent)
    AND NOT EXISTS (
      SELECT 1
      FROM subtree
      WHERE subtree.id = NULLIF($3, '')::uuid
    )
  RETURNING
    n.id::text,
    n.company_id::text,
    n.domain_id::text,
    n.user_id::text,
    COALESCE(n.parent_id::text, ''),
    n.node_type,
    n.name,
    n.normalized_name,
    n.mime_type,
    n.size,
    n.storage_backend,
    n.storage_path,
    n.checksum_sha256,
    n.status,
    n.created_at,
    n.updated_at
)
SELECT
  id,
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
  status,
  created_at,
  updated_at
FROM updated`
	var node Node
	err = r.db.QueryRowContext(ctx, query, req.UserID, req.NodeID, req.ParentID).Scan(
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
			return Node{}, fmt.Errorf("active drive node or destination folder not found")
		}
		return Node{}, fmt.Errorf("move drive node: %w", err)
	}
	return node, nil
}

func (r *Repository) PermanentDeleteNode(ctx context.Context, req PermanentDeleteNodeRequest) (PermanentDeleteResult, error) {
	if r == nil || r.db == nil {
		return PermanentDeleteResult{}, fmt.Errorf("database handle is required")
	}
	req, err := ValidatePermanentDeleteNodeRequest(req)
	if err != nil {
		return PermanentDeleteResult{}, err
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return PermanentDeleteResult{}, fmt.Errorf("begin permanent-delete drive node transaction: %w", err)
	}
	defer tx.Rollback()

	root, err := lockTrashedDriveNodeForDelete(ctx, tx, req.UserID, req.NodeID)
	if err != nil {
		return PermanentDeleteResult{}, err
	}
	deletedNodes, releasedBytes, objects, err := markDriveNodeTreeDeleted(ctx, tx, req.UserID, req.NodeID)
	if err != nil {
		return PermanentDeleteResult{}, err
	}
	if err := decrementDriveQuota(ctx, tx, req.UserID, releasedBytes); err != nil {
		return PermanentDeleteResult{}, err
	}
	if err := tx.Commit(); err != nil {
		return PermanentDeleteResult{}, fmt.Errorf("commit permanent-delete drive node transaction: %w", err)
	}
	root.Status = NodeStatusDeleted
	root.UpdatedAt = time.Now().UTC()
	return PermanentDeleteResult{
		Root:          root,
		DeletedNodes:  deletedNodes,
		ReleasedBytes: releasedBytes,
		Objects:       objects,
	}, nil
}
