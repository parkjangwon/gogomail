package maildb

import (
	"context"
	"database/sql"
	"fmt"
	"net/mail"
	"sort"
	"strings"
	"time"

	"github.com/gogomail/gogomail/internal/imapgw"
)

type IMAPUIDState = imapgw.UIDState
type IMAPMessageUID = imapgw.MessageUID

func (r *Repository) ListIMAPMailboxes(ctx context.Context, userID string) ([]imapgw.Mailbox, error) {
	if r.db == nil {
		return nil, fmt.Errorf("database handle is required")
	}
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return nil, fmt.Errorf("user_id is required")
	}

	folders, err := r.ListFolders(ctx, userID)
	if err != nil {
		return nil, err
	}
	mailboxes := make([]imapgw.Mailbox, 0, len(folders))
	for _, folder := range folders {
		state, err := r.EnsureIMAPMailboxState(ctx, userID, folder.ID)
		if err != nil {
			return nil, err
		}
		mailboxes = append(mailboxes, imapMailboxFromFolder(folder, state))
	}
	return mailboxes, nil
}

func (r *Repository) GetIMAPMailbox(ctx context.Context, userID string, mailboxID string) (imapgw.Mailbox, error) {
	if r.db == nil {
		return imapgw.Mailbox{}, fmt.Errorf("database handle is required")
	}
	userID = strings.TrimSpace(userID)
	mailboxID = strings.TrimSpace(mailboxID)
	if userID == "" {
		return imapgw.Mailbox{}, fmt.Errorf("user_id is required")
	}
	if mailboxID == "" {
		return imapgw.Mailbox{}, fmt.Errorf("mailbox_id is required")
	}

	const query = `
SELECT
  f.id::text,
  COALESCE(f.parent_id::text, ''),
  f.name,
  f.full_path,
  f.type,
  COALESCE(f.system_type, ''),
  f.order_index,
  COALESCE(c.total, 0) AS total,
  COALESCE(c.unread, 0) AS unread,
  COALESCE(c.starred, 0) AS starred
FROM folders f
LEFT JOIN (
  SELECT
    folder_id,
    COUNT(*) AS total,
    COUNT(*) FILTER (WHERE COALESCE((flags->>'read')::boolean, false) = false) AS unread,
    COUNT(*) FILTER (WHERE COALESCE((flags->>'starred')::boolean, false) = true) AS starred
  FROM messages
  WHERE user_id = $1::uuid
    AND status = 'active'
  GROUP BY folder_id
) c ON c.folder_id = f.id
WHERE f.user_id = $1::uuid
  AND f.id = $2::uuid`

	var folder Folder
	if err := r.db.QueryRowContext(ctx, query, userID, mailboxID).Scan(
		&folder.ID,
		&folder.ParentID,
		&folder.Name,
		&folder.FullPath,
		&folder.Type,
		&folder.SystemType,
		&folder.OrderIndex,
		&folder.Total,
		&folder.Unread,
		&folder.Starred,
	); err != nil {
		if err == sql.ErrNoRows {
			return imapgw.Mailbox{}, fmt.Errorf("imap mailbox %q not found", mailboxID)
		}
		return imapgw.Mailbox{}, fmt.Errorf("get imap mailbox: %w", err)
	}
	state, err := r.EnsureIMAPMailboxState(ctx, userID, folder.ID)
	if err != nil {
		return imapgw.Mailbox{}, err
	}
	return imapMailboxFromFolder(folder, state), nil
}

func (r *Repository) ListIMAPMessages(ctx context.Context, userID string, mailboxID string, limit int, afterUID imapgw.UID) ([]imapgw.MessageSummary, error) {
	if r.db == nil {
		return nil, fmt.Errorf("database handle is required")
	}
	userID = strings.TrimSpace(userID)
	mailboxID = strings.TrimSpace(mailboxID)
	if userID == "" {
		return nil, fmt.Errorf("user_id is required")
	}
	if mailboxID == "" {
		return nil, fmt.Errorf("mailbox_id is required")
	}
	limit = NormalizeMessageListLimit(limit)

	if _, err := r.EnsureIMAPMailboxState(ctx, userID, mailboxID); err != nil {
		return nil, err
	}

	const query = `
SELECT
  m.id::text,
  m.folder_id::text,
  COALESCE(m.rfc_message_id, ''),
  m.subject,
  m.from_addr,
  m.from_name,
  COALESCE(m.received_at, m.sent_at, m.draft_updated_at, m.created_at) AS internal_date,
  m.size,
  COALESCE((m.flags->>'read')::boolean, false) AS read,
  COALESCE((m.flags->>'starred')::boolean, false) AS starred,
  COALESCE((m.flags->>'answered')::boolean, false) AS answered,
  COALESCE((m.flags->>'forwarded')::boolean, false) AS forwarded,
  COALESCE((m.flags->>'draft')::boolean, false) AS draft,
  m.status,
  i.uid,
  i.modseq
FROM messages m
LEFT JOIN imap_message_uid i ON i.message_id = m.id
WHERE m.user_id = $1::uuid
  AND m.folder_id = $2::uuid
  AND m.status = 'active'
  AND (i.uid IS NULL OR i.uid > $3)
ORDER BY
  CASE WHEN i.uid IS NULL THEN 1 ELSE 0 END,
  i.uid,
  internal_date,
  m.id
LIMIT $4`

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
			&row.InternalDate,
			&row.Size,
			&row.Read,
			&row.Starred,
			&row.Answered,
			&row.Forwarded,
			&row.Draft,
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
	return messages, nil
}

func imapMailboxFromFolder(folder Folder, state IMAPUIDState) imapgw.Mailbox {
	return imapgw.Mailbox{
		ID:          imapgw.MailboxID(folder.ID),
		ParentID:    imapgw.MailboxID(folder.ParentID),
		Name:        folder.Name,
		FullPath:    folder.FullPath,
		SystemType:  folder.SystemType,
		UIDValidity: state.UIDValidity,
		UIDNext:     state.UIDNext,
		Messages:    uint32(folder.Total),
		Unseen:      uint32(folder.Unread),
	}
}

type imapMessageRow struct {
	ID           string
	MailboxID    string
	RFCMessageID string
	Subject      string
	FromAddr     string
	FromName     string
	InternalDate time.Time
	Size         int64
	Read         bool
	Starred      bool
	Answered     bool
	Forwarded    bool
	Draft        bool
	Status       string
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
			Date:      row.InternalDate,
		},
		Flags: imapgw.MessageFlags{
			Read:      row.Read,
			Starred:   row.Starred,
			Answered:  row.Answered,
			Forwarded: row.Forwarded,
			Draft:     row.Draft,
			Status:    row.Status,
		},
		InternalDate: row.InternalDate,
		Size:         row.Size,
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

func (r *Repository) EnsureIMAPMailboxState(ctx context.Context, userID string, mailboxID string) (IMAPUIDState, error) {
	if r.db == nil {
		return IMAPUIDState{}, fmt.Errorf("database handle is required")
	}
	userID = strings.TrimSpace(userID)
	mailboxID = strings.TrimSpace(mailboxID)
	if userID == "" {
		return IMAPUIDState{}, fmt.Errorf("user_id is required")
	}
	if mailboxID == "" {
		return IMAPUIDState{}, fmt.Errorf("mailbox_id is required")
	}

	const query = `
INSERT INTO imap_mailbox_state (mailbox_id, user_id)
SELECT id, user_id
FROM folders
WHERE id = $1::uuid
  AND user_id = $2::uuid
ON CONFLICT (mailbox_id) DO UPDATE
SET updated_at = imap_mailbox_state.updated_at
RETURNING mailbox_id::text, uidvalidity, uidnext, highest_modseq`

	var state IMAPUIDState
	if err := r.db.QueryRowContext(ctx, query, mailboxID, userID).Scan(
		&state.MailboxID,
		&state.UIDValidity,
		&state.UIDNext,
		&state.HighestModSeq,
	); err != nil {
		return IMAPUIDState{}, fmt.Errorf("ensure imap mailbox state: %w", err)
	}
	if err := imapgw.ValidateUIDState(state); err != nil {
		return IMAPUIDState{}, err
	}
	return state, nil
}

func (r *Repository) EnsureIMAPMessageUID(ctx context.Context, userID string, mailboxID string, messageID string) (IMAPMessageUID, error) {
	if r.db == nil {
		return IMAPMessageUID{}, fmt.Errorf("database handle is required")
	}
	userID = strings.TrimSpace(userID)
	mailboxID = strings.TrimSpace(mailboxID)
	messageID = strings.TrimSpace(messageID)
	if userID == "" {
		return IMAPMessageUID{}, fmt.Errorf("user_id is required")
	}
	if mailboxID == "" {
		return IMAPMessageUID{}, fmt.Errorf("mailbox_id is required")
	}
	if messageID == "" {
		return IMAPMessageUID{}, fmt.Errorf("message_id is required")
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return IMAPMessageUID{}, fmt.Errorf("begin imap uid transaction: %w", err)
	}
	defer tx.Rollback()

	const ensureState = `
INSERT INTO imap_mailbox_state (mailbox_id, user_id)
SELECT id, user_id
FROM folders
WHERE id = $1::uuid
  AND user_id = $2::uuid
ON CONFLICT (mailbox_id) DO NOTHING`
	if _, err := tx.ExecContext(ctx, ensureState, mailboxID, userID); err != nil {
		return IMAPMessageUID{}, fmt.Errorf("ensure imap mailbox state: %w", err)
	}

	var uid uint32
	var modseq uint64
	const assign = `
WITH mailbox AS (
  SELECT mailbox_id, user_id, uidnext, highest_modseq
  FROM imap_mailbox_state
  WHERE mailbox_id = $2::uuid
    AND user_id = $3::uuid
  FOR UPDATE
),
message AS (
  SELECT id, folder_id, user_id
  FROM messages
  WHERE id = $1::uuid
    AND folder_id = $2::uuid
    AND user_id = $3::uuid
    AND status = 'active'
),
inserted AS (
  INSERT INTO imap_message_uid (message_id, mailbox_id, user_id, uid, modseq)
  SELECT message.id, mailbox.mailbox_id, mailbox.user_id, mailbox.uidnext, mailbox.highest_modseq + 1
  FROM mailbox, message
  ON CONFLICT (message_id) DO NOTHING
  RETURNING uid, modseq
),
bumped AS (
  UPDATE imap_mailbox_state
  SET uidnext = CASE WHEN EXISTS (SELECT 1 FROM inserted) THEN uidnext + 1 ELSE uidnext END,
      highest_modseq = CASE WHEN EXISTS (SELECT 1 FROM inserted) THEN highest_modseq + 1 ELSE highest_modseq END,
      updated_at = CASE WHEN EXISTS (SELECT 1 FROM inserted) THEN now() ELSE updated_at END
  WHERE mailbox_id = $2::uuid
  RETURNING 1
)
SELECT uid, modseq FROM inserted
UNION ALL
SELECT uid, modseq FROM imap_message_uid WHERE message_id = $1::uuid
LIMIT 1`
	if err := tx.QueryRowContext(ctx, assign, messageID, mailboxID, userID).Scan(&uid, &modseq); err != nil {
		return IMAPMessageUID{}, fmt.Errorf("ensure imap message uid: %w", err)
	}

	result := IMAPMessageUID{
		MessageID: imapgw.MessageID(messageID),
		MailboxID: imapgw.MailboxID(mailboxID),
		UID:       imapgw.UID(uid),
		ModSeq:    modseq,
	}
	if err := imapgw.ValidateMessageUID(result); err != nil {
		return IMAPMessageUID{}, err
	}
	if err := tx.Commit(); err != nil {
		return IMAPMessageUID{}, fmt.Errorf("commit imap uid transaction: %w", err)
	}
	return result, nil
}
