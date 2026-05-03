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

type Sender struct {
	CompanyID   string
	DomainID    string
	UserID      string
	Address     string
	DisplayName string
}

type OutgoingMessage struct {
	CompanyID       string
	DomainID        string
	UserID          string
	ComposeIntent   string
	SourceMessageID string
	RFCMessageID    string
	Subject         string
	From            outbound.Address
	To              []outbound.Address
	Cc              []outbound.Address
	Bcc             []outbound.Address
	SentAt          time.Time
	Size            int64
	StoragePath     string
	Farm            outbound.Farm
}

func (r *Repository) SenderForUser(ctx context.Context, userID string, fromAddress string) (Sender, error) {
	if r.db == nil {
		return Sender{}, fmt.Errorf("database handle is required")
	}

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
  AND (
    ($2 = '' AND ua.is_primary = true)
    OR lower(ua.address) = lower($2)
  )
ORDER BY ua.is_primary DESC
LIMIT 1`

	var sender Sender
	if err := r.db.QueryRowContext(ctx, query, userID, strings.TrimSpace(fromAddress)).Scan(
		&sender.CompanyID,
		&sender.DomainID,
		&sender.UserID,
		&sender.Address,
		&sender.DisplayName,
	); err != nil {
		if err == sql.ErrNoRows {
			return Sender{}, fmt.Errorf("sender address is not available for user %q", userID)
		}
		return Sender{}, fmt.Errorf("resolve sender address: %w", err)
	}
	return sender, nil
}

func (r *Repository) RecordOutgoing(ctx context.Context, msg OutgoingMessage) (string, error) {
	if r.db == nil {
		return "", fmt.Errorf("database handle is required")
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return "", fmt.Errorf("begin outgoing message transaction: %w", err)
	}
	defer tx.Rollback()

	folderID, err := sentFolderID(ctx, tx, msg.UserID)
	if err != nil {
		return "", err
	}
	if err := ensureOutgoingSource(ctx, tx, msg.UserID, msg.SourceMessageID); err != nil {
		return "", err
	}

	toJSON, err := outboundAddressesJSON(msg.To)
	if err != nil {
		return "", err
	}
	ccJSON, err := outboundAddressesJSON(msg.Cc)
	if err != nil {
		return "", err
	}
	bccJSON, err := outboundAddressesJSON(msg.Bcc)
	if err != nil {
		return "", err
	}

	const insertMessage = `
INSERT INTO messages (
  tenant_id, domain_id, user_id, folder_id,
  compose_intent, source_message_id,
  rfc_message_id, subject, from_addr, from_name,
  to_addrs, cc_addrs, bcc_addrs,
  sent_at, size, has_attachment, storage_path, flags, status
) VALUES (
  $1, $2, $3, $4,
  $5, NULLIF($6, '')::uuid,
  $7, $8, $9, $10,
  $11::jsonb, $12::jsonb, $13::jsonb,
  $14, $15, false, $16, '{"read":true}'::jsonb, 'active'
) RETURNING id::text`

	var messageID string
	if err := tx.QueryRowContext(
		ctx,
		insertMessage,
		msg.DomainID,
		msg.DomainID,
		msg.UserID,
		folderID,
		normalizeOutgoingIntent(msg.ComposeIntent),
		strings.TrimSpace(msg.SourceMessageID),
		msg.RFCMessageID,
		msg.Subject,
		msg.From.Email,
		msg.From.Name,
		string(toJSON),
		string(ccJSON),
		string(bccJSON),
		msg.SentAt,
		msg.Size,
		msg.StoragePath,
	).Scan(&messageID); err != nil {
		return "", fmt.Errorf("insert outgoing message metadata: %w", err)
	}

	if err := insertOutgoingOutbox(ctx, tx, messageID, msg); err != nil {
		return "", err
	}

	if err := tx.Commit(); err != nil {
		return "", fmt.Errorf("commit outgoing message transaction: %w", err)
	}
	return messageID, nil
}

func sentFolderID(ctx context.Context, tx *sql.Tx, userID string) (string, error) {
	const query = `
SELECT id::text
FROM folders
WHERE user_id = $1
  AND system_type = 'sent'
ORDER BY created_at
LIMIT 1`

	var folderID string
	if err := tx.QueryRowContext(ctx, query, userID).Scan(&folderID); err != nil {
		if err == sql.ErrNoRows {
			return createSentFolder(ctx, tx, userID)
		}
		return "", fmt.Errorf("lookup sent folder: %w", err)
	}
	return folderID, nil
}

func createSentFolder(ctx context.Context, tx *sql.Tx, userID string) (string, error) {
	const query = `
INSERT INTO folders (user_id, name, full_path, type, system_type)
VALUES ($1, 'Sent', '/Sent', 'system', 'sent')
RETURNING id::text`

	var folderID string
	if err := tx.QueryRowContext(ctx, query, userID).Scan(&folderID); err != nil {
		return "", fmt.Errorf("create sent folder: %w", err)
	}
	return folderID, nil
}

func insertOutgoingOutbox(ctx context.Context, tx *sql.Tx, messageID string, msg OutgoingMessage) error {
	payload, err := outgoingEventPayload(messageID, msg)
	if err != nil {
		return err
	}

	const query = `
INSERT INTO outbox (topic, partition_key, payload, status)
VALUES ($1, $2, $3::jsonb, 'pending')`

	topic := "mail.outbound." + string(msg.Farm)
	if _, err := tx.ExecContext(ctx, query, topic, messageID, string(payload)); err != nil {
		return fmt.Errorf("insert outgoing outbox event: %w", err)
	}
	return nil
}

func outgoingEventPayload(messageID string, msg OutgoingMessage) ([]byte, error) {
	payload := map[string]any{
		"event":             "mail.queued",
		"message_id":        messageID,
		"compose_intent":    normalizeOutgoingIntent(msg.ComposeIntent),
		"source_message_id": strings.TrimSpace(msg.SourceMessageID),
		"rfc_message_id":    msg.RFCMessageID,
		"company_id":        msg.CompanyID,
		"domain_id":         msg.DomainID,
		"user_id":           msg.UserID,
		"farm":              msg.Farm,
		"from":              msg.From,
		"to":                msg.To,
		"cc":                msg.Cc,
		"bcc":               msg.Bcc,
		"subject":           msg.Subject,
		"storage_path":      msg.StoragePath,
		"sent_at":           msg.SentAt,
		"size":              msg.Size,
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal mail.queued event: %w", err)
	}
	return raw, nil
}

func normalizeOutgoingIntent(intent string) string {
	switch strings.ToLower(strings.TrimSpace(intent)) {
	case "reply", "forward":
		return strings.ToLower(strings.TrimSpace(intent))
	default:
		return "new"
	}
}

func ensureOutgoingSource(ctx context.Context, tx *sql.Tx, userID string, sourceMessageID string) error {
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
		return fmt.Errorf("verify outgoing source message: %w", err)
	}
	if !exists {
		return fmt.Errorf("source message %q not found", sourceMessageID)
	}
	return nil
}

func outboundAddressesJSON(addrs []outbound.Address) ([]byte, error) {
	type dto struct {
		Name    string `json:"name"`
		Address string `json:"address"`
	}
	values := make([]dto, 0, len(addrs))
	for _, addr := range addrs {
		values = append(values, dto{Name: addr.Name, Address: addr.Email})
	}
	raw, err := json.Marshal(values)
	if err != nil {
		return nil, fmt.Errorf("marshal outbound addresses: %w", err)
	}
	return raw, nil
}
