package maildb

import (
	"context"
	"database/sql"
	"fmt"
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
