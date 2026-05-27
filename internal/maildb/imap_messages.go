package maildb

import (
	"errors"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/mail"
	"sort"
	"strings"
	"time"

	"github.com/gogomail/gogomail/internal/imapgw"
)

type imapMessageRow struct {
	ID           string
	MailboxID    string
	RFCMessageID string
	Subject      string
	FromAddr     string
	FromName     string
	ToAddrs      json.RawMessage
	CcAddrs      json.RawMessage
	BccAddrs     json.RawMessage
	InternalDate time.Time
	Size         int64
	Read         bool
	Starred      bool
	Answered     bool
	Forwarded    bool
	Draft        bool
	Deleted      bool
	Keywords     imapKeywordList
	Status       string
}

func (r *Repository) ListIMAPMessages(ctx context.Context, userID string, mailboxID string, limit int, afterUID imapgw.UID) ([]imapgw.MessageSummary, error) {
	if r.db == nil {
		return nil, fmt.Errorf("database handle is required")
	}
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return nil, fmt.Errorf("user_id is required")
	}
	if strings.TrimSpace(mailboxID) == "" {
		return nil, fmt.Errorf("mailbox_id is required")
	}
	limit = NormalizeMessageListLimit(limit)

	if _, err := r.EnsureIMAPMailboxState(ctx, userID, mailboxID); err != nil {
		return nil, err
	}

	query := buildListIMAPMessagesQuery(afterUID)

	rows, err := r.db.QueryContext(ctx, query, userID, mailboxID, int64(afterUID), limit)
	if err != nil {
		return nil, fmt.Errorf("list imap messages: %w", err)
	}
	defer rows.Close()

	messages := make([]imapgw.MessageSummary, 0, limit)
	for rows.Next() {
		var row imapMessageRow
		var uid sql.NullInt64
		var modseq sql.NullInt64
		if err := rows.Scan(
			&row.ID,
			&row.MailboxID,
			&row.RFCMessageID,
			&row.Subject,
			&row.FromAddr,
			&row.FromName,
			&row.ToAddrs,
			&row.CcAddrs,
			&row.BccAddrs,
			&row.InternalDate,
			&row.Size,
			&row.Read,
			&row.Starred,
			&row.Answered,
			&row.Forwarded,
			&row.Draft,
			&row.Deleted,
			&row.Keywords,
			&row.Status,
			&uid,
			&modseq,
		); err != nil {
			return nil, fmt.Errorf("scan imap message summary: %w", err)
		}

		messageUID := IMAPMessageUID{
			MessageID: imapgw.MessageID(row.ID),
			MailboxID: imapgw.MailboxID(row.MailboxID),
			UID:       imapgw.UID(uid.Int64),
			ModSeq:    uint64(modseq.Int64),
		}
		if !uid.Valid {
			messageUID, err = r.EnsureIMAPMessageUID(ctx, userID, mailboxID, row.ID)
			if err != nil {
				return nil, err
			}
		}
		if messageUID.UID <= afterUID {
			continue
		}
		messages = append(messages, imapMessageFromRow(row, messageUID))
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate imap message summaries: %w", err)
	}
	sort.Slice(messages, func(i, j int) bool {
		return messages[i].UID < messages[j].UID
	})
	if len(messages) > 0 {
		baseSequence, err := imapSequenceBaseForAfterUID(ctx, r.db, userID, mailboxID, afterUID)
		if err != nil {
			return nil, err
		}
		if err := assignIMAPListSequenceNumbers(messages, baseSequence); err != nil {
			return nil, err
		}
	}
	return messages, nil
}

func buildListIMAPMessagesQuery(afterUID imapgw.UID) string {
	if afterUID > 0 {
		return listIMAPMessagesAfterUIDQuery
	}
	return listIMAPMessagesInitialQuery
}

const listIMAPMessagesSelectColumns = `
SELECT
  m.id::text AS id,
  m.folder_id::text AS folder_id,
  COALESCE(m.rfc_message_id, '') AS rfc_message_id,
  m.subject AS subject,
  m.from_addr AS from_addr,
  m.from_name AS from_name,
  m.to_addrs AS to_addrs,
  m.cc_addrs AS cc_addrs,
  m.bcc_addrs AS bcc_addrs,
  COALESCE(m.received_at, m.sent_at, m.draft_updated_at, m.created_at) AS internal_date,
  m.size AS size,
  COALESCE((m.flags->>'read')::boolean, false) AS read,
  COALESCE((m.flags->>'starred')::boolean, false) AS starred,
  COALESCE((m.flags->>'answered')::boolean, false) AS answered,
  COALESCE((m.flags->>'forwarded')::boolean, false) AS forwarded,
  COALESCE((m.flags->>'draft')::boolean, false) AS draft,
  COALESCE((m.flags->>'imap_deleted')::boolean, false) AS deleted,
  CASE
    WHEN jsonb_typeof(m.flags->'imap_keywords') = 'array' THEN m.flags->'imap_keywords'
    ELSE '[]'::jsonb
  END AS imap_keywords,
  m.status AS status,
  i.uid AS uid,
  i.modseq AS modseq`

const listIMAPMessagesOuterColumns = `
SELECT
  id,
  folder_id,
  rfc_message_id,
  subject,
  from_addr,
  from_name,
  to_addrs,
  cc_addrs,
  bcc_addrs,
  internal_date,
  size,
  read,
  starred,
  answered,
  forwarded,
  draft,
  deleted,
  imap_keywords,
  status,
  uid,
  modseq`

const listIMAPMessagesInitialQuery = listIMAPMessagesSelectColumns + `
FROM messages m
LEFT JOIN imap_message_uid i ON i.message_id = m.id
WHERE m.user_id = $1::uuid
  AND m.folder_id = $2::uuid
  AND m.status = 'active'
ORDER BY
  CASE WHEN i.uid IS NULL THEN 1 ELSE 0 END,
  i.uid,
  internal_date,
  m.id
LIMIT $4`

const listIMAPMessagesAfterUIDQuery = listIMAPMessagesOuterColumns + `
FROM (` + listIMAPMessagesSelectColumns + `
  FROM messages m
  JOIN imap_message_uid i ON i.message_id = m.id
  WHERE m.user_id = $1::uuid
    AND m.folder_id = $2::uuid
    AND m.status = 'active'
    AND i.uid > $3
  UNION ALL` + listIMAPMessagesSelectColumns + `
  FROM messages m
  LEFT JOIN imap_message_uid i ON i.message_id = m.id
  WHERE m.user_id = $1::uuid
    AND m.folder_id = $2::uuid
    AND m.status = 'active'
    AND i.message_id IS NULL
) imap_candidates
ORDER BY
  CASE WHEN uid IS NULL THEN 1 ELSE 0 END,
  uid,
  internal_date,
  id
LIMIT $4`

func (r *Repository) GetIMAPMessage(ctx context.Context, userID string, mailboxID string, uid imapgw.UID) (IMAPStoredMessage, error) {
	if r.db == nil {
		return IMAPStoredMessage{}, fmt.Errorf("database handle is required")
	}
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return IMAPStoredMessage{}, fmt.Errorf("user_id is required")
	}
	if strings.TrimSpace(mailboxID) == "" {
		return IMAPStoredMessage{}, fmt.Errorf("mailbox_id is required")
	}
	if uid == 0 {
		return IMAPStoredMessage{}, fmt.Errorf("uid is required")
	}

	const query = `
SELECT
  m.id::text,
  m.folder_id::text,
  COALESCE(m.rfc_message_id, ''),
  m.subject,
  m.from_addr,
  m.from_name,
  m.to_addrs,
  m.cc_addrs,
  m.bcc_addrs,
  COALESCE(m.received_at, m.sent_at, m.draft_updated_at, m.created_at) AS internal_date,
  m.size,
  COALESCE((m.flags->>'read')::boolean, false) AS read,
  COALESCE((m.flags->>'starred')::boolean, false) AS starred,
  COALESCE((m.flags->>'answered')::boolean, false) AS answered,
  COALESCE((m.flags->>'forwarded')::boolean, false) AS forwarded,
  COALESCE((m.flags->>'draft')::boolean, false) AS draft,
  COALESCE((m.flags->>'imap_deleted')::boolean, false) AS deleted,
  CASE
    WHEN jsonb_typeof(m.flags->'imap_keywords') = 'array' THEN m.flags->'imap_keywords'
    ELSE '[]'::jsonb
  END AS imap_keywords,
  m.status,
  m.storage_path,
  i.uid,
  i.modseq
FROM imap_message_uid i
JOIN messages m ON m.id = i.message_id
WHERE i.user_id = $1::uuid
  AND i.mailbox_id = $2::uuid
  AND i.uid = $3
  AND m.user_id = $1::uuid
  AND m.folder_id = $2::uuid
  AND m.status = 'active'
LIMIT 1`

	var row imapMessageRow
	var storagePath string
	var messageUID IMAPMessageUID
	if err := r.db.QueryRowContext(ctx, query, userID, mailboxID, int64(uid)).Scan(
		&row.ID,
		&row.MailboxID,
		&row.RFCMessageID,
		&row.Subject,
		&row.FromAddr,
		&row.FromName,
		&row.ToAddrs,
		&row.CcAddrs,
		&row.BccAddrs,
		&row.InternalDate,
		&row.Size,
		&row.Read,
		&row.Starred,
		&row.Answered,
		&row.Forwarded,
		&row.Draft,
		&row.Deleted,
		&row.Keywords,
		&row.Status,
		&storagePath,
		&messageUID.UID,
		&messageUID.ModSeq,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return IMAPStoredMessage{}, fmt.Errorf("imap message uid %d not found", uid)
		}
		return IMAPStoredMessage{}, fmt.Errorf("get imap message: %w", err)
	}
	messageUID.MessageID = imapgw.MessageID(row.ID)
	messageUID.MailboxID = imapgw.MailboxID(row.MailboxID)
	if err := imapgw.ValidateMessageUID(messageUID); err != nil {
		return IMAPStoredMessage{}, err
	}
	summary := imapMessageFromRow(row, messageUID)
	sequenceNumber, err := imapSequenceNumberForUID(ctx, r.db, userID, mailboxID, uid)
	if err != nil {
		return IMAPStoredMessage{}, err
	}
	summary.SequenceNumber = sequenceNumber
	return IMAPStoredMessage{
		Summary:     summary,
		StoragePath: strings.TrimSpace(storagePath),
	}, nil
}

func imapMessageFromRow(row imapMessageRow, uid IMAPMessageUID) imapgw.MessageSummary {
	return imapgw.MessageSummary{
		ID:        imapgw.MessageID(row.ID),
		MailboxID: imapgw.MailboxID(row.MailboxID),
		UID:       uid.UID,
		Envelope: imapgw.Envelope{
			MessageID: row.RFCMessageID,
			Subject:   row.Subject,
			From:      imapEnvelopeAddress(row.FromName, row.FromAddr),
			To:        imapEnvelopeAddresses(row.ToAddrs),
			Cc:        imapEnvelopeAddresses(row.CcAddrs),
			Bcc:       imapEnvelopeAddresses(row.BccAddrs),
			Date:      row.InternalDate,
		},
		Flags: imapgw.MessageFlags{
			Read:      row.Read,
			Starred:   row.Starred,
			Answered:  row.Answered,
			Forwarded: row.Forwarded,
			Draft:     row.Draft,
			Deleted:   row.Deleted,
			Keywords:  append([]string(nil), row.Keywords...),
			Status:    row.Status,
		},
		InternalDate: row.InternalDate,
		Size:         row.Size,
		ModSeq:       uid.ModSeq,
	}
}

func imapEnvelopeAddress(name string, address string) []imapgw.Address {
	name = strings.TrimSpace(name)
	address = strings.TrimSpace(address)
	if address == "" {
		return nil
	}
	if parsed, err := mail.ParseAddress(address); err == nil {
		address = parsed.Address
		if name == "" {
			name = parsed.Name
		}
	}
	mailbox, host, ok := strings.Cut(address, "@")
	if !ok {
		return []imapgw.Address{{Name: name, Mailbox: address}}
	}
	return []imapgw.Address{{Name: name, Mailbox: mailbox, Host: host}}
}

func imapEnvelopeAddresses(raw json.RawMessage) []imapgw.Address {
	if len(raw) == 0 {
		return nil
	}
	var items []struct {
		Name    string `json:"name"`
		Address string `json:"address"`
	}
	if err := json.Unmarshal(raw, &items); err != nil {
		return nil
	}
	addresses := make([]imapgw.Address, 0, len(items))
	for _, item := range items {
		addresses = append(addresses, imapEnvelopeAddress(item.Name, item.Address)...)
	}
	return addresses
}

func scanIMAPMessageByUID(ctx context.Context, tx *sql.Tx, userID string, mailboxID string, uid imapgw.UID) (imapMessageRow, IMAPMessageUID, error) {
	const query = `
SELECT
  m.id::text,
  m.folder_id::text,
  COALESCE(m.rfc_message_id, ''),
  m.subject,
  m.from_addr,
  m.from_name,
  m.to_addrs,
  m.cc_addrs,
  m.bcc_addrs,
  COALESCE(m.received_at, m.sent_at, m.draft_updated_at, m.created_at) AS internal_date,
  m.size,
  COALESCE((m.flags->>'read')::boolean, false) AS read,
  COALESCE((m.flags->>'starred')::boolean, false) AS starred,
  COALESCE((m.flags->>'answered')::boolean, false) AS answered,
  COALESCE((m.flags->>'forwarded')::boolean, false) AS forwarded,
  COALESCE((m.flags->>'draft')::boolean, false) AS draft,
  COALESCE((m.flags->>'imap_deleted')::boolean, false) AS deleted,
  CASE
    WHEN jsonb_typeof(m.flags->'imap_keywords') = 'array' THEN m.flags->'imap_keywords'
    ELSE '[]'::jsonb
  END AS imap_keywords,
  m.status,
  i.uid,
  i.modseq
FROM imap_message_uid i
JOIN messages m ON m.id = i.message_id
WHERE i.user_id = $1::uuid
  AND i.mailbox_id = $2::uuid
  AND i.uid = $3
  AND m.user_id = $1::uuid
  AND m.folder_id = $2::uuid
  AND m.status = 'active'
LIMIT 1
FOR UPDATE OF i, m`

	var row imapMessageRow
	var messageUID IMAPMessageUID
	if err := tx.QueryRowContext(ctx, query, userID, mailboxID, int64(uid)).Scan(
		&row.ID,
		&row.MailboxID,
		&row.RFCMessageID,
		&row.Subject,
		&row.FromAddr,
		&row.FromName,
		&row.ToAddrs,
		&row.CcAddrs,
		&row.BccAddrs,
		&row.InternalDate,
		&row.Size,
		&row.Read,
		&row.Starred,
		&row.Answered,
		&row.Forwarded,
		&row.Draft,
		&row.Deleted,
		&row.Keywords,
		&row.Status,
		&messageUID.UID,
		&messageUID.ModSeq,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return imapMessageRow{}, IMAPMessageUID{}, fmt.Errorf("imap message uid %d not found", uid)
		}
		return imapMessageRow{}, IMAPMessageUID{}, fmt.Errorf("scan imap message by uid: %w", err)
	}
	messageUID.MessageID = imapgw.MessageID(row.ID)
	messageUID.MailboxID = imapgw.MailboxID(row.MailboxID)
	if err := imapgw.ValidateMessageUID(messageUID); err != nil {
		return imapMessageRow{}, IMAPMessageUID{}, err
	}
	return row, messageUID, nil
}
