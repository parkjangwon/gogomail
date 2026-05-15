package maildb

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/gogomail/gogomail/internal/outbound"
)

type DraftForSend struct {
	ID              string
	UserID          string
	Intent          string
	SourceMessageID string
	From            string
	To              []outbound.Address
	Cc              []outbound.Address
	Bcc             []outbound.Address
	Subject         string
	TextBody        string
	HTMLBody        string
	AttachmentIDs   []string
	TrackOpens      bool
	ScheduledAt     time.Time
}

func (r *Repository) GetDraftForSend(ctx context.Context, userID string, draftID string) (DraftForSend, error) {
	if r.db == nil {
		return DraftForSend{}, fmt.Errorf("database handle is required")
	}

	const query = `
SELECT
  id::text,
  user_id::text,
  compose_intent,
  COALESCE(source_message_id::text, ''),
  from_addr,
  to_addrs,
  cc_addrs,
  bcc_addrs,
  subject,
  COALESCE(draft_text_body, ''),
  COALESCE(html_body, ''),
  COALESCE((flags->>'track_opens')::boolean, false),
  COALESCE(flags->>'scheduled_at', '')
FROM messages
WHERE user_id = $1
  AND id = $2
  AND status = 'draft'
LIMIT 1`

	var draft DraftForSend
	var toJSON, ccJSON, bccJSON []byte
	var scheduledAtText string
	if err := r.db.QueryRowContext(ctx, query, strings.TrimSpace(userID), strings.TrimSpace(draftID)).Scan(
		&draft.ID,
		&draft.UserID,
		&draft.Intent,
		&draft.SourceMessageID,
		&draft.From,
		&toJSON,
		&ccJSON,
		&bccJSON,
		&draft.Subject,
		&draft.TextBody,
		&draft.HTMLBody,
		&draft.TrackOpens,
		&scheduledAtText,
	); err != nil {
		if err == sql.ErrNoRows {
			return DraftForSend{}, fmt.Errorf("draft %q not found", draftID)
		}
		return DraftForSend{}, fmt.Errorf("get draft for send: %w", err)
	}
	var err error
	if draft.To, err = draftOutboundAddresses(toJSON); err != nil {
		return DraftForSend{}, err
	}
	if draft.Cc, err = draftOutboundAddresses(ccJSON); err != nil {
		return DraftForSend{}, err
	}
	if draft.Bcc, err = draftOutboundAddresses(bccJSON); err != nil {
		return DraftForSend{}, err
	}
	attachments, err := r.draftAttachmentIDs(ctx, userID, draftID)
	if err != nil {
		return DraftForSend{}, err
	}
	draft.AttachmentIDs = attachments
	if strings.TrimSpace(scheduledAtText) != "" {
		scheduledAt, err := time.Parse(time.RFC3339, strings.TrimSpace(scheduledAtText))
		if err != nil {
			return DraftForSend{}, fmt.Errorf("parse draft scheduled_at: %w", err)
		}
		draft.ScheduledAt = scheduledAt
	}
	return draft, nil
}

func (r *Repository) MarkDraftSent(ctx context.Context, userID string, draftID string, sentMessageID string) error {
	if r.db == nil {
		return fmt.Errorf("database handle is required")
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin mark draft sent transaction: %w", err)
	}
	defer tx.Rollback()

	result, err := tx.ExecContext(ctx, `
UPDATE messages AS draft
SET status = 'deleted',
    deleted_at = now(),
    updated_at = now(),
    source_message_id = sent.id
FROM messages AS sent
WHERE draft.user_id = $1
  AND draft.id = $2
  AND draft.status = 'draft'
  AND sent.user_id = draft.user_id
  AND sent.id = $3
  AND sent.status = 'active'`, strings.TrimSpace(userID), strings.TrimSpace(draftID), strings.TrimSpace(sentMessageID))
	if err != nil {
		return fmt.Errorf("mark draft sent: %w", err)
	}
	affected, err := result.RowsAffected()
	if err == nil && affected == 0 {
		return fmt.Errorf("draft %q not found for sent message %q", draftID, sentMessageID)
	}
	if _, err := tx.ExecContext(ctx, `
UPDATE attachments
SET draft_id = NULL,
    message_id = $3,
    status = 'stored'
WHERE user_id = $1
  AND draft_id = $2
  AND message_id IS NULL
  AND status = 'uploading'`, strings.TrimSpace(userID), strings.TrimSpace(draftID), strings.TrimSpace(sentMessageID)); err != nil {
		return fmt.Errorf("attach sent draft uploads: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `
UPDATE messages
SET has_attachment = EXISTS (
    SELECT 1
    FROM attachments
    WHERE message_id = $2
      AND user_id = $1
      AND status = 'stored'
  ),
  updated_at = now()
WHERE user_id = $1
  AND id = $2
  AND status = 'active'`, strings.TrimSpace(userID), strings.TrimSpace(sentMessageID)); err != nil {
		return fmt.Errorf("refresh sent attachment state: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit mark draft sent transaction: %w", err)
	}
	return nil
}

func (r *Repository) SaveDraft(ctx context.Context, req SaveDraftRequest) (MessageDetail, error) {
	if r.db == nil {
		return MessageDetail{}, fmt.Errorf("database handle is required")
	}
	if strings.TrimSpace(req.DraftID) != "" {
		return r.updateDraft(ctx, req)
	}
	return r.createDraft(ctx, req)
}

func (r *Repository) createDraft(ctx context.Context, req SaveDraftRequest) (MessageDetail, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return MessageDetail{}, fmt.Errorf("begin draft transaction: %w", err)
	}
	defer tx.Rollback()

	sender, err := senderForDraft(ctx, tx, req.UserID, req.From)
	if err != nil {
		return MessageDetail{}, err
	}
	folderID, err := draftFolderID(ctx, tx, req.UserID)
	if err != nil {
		return MessageDetail{}, err
	}
	if err := ensureDraftSource(ctx, tx, req.UserID, req.SourceMessageID); err != nil {
		return MessageDetail{}, err
	}
	toJSON, ccJSON, bccJSON, err := draftAddressJSON(req)
	if err != nil {
		return MessageDetail{}, err
	}

	now := time.Now().UTC()
	const query = `
INSERT INTO messages (
  tenant_id, domain_id, user_id, folder_id,
  source_message_id, compose_intent,
  subject, from_addr, from_name,
  to_addrs, cc_addrs, bcc_addrs,
  draft_text_body, html_body, draft_updated_at, storage_path, flags, status
) VALUES (
  $1, $2, $3, $4,
  NULLIF($5, '')::uuid, $6,
  $7, $8, $9,
  $10::jsonb, $11::jsonb, $12::jsonb,
  $13, $14, $15, '', jsonb_build_object('read', true, 'track_opens', $16::boolean, 'scheduled_at', $17::text), 'draft'
) RETURNING id::text, COALESCE(rfc_message_id, ''), subject, from_addr, from_name,
  to_addrs, cc_addrs, bcc_addrs, draft_updated_at, size, has_attachment, flags, storage_path, draft_text_body, html_body`

	var draft MessageDetail
	if err := tx.QueryRowContext(
		ctx,
		query,
		sender.DomainID,
		sender.DomainID,
		sender.UserID,
		folderID,
		strings.TrimSpace(req.SourceMessageID),
		normalizeDraftIntent(req.Intent),
		req.Subject,
		sender.Address,
		sender.DisplayName,
		string(toJSON),
		string(ccJSON),
		string(bccJSON),
		req.TextBody,
		req.HTMLBody,
		now,
		req.TrackOpens,
		draftScheduledAtText(req.ScheduledAt),
	).Scan(
		&draft.ID,
		&draft.MessageID,
		&draft.Subject,
		&draft.FromAddr,
		&draft.FromName,
		&draft.ToAddrs,
		&draft.CcAddrs,
		&draft.BccAddrs,
		&draft.ReceivedAt,
		&draft.Size,
		&draft.HasAttachment,
		&draft.Flags,
		&draft.StoragePath,
		&draft.TextBody,
		&draft.HTMLBody,
	); err != nil {
		return MessageDetail{}, fmt.Errorf("insert draft: %w", err)
	}
	if err := bindDraftAttachments(ctx, tx, req.UserID, draft.ID, req.AttachmentIDs); err != nil {
		return MessageDetail{}, err
	}
	attachments, err := listDraftAttachments(ctx, tx, req.UserID, draft.ID)
	if err != nil {
		return MessageDetail{}, err
	}
	draft.Attachments = attachments
	draft.HasAttachment = len(attachments) > 0
	if err := tx.Commit(); err != nil {
		return MessageDetail{}, fmt.Errorf("commit draft transaction: %w", err)
	}
	return draft, nil
}

func (r *Repository) updateDraft(ctx context.Context, req SaveDraftRequest) (MessageDetail, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return MessageDetail{}, fmt.Errorf("begin draft update transaction: %w", err)
	}
	defer tx.Rollback()

	sender, err := senderForDraft(ctx, tx, req.UserID, req.From)
	if err != nil {
		return MessageDetail{}, err
	}
	if err := ensureDraftSource(ctx, tx, req.UserID, req.SourceMessageID); err != nil {
		return MessageDetail{}, err
	}
	toJSON, ccJSON, bccJSON, err := draftAddressJSON(req)
	if err != nil {
		return MessageDetail{}, err
	}

	now := time.Now().UTC()
	const query = `
UPDATE messages
	SET source_message_id = NULLIF($3, '')::uuid,
    compose_intent = $4,
    subject = $5,
    from_addr = $6,
    from_name = $7,
    to_addrs = $8::jsonb,
    cc_addrs = $9::jsonb,
    bcc_addrs = $10::jsonb,
    draft_text_body = $11,
    html_body = $12,
    draft_updated_at = $13,
    updated_at = $13,
    flags = COALESCE(flags, '{}'::jsonb) || jsonb_build_object('read', true, 'track_opens', $14::boolean, 'scheduled_at', $15::text)
WHERE user_id = $1
  AND id = $2
  AND status = 'draft'
RETURNING id::text, COALESCE(rfc_message_id, ''), subject, from_addr, from_name,
  to_addrs, cc_addrs, bcc_addrs, draft_updated_at, size, has_attachment, flags, storage_path, draft_text_body, html_body`

	var draft MessageDetail
	if err := tx.QueryRowContext(
		ctx,
		query,
		sender.UserID,
		strings.TrimSpace(req.DraftID),
		strings.TrimSpace(req.SourceMessageID),
		normalizeDraftIntent(req.Intent),
		req.Subject,
		sender.Address,
		sender.DisplayName,
		string(toJSON),
		string(ccJSON),
		string(bccJSON),
		req.TextBody,
		req.HTMLBody,
		now,
		req.TrackOpens,
		draftScheduledAtText(req.ScheduledAt),
	).Scan(
		&draft.ID,
		&draft.MessageID,
		&draft.Subject,
		&draft.FromAddr,
		&draft.FromName,
		&draft.ToAddrs,
		&draft.CcAddrs,
		&draft.BccAddrs,
		&draft.ReceivedAt,
		&draft.Size,
		&draft.HasAttachment,
		&draft.Flags,
		&draft.StoragePath,
		&draft.TextBody,
		&draft.HTMLBody,
	); err != nil {
		if err == sql.ErrNoRows {
			return MessageDetail{}, fmt.Errorf("draft %q not found", req.DraftID)
		}
		return MessageDetail{}, fmt.Errorf("update draft: %w", err)
	}
	if err := bindDraftAttachments(ctx, tx, req.UserID, draft.ID, req.AttachmentIDs); err != nil {
		return MessageDetail{}, err
	}
	attachments, err := listDraftAttachments(ctx, tx, req.UserID, draft.ID)
	if err != nil {
		return MessageDetail{}, err
	}
	draft.Attachments = attachments
	draft.HasAttachment = len(attachments) > 0
	if err := tx.Commit(); err != nil {
		return MessageDetail{}, fmt.Errorf("commit draft update transaction: %w", err)
	}
	return draft, nil
}

func (r *Repository) DeleteDraft(ctx context.Context, userID string, draftID string) error {
	if r.db == nil {
		return fmt.Errorf("database handle is required")
	}

	const query = `
UPDATE messages
SET status = 'deleted',
    deleted_at = now(),
    updated_at = now()
WHERE user_id = $1
  AND id = $2
  AND status = 'draft'`

	result, err := r.db.ExecContext(ctx, query, strings.TrimSpace(userID), strings.TrimSpace(draftID))
	if err != nil {
		return fmt.Errorf("delete draft: %w", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("inspect draft delete: %w", err)
	}
	if affected == 0 {
		return fmt.Errorf("draft %q not found", draftID)
	}
	return nil
}

func draftFolderID(ctx context.Context, tx *sql.Tx, userID string) (string, error) {
	const query = `
SELECT id::text
FROM folders
WHERE user_id = $1
  AND system_type = 'drafts'
ORDER BY created_at
LIMIT 1`

	var folderID string
	if err := tx.QueryRowContext(ctx, query, userID).Scan(&folderID); err != nil {
		if err == sql.ErrNoRows {
			return createDraftFolder(ctx, tx, userID)
		}
		return "", fmt.Errorf("lookup drafts folder: %w", err)
	}
	return folderID, nil
}

func createDraftFolder(ctx context.Context, tx *sql.Tx, userID string) (string, error) {
	const query = `
INSERT INTO folders (user_id, name, full_path, type, system_type)
VALUES ($1, 'Drafts', '/Drafts', 'system', 'drafts')
RETURNING id::text`

	var folderID string
	if err := tx.QueryRowContext(ctx, query, userID).Scan(&folderID); err != nil {
		return "", fmt.Errorf("create drafts folder: %w", err)
	}
	return folderID, nil
}

func bindDraftAttachments(ctx context.Context, tx *sql.Tx, userID string, draftID string, attachmentIDs []string) error {
	if _, err := tx.ExecContext(ctx, `
UPDATE attachments
SET draft_id = NULL
WHERE user_id = $1
  AND draft_id = $2`, strings.TrimSpace(userID), strings.TrimSpace(draftID)); err != nil {
		return fmt.Errorf("clear draft attachments: %w", err)
	}

	for _, attachmentID := range attachmentIDs {
		result, err := tx.ExecContext(ctx, `
UPDATE attachments
SET draft_id = $3
WHERE user_id = $1
  AND id = $2
  AND message_id IS NULL
  AND status = 'uploading'`, strings.TrimSpace(userID), strings.TrimSpace(attachmentID), strings.TrimSpace(draftID))
		if err != nil {
			return fmt.Errorf("bind draft attachment %q: %w", attachmentID, err)
		}
		affected, err := result.RowsAffected()
		if err != nil {
			return fmt.Errorf("inspect draft attachment bind: %w", err)
		}
		if affected == 0 {
			return fmt.Errorf("attachment %q not found for draft", attachmentID)
		}
	}

	if _, err := tx.ExecContext(ctx, `
UPDATE messages
SET has_attachment = EXISTS (
    SELECT 1
    FROM attachments
    WHERE draft_id = $2
      AND user_id = $1
      AND status = 'uploading'
  ),
  updated_at = now()
WHERE user_id = $1
  AND id = $2
  AND status = 'draft'`, strings.TrimSpace(userID), strings.TrimSpace(draftID)); err != nil {
		return fmt.Errorf("refresh draft attachment state: %w", err)
	}
	return nil
}

func ensureDraftSource(ctx context.Context, tx *sql.Tx, userID string, sourceMessageID string) error {
	sourceMessageID = strings.TrimSpace(sourceMessageID)
	if sourceMessageID == "" {
		return nil
	}

	var exists bool
	if err := tx.QueryRowContext(ctx, `
SELECT EXISTS (
  SELECT 1
  FROM messages
  WHERE user_id = $1
    AND id = $2
    AND status = 'active'
)`, strings.TrimSpace(userID), sourceMessageID).Scan(&exists); err != nil {
		return fmt.Errorf("verify draft source message: %w", err)
	}
	if !exists {
		return fmt.Errorf("source message %q not found", sourceMessageID)
	}
	return nil
}

func listDraftAttachments(ctx context.Context, tx *sql.Tx, userID string, draftID string) ([]Attachment, error) {
	const query = `
SELECT
  id::text,
  COALESCE(message_id::text, ''),
  upload_id,
  storage_path,
  filename,
  size,
  mime_type,
  status,
  created_at
FROM attachments
WHERE user_id = $1
  AND draft_id = $2
  AND status = 'uploading'
ORDER BY created_at ASC, filename ASC`

	rows, err := tx.QueryContext(ctx, query, strings.TrimSpace(userID), strings.TrimSpace(draftID))
	if err != nil {
		return nil, fmt.Errorf("list draft attachments: %w", err)
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
			return nil, fmt.Errorf("scan draft attachment: %w", err)
		}
		attachments = append(attachments, attachment)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate draft attachments: %w", err)
	}
	return attachments, nil
}

func draftOutboundAddresses(raw []byte) ([]outbound.Address, error) {
	if len(raw) == 0 {
		return nil, nil
	}
	type dto struct {
		Name    string `json:"name"`
		Email   string `json:"email"`
		Address string `json:"address"`
	}
	var values []dto
	if err := json.Unmarshal(raw, &values); err != nil {
		return nil, fmt.Errorf("decode draft addresses: %w", err)
	}
	addresses := make([]outbound.Address, 0, len(values))
	for _, value := range values {
		email := strings.TrimSpace(value.Email)
		if email == "" {
			email = strings.TrimSpace(value.Address)
		}
		addresses = append(addresses, outbound.Address{
			Name:  value.Name,
			Email: email,
		})
	}
	return addresses, nil
}

func draftScheduledAtText(scheduledAt time.Time) string {
	if scheduledAt.IsZero() {
		return ""
	}
	return scheduledAt.UTC().Format(time.RFC3339)
}

func (r *Repository) draftAttachmentIDs(ctx context.Context, userID string, draftID string) ([]string, error) {
	rows, err := r.db.QueryContext(ctx, `
SELECT id::text
FROM attachments
WHERE user_id = $1
  AND draft_id = $2
  AND status = 'uploading'
ORDER BY created_at ASC, filename ASC`, strings.TrimSpace(userID), strings.TrimSpace(draftID))
	if err != nil {
		return nil, fmt.Errorf("list draft attachment ids: %w", err)
	}
	defer rows.Close()

	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("scan draft attachment id: %w", err)
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate draft attachment ids: %w", err)
	}
	return ids, nil
}

func senderForDraft(ctx context.Context, tx *sql.Tx, userID string, fromAddress string) (Sender, error) {
	const query = `
SELECT
  d.company_id::text,
  u.domain_id::text,
  u.id::text,
  ua.address,
  u.display_name
FROM users u
JOIN domains d ON d.id = u.domain_id
JOIN user_addresses ua ON ua.user_id = u.id
WHERE u.id = $1
  AND u.status = 'active'
  AND d.status = 'active'
  AND (
    ($2 = '' AND ua.is_primary = true)
    OR lower(ua.address) = lower($2)
  )
ORDER BY ua.is_primary DESC
LIMIT 1`

	var sender Sender
	if err := tx.QueryRowContext(ctx, query, userID, strings.TrimSpace(fromAddress)).Scan(
		&sender.CompanyID,
		&sender.DomainID,
		&sender.UserID,
		&sender.Address,
		&sender.DisplayName,
	); err != nil {
		if err == sql.ErrNoRows {
			return Sender{}, fmt.Errorf("sender address is not available for user %q", userID)
		}
		return Sender{}, fmt.Errorf("resolve draft sender address: %w", err)
	}
	return sender, nil
}

func draftAddressJSON(req SaveDraftRequest) ([]byte, []byte, []byte, error) {
	toJSON, err := outboundAddressesJSON(req.To)
	if err != nil {
		return nil, nil, nil, err
	}
	ccJSON, err := outboundAddressesJSON(req.Cc)
	if err != nil {
		return nil, nil, nil, err
	}
	bccJSON, err := outboundAddressesJSON(req.Bcc)
	if err != nil {
		return nil, nil, nil, err
	}
	return toJSON, ccJSON, bccJSON, nil
}

func normalizeDraftIntent(intent string) string {
	switch strings.ToLower(strings.TrimSpace(intent)) {
	case "reply", "forward":
		return strings.ToLower(strings.TrimSpace(intent))
	default:
		return "new"
	}
}

func draftEventPayload(event string, draft MessageDetail, req SaveDraftRequest) ([]byte, error) {
	payload := map[string]any{
		"event":             event,
		"draft_id":          draft.ID,
		"user_id":           req.UserID,
		"source_message_id": strings.TrimSpace(req.SourceMessageID),
		"compose_intent":    normalizeDraftIntent(req.Intent),
		"subject":           req.Subject,
		"updated_at":        draft.ReceivedAt,
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal draft event: %w", err)
	}
	return raw, nil
}
