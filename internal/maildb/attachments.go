package maildb

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"fmt"
	"strings"
	"time"
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

	type staleAttachment struct {
		Attachment
		UserID string
	}
	stale := make([]staleAttachment, 0)
	for rows.Next() {
		var item staleAttachment
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
	for _, item := range stale {
		if _, err := tx.ExecContext(ctx, `
UPDATE attachments
SET status = 'deleted'
WHERE id = $1
  AND status = 'uploading'`, item.ID); err != nil {
			return nil, fmt.Errorf("expire stale attachment upload: %w", err)
		}
		if err := decrementUserQuota(ctx, tx, item.UserID, item.Size); err != nil {
			return nil, err
		}
		item.Status = "deleted"
		attachments = append(attachments, item.Attachment)
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit stale attachment cleanup transaction: %w", err)
	}
	return attachments, nil
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
