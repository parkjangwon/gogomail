package maildb

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"
)

const (
	AttachmentShareLinkPermissionDownload = "download"

	AttachmentShareLinkStatusActive  = "active"
	AttachmentShareLinkStatusRevoked = "revoked"

	DefaultAttachmentShareLinkTTL = 7 * 24 * time.Hour
	MaxAttachmentShareLinkTTL     = 30 * 24 * time.Hour
)

var ErrAttachmentShareLinkPermissionDenied = errors.New("attachment share link does not allow download")

type AttachmentShareLink struct {
	ID           string    `json:"id"`
	UserID       string    `json:"user_id"`
	AttachmentID string    `json:"attachment_id"`
	Token        string    `json:"token,omitempty"`
	TokenSuffix  string    `json:"token_suffix"`
	Permission   string    `json:"permission"`
	Status       string    `json:"status"`
	ExpiresAt    time.Time `json:"expires_at"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
	RevokedAt    time.Time `json:"revoked_at,omitempty"`
}

type CreateAttachmentShareLinkRequest struct {
	UserID       string
	AttachmentID string
	Permission   string
	ExpiresAt    time.Time
}

type ListAttachmentShareLinksRequest struct {
	UserID       string
	AttachmentID string
	Status       string
	Limit        int
}

type RevokeAttachmentShareLinkRequest struct {
	UserID string
	LinkID string
}

type ResolveAttachmentShareLinkRequest struct {
	Token string
	Now   time.Time
}

type ResolvedAttachmentShareLink struct {
	ShareLink  AttachmentShareLink
	Attachment Attachment
}

func (r *Repository) CreateAttachmentShareLink(ctx context.Context, req CreateAttachmentShareLinkRequest) (AttachmentShareLink, error) {
	if r.db == nil {
		return AttachmentShareLink{}, fmt.Errorf("database handle is required")
	}

	token, err := generateAttachmentShareToken()
	if err != nil {
		return AttachmentShareLink{}, err
	}
	tokenHash, tokenSuffix, err := hashAttachmentShareToken(token)
	if err != nil {
		return AttachmentShareLink{}, err
	}

	now := time.Now().UTC()
	expiresAt := req.ExpiresAt.UTC()
	if expiresAt.IsZero() {
		expiresAt = now.Add(DefaultAttachmentShareLinkTTL)
	}

	const query = `
WITH target AS (
  SELECT a.id, a.user_id
  FROM attachments a
  JOIN users u ON u.id = a.user_id
  JOIN domains d ON d.id = u.domain_id
  WHERE a.id = $2::uuid
    AND a.user_id = $1::uuid
    AND a.status IN ('uploading', 'stored')
    AND u.status = 'active'
    AND d.status = 'active'
)
INSERT INTO attachment_share_links (
  user_id,
  attachment_id,
  token_hash,
  token_suffix,
  permission,
  status,
  expires_at
)
SELECT
  target.user_id,
  target.id,
  $3,
  $4,
  $5,
  'active',
  $6
FROM target
RETURNING
  id::text,
  user_id::text,
  attachment_id::text,
  token_suffix,
  permission,
  status,
  expires_at,
  created_at,
  updated_at,
  revoked_at`

	var link AttachmentShareLink
	var revokedAt sql.NullTime
	if err := r.db.QueryRowContext(
		ctx,
		query,
		strings.TrimSpace(req.UserID),
		strings.TrimSpace(req.AttachmentID),
		tokenHash,
		tokenSuffix,
		AttachmentShareLinkPermissionDownload,
		expiresAt,
	).Scan(
		&link.ID,
		&link.UserID,
		&link.AttachmentID,
		&link.TokenSuffix,
		&link.Permission,
		&link.Status,
		&link.ExpiresAt,
		&link.CreatedAt,
		&link.UpdatedAt,
		&revokedAt,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return AttachmentShareLink{}, fmt.Errorf("active attachment not found")
		}
		return AttachmentShareLink{}, fmt.Errorf("create attachment share link: %w", err)
	}
	link.Token = token
	return link, nil
}

func (r *Repository) ResolveAttachmentShareLink(ctx context.Context, req ResolveAttachmentShareLinkRequest) (ResolvedAttachmentShareLink, error) {
	if r.db == nil {
		return ResolvedAttachmentShareLink{}, fmt.Errorf("database handle is required")
	}
	tokenHash, _, err := hashAttachmentShareToken(req.Token)
	if err != nil {
		return ResolvedAttachmentShareLink{}, err
	}
	now := req.Now
	if now.IsZero() {
		now = time.Now().UTC()
	}

	const query = `
SELECT
  l.id::text,
  l.user_id::text,
  l.attachment_id::text,
  l.token_suffix,
  l.permission,
  l.status,
  l.expires_at,
  l.created_at,
  l.updated_at,
  l.revoked_at,
  a.id::text,
  COALESCE(a.message_id::text, ''),
  a.upload_id,
  a.storage_path,
  a.filename,
  a.size,
  a.mime_type,
  a.status,
  a.created_at
FROM attachment_share_links l
JOIN attachments a ON a.id = l.attachment_id AND a.user_id = l.user_id
JOIN users u ON u.id = l.user_id
JOIN domains d ON d.id = u.domain_id
WHERE l.token_hash = $1
  AND l.status = 'active'
  AND l.expires_at > $2
  AND a.status IN ('uploading', 'stored')
  AND u.status = 'active'
  AND d.status = 'active'`

	var resolved ResolvedAttachmentShareLink
	var revokedAt sql.NullTime
	if err := r.db.QueryRowContext(ctx, query, tokenHash, now.UTC()).Scan(
		&resolved.ShareLink.ID,
		&resolved.ShareLink.UserID,
		&resolved.ShareLink.AttachmentID,
		&resolved.ShareLink.TokenSuffix,
		&resolved.ShareLink.Permission,
		&resolved.ShareLink.Status,
		&resolved.ShareLink.ExpiresAt,
		&resolved.ShareLink.CreatedAt,
		&resolved.ShareLink.UpdatedAt,
		&revokedAt,
		&resolved.Attachment.ID,
		&resolved.Attachment.MessageID,
		&resolved.Attachment.UploadID,
		&resolved.Attachment.StoragePath,
		&resolved.Attachment.Filename,
		&resolved.Attachment.Size,
		&resolved.Attachment.MIMEType,
		&resolved.Attachment.Status,
		&resolved.Attachment.CreatedAt,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ResolvedAttachmentShareLink{}, fmt.Errorf("active attachment share link not found")
		}
		return ResolvedAttachmentShareLink{}, fmt.Errorf("resolve attachment share link: %w", err)
	}
	if revokedAt.Valid {
		resolved.ShareLink.RevokedAt = revokedAt.Time
	}
	return resolved, nil
}

func (r *Repository) ListAttachmentShareLinks(ctx context.Context, req ListAttachmentShareLinksRequest) ([]AttachmentShareLink, error) {
	if r.db == nil {
		return nil, fmt.Errorf("database handle is required")
	}
	query, args := buildListAttachmentShareLinksQuery(req)
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list attachment share links: %w", err)
	}
	defer rows.Close()

	links := make([]AttachmentShareLink, 0)
	for rows.Next() {
		var link AttachmentShareLink
		var revokedAt sql.NullTime
		if err := rows.Scan(
			&link.ID,
			&link.UserID,
			&link.AttachmentID,
			&link.TokenSuffix,
			&link.Permission,
			&link.Status,
			&link.ExpiresAt,
			&link.CreatedAt,
			&link.UpdatedAt,
			&revokedAt,
		); err != nil {
			return nil, fmt.Errorf("scan attachment share link: %w", err)
		}
		if revokedAt.Valid {
			link.RevokedAt = revokedAt.Time
		}
		links = append(links, link)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate attachment share links: %w", err)
	}
	return links, nil
}

const listAttachmentShareLinksBaseSQL = `
SELECT
  id::text,
  user_id::text,
  attachment_id::text,
  token_suffix,
  permission,
  status,
  expires_at,
  created_at,
  updated_at,
  revoked_at
FROM attachment_share_links
WHERE user_id = $1::uuid`

func buildListAttachmentShareLinksQuery(req ListAttachmentShareLinksRequest) (string, []any) {
	args := []any{strings.TrimSpace(req.UserID)}
	conditions := make([]string, 0, 2)
	if attachmentID := strings.TrimSpace(req.AttachmentID); attachmentID != "" {
		args = append(args, attachmentID)
		conditions = append(conditions, fmt.Sprintf("attachment_id = $%d::uuid", len(args)))
	}
	if status := strings.TrimSpace(req.Status); status != "" {
		args = append(args, status)
		conditions = append(conditions, fmt.Sprintf("status = $%d", len(args)))
	}
	args = append(args, normalizeLimit(req.Limit))
	query := listAttachmentShareLinksBaseSQL
	if len(conditions) > 0 {
		query += "\n  AND " + strings.Join(conditions, "\n  AND ")
	}
	query += fmt.Sprintf(`
ORDER BY updated_at DESC, id DESC
LIMIT $%d`, len(args))
	return query, args
}

func (r *Repository) RevokeAttachmentShareLink(ctx context.Context, req RevokeAttachmentShareLinkRequest) (AttachmentShareLink, error) {
	if r.db == nil {
		return AttachmentShareLink{}, fmt.Errorf("database handle is required")
	}

	const query = `
UPDATE attachment_share_links
SET status = 'revoked',
    revoked_at = now(),
    updated_at = now()
WHERE id = $2::uuid
  AND user_id = $1::uuid
  AND status = 'active'
RETURNING
  id::text,
  user_id::text,
  attachment_id::text,
  token_suffix,
  permission,
  status,
  expires_at,
  created_at,
  updated_at,
  revoked_at`

	var link AttachmentShareLink
	var revokedAt sql.NullTime
	if err := r.db.QueryRowContext(ctx, query, strings.TrimSpace(req.UserID), strings.TrimSpace(req.LinkID)).Scan(
		&link.ID,
		&link.UserID,
		&link.AttachmentID,
		&link.TokenSuffix,
		&link.Permission,
		&link.Status,
		&link.ExpiresAt,
		&link.CreatedAt,
		&link.UpdatedAt,
		&revokedAt,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return AttachmentShareLink{}, fmt.Errorf("active attachment share link not found")
		}
		return AttachmentShareLink{}, fmt.Errorf("revoke attachment share link: %w", err)
	}
	if revokedAt.Valid {
		link.RevokedAt = revokedAt.Time
	}
	return link, nil
}

func generateAttachmentShareToken() (string, error) {
	var random [32]byte
	if _, err := rand.Read(random[:]); err != nil {
		return "", fmt.Errorf("generate attachment share token: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(random[:]), nil
}

func hashAttachmentShareToken(token string) (string, string, error) {
	token = strings.TrimSpace(token)
	if token == "" {
		return "", "", fmt.Errorf("attachment share token is required")
	}
	if len(token) < 32 {
		return "", "", fmt.Errorf("attachment share token is too short")
	}
	sum := sha256.Sum256([]byte(token))
	suffix := token
	if len(suffix) > 8 {
		suffix = suffix[len(suffix)-8:]
	}
	return hex.EncodeToString(sum[:]), suffix, nil
}
