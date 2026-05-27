package drive

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

	"github.com/gogomail/gogomail/internal/auth"
)

const (
	ShareLinkPermissionView     = "view"
	ShareLinkPermissionDownload = "download"

	ShareLinkStatusActive  = "active"
	ShareLinkStatusRevoked = "revoked"

	DefaultShareLinkTTL       = 7 * 24 * time.Hour
	MaxShareLinkTTL           = 30 * 24 * time.Hour
	MaxShareLinkTokenBytes    = 256
	MaxShareLinkPasswordBytes = 256
)

var ErrShareLinkPermissionDenied = errors.New("drive share link does not allow download")
var ErrShareLinkPasswordRequired = errors.New("drive share password is required")
var ErrShareLinkPasswordInvalid = errors.New("drive share password is invalid")

type ShareLink struct {
	ID                string    `json:"id"`
	UserID            string    `json:"user_id"`
	NodeID            string    `json:"node_id"`
	Token             string    `json:"token,omitempty"`
	TokenSuffix       string    `json:"token_suffix"`
	Permission        string    `json:"permission"`
	Status            string    `json:"status"`
	ExpiresAt         time.Time `json:"expires_at"`
	CreatedAt         time.Time `json:"created_at"`
	UpdatedAt         time.Time `json:"updated_at"`
	PasswordProtected bool      `json:"password_protected"`
	RevokedAt         time.Time `json:"revoked_at,omitempty"`
}

type CreateShareLinkRequest struct {
	UserID     string
	NodeID     string
	Permission string
	ExpiresAt  time.Time
	Token      string
	Password   string
}

type ListShareLinksRequest struct {
	UserID string
	NodeID string
	Status string
	Limit  int
}

type RevokeShareLinkRequest struct {
	UserID string
	LinkID string
}

type ResolveShareLinkRequest struct {
	Token    string
	Password string
	Now      time.Time
}

type ResolvedShareLink struct {
	ShareLink ShareLink
	Node      Node
}

func NewShareLinkToken() (string, error) {
	var random [32]byte
	if _, err := rand.Read(random[:]); err != nil {
		return "", fmt.Errorf("generate drive share token: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(random[:]), nil
}

func ValidateCreateShareLinkRequest(req CreateShareLinkRequest, now time.Time) (CreateShareLinkRequest, string, error) {
	userID, err := validateDriveID("user_id", req.UserID, true)
	if err != nil {
		return CreateShareLinkRequest{}, "", err
	}
	nodeID, err := validateDriveID("node_id", req.NodeID, true)
	if err != nil {
		return CreateShareLinkRequest{}, "", err
	}
	permission, err := ValidateShareLinkPermission(req.Permission)
	if err != nil {
		return CreateShareLinkRequest{}, "", err
	}
	if now.IsZero() {
		now = time.Now().UTC()
	}
	now = now.UTC()
	expiresAt := req.ExpiresAt.UTC()
	if expiresAt.IsZero() {
		expiresAt = now.Add(DefaultShareLinkTTL)
	}
	if !expiresAt.After(now) {
		return CreateShareLinkRequest{}, "", fmt.Errorf("expires_at must be in the future")
	}
	if expiresAt.After(now.Add(MaxShareLinkTTL)) {
		return CreateShareLinkRequest{}, "", fmt.Errorf("expires_at exceeds maximum drive share link TTL")
	}
	token := req.Token
	if strings.TrimSpace(token) == "" {
		token, err = NewShareLinkToken()
		if err != nil {
			return CreateShareLinkRequest{}, "", err
		}
	}
	tokenHash, suffix, err := hashShareLinkToken(token)
	if err != nil {
		return CreateShareLinkRequest{}, "", err
	}
	passwordHash, err := hashOptionalShareLinkPassword(req.Password)
	if err != nil {
		return CreateShareLinkRequest{}, "", err
	}
	return CreateShareLinkRequest{
		UserID:     userID,
		NodeID:     nodeID,
		Permission: permission,
		ExpiresAt:  expiresAt,
		Token:      token,
		Password:   passwordHash,
	}, tokenHash + ":" + suffix, nil
}

func ValidateListShareLinksRequest(req ListShareLinksRequest) (ListShareLinksRequest, error) {
	userID, err := validateDriveID("user_id", req.UserID, true)
	if err != nil {
		return ListShareLinksRequest{}, err
	}
	nodeID, err := validateDriveID("node_id", req.NodeID, false)
	if err != nil {
		return ListShareLinksRequest{}, err
	}
	status := strings.TrimSpace(req.Status)
	if status == "" {
		status = ShareLinkStatusActive
	}
	status, err = ValidateShareLinkStatus(status)
	if err != nil {
		return ListShareLinksRequest{}, err
	}
	if req.Limit < 0 {
		return ListShareLinksRequest{}, fmt.Errorf("limit must not be negative")
	}
	return ListShareLinksRequest{
		UserID: userID,
		NodeID: nodeID,
		Status: status,
		Limit:  normalizeDriveListLimit(req.Limit),
	}, nil
}

func ValidateRevokeShareLinkRequest(req RevokeShareLinkRequest) (RevokeShareLinkRequest, error) {
	userID, err := validateDriveID("user_id", req.UserID, true)
	if err != nil {
		return RevokeShareLinkRequest{}, err
	}
	linkID, err := validateDriveID("link_id", req.LinkID, true)
	if err != nil {
		return RevokeShareLinkRequest{}, err
	}
	return RevokeShareLinkRequest{UserID: userID, LinkID: linkID}, nil
}

func ValidateResolveShareLinkRequest(req ResolveShareLinkRequest) (ResolveShareLinkRequest, string, error) {
	tokenHash, _, err := hashShareLinkToken(req.Token)
	if err != nil {
		return ResolveShareLinkRequest{}, "", err
	}
	now := req.Now
	if now.IsZero() {
		now = time.Now().UTC()
	}
	return ResolveShareLinkRequest{Token: req.Token, Password: req.Password, Now: now.UTC()}, tokenHash, nil
}

func hashOptionalShareLinkPassword(password string) (string, error) {
	password = strings.TrimSpace(password)
	if password == "" {
		return "", nil
	}
	if len(password) > MaxShareLinkPasswordBytes {
		return "", fmt.Errorf("drive share password is too long")
	}
	if strings.ContainsAny(password, "\r\n") {
		return "", fmt.Errorf("drive share password must not contain line breaks")
	}
	var salt [16]byte
	if _, err := rand.Read(salt[:]); err != nil {
		return "", fmt.Errorf("generate drive share password salt: %w", err)
	}
	return auth.HashPasswordPBKDF2SHA256(password, salt[:], 210_000)
}

func ValidateShareLinkPermission(permission string) (string, error) {
	permission = strings.TrimSpace(strings.ToLower(permission))
	if permission == "" {
		return ShareLinkPermissionView, nil
	}
	switch permission {
	case ShareLinkPermissionView, ShareLinkPermissionDownload:
		return permission, nil
	default:
		return "", fmt.Errorf("unsupported drive share link permission %q", permission)
	}
}

func ValidateShareLinkStatus(status string) (string, error) {
	status = strings.TrimSpace(strings.ToLower(status))
	switch status {
	case ShareLinkStatusActive, ShareLinkStatusRevoked:
		return status, nil
	default:
		return "", fmt.Errorf("unsupported drive share link status %q", status)
	}
}

func hashShareLinkToken(token string) (string, string, error) {
	if token != strings.TrimSpace(token) {
		return "", "", fmt.Errorf("drive share token must not contain surrounding whitespace")
	}
	if len(token) < 32 {
		return "", "", fmt.Errorf("drive share token is too short")
	}
	if len(token) > MaxShareLinkTokenBytes {
		return "", "", fmt.Errorf("drive share token is too long")
	}
	if strings.ContainsAny(token, "\r\n\t ") {
		return "", "", fmt.Errorf("drive share token must not contain whitespace")
	}
	for _, r := range token {
		if r < 0x21 || r > 0x7e {
			return "", "", fmt.Errorf("drive share token must be printable ASCII")
		}
	}
	sum := sha256.Sum256([]byte(token))
	suffix := token
	if len(suffix) > 8 {
		suffix = suffix[len(suffix)-8:]
	}
	return hex.EncodeToString(sum[:]), suffix, nil
}

func (r *Repository) ResolveShareLink(ctx context.Context, req ResolveShareLinkRequest) (ResolvedShareLink, error) {
	if r == nil || r.db == nil {
		return ResolvedShareLink{}, fmt.Errorf("database handle is required")
	}
	req, tokenHash, err := ValidateResolveShareLinkRequest(req)
	if err != nil {
		return ResolvedShareLink{}, err
	}
	const query = `
SELECT
  l.id::text,
  l.user_id::text,
  l.node_id::text,
  l.token_suffix,
  l.permission,
  COALESCE(l.password_hash, '') <> '',
  l.password_hash,
  l.status,
  l.expires_at,
  l.created_at,
  l.updated_at,
  l.revoked_at,
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
FROM drive_share_links l
JOIN drive_nodes n ON n.id = l.node_id AND n.user_id = l.user_id
JOIN users u ON u.id = l.user_id
JOIN domains d ON d.id = u.domain_id
WHERE l.token_hash = $1
  AND l.status = 'active'
  AND l.expires_at > $2
  AND n.node_type = 'file'
  AND n.status = 'active'
  AND u.status = 'active'
  AND d.status = 'active'`
	var resolved ResolvedShareLink
	var passwordHash string
	var revokedAt sql.NullTime
	err = r.db.QueryRowContext(ctx, query, tokenHash, req.Now).Scan(
		&resolved.ShareLink.ID,
		&resolved.ShareLink.UserID,
		&resolved.ShareLink.NodeID,
		&resolved.ShareLink.TokenSuffix,
		&resolved.ShareLink.Permission,
		&resolved.ShareLink.PasswordProtected,
		&passwordHash,
		&resolved.ShareLink.Status,
		&resolved.ShareLink.ExpiresAt,
		&resolved.ShareLink.CreatedAt,
		&resolved.ShareLink.UpdatedAt,
		&revokedAt,
		&resolved.Node.ID,
		&resolved.Node.CompanyID,
		&resolved.Node.DomainID,
		&resolved.Node.UserID,
		&resolved.Node.ParentID,
		&resolved.Node.Type,
		&resolved.Node.Name,
		&resolved.Node.NormalizedName,
		&resolved.Node.MIMEType,
		&resolved.Node.Size,
		&resolved.Node.StorageBackend,
		&resolved.Node.StoragePath,
		&resolved.Node.ChecksumSHA256,
		&resolved.Node.Status,
		&resolved.Node.CreatedAt,
		&resolved.Node.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ResolvedShareLink{}, fmt.Errorf("active drive share link not found")
		}
		return ResolvedShareLink{}, fmt.Errorf("resolve drive share link: %w", err)
	}
	if revokedAt.Valid {
		resolved.ShareLink.RevokedAt = revokedAt.Time
	}
	if passwordHash != "" {
		password := strings.TrimSpace(req.Password)
		if password == "" {
			return ResolvedShareLink{}, ErrShareLinkPasswordRequired
		}
		if !auth.VerifyPasswordHash(password, passwordHash) {
			return ResolvedShareLink{}, ErrShareLinkPasswordInvalid
		}
	}
	return resolved, nil
}

func (r *Repository) CreateShareLink(ctx context.Context, req CreateShareLinkRequest) (ShareLink, error) {
	if r == nil || r.db == nil {
		return ShareLink{}, fmt.Errorf("database handle is required")
	}
	req, tokenDigest, err := ValidateCreateShareLinkRequest(req, time.Now().UTC())
	if err != nil {
		return ShareLink{}, err
	}
	tokenHash, tokenSuffix, ok := strings.Cut(tokenDigest, ":")
	if !ok {
		return ShareLink{}, fmt.Errorf("drive share token digest is invalid")
	}
	const query = `
WITH target AS (
  SELECT n.id, n.user_id
  FROM drive_nodes n
  JOIN users u ON u.id = n.user_id
  JOIN domains d ON d.id = u.domain_id
  WHERE n.id = $2::uuid
    AND n.user_id = $1::uuid
    AND n.node_type = 'file'
    AND n.status = 'active'
    AND u.status = 'active'
    AND d.status = 'active'
)
INSERT INTO drive_share_links (
  user_id,
  node_id,
  token_hash,
  token_suffix,
  permission,
  password_hash,
  status,
  expires_at
)
SELECT
  target.user_id,
  target.id,
  $3,
  $4,
  $5,
  $6,
  'active',
  $7
FROM target
RETURNING
  id::text,
  user_id::text,
  node_id::text,
  token_suffix,
  permission,
  COALESCE(password_hash, '') <> '',
  status,
  expires_at,
  created_at,
  updated_at,
  revoked_at`
	link, err := scanShareLink(r.db.QueryRowContext(ctx, query, req.UserID, req.NodeID, tokenHash, tokenSuffix, req.Permission, req.Password, req.ExpiresAt))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ShareLink{}, fmt.Errorf("active drive file not found")
		}
		return ShareLink{}, fmt.Errorf("create drive share link: %w", err)
	}
	link.Token = req.Token
	return link, nil
}

func (r *Repository) ListShareLinks(ctx context.Context, req ListShareLinksRequest) ([]ShareLink, error) {
	if r == nil || r.db == nil {
		return nil, fmt.Errorf("database handle is required")
	}
	req, err := ValidateListShareLinksRequest(req)
	if err != nil {
		return nil, err
	}
	query, args := buildListShareLinksQuery(req)
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list drive share links: %w", err)
	}
	defer rows.Close()

	links := make([]ShareLink, 0, req.Limit)
	for rows.Next() {
		link, err := scanShareLink(rows)
		if err != nil {
			return nil, fmt.Errorf("scan drive share link: %w", err)
		}
		links = append(links, link)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate drive share links: %w", err)
	}
	return links, nil
}

func buildListShareLinksQuery(req ListShareLinksRequest) (string, []any) {
	args := []any{req.UserID, req.Status}
	nodePredicate := ""
	if req.NodeID != "" {
		args = append(args, req.NodeID)
		nodePredicate = fmt.Sprintf("  AND node_id = $%d::uuid\n", len(args))
	}
	args = append(args, req.Limit)

	query := `
SELECT
  id::text,
  user_id::text,
  node_id::text,
  token_suffix,
  permission,
  COALESCE(password_hash, '') <> '',
  status,
  expires_at,
  created_at,
  updated_at,
  revoked_at
FROM drive_share_links
WHERE user_id = $1::uuid
  AND status = $2
` + nodePredicate + fmt.Sprintf(`ORDER BY updated_at DESC, id DESC
LIMIT $%d`, len(args))
	return query, args
}

func (r *Repository) RevokeShareLink(ctx context.Context, req RevokeShareLinkRequest) (ShareLink, error) {
	if r == nil || r.db == nil {
		return ShareLink{}, fmt.Errorf("database handle is required")
	}
	req, err := ValidateRevokeShareLinkRequest(req)
	if err != nil {
		return ShareLink{}, err
	}
	const query = `
UPDATE drive_share_links
SET status = 'revoked',
    revoked_at = now(),
    updated_at = now()
WHERE id = $2::uuid
  AND user_id = $1::uuid
  AND status = 'active'
RETURNING
  id::text,
  user_id::text,
  node_id::text,
  token_suffix,
  permission,
  COALESCE(password_hash, '') <> '',
  status,
  expires_at,
  created_at,
  updated_at,
  revoked_at`
	link, err := scanShareLink(r.db.QueryRowContext(ctx, query, req.UserID, req.LinkID))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ShareLink{}, fmt.Errorf("active drive share link not found")
		}
		return ShareLink{}, fmt.Errorf("revoke drive share link: %w", err)
	}
	return link, nil
}

type shareLinkScanner interface {
	Scan(dest ...any) error
}

func scanShareLink(scanner shareLinkScanner) (ShareLink, error) {
	var link ShareLink
	var revokedAt sql.NullTime
	if err := scanner.Scan(
		&link.ID,
		&link.UserID,
		&link.NodeID,
		&link.TokenSuffix,
		&link.Permission,
		&link.PasswordProtected,
		&link.Status,
		&link.ExpiresAt,
		&link.CreatedAt,
		&link.UpdatedAt,
		&revokedAt,
	); err != nil {
		return ShareLink{}, err
	}
	if revokedAt.Valid {
		link.RevokedAt = revokedAt.Time
	}
	return link, nil
}
