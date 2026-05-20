package maildb

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"github.com/lib/pq"
)

type Attachment struct {
	ID          string    `json:"id"`
	MessageID   string    `json:"message_id"`
	UploadID    string    `json:"upload_id"`
	StoragePath string    `json:"storage_path"`
	Filename    string    `json:"filename"`
	Size        int64     `json:"size"`
	MIMEType    string    `json:"mime_type"`
	Status      string    `json:"status"`
	CreatedAt   time.Time `json:"created_at"`
}

type ExpireStaleAttachmentUploadsRequest struct {
	Before time.Time
	Limit  int
}

type StaleAttachmentUploadCount struct {
	TotalCount   int64
	LimitedCount int64
}

type StaleAttachmentUploadCandidate struct {
	ID          string    `json:"id"`
	UserID      string    `json:"user_id"`
	MessageID   string    `json:"message_id"`
	UploadID    string    `json:"upload_id"`
	StoragePath string    `json:"storage_path"`
	Filename    string    `json:"filename"`
	Size        int64     `json:"size"`
	MIMEType    string    `json:"mime_type"`
	Status      string    `json:"status"`
	CreatedAt   time.Time `json:"created_at"`
}

const (
	AttachmentCleanupDefaultLimit = 100
	AttachmentCleanupMaxLimit     = 1000
)

func ValidateExpireStaleAttachmentUploadsRequest(req ExpireStaleAttachmentUploadsRequest) error {
	if req.Before.IsZero() {
		return fmt.Errorf("before is required")
	}
	if req.Limit < 0 {
		return fmt.Errorf("limit must not be negative")
	}
	return nil
}

func (r *Repository) CreateAttachmentUpload(ctx context.Context, req CreateAttachmentUploadRequest) (Attachment, error) {
	if r.db == nil {
		return Attachment{}, fmt.Errorf("database handle is required")
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return Attachment{}, fmt.Errorf("begin attachment upload transaction: %w", err)
	}
	defer tx.Rollback()

	if err := checkAndIncrementUserQuota(ctx, tx, req.UserID, req.Size); err != nil {
		return Attachment{}, err
	}

	uploadID := newUploadID()
	storagePath := strings.TrimSpace(req.StoragePath)
	if storagePath == "" {
		storagePath = attachmentUploadStoragePath(req.UserID, uploadID, req.Filename)
	}

	const query = `
INSERT INTO attachments (
  user_id, draft_id, upload_id, storage_path, filename, size, mime_type, status
) VALUES (
  $1,
  NULLIF($2, '')::uuid,
  $3, $4, $5, $6, $7, 'uploading'
) RETURNING id::text, COALESCE(message_id::text, ''), upload_id, storage_path, filename, size, mime_type, status, created_at`

	var attachment Attachment
	if err := tx.QueryRowContext(
		ctx,
		query,
		strings.TrimSpace(req.UserID),
		strings.TrimSpace(req.DraftID),
		uploadID,
		storagePath,
		strings.TrimSpace(req.Filename),
		req.Size,
		strings.TrimSpace(req.MIMEType),
	).Scan(
		&attachment.ID,
		&attachment.MessageID,
		&attachment.UploadID,
		&attachment.StoragePath,
		&attachment.Filename,
		&attachment.Size,
		&attachment.MIMEType,
		&attachment.Status,
		&attachment.CreatedAt,
	); err != nil {
		return Attachment{}, fmt.Errorf("create attachment upload: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return Attachment{}, fmt.Errorf("commit attachment upload transaction: %w", err)
	}
	return attachment, nil
}

func (r *Repository) ListAttachments(ctx context.Context, userID string, messageID string) ([]Attachment, error) {
	if r.db == nil {
		return nil, fmt.Errorf("database handle is required")
	}

	const query = `
SELECT
  a.id::text,
  a.message_id::text,
  a.upload_id,
  a.storage_path,
  a.filename,
  a.size,
  a.mime_type,
  a.status,
  a.created_at
FROM attachments a
JOIN messages m ON m.id = a.message_id
WHERE m.user_id = $1
  AND m.id = $2
  AND m.status = 'active'
ORDER BY a.created_at ASC, a.filename ASC`

	rows, err := r.db.QueryContext(ctx, query, userID, messageID)
	if err != nil {
		return nil, fmt.Errorf("list attachments: %w", err)
	}
	defer rows.Close()

	attachments := make([]Attachment, 0)
	for rows.Next() {
		var attachment Attachment
		if err := rows.Scan(
			&attachment.ID,
			&attachment.MessageID,
			&attachment.UploadID,
			&attachment.StoragePath,
			&attachment.Filename,
			&attachment.Size,
			&attachment.MIMEType,
			&attachment.Status,
			&attachment.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan attachment: %w", err)
		}
		attachments = append(attachments, attachment)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate attachments: %w", err)
	}
	return attachments, nil
}

func newUploadID() string {
	var random [16]byte
	if _, err := rand.Read(random[:]); err != nil {
		return fmt.Sprintf("upload-%d", time.Now().UnixNano())
	}
	return "upload-" + hex.EncodeToString(random[:])
}

func attachmentUploadStoragePath(userID string, uploadID string, filename string) string {
	filename = strings.ReplaceAll(strings.TrimSpace(filename), "/", "_")
	filename = strings.ReplaceAll(filename, `\`, "_")
	if filename == "" {
		filename = "attachment"
	}
	return strings.Join([]string{"uploads", safeAttachmentPathSegment(userID), safeAttachmentPathSegment(uploadID), filename}, "/")
}

func safeAttachmentPathSegment(value string) string {
	value = strings.TrimSpace(value)
	var b strings.Builder
	b.Grow(len(value))
	lastUnderscore := false
	for _, r := range value {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_' || r == '.' {
			b.WriteRune(r)
			lastUnderscore = false
			continue
		}
		if !lastUnderscore {
			b.WriteByte('_')
			lastUnderscore = true
		}
	}
	out := strings.Trim(b.String(), "_.")
	if out == "" {
		return "unknown"
	}
	return out
}

func (r *Repository) GetAttachment(ctx context.Context, userID string, messageID string, attachmentID string) (Attachment, error) {
	if r.db == nil {
		return Attachment{}, fmt.Errorf("database handle is required")
	}

	const query = `
SELECT
  a.id::text,
  a.message_id::text,
  a.upload_id,
  a.storage_path,
  a.filename,
  a.size,
  a.mime_type,
  a.status,
  a.created_at
FROM attachments a
JOIN messages m ON m.id = a.message_id
WHERE m.user_id = $1
  AND m.id = $2
  AND m.status = 'active'
  AND a.id = $3
LIMIT 1`

	var attachment Attachment
	err := r.db.QueryRowContext(ctx, query, userID, messageID, attachmentID).Scan(
		&attachment.ID,
		&attachment.MessageID,
		&attachment.UploadID,
		&attachment.StoragePath,
		&attachment.Filename,
		&attachment.Size,
		&attachment.MIMEType,
		&attachment.Status,
		&attachment.CreatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return Attachment{}, fmt.Errorf("attachment %q not found", attachmentID)
		}
		return Attachment{}, fmt.Errorf("get attachment: %w", err)
	}
	return attachment, nil
}

const attachmentsByIDsSQL = `
WITH requested AS (
  SELECT value AS id, ordinality
  FROM unnest($2::uuid[]) WITH ORDINALITY AS requested(value, ordinality)
)
SELECT
  attachments.id::text,
  COALESCE(attachments.message_id::text, ''),
  attachments.upload_id,
  attachments.storage_path,
  attachments.filename,
  attachments.size,
  attachments.mime_type,
  attachments.status,
  attachments.created_at
FROM attachments
JOIN requested ON requested.id = attachments.id
WHERE attachments.user_id = $1::uuid
  AND attachments.message_id IS NULL
  AND attachments.status = 'uploading'
ORDER BY requested.ordinality, attachments.created_at ASC, attachments.filename ASC`

func (r *Repository) AttachmentsByIDs(ctx context.Context, userID string, attachmentIDs []string) ([]Attachment, error) {
	if r.db == nil {
		return nil, fmt.Errorf("database handle is required")
	}
	if len(attachmentIDs) == 0 {
		return nil, nil
	}

	rows, err := r.db.QueryContext(ctx, attachmentsByIDsSQL, strings.TrimSpace(userID), pq.Array(attachmentIDs))
	if err != nil {
		return nil, fmt.Errorf("list attachments by ids: %w", err)
	}
	defer rows.Close()

	attachments := make([]Attachment, 0, len(attachmentIDs))
	for rows.Next() {
		var attachment Attachment
		if err := rows.Scan(
			&attachment.ID,
			&attachment.MessageID,
			&attachment.UploadID,
			&attachment.StoragePath,
			&attachment.Filename,
			&attachment.Size,
			&attachment.MIMEType,
			&attachment.Status,
			&attachment.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan attachment by id: %w", err)
		}
		attachments = append(attachments, attachment)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate attachments by ids: %w", err)
	}
	if len(attachments) != len(attachmentIDs) {
		return nil, fmt.Errorf("one or more attachments were not found")
	}
	return attachments, nil
}

func (r *Repository) CancelAttachmentUpload(ctx context.Context, userID string, attachmentID string) (Attachment, error) {
	if r.db == nil {
		return Attachment{}, fmt.Errorf("database handle is required")
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return Attachment{}, fmt.Errorf("begin attachment cancel transaction: %w", err)
	}
	defer tx.Rollback()

	const query = `
WITH target AS (
  SELECT id, draft_id
  FROM attachments
  WHERE user_id = $1
    AND id = $2
    AND status = 'uploading'
    AND message_id IS NULL
  FOR UPDATE
)
UPDATE attachments a
SET status = 'deleted',
    draft_id = NULL
FROM target
WHERE a.id = target.id
RETURNING a.id::text, COALESCE(a.message_id::text, ''), COALESCE(target.draft_id::text, ''), a.upload_id, a.storage_path, a.filename, a.size, a.mime_type, a.status, a.created_at`

	var attachment Attachment
	var draftID string
	if err := tx.QueryRowContext(ctx, query, strings.TrimSpace(userID), strings.TrimSpace(attachmentID)).Scan(
		&attachment.ID,
		&attachment.MessageID,
		&draftID,
		&attachment.UploadID,
		&attachment.StoragePath,
		&attachment.Filename,
		&attachment.Size,
		&attachment.MIMEType,
		&attachment.Status,
		&attachment.CreatedAt,
	); err != nil {
		if err == sql.ErrNoRows {
			return Attachment{}, fmt.Errorf("attachment %q not found for active upload", attachmentID)
		}
		return Attachment{}, fmt.Errorf("cancel attachment upload: %w", err)
	}
	if err := decrementUserQuota(ctx, tx, strings.TrimSpace(userID), attachment.Size); err != nil {
		return Attachment{}, err
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
  AND status = 'draft'`, strings.TrimSpace(userID), strings.TrimSpace(draftID)); err != nil {
			return Attachment{}, fmt.Errorf("refresh draft attachment state: %w", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return Attachment{}, fmt.Errorf("commit attachment cancel transaction: %w", err)
	}
	return attachment, nil
}

func (r *Repository) ExpireStaleAttachmentUploads(ctx context.Context, req ExpireStaleAttachmentUploadsRequest) ([]Attachment, error) {
	if r.db == nil {
		return nil, fmt.Errorf("database handle is required")
	}
	if err := ValidateExpireStaleAttachmentUploadsRequest(req); err != nil {
		return nil, err
	}
	limit := NormalizeAttachmentCleanupLimit(req.Limit)

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin stale attachment cleanup transaction: %w", err)
	}
	defer tx.Rollback()

	const selectQ = `
SELECT
  id::text,
  user_id::text,
  COALESCE(message_id::text, ''),
  upload_id,
  storage_path,
  filename,
  size,
  mime_type,
  status,
  created_at
FROM attachments
WHERE status = 'uploading'
  AND created_at < $1
ORDER BY created_at ASC
LIMIT $2
FOR UPDATE SKIP LOCKED`

	rows, err := tx.QueryContext(ctx, selectQ, req.Before.UTC(), limit)
	if err != nil {
		return nil, fmt.Errorf("select stale attachment uploads: %w", err)
	}
	defer rows.Close()

	stale := make([]staleAttachmentUpload, 0)
	for rows.Next() {
		var item staleAttachmentUpload
		if err := rows.Scan(
			&item.ID,
			&item.UserID,
			&item.MessageID,
			&item.UploadID,
			&item.StoragePath,
			&item.Filename,
			&item.Size,
			&item.MIMEType,
			&item.Status,
			&item.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan stale attachment upload: %w", err)
		}
		stale = append(stale, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate stale attachment uploads: %w", err)
	}
	if err := rows.Close(); err != nil {
		return nil, fmt.Errorf("close stale attachment rows: %w", err)
	}

	attachments := make([]Attachment, 0)
	if err := markExpiredStaleAttachmentUploads(ctx, tx, stale); err != nil {
		return nil, err
	}
	if err := decrementExpiredStaleAttachmentUploadQuota(ctx, tx, stale); err != nil {
		return nil, err
	}
	for _, item := range stale {
		item.Status = "deleted"
		attachments = append(attachments, item.Attachment)
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit stale attachment cleanup transaction: %w", err)
	}
	return attachments, nil
}

type staleAttachmentUpload struct {
	Attachment
	UserID string
}

const expireStaleAttachmentUploadsSQL = `
WITH input AS (
  SELECT value AS id
  FROM unnest($1::uuid[]) AS input(value)
)
UPDATE attachments a
SET status = 'deleted'
FROM input
WHERE a.id = input.id
  AND a.status = 'uploading'`

func markExpiredStaleAttachmentUploads(ctx context.Context, tx *sql.Tx, stale []staleAttachmentUpload) error {
	if len(stale) == 0 {
		return nil
	}
	ids := make([]string, 0, len(stale))
	for _, item := range stale {
		ids = append(ids, item.ID)
	}
	if _, err := tx.ExecContext(ctx, expireStaleAttachmentUploadsSQL, pq.Array(ids)); err != nil {
		return fmt.Errorf("expire stale attachment uploads: %w", err)
	}
	return nil
}

func decrementExpiredStaleAttachmentUploadQuota(ctx context.Context, tx *sql.Tx, stale []staleAttachmentUpload) error {
	if len(stale) == 0 {
		return nil
	}
	userIDs := make([]string, 0, len(stale))
	sizes := make([]int64, 0, len(stale))
	for _, item := range stale {
		if item.Size <= 0 {
			continue
		}
		userIDs = append(userIDs, item.UserID)
		sizes = append(sizes, item.Size)
	}
	if len(userIDs) == 0 {
		return nil
	}
	if err := decrementUserQuotas(ctx, tx, userIDs, sizes); err != nil {
		return fmt.Errorf("decrement expired stale attachment upload quota: %w", err)
	}
	return nil
}

func (r *Repository) CountStaleAttachmentUploads(ctx context.Context, req ExpireStaleAttachmentUploadsRequest) (StaleAttachmentUploadCount, error) {
	if r.db == nil {
		return StaleAttachmentUploadCount{}, fmt.Errorf("database handle is required")
	}
	if err := ValidateExpireStaleAttachmentUploadsRequest(req); err != nil {
		return StaleAttachmentUploadCount{}, err
	}
	limit := NormalizeAttachmentCleanupLimit(req.Limit)

	const query = `
SELECT COUNT(*)
FROM attachments
WHERE status = 'uploading'
  AND created_at < $1`

	var total int64
	if err := r.db.QueryRowContext(ctx, query, req.Before.UTC()).Scan(&total); err != nil {
		return StaleAttachmentUploadCount{}, fmt.Errorf("count stale attachment uploads: %w", err)
	}
	limited := total
	if limited > int64(limit) {
		limited = int64(limit)
	}
	return StaleAttachmentUploadCount{TotalCount: total, LimitedCount: limited}, nil
}

func (r *Repository) ListStaleAttachmentUploads(ctx context.Context, req ExpireStaleAttachmentUploadsRequest) ([]StaleAttachmentUploadCandidate, error) {
	if r.db == nil {
		return nil, fmt.Errorf("database handle is required")
	}
	if err := ValidateExpireStaleAttachmentUploadsRequest(req); err != nil {
		return nil, err
	}
	limit := NormalizeAttachmentCleanupLimit(req.Limit)

	const query = `
SELECT
  id::text,
  user_id::text,
  COALESCE(message_id::text, ''),
  upload_id,
  storage_path,
  filename,
  size,
  mime_type,
  status,
  created_at
FROM attachments
WHERE status = 'uploading'
  AND created_at < $1
ORDER BY created_at ASC
LIMIT $2`

	rows, err := r.db.QueryContext(ctx, query, req.Before.UTC(), limit)
	if err != nil {
		return nil, fmt.Errorf("list stale attachment uploads: %w", err)
	}
	defer rows.Close()

	candidates := make([]StaleAttachmentUploadCandidate, 0)
	for rows.Next() {
		var candidate StaleAttachmentUploadCandidate
		if err := rows.Scan(
			&candidate.ID,
			&candidate.UserID,
			&candidate.MessageID,
			&candidate.UploadID,
			&candidate.StoragePath,
			&candidate.Filename,
			&candidate.Size,
			&candidate.MIMEType,
			&candidate.Status,
			&candidate.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan stale attachment upload candidate: %w", err)
		}
		candidates = append(candidates, candidate)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate stale attachment upload candidates: %w", err)
	}
	return candidates, nil
}

func NormalizeAttachmentCleanupLimit(limit int) int {
	if limit <= 0 {
		return AttachmentCleanupDefaultLimit
	}
	if limit > AttachmentCleanupMaxLimit {
		return AttachmentCleanupMaxLimit
	}
	return limit
}
