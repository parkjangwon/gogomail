package maildb

import (
	"errors"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/gogomail/gogomail/internal/mail"
	"github.com/gogomail/gogomail/internal/message"
	smtpd "github.com/gogomail/gogomail/internal/smtp"
	"github.com/lib/pq"
)

type Repository struct {
	db *sql.DB
}

func NewRepository(db *sql.DB) *Repository {
	return &Repository{db: db}
}

// DB returns the underlying *sql.DB for direct query access.
func (r *Repository) DB() *sql.DB { return r.db }

func normalizeAddressACE(raw string) (string, error) {
	address, err := mail.NormalizeAddress(raw)
	if err != nil {
		return "", err
	}
	return strings.ToLower(strings.TrimSpace(address)), nil
}

func (r *Repository) ResolveRecipient(ctx context.Context, address string) (smtpd.Mailbox, error) {
	if r.db == nil {
		return smtpd.Mailbox{}, fmt.Errorf("database handle is required")
	}
	addressACE, err := normalizeAddressACE(address)
	if err != nil {
		return smtpd.Mailbox{}, err
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
WHERE ua.address_ace = $1
  AND u.status = 'active'
  AND d.status = 'active'
LIMIT 1`

	var mailbox smtpd.Mailbox
	err = r.db.QueryRowContext(ctx, query, addressACE).Scan(
		&mailbox.CompanyID,
		&mailbox.DomainID,
		&mailbox.UserID,
		&mailbox.Address,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return smtpd.Mailbox{}, fmt.Errorf("recipient %q not found", address)
		}
		return smtpd.Mailbox{}, fmt.Errorf("resolve recipient %q: %w", address, err)
	}
	return mailbox, nil
}

func (r *Repository) ResolveLocalRecipient(ctx context.Context, address string) (smtpd.Mailbox, bool, error) {
	if r.db == nil {
		return smtpd.Mailbox{}, false, fmt.Errorf("database handle is required")
	}
	addressACE, err := normalizeAddressACE(address)
	if err != nil {
		return smtpd.Mailbox{}, false, err
	}
	_, domainACE, ok := strings.Cut(addressACE, "@")
	if !ok || strings.TrimSpace(domainACE) == "" {
		return smtpd.Mailbox{}, false, fmt.Errorf("recipient %q has no domain", address)
	}

	const query = `
SELECT
  d.company_id::text,
  d.id::text,
  COALESCE(u.id::text, ''),
  COALESCE(ua.address, ''),
  u.id IS NOT NULL
FROM domains d
LEFT JOIN user_addresses ua ON ua.domain_id = d.id AND ua.address_ace = $1
LEFT JOIN users u ON u.id = ua.user_id AND u.status = 'active'
WHERE d.name_ace = $2
  AND d.status = 'active'
LIMIT 1`

	var mailbox smtpd.Mailbox
	var recipientExists bool
	err = r.db.QueryRowContext(ctx, query, addressACE, strings.ToLower(strings.TrimSpace(domainACE))).Scan(
		&mailbox.CompanyID,
		&mailbox.DomainID,
		&mailbox.UserID,
		&mailbox.Address,
		&recipientExists,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return smtpd.Mailbox{}, false, nil
		}
		return smtpd.Mailbox{}, false, fmt.Errorf("resolve local recipient %q: %w", address, err)
	}
	return mailbox, true, nil
}

// Record persists the received message to the database and returns the
// database-assigned message UUID, which can be used to correlate log
// entries across services (SMTP receiver → outbox relay → delivery worker).
func (r *Repository) Record(ctx context.Context, msg smtpd.ReceivedMessage) (string, error) {
	if r.db == nil {
		return "", fmt.Errorf("database handle is required")
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return "", fmt.Errorf("begin record message transaction: %w", err)
	}
	defer tx.Rollback()

	if err := checkAndIncrementUserQuota(ctx, tx, msg.Mailbox.UserID, msg.Size); err != nil {
		return "", err
	}

	folderID, err := r.deliveryFolderID(ctx, msg.Mailbox.UserID, msg.FolderSystemType)
	if err != nil {
		return "", err
	}

	toAddrs, err := addressesJSON(msg.Parsed.To)
	if err != nil {
		return "", err
	}
	ccAddrs, err := addressesJSON(msg.Parsed.Cc)
	if err != nil {
		return "", err
	}
	bccAddrs, err := addressesJSON(msg.Parsed.Bcc)
	if err != nil {
		return "", err
	}
	threadID, err := r.resolveThreadID(ctx, tx, msg.Mailbox.UserID, threadCandidates(msg.Parsed.InReplyTo, msg.Parsed.References))
	if err != nil {
		return "", err
	}

	const insert = `
INSERT INTO messages (
  tenant_id,
  domain_id,
  user_id,
  folder_id,
  rfc_message_id,
  in_reply_to,
  thread_id,
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
  $1, $2, $3, $4, $5, $6, NULLIF($7, '')::uuid, $8, $9, $10,
  $11::jsonb, $12::jsonb, $13::jsonb,
  $14, $15, $16, $17, 'active'
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
		emptyToNull(msg.Parsed.InReplyTo),
		threadID,
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
		return "", fmt.Errorf("insert message metadata: %w", err)
	}

	if err := r.insertStoredOutbox(ctx, tx, insertedMessageID, folderID, msg); err != nil {
		return "", err
	}

	if err := tx.Commit(); err != nil {
		return "", fmt.Errorf("commit record message transaction: %w", err)
	}
	return insertedMessageID, nil
}

func (r *Repository) deliveryFolderID(ctx context.Context, userID string, systemType string) (string, error) {
	systemType = strings.TrimSpace(systemType)
	if systemType == "" {
		systemType = "inbox"
	}
	const query = `
SELECT id::text
FROM folders
WHERE user_id = $1
  AND type = 'system'
  AND system_type = $2
LIMIT 1`

	var folderID string
	if err := r.db.QueryRowContext(ctx, query, userID, systemType).Scan(&folderID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			if systemType != "inbox" {
				return r.deliveryFolderID(ctx, userID, "inbox")
			}
			return "", fmt.Errorf("inbox folder for user %q not found", userID)
		}
		return "", fmt.Errorf("lookup %s folder for user %q: %w", systemType, userID, err)
	}
	return folderID, nil
}

func (r *Repository) insertStoredOutbox(ctx context.Context, tx *sql.Tx, messageID string, folderID string, msg smtpd.ReceivedMessage) error {
	payload, err := storedEventPayload(messageID, folderID, msg)
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

const resolveThreadIDSQL = `
WITH requested AS (
  SELECT value AS rfc_message_id, ordinality
  FROM unnest($2::text[]) WITH ORDINALITY AS requested(value, ordinality)
)
SELECT COALESCE(thread_id, id)::text
FROM messages
JOIN requested ON requested.rfc_message_id = messages.rfc_message_id
WHERE user_id = $1::uuid
ORDER BY requested.ordinality
LIMIT 1`

func (r *Repository) resolveThreadID(ctx context.Context, tx *sql.Tx, userID string, candidates []string) (string, error) {
	candidates = normalizeThreadCandidates(candidates)
	if len(candidates) == 0 {
		return "", nil
	}

	var threadID string
	if err := tx.QueryRowContext(ctx, resolveThreadIDSQL, strings.TrimSpace(userID), pq.Array(candidates)).Scan(&threadID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", nil
		}
		return "", fmt.Errorf("resolve message thread: %w", err)
	}
	return threadID, nil
}

func threadCandidates(inReplyTo string, references []string) []string {
	candidates := make([]string, 0, len(references)+1)
	candidates = append(candidates, references...)
	if strings.TrimSpace(inReplyTo) != "" {
		candidates = append(candidates, inReplyTo)
	}
	return candidates
}

func normalizeThreadCandidates(candidates []string) []string {
	seen := make(map[string]struct{}, len(candidates))
	out := make([]string, 0, len(candidates))
	for _, candidate := range candidates {
		candidate = strings.TrimSpace(candidate)
		if candidate == "" {
			continue
		}
		if !strings.HasPrefix(candidate, "<") {
			candidate = "<" + candidate
		}
		if !strings.HasSuffix(candidate, ">") {
			candidate += ">"
		}
		key := strings.ToLower(candidate)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, candidate)
	}
	return out
}

func storedEventPayload(messageID string, folderID string, msg smtpd.ReceivedMessage) ([]byte, error) {
	payload := map[string]any{
		"event":                  "mail.stored",
		"schema_version":         "2026-05-04.mail-stored.v1",
		"message_id":             messageID,
		"rfc_message_id":         msg.Parsed.MessageID,
		"in_reply_to":            msg.Parsed.InReplyTo,
		"references":             append([]string(nil), msg.Parsed.References...),
		"company_id":             msg.Mailbox.CompanyID,
		"domain_id":              msg.Mailbox.DomainID,
		"user_id":                msg.Mailbox.UserID,
		"folder_id":              strings.TrimSpace(folderID),
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
