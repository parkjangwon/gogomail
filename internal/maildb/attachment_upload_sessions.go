package maildb

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"
)

type AttachmentUploadSession struct {
	ID             string    `json:"id"`
	UserID         string    `json:"user_id"`
	DraftID        string    `json:"draft_id"`
	UploadID       string    `json:"upload_id"`
	Filename       string    `json:"filename"`
	DeclaredSize   int64     `json:"declared_size"`
	ReceivedSize   int64     `json:"received_size"`
	MIMEType       string    `json:"mime_type"`
	Status         string    `json:"status"`
	StorageBackend string    `json:"storage_backend"`
	StoragePath    string    `json:"storage_path"`
	ChecksumSHA256 string    `json:"checksum_sha256"`
	ExpiresAt      time.Time `json:"expires_at"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
	FinalizedAt    time.Time `json:"finalized_at,omitempty"`
	CanceledAt     time.Time `json:"canceled_at,omitempty"`
}

type CreateAttachmentUploadSessionRequest struct {
	UserID       string
	DraftID      string
	Filename     string
	DeclaredSize int64
	MIMEType     string
	ExpiresAt    time.Time
}

type CancelAttachmentUploadSessionRequest struct {
	UserID    string
	SessionID string
}

type GetAttachmentUploadSessionRequest struct {
	UserID    string
	SessionID string
}

type StoreAttachmentUploadSessionBodyRequest struct {
	UserID         string
	SessionID      string
	ReceivedSize   int64
	StoragePath    string
	ChecksumSHA256 string
}

type FinalizeAttachmentUploadSessionRequest struct {
	UserID    string
	SessionID string
}

type ExpireAttachmentUploadSessionsRequest struct {
	Before time.Time
	Limit  int
}

type StaleAttachmentUploadSessionCount struct {
	TotalCount   int64
	LimitedCount int64
}

func ValidateCreateAttachmentUploadSessionRequest(req CreateAttachmentUploadSessionRequest) error {
	if strings.TrimSpace(req.UserID) == "" {
		return fmt.Errorf("user_id is required")
	}
	if strings.TrimSpace(req.Filename) == "" {
		return fmt.Errorf("filename is required")
	}
	if strings.ContainsAny(req.Filename, "\r\n") {
		return fmt.Errorf("filename must not contain newlines")
	}
	if len(strings.TrimSpace(req.Filename)) > 255 {
		return fmt.Errorf("filename is too long")
	}
	if req.DeclaredSize < 0 {
		return fmt.Errorf("declared_size must not be negative")
	}
	if strings.TrimSpace(req.MIMEType) == "" {
		return fmt.Errorf("mime_type is required")
	}
	if strings.ContainsAny(req.MIMEType, "\r\n") {
		return fmt.Errorf("mime_type must not contain newlines")
	}
	if len(strings.TrimSpace(req.MIMEType)) > 255 {
		return fmt.Errorf("mime_type is too long")
	}
	if req.ExpiresAt.IsZero() {
		return fmt.Errorf("expires_at is required")
	}
	return nil
}

func ValidateCancelAttachmentUploadSessionRequest(req CancelAttachmentUploadSessionRequest) error {
	return validateAttachmentUploadSessionIdentity(req.UserID, req.SessionID)
}

func ValidateGetAttachmentUploadSessionRequest(req GetAttachmentUploadSessionRequest) error {
	return validateAttachmentUploadSessionIdentity(req.UserID, req.SessionID)
}

func ValidateStoreAttachmentUploadSessionBodyRequest(req StoreAttachmentUploadSessionBodyRequest) error {
	if err := validateAttachmentUploadSessionIdentity(req.UserID, req.SessionID); err != nil {
		return err
	}
	if req.ReceivedSize < 0 {
		return fmt.Errorf("received_size must not be negative")
	}
	if strings.TrimSpace(req.StoragePath) == "" {
		return fmt.Errorf("storage_path is required")
	}
	if strings.ContainsAny(req.StoragePath, "\r\n") {
		return fmt.Errorf("storage_path must not contain newlines")
	}
	if strings.TrimSpace(req.ChecksumSHA256) == "" {
		return fmt.Errorf("checksum_sha256 is required")
	}
	checksum := strings.TrimSpace(req.ChecksumSHA256)
	if len(checksum) != 64 {
		return fmt.Errorf("checksum_sha256 must be a lowercase SHA-256 hex digest")
	}
	for _, r := range checksum {
		if !((r >= '0' && r <= '9') || (r >= 'a' && r <= 'f')) {
			return fmt.Errorf("checksum_sha256 must be a lowercase SHA-256 hex digest")
		}
	}
	return nil
}

func ValidateFinalizeAttachmentUploadSessionRequest(req FinalizeAttachmentUploadSessionRequest) error {
	return validateAttachmentUploadSessionIdentity(req.UserID, req.SessionID)
}

func validateAttachmentUploadSessionIdentity(userID string, sessionID string) error {
	if strings.TrimSpace(userID) == "" {
		return fmt.Errorf("user_id is required")
	}
	if strings.TrimSpace(sessionID) == "" {
		return fmt.Errorf("session_id is required")
	}
	if strings.ContainsAny(userID, "\r\n") {
		return fmt.Errorf("user_id must not contain newlines")
	}
	if strings.ContainsAny(sessionID, "\r\n") {
		return fmt.Errorf("session_id must not contain newlines")
	}
	if len(strings.TrimSpace(userID)) > 200 {
		return fmt.Errorf("user_id is too long")
	}
	if len(strings.TrimSpace(sessionID)) > 200 {
		return fmt.Errorf("session_id is too long")
	}
	return nil
}

func ValidateExpireAttachmentUploadSessionsRequest(req ExpireAttachmentUploadSessionsRequest) error {
	if req.Before.IsZero() {
		return fmt.Errorf("before is required")
	}
	if req.Limit < 0 {
		return fmt.Errorf("limit must not be negative")
	}
	return nil
}

func (r *Repository) CreateAttachmentUploadSession(ctx context.Context, req CreateAttachmentUploadSessionRequest) (AttachmentUploadSession, error) {
	if r.db == nil {
		return AttachmentUploadSession{}, fmt.Errorf("database handle is required")
	}
	if err := ValidateCreateAttachmentUploadSessionRequest(req); err != nil {
		return AttachmentUploadSession{}, err
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return AttachmentUploadSession{}, fmt.Errorf("begin attachment upload session transaction: %w", err)
	}
	defer tx.Rollback()

	if err := checkAndIncrementUserQuota(ctx, tx, req.UserID, req.DeclaredSize); err != nil {
		return AttachmentUploadSession{}, err
	}

	const query = `
INSERT INTO attachment_upload_sessions (
  user_id, draft_id, upload_id, filename, declared_size, mime_type, expires_at
) VALUES (
  $1,
  NULLIF($2, '')::uuid,
  $3, $4, $5, $6, $7
) RETURNING
  id::text,
  user_id::text,
  COALESCE(draft_id::text, ''),
  upload_id,
  filename,
  declared_size,
  received_size,
  mime_type,
  status,
  storage_backend,
  storage_path,
  checksum_sha256,
  expires_at,
  created_at,
  updated_at`

	var session AttachmentUploadSession
	if err := tx.QueryRowContext(
		ctx,
		query,
		strings.TrimSpace(req.UserID),
		strings.TrimSpace(req.DraftID),
		newUploadID(),
		strings.TrimSpace(req.Filename),
		req.DeclaredSize,
		strings.TrimSpace(req.MIMEType),
		req.ExpiresAt.UTC(),
	).Scan(
		&session.ID,
		&session.UserID,
		&session.DraftID,
		&session.UploadID,
		&session.Filename,
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
	); err != nil {
		return AttachmentUploadSession{}, fmt.Errorf("create attachment upload session: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return AttachmentUploadSession{}, fmt.Errorf("commit attachment upload session transaction: %w", err)
	}
	return session, nil
}

func (r *Repository) CancelAttachmentUploadSession(ctx context.Context, req CancelAttachmentUploadSessionRequest) (AttachmentUploadSession, error) {
	if r.db == nil {
		return AttachmentUploadSession{}, fmt.Errorf("database handle is required")
	}
	if err := ValidateCancelAttachmentUploadSessionRequest(req); err != nil {
		return AttachmentUploadSession{}, err
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return AttachmentUploadSession{}, fmt.Errorf("begin attachment upload session cancel transaction: %w", err)
	}
	defer tx.Rollback()

	const query = `
WITH target AS (
  SELECT id, declared_size
  FROM attachment_upload_sessions
  WHERE user_id = $1
    AND id = $2
    AND status IN ('pending', 'uploading', 'failed')
  FOR UPDATE
)
UPDATE attachment_upload_sessions s
SET status = 'canceled',
    canceled_at = now(),
    updated_at = now()
FROM target
WHERE s.id = target.id
RETURNING
  s.id::text,
  s.user_id::text,
  COALESCE(s.draft_id::text, ''),
  s.upload_id,
  s.filename,
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

	var session AttachmentUploadSession
	var finalizedAt sql.NullTime
	var canceledAt sql.NullTime
	if err := tx.QueryRowContext(ctx, query, strings.TrimSpace(req.UserID), strings.TrimSpace(req.SessionID)).Scan(
		&session.ID,
		&session.UserID,
		&session.DraftID,
		&session.UploadID,
		&session.Filename,
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
		if err == sql.ErrNoRows {
			return AttachmentUploadSession{}, fmt.Errorf("attachment upload session %q not found for cancellation", req.SessionID)
		}
		return AttachmentUploadSession{}, fmt.Errorf("cancel attachment upload session: %w", err)
	}
	if finalizedAt.Valid {
		session.FinalizedAt = finalizedAt.Time
	}
	if canceledAt.Valid {
		session.CanceledAt = canceledAt.Time
	}
	if err := decrementUserQuota(ctx, tx, strings.TrimSpace(req.UserID), session.DeclaredSize); err != nil {
		return AttachmentUploadSession{}, err
	}
	if err := tx.Commit(); err != nil {
		return AttachmentUploadSession{}, fmt.Errorf("commit attachment upload session cancel transaction: %w", err)
	}
	return session, nil
}

func (r *Repository) GetAttachmentUploadSession(ctx context.Context, req GetAttachmentUploadSessionRequest) (AttachmentUploadSession, error) {
	if r.db == nil {
		return AttachmentUploadSession{}, fmt.Errorf("database handle is required")
	}
	if err := ValidateGetAttachmentUploadSessionRequest(req); err != nil {
		return AttachmentUploadSession{}, err
	}

	const query = `
SELECT
  id::text,
  user_id::text,
  COALESCE(draft_id::text, ''),
  upload_id,
  filename,
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
FROM attachment_upload_sessions
WHERE user_id = $1
  AND id = $2`

	session, err := scanAttachmentUploadSession(r.db.QueryRowContext(ctx, query, strings.TrimSpace(req.UserID), strings.TrimSpace(req.SessionID)))
	if err != nil {
		if err == sql.ErrNoRows {
			return AttachmentUploadSession{}, fmt.Errorf("attachment upload session %q not found", req.SessionID)
		}
		return AttachmentUploadSession{}, fmt.Errorf("get attachment upload session: %w", err)
	}
	return session, nil
}

func (r *Repository) StoreAttachmentUploadSessionBody(ctx context.Context, req StoreAttachmentUploadSessionBodyRequest) (AttachmentUploadSession, error) {
	if r.db == nil {
		return AttachmentUploadSession{}, fmt.Errorf("database handle is required")
	}
	if err := ValidateStoreAttachmentUploadSessionBodyRequest(req); err != nil {
		return AttachmentUploadSession{}, err
	}

	const query = `
UPDATE attachment_upload_sessions
SET status = 'uploading',
    received_size = $3,
    storage_path = $4,
    checksum_sha256 = $5,
    updated_at = now()
WHERE user_id = $1
  AND id = $2
  AND status IN ('pending', 'uploading', 'failed')
  AND expires_at > now()
  AND declared_size = $3
RETURNING
  id::text,
  user_id::text,
  COALESCE(draft_id::text, ''),
  upload_id,
  filename,
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
  canceled_at`

	session, err := scanAttachmentUploadSession(r.db.QueryRowContext(
		ctx,
		query,
		strings.TrimSpace(req.UserID),
		strings.TrimSpace(req.SessionID),
		req.ReceivedSize,
		strings.TrimSpace(req.StoragePath),
		strings.TrimSpace(req.ChecksumSHA256),
	))
	if err != nil {
		if err == sql.ErrNoRows {
			return AttachmentUploadSession{}, fmt.Errorf("attachment upload session %q not found for body storage", req.SessionID)
		}
		return AttachmentUploadSession{}, fmt.Errorf("store attachment upload session body: %w", err)
	}
	return session, nil
}

func (r *Repository) FinalizeAttachmentUploadSession(ctx context.Context, req FinalizeAttachmentUploadSessionRequest) (Attachment, error) {
	if r.db == nil {
		return Attachment{}, fmt.Errorf("database handle is required")
	}
	if err := ValidateFinalizeAttachmentUploadSessionRequest(req); err != nil {
		return Attachment{}, err
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return Attachment{}, fmt.Errorf("begin attachment upload session finalize transaction: %w", err)
	}
	defer tx.Rollback()

	const query = `
WITH target AS (
  SELECT *
  FROM attachment_upload_sessions
  WHERE user_id = $1
    AND id = $2
    AND status = 'uploading'
    AND received_size = declared_size
    AND storage_path <> ''
    AND checksum_sha256 <> ''
    AND expires_at > now()
  FOR UPDATE
),
inserted AS (
  INSERT INTO attachments (
    user_id, draft_id, upload_id, storage_path, filename, size, mime_type, status
  )
  SELECT
    user_id,
    draft_id,
    upload_id,
    storage_path,
    filename,
    declared_size,
    mime_type,
    'uploading'
  FROM target
  RETURNING id::text, COALESCE(message_id::text, '') AS message_id, upload_id, storage_path, filename, size, mime_type, status, created_at
),
finalized AS (
  UPDATE attachment_upload_sessions s
  SET status = 'finalized',
      finalized_at = now(),
      updated_at = now()
  FROM target
  WHERE s.id = target.id
  RETURNING COALESCE(target.draft_id::text, '') AS draft_id
)
SELECT
  inserted.id,
  inserted.message_id,
  inserted.upload_id,
  inserted.storage_path,
  inserted.filename,
  inserted.size,
  inserted.mime_type,
  inserted.status,
  inserted.created_at,
  finalized.draft_id
FROM inserted
CROSS JOIN finalized`

	var attachment Attachment
	var draftID string
	if err := tx.QueryRowContext(ctx, query, strings.TrimSpace(req.UserID), strings.TrimSpace(req.SessionID)).Scan(
		&attachment.ID,
		&attachment.MessageID,
		&attachment.UploadID,
		&attachment.StoragePath,
		&attachment.Filename,
		&attachment.Size,
		&attachment.MIMEType,
		&attachment.Status,
		&attachment.CreatedAt,
		&draftID,
	); err != nil {
		if err == sql.ErrNoRows {
			return Attachment{}, fmt.Errorf("attachment upload session %q not ready for finalization", req.SessionID)
		}
		return Attachment{}, fmt.Errorf("finalize attachment upload session: %w", err)
	}
	if strings.TrimSpace(draftID) != "" {
		if _, err := tx.ExecContext(ctx, `
UPDATE messages
SET has_attachment = EXISTS (
    SELECT 1
    FROM attachments
    WHERE user_id = $1
      AND draft_id = $2
      AND status = 'uploading'
  ),
  updated_at = now()
WHERE user_id = $1
  AND id = $2
  AND status = 'draft'`, strings.TrimSpace(req.UserID), strings.TrimSpace(draftID)); err != nil {
			return Attachment{}, fmt.Errorf("refresh draft attachment state: %w", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return Attachment{}, fmt.Errorf("commit attachment upload session finalize transaction: %w", err)
	}
	return attachment, nil
}

func (r *Repository) ExpireAttachmentUploadSessions(ctx context.Context, req ExpireAttachmentUploadSessionsRequest) ([]AttachmentUploadSession, error) {
	if r.db == nil {
		return nil, fmt.Errorf("database handle is required")
	}
	if err := ValidateExpireAttachmentUploadSessionsRequest(req); err != nil {
		return nil, err
	}
	limit := NormalizeAttachmentCleanupLimit(req.Limit)

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin attachment upload session expiry transaction: %w", err)
	}
	defer tx.Rollback()

	const selectQ = `
SELECT
  id::text,
  user_id::text,
  COALESCE(draft_id::text, ''),
  upload_id,
  filename,
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
FROM attachment_upload_sessions
WHERE status IN ('pending', 'uploading', 'failed')
  AND expires_at < $1
ORDER BY expires_at ASC, created_at ASC
LIMIT $2
FOR UPDATE SKIP LOCKED`

	rows, err := tx.QueryContext(ctx, selectQ, req.Before.UTC(), limit)
	if err != nil {
		return nil, fmt.Errorf("select expired attachment upload sessions: %w", err)
	}
	defer rows.Close()

	expired := make([]AttachmentUploadSession, 0)
	for rows.Next() {
		session, err := scanAttachmentUploadSession(rows)
		if err != nil {
			return nil, fmt.Errorf("scan expired attachment upload session: %w", err)
		}
		expired = append(expired, session)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate expired attachment upload sessions: %w", err)
	}
	if err := rows.Close(); err != nil {
		return nil, fmt.Errorf("close expired attachment upload session rows: %w", err)
	}

	for i := range expired {
		if _, err := tx.ExecContext(ctx, `
UPDATE attachment_upload_sessions
SET status = 'expired',
    updated_at = now()
WHERE id = $1
  AND status IN ('pending', 'uploading', 'failed')`, expired[i].ID); err != nil {
			return nil, fmt.Errorf("expire attachment upload session: %w", err)
		}
		if err := decrementUserQuota(ctx, tx, expired[i].UserID, expired[i].DeclaredSize); err != nil {
			return nil, err
		}
		expired[i].Status = "expired"
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit attachment upload session expiry transaction: %w", err)
	}
	return expired, nil
}

func (r *Repository) CountStaleAttachmentUploadSessions(ctx context.Context, req ExpireAttachmentUploadSessionsRequest) (StaleAttachmentUploadSessionCount, error) {
	if r.db == nil {
		return StaleAttachmentUploadSessionCount{}, fmt.Errorf("database handle is required")
	}
	if err := ValidateExpireAttachmentUploadSessionsRequest(req); err != nil {
		return StaleAttachmentUploadSessionCount{}, err
	}
	limit := NormalizeAttachmentCleanupLimit(req.Limit)

	const query = `
SELECT COUNT(*)
FROM attachment_upload_sessions
WHERE status IN ('pending', 'uploading', 'failed')
  AND expires_at < $1`

	var total int64
	if err := r.db.QueryRowContext(ctx, query, req.Before.UTC()).Scan(&total); err != nil {
		return StaleAttachmentUploadSessionCount{}, fmt.Errorf("count stale attachment upload sessions: %w", err)
	}
	limited := total
	if limited > int64(limit) {
		limited = int64(limit)
	}
	return StaleAttachmentUploadSessionCount{TotalCount: total, LimitedCount: limited}, nil
}

type attachmentUploadSessionScanner interface {
	Scan(dest ...any) error
}

func scanAttachmentUploadSession(scanner attachmentUploadSessionScanner) (AttachmentUploadSession, error) {
	var session AttachmentUploadSession
	var finalizedAt sql.NullTime
	var canceledAt sql.NullTime
	if err := scanner.Scan(
		&session.ID,
		&session.UserID,
		&session.DraftID,
		&session.UploadID,
		&session.Filename,
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
		return AttachmentUploadSession{}, err
	}
	if finalizedAt.Valid {
		session.FinalizedAt = finalizedAt.Time
	}
	if canceledAt.Valid {
		session.CanceledAt = canceledAt.Time
	}
	return session, nil
}
