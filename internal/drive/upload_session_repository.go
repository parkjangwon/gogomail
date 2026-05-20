package drive

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/gogomail/gogomail/internal/storage"
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
SELECT
  id,
  user_id,
  parent_id,
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
FROM inserted`
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

func (r *Repository) ListUploadSessions(ctx context.Context, req ListUploadSessionsRequest) ([]UploadSession, error) {
	if r == nil || r.db == nil {
		return nil, fmt.Errorf("database handle is required")
	}
	req, err := ValidateListUploadSessionsRequest(req)
	if err != nil {
		return nil, err
	}
	query, args := buildListUploadSessionsQuery(req)
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list drive upload sessions: %w", err)
	}
	defer rows.Close()
	sessions := make([]UploadSession, 0)
	for rows.Next() {
		session, err := scanUploadSession(rows)
		if err != nil {
			return nil, fmt.Errorf("scan drive upload session: %w", err)
		}
		sessions = append(sessions, session)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate drive upload sessions: %w", err)
	}
	return sessions, nil
}

const listUploadSessionsBaseSQL = `
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
WHERE s.user_id = $1::uuid`

func buildListUploadSessionsQuery(req ListUploadSessionsRequest) (string, []any) {
	args := []any{req.UserID}
	conditions := []string{
		"u.status = 'active'",
		"d.status = 'active'",
	}
	if req.Status != "" {
		args = append(args, req.Status)
		conditions = append(conditions, fmt.Sprintf("s.status = $%d", len(args)))
	}
	args = append(args, req.Limit)
	query := listUploadSessionsBaseSQL + "\n  AND " + strings.Join(conditions, "\n  AND ") + fmt.Sprintf(`
ORDER BY s.updated_at DESC, s.created_at DESC
LIMIT $%d`, len(args))
	return query, args
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

func (r *Repository) ExpireUploadSessions(ctx context.Context, req ExpireUploadSessionsRequest) ([]UploadSession, error) {
	if r == nil || r.db == nil {
		return nil, fmt.Errorf("database handle is required")
	}
	req, err := ValidateExpireUploadSessionsRequest(req)
	if err != nil {
		return nil, err
	}
	rows, err := r.db.QueryContext(ctx, buildExpireUploadSessionsQuery(), req.Before, req.Limit)
	if err != nil {
		return nil, fmt.Errorf("expire drive upload sessions: %w", err)
	}
	defer rows.Close()

	expired := make([]UploadSession, 0)
	for rows.Next() {
		session, err := scanUploadSession(rows)
		if err != nil {
			return nil, fmt.Errorf("scan expired drive upload session: %w", err)
		}
		expired = append(expired, session)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate expired drive upload sessions: %w", err)
	}
	return expired, nil
}

func buildExpireUploadSessionsQuery() string {
	return `
WITH expired AS (
  SELECT id
  FROM drive_upload_sessions
  WHERE status IN ('pending', 'uploading', 'failed')
    AND expires_at < $1
  ORDER BY expires_at ASC, created_at ASC
  LIMIT $2
  FOR UPDATE SKIP LOCKED
),
updated AS (
  UPDATE drive_upload_sessions s
  SET status = 'expired',
      updated_at = now()
  FROM expired
  WHERE s.id = expired.id
  RETURNING
    s.id::text,
    s.user_id::text,
    COALESCE(s.parent_id::text, '') AS parent_id,
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
)
SELECT
  id,
  user_id,
  parent_id,
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
FROM updated
ORDER BY expires_at ASC, created_at ASC`
}

func (r *Repository) CountStaleUploadSessions(ctx context.Context, req ExpireUploadSessionsRequest) (StaleUploadSessionCount, error) {
	if r == nil || r.db == nil {
		return StaleUploadSessionCount{}, fmt.Errorf("database handle is required")
	}
	req, err := ValidateExpireUploadSessionsRequest(req)
	if err != nil {
		return StaleUploadSessionCount{}, err
	}
	const query = `
SELECT COUNT(*)
FROM drive_upload_sessions
WHERE status IN ('pending', 'uploading', 'failed')
  AND expires_at < $1`
	var total int64
	if err := r.db.QueryRowContext(ctx, query, req.Before).Scan(&total); err != nil {
		return StaleUploadSessionCount{}, fmt.Errorf("count stale drive upload sessions: %w", err)
	}
	limited := total
	if limited > int64(req.Limit) {
		limited = int64(req.Limit)
	}
	return StaleUploadSessionCount{TotalCount: total, LimitedCount: limited}, nil
}

func (r *Repository) ListStaleUploadSessions(ctx context.Context, req ExpireUploadSessionsRequest) ([]UploadSession, error) {
	if r == nil || r.db == nil {
		return nil, fmt.Errorf("database handle is required")
	}
	req, err := ValidateExpireUploadSessionsRequest(req)
	if err != nil {
		return nil, err
	}
	const query = `
SELECT
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
FROM drive_upload_sessions
WHERE status IN ('pending', 'uploading', 'failed')
  AND expires_at < $1
ORDER BY expires_at ASC, created_at ASC
LIMIT $2`
	rows, err := r.db.QueryContext(ctx, query, req.Before, req.Limit)
	if err != nil {
		return nil, fmt.Errorf("list stale drive upload sessions: %w", err)
	}
	defer rows.Close()
	sessions := make([]UploadSession, 0)
	for rows.Next() {
		session, err := scanUploadSession(rows)
		if err != nil {
			return nil, fmt.Errorf("scan stale drive upload session: %w", err)
		}
		sessions = append(sessions, session)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate stale drive upload sessions: %w", err)
	}
	return sessions, nil
}

// lockWritableUploadSession acquires a FOR UPDATE row lock inside tx.
// Concurrent writers for the same session block until this transaction ends,
// eliminating the TOCTOU window between reading the old storage_path and
// overwriting it. Returns the locked session so the caller has an authoritative
// view of the current state (including the prior storage_path).
func lockWritableUploadSession(ctx context.Context, tx *sql.Tx, userID, sessionID string) (UploadSession, error) {
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
  COALESCE(s.storage_path, ''),
  COALESCE(s.checksum_sha256, ''),
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
  AND d.status = 'active'
  AND s.status IN ('pending', 'uploading', 'failed')
  AND s.expires_at > now()
FOR UPDATE`
	var session UploadSession
	var finalizedAt sql.NullTime
	var canceledAt sql.NullTime
	err := tx.QueryRowContext(ctx, query, userID, sessionID).Scan(
		&session.ID, &session.UserID, &session.ParentID,
		&session.UploadID, &session.Name,
		&session.DeclaredSize, &session.ReceivedSize,
		&session.MIMEType, &session.Status, &session.StorageBackend,
		&session.StoragePath, &session.ChecksumSHA256,
		&session.ExpiresAt, &session.CreatedAt, &session.UpdatedAt,
		&finalizedAt, &canceledAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return UploadSession{}, fmt.Errorf("writable drive upload session not found")
		}
		return UploadSession{}, fmt.Errorf("lock drive upload session for write: %w", err)
	}
	if finalizedAt.Valid {
		session.FinalizedAt = finalizedAt.Time
	}
	if canceledAt.Valid {
		session.CanceledAt = canceledAt.Time
	}
	return session, nil
}

// StoreUploadSessionBody atomically locks the session row, records the new
// storage_path, and returns the prior storage_path alongside the updated
// session. The FOR UPDATE lock serialises concurrent chunk writes for the same
// session so the caller can safely delete the replaced object without racing.
func (r *Repository) StoreUploadSessionBody(ctx context.Context, req RecordUploadSessionBodyRequest) (UploadSession, string, error) {
	if r == nil || r.db == nil {
		return UploadSession{}, "", fmt.Errorf("database handle is required")
	}
	req, err := ValidateRecordUploadSessionBodyRequest(req)
	if err != nil {
		return UploadSession{}, "", err
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return UploadSession{}, "", fmt.Errorf("begin store upload session body transaction: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck

	locked, err := lockWritableUploadSession(ctx, tx, req.UserID, req.SessionID)
	if err != nil {
		return UploadSession{}, "", err
	}
	priorPath := locked.StoragePath // authoritative old path from the locked row

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
  AND (NOT $6::boolean OR s.received_size = $7)
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
	err = tx.QueryRowContext(ctx, query,
		req.UserID, req.SessionID, req.ReceivedSize, req.StoragePath, req.ChecksumSHA256,
		req.EnforcePreviousReceivedSize, req.PreviousReceivedSize,
	).Scan(
		&session.ID, &session.UserID, &session.ParentID,
		&session.UploadID, &session.Name,
		&session.DeclaredSize, &session.ReceivedSize,
		&session.MIMEType, &session.Status, &session.StorageBackend,
		&session.StoragePath, &session.ChecksumSHA256,
		&session.ExpiresAt, &session.CreatedAt, &session.UpdatedAt,
		&finalizedAt, &canceledAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return UploadSession{}, "", fmt.Errorf("writable drive upload session not found")
		}
		return UploadSession{}, "", fmt.Errorf("store drive upload session body: %w", err)
	}
	if finalizedAt.Valid {
		session.FinalizedAt = finalizedAt.Time
	}
	if canceledAt.Valid {
		session.CanceledAt = canceledAt.Time
	}
	if err := tx.Commit(); err != nil {
		return UploadSession{}, "", fmt.Errorf("commit store upload session body transaction: %w", err)
	}
	return session, priorPath, nil
}

type uploadSessionScanner interface {
	Scan(dest ...any) error
}

func scanUploadSession(scanner uploadSessionScanner) (UploadSession, error) {
	var session UploadSession
	var finalizedAt sql.NullTime
	var canceledAt sql.NullTime
	if err := scanner.Scan(
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
	); err != nil {
		return UploadSession{}, err
	}
	if finalizedAt.Valid {
		session.FinalizedAt = finalizedAt.Time
	}
	if canceledAt.Valid {
		session.CanceledAt = canceledAt.Time
	}
	return session, nil
}

func (r *Repository) FinalizeUploadSession(ctx context.Context, store storage.Store, req FinalizeUploadSessionRequest) (Node, error) {
	if r == nil || r.db == nil {
		return Node{}, fmt.Errorf("database handle is required")
	}
	if store == nil {
		return Node{}, fmt.Errorf("storage store is required")
	}
	req, err := ValidateFinalizeUploadSessionRequest(req)
	if err != nil {
		return Node{}, err
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return Node{}, fmt.Errorf("begin finalize drive upload session transaction: %w", err)
	}
	defer tx.Rollback()

	session, err := lockFinalizableUploadSession(ctx, tx, req.UserID, req.SessionID)
	if err != nil {
		return Node{}, err
	}
	if session.StoragePath == "" {
		return Node{}, fmt.Errorf("drive upload session body is required")
	}
	storagePath, err := validateUserObjectPath(session.UserID, session.StoragePath)
	if err != nil {
		return Node{}, fmt.Errorf("drive upload session body path is invalid: %w", err)
	}
	info, err := store.Stat(ctx, storagePath)
	if err != nil {
		return Node{}, fmt.Errorf("stat drive upload session body: %w", err)
	}
	if info.Size != session.ReceivedSize || info.Size != session.DeclaredSize {
		return Node{}, fmt.Errorf("drive upload session body size mismatch")
	}
	createReq := CreateFileFromObjectRequest{
		UserID:         session.UserID,
		ParentID:       session.ParentID,
		Name:           session.Name,
		StorageBackend: session.StorageBackend,
		StoragePath:    storagePath,
		MIMEType:       session.MIMEType,
		ChecksumSHA256: session.ChecksumSHA256,
	}
	createReq, normalizedName, err := ValidateCreateFileFromObjectRequest(createReq)
	if err != nil {
		return Node{}, err
	}
	if err := incrementDriveQuota(ctx, tx, createReq.UserID, info.Size); err != nil {
		return Node{}, err
	}
	node, err := insertDriveFileNode(ctx, tx, createReq, normalizedName, info.Size)
	if err != nil {
		return Node{}, err
	}
	if err := markUploadSessionFinalized(ctx, tx, req.UserID, req.SessionID); err != nil {
		return Node{}, err
	}
	if err := tx.Commit(); err != nil {
		return Node{}, fmt.Errorf("commit finalize drive upload session transaction: %w", err)
	}
	return node, nil
}

func lockFinalizableUploadSession(ctx context.Context, tx *sql.Tx, userID string, sessionID string) (UploadSession, error) {
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
  AND d.status = 'active'
  AND s.status = 'uploading'
  AND s.expires_at > now()
FOR UPDATE`
	var session UploadSession
	var finalizedAt sql.NullTime
	var canceledAt sql.NullTime
	err := tx.QueryRowContext(ctx, query, userID, sessionID).Scan(
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
			return UploadSession{}, fmt.Errorf("finalizable drive upload session not found")
		}
		return UploadSession{}, fmt.Errorf("lock drive upload session for finalize: %w", err)
	}
	if finalizedAt.Valid {
		session.FinalizedAt = finalizedAt.Time
	}
	if canceledAt.Valid {
		session.CanceledAt = canceledAt.Time
	}
	return session, nil
}

func markUploadSessionFinalized(ctx context.Context, tx *sql.Tx, userID string, sessionID string) error {
	const query = `
UPDATE drive_upload_sessions
SET
  status = 'finalized',
  finalized_at = now(),
  updated_at = now()
WHERE id = $2::uuid
  AND user_id = $1::uuid
  AND status = 'uploading'`
	result, err := tx.ExecContext(ctx, query, userID, sessionID)
	if err != nil {
		return fmt.Errorf("mark drive upload session finalized: %w", err)
	}
	updated, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("read drive upload session finalize count: %w", err)
	}
	if updated != 1 {
		return fmt.Errorf("drive upload session finalize state changed")
	}
	return nil
}
