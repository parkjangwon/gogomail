package maildb

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/gogomail/gogomail/internal/imapgw"
	"github.com/lib/pq"
)

type IMAPUIDState = imapgw.UIDState
type IMAPMessageUID = imapgw.MessageUID

var ErrIMAPMessageNotActive = errors.New("imap message is not active in mailbox")

const maxIMAPUIDBackfillAuditSample = 10

type IMAPStoredMessage struct {
	Summary     imapgw.MessageSummary
	StoragePath string
}

type imapSequenceQuerier interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

const (
	imapUIDBackfillDefaultLimit = 500
	imapUIDBackfillMaxLimit     = 5000
)

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
	if len(folders) == 0 {
		return nil, nil
	}

	states, err := r.ensureIMAPMailboxStates(ctx, userID, folders)
	if err != nil {
		return nil, err
	}

	mailboxes := make([]imapgw.Mailbox, 0, len(folders))
	for _, folder := range folders {
		state, ok := states[folder.ID]
		if !ok {
			return nil, fmt.Errorf("imap mailbox state missing for folder %q", folder.ID)
		}
		mailboxes = append(mailboxes, imapMailboxFromFolder(folder, state))
	}
	return mailboxes, nil
}

// ensureIMAPMailboxStates upserts imap_mailbox_state rows for all given folders
// in a single query and returns a map of folderID → IMAPUIDState.
func (r *Repository) ensureIMAPMailboxStates(ctx context.Context, userID string, folders []Folder) (map[string]IMAPUIDState, error) {
	folderIDs := make([]string, len(folders))
	for i, f := range folders {
		folderIDs[i] = f.ID
	}

	const query = `
INSERT INTO imap_mailbox_state (mailbox_id, user_id)
SELECT id, user_id
FROM folders
WHERE user_id = $1::uuid
  AND id = ANY($2::uuid[])
ON CONFLICT (mailbox_id) DO UPDATE
SET updated_at = imap_mailbox_state.updated_at
RETURNING mailbox_id::text, uidvalidity, uidnext, highest_modseq`

	rows, err := r.db.QueryContext(ctx, query, userID, pq.Array(folderIDs))
	if err != nil {
		return nil, fmt.Errorf("ensure imap mailbox states: %w", err)
	}
	defer rows.Close()

	states := make(map[string]IMAPUIDState, len(folders))
	for rows.Next() {
		var state IMAPUIDState
		if err := rows.Scan(&state.MailboxID, &state.UIDValidity, &state.UIDNext, &state.HighestModSeq); err != nil {
			return nil, fmt.Errorf("scan imap mailbox state: %w", err)
		}
		if err := imapgw.ValidateUIDState(state); err != nil {
			return nil, err
		}
		states[string(state.MailboxID)] = state
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate imap mailbox states: %w", err)
	}
	return states, nil
}

func (r *Repository) GetIMAPMailbox(ctx context.Context, userID string, mailboxID string) (imapgw.Mailbox, error) {
	if r.db == nil {
		return imapgw.Mailbox{}, fmt.Errorf("database handle is required")
	}
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return imapgw.Mailbox{}, fmt.Errorf("user_id is required")
	}
	if strings.TrimSpace(mailboxID) == "" {
		return imapgw.Mailbox{}, fmt.Errorf("mailbox_id is required")
	}
	exactMailboxName := strings.ToLower(mailboxID)
	compatMailboxName, allowCompatMailboxLookup := normalizeIMAPMailboxLookupName(mailboxID)
	var mailboxIDUUID any
	if isUUIDLike(mailboxID) {
		mailboxIDUUID = strings.TrimSpace(mailboxID)
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
  COALESCE(c.starred, 0) AS starred,
  COALESCE(c.total_size, 0) AS total_size,
  COALESCE(c.imap_unassigned, 0) AS imap_unassigned
FROM folders f
LEFT JOIN (
  SELECT
    m.folder_id,
    COUNT(*) AS total,
    COUNT(*) FILTER (WHERE COALESCE((flags->>'read')::boolean, false) = false) AS unread,
    COUNT(*) FILTER (WHERE COALESCE((flags->>'starred')::boolean, false) = true) AS starred,
    SUM(size) AS total_size,
    COUNT(*) FILTER (WHERE i.message_id IS NULL) AS imap_unassigned
  FROM messages m
  LEFT JOIN imap_message_uid i
    ON i.message_id = m.id
   AND i.user_id = m.user_id
   AND i.mailbox_id = m.folder_id
  WHERE m.user_id = $1::uuid
    AND m.status = 'active'
  GROUP BY m.folder_id
) c ON c.folder_id = f.id
WHERE f.user_id = $1::uuid
  AND (
    ($2::uuid IS NOT NULL AND f.id = $2::uuid)
    OR lower(f.name) = $3
    OR (
      $5
      AND (
        lower(f.name) = $4
        OR lower(trim(both '/' from f.full_path)) = $4
        OR ($4 = 'inbox' AND lower(COALESCE(f.system_type, '')) = 'inbox')
      )
    )
  )
ORDER BY
  CASE WHEN lower(COALESCE(f.system_type, '')) = 'inbox' THEN 0 ELSE 1 END,
  f.full_path,
  f.name
LIMIT 1`

	var folder Folder
	if err := r.db.QueryRowContext(ctx, query, userID, mailboxIDUUID, exactMailboxName, compatMailboxName, allowCompatMailboxLookup).Scan(
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
		&folder.TotalSize,
		&folder.IMAPUnassigned,
	); err != nil {
		if err == sql.ErrNoRows {
			return imapgw.Mailbox{}, fmt.Errorf("%w: %q", imapgw.ErrMailboxNotFound, mailboxID)
		}
		return imapgw.Mailbox{}, fmt.Errorf("get imap mailbox: %w", err)
	}
	state, err := r.EnsureIMAPMailboxState(ctx, userID, folder.ID)
	if err != nil {
		return imapgw.Mailbox{}, err
	}
	return imapMailboxFromFolder(folder, state), nil
}

func normalizeIMAPMailboxLookupName(value string) (string, bool) {
	allowCompatLookup := value == strings.TrimSpace(value)
	value = strings.Trim(value, `"`)
	value = strings.Trim(value, "/")
	value = collapseIMAPMailboxLookupWhitespace(value)
	return strings.ToLower(value), allowCompatLookup
}

func collapseIMAPMailboxLookupWhitespace(value string) string {
	if value == "" {
		return ""
	}
	out := make([]byte, 0, len(value))
	inSpace := false
	for i := 0; i < len(value); i++ {
		switch value[i] {
		case ' ', '\t', '\r', '\n':
			if len(out) > 0 {
				inSpace = true
			}
		default:
			if inSpace {
				out = append(out, ' ')
				inSpace = false
			}
			out = append(out, value[i])
		}
	}
	return string(out)
}

func imapMailboxFromFolder(folder Folder, state IMAPUIDState) imapgw.Mailbox {
	uidNext := state.UIDNext
	highestModSeq := state.HighestModSeq
	if folder.IMAPUnassigned > 0 {
		uidNext = addIMAPUIDOffset(uidNext, folder.IMAPUnassigned)
		highestModSeq = addIMAPModSeqOffset(highestModSeq, folder.IMAPUnassigned)
	}
	return imapgw.Mailbox{
		ID:            imapgw.MailboxID(folder.ID),
		ParentID:      imapgw.MailboxID(folder.ParentID),
		Name:          folder.Name,
		FullPath:      folder.FullPath,
		SystemType:    folder.SystemType,
		UIDValidity:   state.UIDValidity,
		UIDNext:       uidNext,
		HighestModSeq: highestModSeq,
		Messages:      uint32(folder.Total),
		Unseen:        uint32(folder.Unread),
		Size:          folder.TotalSize,
	}
}

func (r *Repository) EnsureIMAPMailboxState(ctx context.Context, userID string, mailboxID string) (IMAPUIDState, error) {
	if r.db == nil {
		return IMAPUIDState{}, fmt.Errorf("database handle is required")
	}
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return IMAPUIDState{}, fmt.Errorf("user_id is required")
	}
	if strings.TrimSpace(mailboxID) == "" {
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
