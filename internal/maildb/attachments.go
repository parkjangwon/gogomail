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

func (r *Repository) CreateAttachmentUpload(ctx context.Context, req CreateAttachmentUploadRequest) (Attachment, error) {
	if r.db == nil {
		return Attachment{}, fmt.Errorf("database handle is required")
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
	if err := r.db.QueryRowContext(
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
	return strings.Join([]string{"uploads", strings.TrimSpace(userID), uploadID, filename}, "/")
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
