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
)`

	_, err = r.db.ExecContext(
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
	)
	if err != nil {
		return fmt.Errorf("insert message metadata: %w", err)
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
