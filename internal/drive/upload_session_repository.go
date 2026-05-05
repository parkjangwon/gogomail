package drive

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

func (r *Repository) CreateUploadSession(ctx context.Context, req CreateUploadSessionRequest) (UploadSession, error) {
	if r == nil || r.db == nil {
		return UploadSession{}, fmt.Errorf("database handle is required")
	}
	req, err := ValidateCreateUploadSessionRequest(req, time.Now().UTC())
	if err != nil {
		return UploadSession{}, err
	}
	const query = `
WITH owner AS (
  SELECT u.id AS user_id
  FROM users u
  JOIN domains d ON d.id = u.domain_id
  WHERE u.id = $1::uuid
    AND u.status = 'active'
    AND d.status = 'active'
),
parent AS (
  SELECT NULL::uuid AS id
  WHERE NULLIF($2, '') IS NULL
  UNION ALL
  SELECT n.id
  FROM drive_nodes n
  JOIN owner ON owner.user_id = n.user_id
  WHERE n.id = NULLIF($2, '')::uuid
    AND n.node_type = 'folder'
    AND n.status = 'active'
),
inserted AS (
  INSERT INTO drive_upload_sessions (
    user_id,
    parent_id,
    upload_id,
    name,
    declared_size,
    mime_type,
    storage_backend,
    expires_at
  )
  SELECT
    owner.user_id,
    parent.id,
    $3,
    $4,
    $5,
    $6,
    $7,
    $8
  FROM owner
  JOIN parent ON true
  RETURNING
    id::text,
    user_id::text,
    COALESCE(parent_id::text, ''),
    upload_id,
    name,
    declared_size,
    received_size,
    mime_type,
    status,
    storage_backend,
    storage_path,
    checksum_sha256,
    expires_at,
    created_at,
    updated_at,
    finalized_at,
    canceled_at
)
SELECT * FROM inserted`
	var session UploadSession
	var finalizedAt sql.NullTime
	var canceledAt sql.NullTime
	err = r.db.QueryRowContext(
		ctx,
		query,
		req.UserID,
		req.ParentID,
		req.UploadID,
		req.Name,
		req.DeclaredSize,
		req.MIMEType,
		req.StorageBackend,
		req.ExpiresAt,
	).Scan(
		&session.ID,
		&session.UserID,
		&session.ParentID,
		&session.UploadID,
		&session.Name,
		&session.DeclaredSize,
		&session.ReceivedSize,
		&session.MIMEType,
		&session.Status,
		&session.StorageBackend,
		&session.StoragePath,
		&session.ChecksumSHA256,
		&session.ExpiresAt,
		&session.CreatedAt,
		&session.UpdatedAt,
		&finalizedAt,
		&canceledAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return UploadSession{}, fmt.Errorf("active user or parent folder not found")
		}
		return UploadSession{}, fmt.Errorf("create drive upload session: %w", err)
	}
	if finalizedAt.Valid {
		session.FinalizedAt = finalizedAt.Time
	}
	if canceledAt.Valid {
		session.CanceledAt = canceledAt.Time
	}
	return session, nil
}

func (r *Repository) GetUploadSession(ctx context.Context, req GetUploadSessionRequest) (UploadSession, error) {
	if r == nil || r.db == nil {
		return UploadSession{}, fmt.Errorf("database handle is required")
	}
	req, err := ValidateGetUploadSessionRequest(req)
	if err != nil {
		return UploadSession{}, err
	}
	const query = `
SELECT
  s.id::text,
  s.user_id::text,
  COALESCE(s.parent_id::text, ''),
  s.upload_id,
  s.name,
  s.declared_size,
  s.received_size,
  s.mime_type,
  s.status,
  s.storage_backend,
  s.storage_path,
  s.checksum_sha256,
  s.expires_at,
  s.created_at,
  s.updated_at,
  s.finalized_at,
  s.canceled_at
FROM drive_upload_sessions s
JOIN users u ON u.id = s.user_id
JOIN domains d ON d.id = u.domain_id
WHERE s.id = $2::uuid
  AND s.user_id = $1::uuid
  AND u.status = 'active'
  AND d.status = 'active'`
	var session UploadSession
	var finalizedAt sql.NullTime
	var canceledAt sql.NullTime
	err = r.db.QueryRowContext(ctx, query, req.UserID, req.SessionID).Scan(
		&session.ID,
		&session.UserID,
		&session.ParentID,
		&session.UploadID,
		&session.Name,
		&session.DeclaredSize,
		&session.ReceivedSize,
		&session.MIMEType,
		&session.Status,
		&session.StorageBackend,
		&session.StoragePath,
		&session.ChecksumSHA256,
		&session.ExpiresAt,
		&session.CreatedAt,
		&session.UpdatedAt,
		&finalizedAt,
		&canceledAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return UploadSession{}, fmt.Errorf("drive upload session not found")
		}
		return UploadSession{}, fmt.Errorf("get drive upload session: %w", err)
	}
	if finalizedAt.Valid {
		session.FinalizedAt = finalizedAt.Time
	}
	if canceledAt.Valid {
		session.CanceledAt = canceledAt.Time
	}
	return session, nil
}

func (r *Repository) CancelUploadSession(ctx context.Context, req CancelUploadSessionRequest) (UploadSession, error) {
	if r == nil || r.db == nil {
		return UploadSession{}, fmt.Errorf("database handle is required")
	}
	req, err := ValidateCancelUploadSessionRequest(req)
	if err != nil {
		return UploadSession{}, err
	}
	const query = `
UPDATE drive_upload_sessions s
SET
  status = 'canceled',
  canceled_at = now(),
  updated_at = now()
FROM users u
JOIN domains d ON d.id = u.domain_id
WHERE s.id = $2::uuid
  AND s.user_id = $1::uuid
  AND u.id = s.user_id
  AND u.status = 'active'
  AND d.status = 'active'
  AND s.status IN ('pending', 'uploading', 'failed')
RETURNING
  s.id::text,
  s.user_id::text,
  COALESCE(s.parent_id::text, ''),
  s.upload_id,
  s.name,
  s.declared_size,
  s.received_size,
  s.mime_type,
  s.status,
  s.storage_backend,
  s.storage_path,
  s.checksum_sha256,
  s.expires_at,
  s.created_at,
  s.updated_at,
  s.finalized_at,
  s.canceled_at`
	var session UploadSession
	var finalizedAt sql.NullTime
	var canceledAt sql.NullTime
	err = r.db.QueryRowContext(ctx, query, req.UserID, req.SessionID).Scan(
		&session.ID,
		&session.UserID,
		&session.ParentID,
		&session.UploadID,
		&session.Name,
		&session.DeclaredSize,
		&session.ReceivedSize,
		&session.MIMEType,
		&session.Status,
		&session.StorageBackend,
		&session.StoragePath,
		&session.ChecksumSHA256,
		&session.ExpiresAt,
		&session.CreatedAt,
		&session.UpdatedAt,
		&finalizedAt,
		&canceledAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return UploadSession{}, fmt.Errorf("cancelable drive upload session not found")
		}
		return UploadSession{}, fmt.Errorf("cancel drive upload session: %w", err)
	}
	if finalizedAt.Valid {
		session.FinalizedAt = finalizedAt.Time
	}
	if canceledAt.Valid {
		session.CanceledAt = canceledAt.Time
	}
	return session, nil
}

func (r *Repository) StoreUploadSessionBody(ctx context.Context, req RecordUploadSessionBodyRequest) (UploadSession, error) {
	if r == nil || r.db == nil {
		return UploadSession{}, fmt.Errorf("database handle is required")
	}
	req, err := ValidateRecordUploadSessionBodyRequest(req)
	if err != nil {
		return UploadSession{}, err
	}
	const query = `
UPDATE drive_upload_sessions s
SET
  status = 'uploading',
  received_size = $3,
  storage_path = $4,
  checksum_sha256 = $5,
  updated_at = now()
FROM users u
JOIN domains d ON d.id = u.domain_id
WHERE s.id = $2::uuid
  AND s.user_id = $1::uuid
  AND u.id = s.user_id
  AND u.status = 'active'
  AND d.status = 'active'
  AND s.status IN ('pending', 'uploading', 'failed')
  AND s.expires_at > now()
  AND $3 <= s.declared_size
RETURNING
  s.id::text,
  s.user_id::text,
  COALESCE(s.parent_id::text, ''),
  s.upload_id,
  s.name,
  s.declared_size,
  s.received_size,
  s.mime_type,
  s.status,
  s.storage_backend,
  s.storage_path,
  s.checksum_sha256,
  s.expires_at,
  s.created_at,
  s.updated_at,
  s.finalized_at,
  s.canceled_at`
	var session UploadSession
	var finalizedAt sql.NullTime
	var canceledAt sql.NullTime
	err = r.db.QueryRowContext(
		ctx,
		query,
		req.UserID,
		req.SessionID,
		req.ReceivedSize,
		req.StoragePath,
		req.ChecksumSHA256,
	).Scan(
		&session.ID,
		&session.UserID,
		&session.ParentID,
		&session.UploadID,
		&session.Name,
		&session.DeclaredSize,
		&session.ReceivedSize,
		&session.MIMEType,
		&session.Status,
		&session.StorageBackend,
		&session.StoragePath,
		&session.ChecksumSHA256,
		&session.ExpiresAt,
		&session.CreatedAt,
		&session.UpdatedAt,
		&finalizedAt,
		&canceledAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return UploadSession{}, fmt.Errorf("writable drive upload session not found")
		}
		return UploadSession{}, fmt.Errorf("store drive upload session body: %w", err)
	}
	if finalizedAt.Valid {
		session.FinalizedAt = finalizedAt.Time
	}
	if canceledAt.Valid {
		session.CanceledAt = canceledAt.Time
	}
	return session, nil
}
