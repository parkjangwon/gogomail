package maildb

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/gogomail/gogomail/internal/message"
	smtpd "github.com/gogomail/gogomail/internal/smtp"
)

type Repository struct {
	db *sql.DB
}

func NewRepository(db *sql.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) ResolveRecipient(ctx context.Context, address string) (smtpd.Mailbox, error) {
	if r.db == nil {
		return smtpd.Mailbox{}, fmt.Errorf("database handle is required")
	}

	const query = `
SELECT
  d.company_id::text,
  ua.domain_id::text,
  ua.user_id::text,
  ua.address
FROM user_addresses ua
JOIN users u ON u.id = ua.user_id
JOIN domains d ON d.id = ua.domain_id
WHERE lower(ua.address) = lower($1)
  AND u.status = 'active'
  AND d.status = 'active'
LIMIT 1`

	var mailbox smtpd.Mailbox
	err := r.db.QueryRowContext(ctx, query, strings.TrimSpace(address)).Scan(
		&mailbox.CompanyID,
		&mailbox.DomainID,
		&mailbox.UserID,
		&mailbox.Address,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return smtpd.Mailbox{}, fmt.Errorf("recipient %q not found", address)
		}
		return smtpd.Mailbox{}, fmt.Errorf("resolve recipient %q: %w", address, err)
	}
	return mailbox, nil
}

func (r *Repository) Record(ctx context.Context, msg smtpd.ReceivedMessage) error {
	if r.db == nil {
		return fmt.Errorf("database handle is required")
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin record message transaction: %w", err)
	}
	defer tx.Rollback()

	folderID, err := r.inboxFolderID(ctx, msg.Mailbox.UserID)
	if err != nil {
		return err
	}

	toAddrs, err := addressesJSON(msg.Parsed.To)
	if err != nil {
		return err
	}
	ccAddrs, err := addressesJSON(msg.Parsed.Cc)
	if err != nil {
		return err
	}
	bccAddrs, err := addressesJSON(msg.Parsed.Bcc)
	if err != nil {
		return err
	}

	const insert = `
INSERT INTO messages (
  tenant_id,
  domain_id,
  user_id,
  folder_id,
  rfc_message_id,
  subject,
  from_addr,
  from_name,
  to_addrs,
  cc_addrs,
  bcc_addrs,
  received_at,
  size,
  has_attachment,
  storage_path,
  status
) VALUES (
  $1, $2, $3, $4, $5, $6, $7, $8,
  $9::jsonb, $10::jsonb, $11::jsonb,
  $12, $13, $14, $15, 'active'
) RETURNING id::text`

	var insertedMessageID string
	err = tx.QueryRowContext(
		ctx,
		insert,
		msg.Mailbox.DomainID,
		msg.Mailbox.DomainID,
		msg.Mailbox.UserID,
		folderID,
		emptyToNull(msg.Parsed.MessageID),
		msg.Parsed.Subject,
		msg.Parsed.From.Address,
		msg.Parsed.From.Name,
		string(toAddrs),
		string(ccAddrs),
		string(bccAddrs),
		msg.ReceivedAt,
		msg.Size,
		msg.Parsed.HasAttachment,
		msg.StoragePath,
	).Scan(&insertedMessageID)
	if err != nil {
		return fmt.Errorf("insert message metadata: %w", err)
	}

	if err := r.insertStoredOutbox(ctx, tx, insertedMessageID, msg); err != nil {
		return err
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit record message transaction: %w", err)
	}
	return nil
}

func (r *Repository) inboxFolderID(ctx context.Context, userID string) (string, error) {
	const query = `
SELECT id::text
FROM folders
WHERE user_id = $1
  AND type = 'system'
  AND system_type = 'inbox'
LIMIT 1`

	var folderID string
	if err := r.db.QueryRowContext(ctx, query, userID).Scan(&folderID); err != nil {
		if err == sql.ErrNoRows {
			return "", fmt.Errorf("inbox folder for user %q not found", userID)
		}
		return "", fmt.Errorf("lookup inbox folder for user %q: %w", userID, err)
	}
	return folderID, nil
}

func (r *Repository) insertStoredOutbox(ctx context.Context, tx *sql.Tx, messageID string, msg smtpd.ReceivedMessage) error {
	payload, err := storedEventPayload(messageID, msg)
	if err != nil {
		return err
	}

	const query = `
INSERT INTO outbox (topic, partition_key, payload, status)
VALUES ('mail.event', $1, $2::jsonb, 'pending')`

	if _, err := tx.ExecContext(ctx, query, messageID, string(payload)); err != nil {
		return fmt.Errorf("insert mail.stored outbox event: %w", err)
	}
	return nil
}

func storedEventPayload(messageID string, msg smtpd.ReceivedMessage) ([]byte, error) {
	payload := map[string]any{
		"event":                  "mail.stored",
		"message_id":             messageID,
		"rfc_message_id":         msg.Parsed.MessageID,
		"company_id":             msg.Mailbox.CompanyID,
		"domain_id":              msg.Mailbox.DomainID,
		"user_id":                msg.Mailbox.UserID,
		"recipient":              msg.Mailbox.Address,
		"subject":                msg.Parsed.Subject,
		"storage_path":           msg.StoragePath,
		"received_at":            msg.ReceivedAt,
		"size":                   msg.Size,
		"envelope_from":          msg.EnvelopeFrom,
		"dsn":                    storedEventDSN(msg.DSN),
		"authentication_results": storedEventAuthentication(msg.Authentication),
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal mail.stored event: %w", err)
	}
	return raw, nil
}

func storedEventDSN(dsn smtpd.DSNOptions) map[string]any {
	recipients := make([]map[string]any, 0, len(dsn.Recipients))
	for _, recipient := range dsn.Recipients {
		recipients = append(recipients, map[string]any{
			"address":            recipient.Address,
			"notify":             append([]string(nil), recipient.Notify...),
			"original_recipient": recipient.OriginalRecipient,
		})
	}
	return map[string]any{
		"return":      dsn.Return,
		"envelope_id": dsn.EnvelopeID,
		"recipients":  recipients,
	}
}

func storedEventAuthentication(results smtpd.AuthenticationResults) map[string]any {
	return map[string]any{
		"authserv_id": results.AuthservID,
		"spf":         storedEventAuthCheck(results.SPF),
		"dkim":        storedEventAuthCheck(results.DKIM),
		"dmarc":       storedEventAuthCheck(results.DMARC),
	}
}

func storedEventAuthCheck(result smtpd.AuthCheckResult) map[string]any {
	return map[string]any{
		"result":     string(result.Result),
		"reason":     result.Reason,
		"domain":     result.Domain,
		"identifier": result.Identifier,
		"policy":     result.Policy,
	}
}

func addressesJSON(addrs []message.Address) ([]byte, error) {
	type dto struct {
		Name    string `json:"name"`
		Address string `json:"address"`
	}
	items := make([]dto, 0, len(addrs))
	for _, addr := range addrs {
		items = append(items, dto{Name: addr.Name, Address: addr.Address})
	}
	raw, err := json.Marshal(items)
	if err != nil {
		return nil, fmt.Errorf("marshal addresses: %w", err)
	}
	return raw, nil
}

func emptyToNull(value string) sql.NullString {
	if strings.TrimSpace(value) == "" {
		return sql.NullString{}
	}
	return sql.NullString{String: value, Valid: true}
}
