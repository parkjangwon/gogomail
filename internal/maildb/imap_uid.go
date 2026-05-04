package maildb

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

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
