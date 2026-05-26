package maildb

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/gogomail/gogomail/internal/imapgw"
	"github.com/lib/pq"
)

func (r *Repository) StoreIMAPFlags(ctx context.Context, userID string, mailboxID string, uids []imapgw.UID, flags imapgw.MessageFlags, mode imapgw.StoreFlagsMode, unchangedSince uint64, unchangedSinceSet bool) ([]imapgw.MessageSummary, error) {
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
	if len(uids) == 0 {
		return nil, fmt.Errorf("uids are required")
	}
	if len(uids) > 500 {
		return nil, fmt.Errorf("too many uids")
	}
	for _, uid := range uids {
		if uid == 0 {
			return nil, fmt.Errorf("uid must not be zero")
		}
	}
	changes, err := newIMAPStoreFlagChanges(flags, mode)
	if err != nil {
		return nil, err
	}

	if _, err := r.EnsureIMAPMailboxState(ctx, userID, mailboxID); err != nil {
		return nil, err
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin imap store flags transaction: %w", err)
	}
	defer tx.Rollback()

	var highestModSeq uint64
	const lockMailbox = `
SELECT highest_modseq
FROM imap_mailbox_state
WHERE mailbox_id = $1::uuid
  AND user_id = $2::uuid
FOR UPDATE`
	if err := tx.QueryRowContext(ctx, lockMailbox, mailboxID, userID).Scan(&highestModSeq); err != nil {
		return nil, fmt.Errorf("lock imap mailbox state: %w", err)
	}

	type storeCandidate struct {
		uid        imapgw.UID
		row        imapMessageRow
		messageUID IMAPMessageUID
	}
	// Batch SELECT all rows for the requested UIDs in one query (preserving
	// FOR UPDATE locking on i and m), preserving caller order via WITH
	// ORDINALITY on the input UID array.
	uidsArray := imapUIDArray(uids)
	const batchSelect = `
WITH input AS (
  SELECT value::bigint AS uid, ordinality
  FROM unnest($3::bigint[]) WITH ORDINALITY AS input(value, ordinality)
)
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
FROM input
JOIN imap_message_uid i
  ON i.uid = input.uid
 AND i.user_id = $1::uuid
 AND i.mailbox_id = $2::uuid
JOIN messages m
  ON m.id = i.message_id
 AND m.user_id = $1::uuid
 AND m.folder_id = $2::uuid
 AND m.status = 'active'
ORDER BY input.ordinality
FOR UPDATE OF i, m`
	rows, err := tx.QueryContext(ctx, batchSelect, userID, mailboxID, pq.Array(uidsArray))
	if err != nil {
		return nil, fmt.Errorf("list imap store flags messages: %w", err)
	}
	candidatesByUID := make(map[imapgw.UID]storeCandidate, len(uids))
	for rows.Next() {
		var row imapMessageRow
		var messageUID IMAPMessageUID
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
			&messageUID.UID,
			&messageUID.ModSeq,
		); err != nil {
			rows.Close()
			return nil, fmt.Errorf("scan imap store flags message: %w", err)
		}
		messageUID.MessageID = imapgw.MessageID(row.ID)
		messageUID.MailboxID = imapgw.MailboxID(row.MailboxID)
		if err := imapgw.ValidateMessageUID(messageUID); err != nil {
			rows.Close()
			return nil, err
		}
		candidatesByUID[messageUID.UID] = storeCandidate{uid: messageUID.UID, row: row, messageUID: messageUID}
	}
	if err := rows.Close(); err != nil {
		return nil, fmt.Errorf("close imap store flags rows: %w", err)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate imap store flags rows: %w", err)
	}
	candidates := make([]storeCandidate, 0, len(uids))
	for _, uid := range uids {
		c, ok := candidatesByUID[uid]
		if !ok {
			return nil, fmt.Errorf("imap message uid %d not found", uid)
		}
		candidates = append(candidates, c)
	}

	// Compute sequence numbers for all candidate UIDs in one window-function
	// query, mirroring MoveIMAPMessages.
	seqUIDValues := make([]int64, 0, len(candidates))
	for _, c := range candidates {
		seqUIDValues = append(seqUIDValues, int64(c.uid))
	}
	sequenceNumbers, err := imapSequenceNumbersForUIDs(ctx, tx, userID, mailboxID, seqUIDValues)
	if err != nil {
		return nil, err
	}

	summaries := make([]imapgw.MessageSummary, 0, len(candidates))
	modified := make([]imapgw.UID, 0)
	changedAny := false
	for _, candidate := range candidates {
		row := candidate.row
		messageUID := candidate.messageUID
		if unchangedSinceSet && messageUID.ModSeq > unchangedSince {
			modified = append(modified, candidate.uid)
			continue
		}
		next, changed := applyIMAPStoreFlagChanges(row, changes)
		if changed {
			changedAny = true
			highestModSeq++
			messageUID.ModSeq = highestModSeq
			if err := updateIMAPMessageFlags(ctx, tx, row.ID, messageUID.ModSeq, next); err != nil {
				return nil, err
			}
			row = next
		}
		summary := imapMessageFromRow(row, messageUID)
		sequenceNumber, ok := sequenceNumbers[int64(candidate.uid)]
		if !ok {
			return nil, fmt.Errorf("imap sequence number unavailable for uid %d", candidate.uid)
		}
		summary.SequenceNumber = sequenceNumber
		summaries = append(summaries, summary)
	}
	if changedAny {
		if err := updateIMAPMailboxModSeq(ctx, tx, mailboxID, highestModSeq); err != nil {
			return nil, err
		}
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit imap store flags transaction: %w", err)
	}
	sort.Slice(summaries, func(i, j int) bool {
		return summaries[i].UID < summaries[j].UID
	})
	if len(modified) > 0 {
		return summaries, &imapgw.StoreModifiedError{UIDs: modified, Summaries: summaries}
	}
	return summaries, nil
}

type imapStoreFlagChanges struct {
	Read      *bool
	Starred   *bool
	Answered  *bool
	Forwarded *bool
	Deleted   *bool
	Keywords  imapKeywordList
	Mode      imapgw.StoreFlagsMode
}

func newIMAPStoreFlagChanges(flags imapgw.MessageFlags, mode imapgw.StoreFlagsMode) (imapStoreFlagChanges, error) {
	if flags.Draft || strings.TrimSpace(flags.Status) != "" {
		return imapStoreFlagChanges{}, fmt.Errorf("unsupported imap store flag set")
	}
	keywords, err := canonicalMailDBIMAPKeywords(flags.Keywords)
	if err != nil {
		return imapStoreFlagChanges{}, err
	}
	changes := imapStoreFlagChanges{Keywords: keywords, Mode: mode}
	switch mode {
	case imapgw.StoreFlagsAdd:
		if flags.Read {
			changes.Read = boolPointer(true)
		}
		if flags.Starred {
			changes.Starred = boolPointer(true)
		}
		if flags.Answered {
			changes.Answered = boolPointer(true)
		}
		if flags.Forwarded {
			changes.Forwarded = boolPointer(true)
		}
		if flags.Deleted {
			changes.Deleted = boolPointer(true)
		}
	case imapgw.StoreFlagsRemove:
		if flags.Read {
			changes.Read = boolPointer(false)
		}
		if flags.Starred {
			changes.Starred = boolPointer(false)
		}
		if flags.Answered {
			changes.Answered = boolPointer(false)
		}
		if flags.Forwarded {
			changes.Forwarded = boolPointer(false)
		}
		if flags.Deleted {
			changes.Deleted = boolPointer(false)
		}
	case imapgw.StoreFlagsReplace:
		changes.Read = boolPointer(flags.Read)
		changes.Starred = boolPointer(flags.Starred)
		changes.Answered = boolPointer(flags.Answered)
		changes.Forwarded = boolPointer(flags.Forwarded)
		changes.Deleted = boolPointer(flags.Deleted)
	default:
		return imapStoreFlagChanges{}, fmt.Errorf("unsupported imap store flags mode %q", mode)
	}
	if changes.Read == nil && changes.Starred == nil && changes.Answered == nil && changes.Forwarded == nil && changes.Deleted == nil && len(changes.Keywords) == 0 && mode != imapgw.StoreFlagsReplace {
		return imapStoreFlagChanges{}, fmt.Errorf("imap flags are required")
	}
	return changes, nil
}

func applyIMAPStoreFlagChanges(row imapMessageRow, changes imapStoreFlagChanges) (imapMessageRow, bool) {
	next := row
	if changes.Read != nil {
		next.Read = *changes.Read
	}
	if changes.Starred != nil {
		next.Starred = *changes.Starred
	}
	if changes.Answered != nil {
		next.Answered = *changes.Answered
	}
	if changes.Forwarded != nil {
		next.Forwarded = *changes.Forwarded
	}
	if changes.Deleted != nil {
		next.Deleted = *changes.Deleted
	}
	switch changes.Mode {
	case imapgw.StoreFlagsAdd:
		next.Keywords = addIMAPKeywords(row.Keywords, changes.Keywords)
	case imapgw.StoreFlagsRemove:
		next.Keywords = removeIMAPKeywords(row.Keywords, changes.Keywords)
	case imapgw.StoreFlagsReplace:
		next.Keywords = append(imapKeywordList(nil), changes.Keywords...)
	}
	return next, next.Read != row.Read || next.Starred != row.Starred || next.Answered != row.Answered || next.Forwarded != row.Forwarded || next.Deleted != row.Deleted || !imapKeywordListsEqual(next.Keywords, row.Keywords)
}

type imapKeywordList []string

func (keywords *imapKeywordList) Scan(value any) error {
	if value == nil {
		*keywords = nil
		return nil
	}
	var raw []byte
	switch typed := value.(type) {
	case []byte:
		raw = typed
	case string:
		raw = []byte(typed)
	default:
		return fmt.Errorf("scan imap keywords: unsupported value type %T", value)
	}
	var parsed []string
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return fmt.Errorf("scan imap keywords: %w", err)
	}
	canonical, err := canonicalMailDBIMAPKeywords(parsed)
	if err != nil {
		return err
	}
	*keywords = canonical
	return nil
}

func canonicalMailDBIMAPKeywords(keywords []string) (imapKeywordList, error) {
	if len(keywords) == 0 {
		return nil, nil
	}
	out := make(imapKeywordList, 0, len(keywords))
	seen := make(map[string]struct{}, len(keywords))
	for _, keyword := range keywords {
		canonical := imapgw.CanonicalIMAPFlag(keyword)
		if !imapgw.IMAPKeywordFlagValid(canonical) {
			return nil, fmt.Errorf("invalid imap keyword flag %q", keyword)
		}
		if _, ok := seen[canonical]; ok {
			continue
		}
		seen[canonical] = struct{}{}
		out = append(out, canonical)
	}
	return out, nil
}

func addIMAPKeywords(existing imapKeywordList, additions imapKeywordList) imapKeywordList {
	if len(additions) == 0 {
		return append(imapKeywordList(nil), existing...)
	}
	out := append(imapKeywordList(nil), existing...)
	seen := make(map[string]struct{}, len(existing)+len(additions))
	for _, keyword := range existing {
		seen[keyword] = struct{}{}
	}
	for _, keyword := range additions {
		if _, ok := seen[keyword]; ok {
			continue
		}
		seen[keyword] = struct{}{}
		out = append(out, keyword)
	}
	return out
}

func removeIMAPKeywords(existing imapKeywordList, removals imapKeywordList) imapKeywordList {
	if len(existing) == 0 || len(removals) == 0 {
		return append(imapKeywordList(nil), existing...)
	}
	remove := make(map[string]struct{}, len(removals))
	for _, keyword := range removals {
		remove[keyword] = struct{}{}
	}
	out := make(imapKeywordList, 0, len(existing))
	for _, keyword := range existing {
		if _, ok := remove[keyword]; ok {
			continue
		}
		out = append(out, keyword)
	}
	return out
}

func imapKeywordListsEqual(a imapKeywordList, b imapKeywordList) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func updateIMAPMessageFlags(ctx context.Context, tx *sql.Tx, messageID string, modseq uint64, row imapMessageRow) error {
	keywords, err := json.Marshal([]string(row.Keywords))
	if err != nil {
		return fmt.Errorf("marshal imap message keywords: %w", err)
	}
	const updateMessage = `
UPDATE messages
SET flags = jsonb_set(
      jsonb_set(
        jsonb_set(
          jsonb_set(
            jsonb_set(
              jsonb_set(COALESCE(flags, '{}'::jsonb), '{read}', to_jsonb($2::boolean), true),
              '{starred}', to_jsonb($3::boolean), true
            ),
            '{answered}', to_jsonb($4::boolean), true
          ),
          '{forwarded}', to_jsonb($5::boolean), true
        ),
        '{imap_deleted}', to_jsonb($6::boolean), true
      ),
      '{imap_keywords}', $7::jsonb, true
    ),
    updated_at = now()
WHERE id = $1::uuid`
	if _, err := tx.ExecContext(ctx, updateMessage, messageID, row.Read, row.Starred, row.Answered, row.Forwarded, row.Deleted, string(keywords)); err != nil {
		return fmt.Errorf("update imap message flags: %w", err)
	}

	const updateUID = `
UPDATE imap_message_uid
SET modseq = $2,
    updated_at = now()
WHERE message_id = $1::uuid`
	if _, err := tx.ExecContext(ctx, updateUID, messageID, int64(modseq)); err != nil {
		return fmt.Errorf("update imap message modseq: %w", err)
	}
	return nil
}

func updateIMAPMailboxModSeq(ctx context.Context, tx *sql.Tx, mailboxID string, highestModSeq uint64) error {
	const query = `
UPDATE imap_mailbox_state
SET highest_modseq = $2,
    updated_at = now()
WHERE mailbox_id = $1::uuid`
	if _, err := tx.ExecContext(ctx, query, mailboxID, int64(highestModSeq)); err != nil {
		return fmt.Errorf("update imap mailbox modseq: %w", err)
	}
	return nil
}
