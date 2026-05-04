package maildb

import (
	"context"
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
