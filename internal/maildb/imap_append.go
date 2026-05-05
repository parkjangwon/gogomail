package maildb

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/gogomail/gogomail/internal/imapgw"
	"github.com/gogomail/gogomail/internal/message"
	smtpd "github.com/gogomail/gogomail/internal/smtp"
)

type IMAPAppendTarget struct {
	UserID      string
	MailboxID   string
	CompanyID   string
	DomainID    string
	Address     string
	UIDValidity uint32
}

type AppendStoredIMAPMessageRequest struct {
	Target       IMAPAppendTarget
	StoragePath  string
	Parsed       message.ParsedMessage
	Flags        imapgw.MessageFlags
	InternalDate time.Time
	Size         int64
}

func (r *Repository) ResolveIMAPAppendTarget(ctx context.Context, userID string, mailboxID string) (IMAPAppendTarget, error) {
	if r.db == nil {
		return IMAPAppendTarget{}, fmt.Errorf("database handle is required")
	}
	userID = strings.TrimSpace(userID)
	mailboxID = strings.TrimSpace(mailboxID)
	if userID == "" {
		return IMAPAppendTarget{}, fmt.Errorf("user_id is required")
	}
	if mailboxID == "" {
		return IMAPAppendTarget{}, fmt.Errorf("mailbox_id is required")
	}
	mailboxName := normalizeIMAPMailboxLookupName(mailboxID)

	const query = `
SELECT
  u.id::text,
  f.id::text,
  d.company_id::text,
  u.domain_id::text,
  COALESCE(ua.address, u.username || '@' || d.name)
FROM folders f
JOIN users u ON u.id = f.user_id
JOIN domains d ON d.id = u.domain_id
LEFT JOIN user_addresses ua ON ua.user_id = u.id AND ua.is_primary = true
WHERE f.user_id = $1::uuid
  AND u.status = 'active'
  AND d.status = 'active'
  AND (
    f.id::text = $2
    OR lower(f.name) = $3
    OR lower(trim(both '/' from f.full_path)) = $3
    OR ($3 = 'inbox' AND lower(COALESCE(f.system_type, '')) = 'inbox')
  )
ORDER BY
  CASE WHEN lower(COALESCE(f.system_type, '')) = 'inbox' THEN 0 ELSE 1 END,
  f.full_path,
  f.name
LIMIT 1`

	var target IMAPAppendTarget
	if err := r.db.QueryRowContext(ctx, query, userID, mailboxID, mailboxName).Scan(
		&target.UserID,
		&target.MailboxID,
		&target.CompanyID,
		&target.DomainID,
		&target.Address,
	); err != nil {
		if err == sql.ErrNoRows {
			return IMAPAppendTarget{}, fmt.Errorf("%w: %q", imapgw.ErrMailboxNotFound, mailboxID)
		}
		return IMAPAppendTarget{}, fmt.Errorf("resolve imap append target: %w", err)
	}
	state, err := r.EnsureIMAPMailboxState(ctx, target.UserID, target.MailboxID)
	if err != nil {
		return IMAPAppendTarget{}, err
	}
	target.UIDValidity = state.UIDValidity
	return target, nil
}

func (r *Repository) AppendStoredIMAPMessage(ctx context.Context, req AppendStoredIMAPMessageRequest) (imapgw.AppendMessageResult, error) {
	if r.db == nil {
		return imapgw.AppendMessageResult{}, fmt.Errorf("database handle is required")
	}
	req.Target.UserID = strings.TrimSpace(req.Target.UserID)
	req.Target.MailboxID = strings.TrimSpace(req.Target.MailboxID)
	req.Target.CompanyID = strings.TrimSpace(req.Target.CompanyID)
	req.Target.DomainID = strings.TrimSpace(req.Target.DomainID)
	req.StoragePath = strings.TrimSpace(req.StoragePath)
	if req.Target.UserID == "" {
		return imapgw.AppendMessageResult{}, fmt.Errorf("user_id is required")
	}
	if req.Target.MailboxID == "" {
		return imapgw.AppendMessageResult{}, fmt.Errorf("mailbox_id is required")
	}
	if req.Target.CompanyID == "" || req.Target.DomainID == "" {
		return imapgw.AppendMessageResult{}, fmt.Errorf("append target tenant/domain ids are required")
	}
	if req.StoragePath == "" {
		return imapgw.AppendMessageResult{}, fmt.Errorf("storage_path is required")
	}
	if req.Size < 0 {
		return imapgw.AppendMessageResult{}, fmt.Errorf("size must not be negative")
	}
	if req.InternalDate.IsZero() {
		req.InternalDate = time.Now().UTC()
	}

	flags, err := imapFlagsJSON(req.Flags)
	if err != nil {
		return imapgw.AppendMessageResult{}, err
	}
	toAddrs, err := addressesJSON(req.Parsed.To)
	if err != nil {
		return imapgw.AppendMessageResult{}, err
	}
	ccAddrs, err := addressesJSON(req.Parsed.Cc)
	if err != nil {
		return imapgw.AppendMessageResult{}, err
	}
	bccAddrs, err := addressesJSON(req.Parsed.Bcc)
	if err != nil {
		return imapgw.AppendMessageResult{}, err
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return imapgw.AppendMessageResult{}, fmt.Errorf("begin imap append transaction: %w", err)
	}
	defer tx.Rollback()

	if err := checkAndIncrementUserQuota(ctx, tx, req.Target.UserID, req.Size); err != nil {
		return imapgw.AppendMessageResult{}, err
	}
	threadID, err := r.resolveThreadID(ctx, tx, req.Target.UserID, threadCandidates(req.Parsed.InReplyTo, req.Parsed.References))
	if err != nil {
		return imapgw.AppendMessageResult{}, err
	}

	const ensureState = `
INSERT INTO imap_mailbox_state (mailbox_id, user_id)
SELECT id, user_id
FROM folders
WHERE id = $1::uuid
  AND user_id = $2::uuid
ON CONFLICT (mailbox_id) DO NOTHING`
	if _, err := tx.ExecContext(ctx, ensureState, req.Target.MailboxID, req.Target.UserID); err != nil {
		return imapgw.AppendMessageResult{}, fmt.Errorf("ensure imap append mailbox state: %w", err)
	}

	const insert = `
WITH locked_state AS (
  SELECT s.mailbox_id, s.user_id, s.uidvalidity, s.uidnext, s.highest_modseq
  FROM imap_mailbox_state s
  JOIN folders f
    ON f.id = s.mailbox_id
   AND f.user_id = s.user_id
  JOIN users u
    ON u.id = s.user_id
   AND u.domain_id = $2::uuid
   AND u.status = 'active'
  JOIN domains d
    ON d.id = u.domain_id
   AND d.company_id = $1::uuid
   AND d.status = 'active'
  WHERE s.mailbox_id = $4::uuid
    AND s.user_id = $3::uuid
  FOR UPDATE
),
inserted_message AS (
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
    flags,
    storage_path,
    status
  )
  SELECT
    $1::uuid, $2::uuid, $3::uuid, $4::uuid,
    $5, $6, NULLIF($7, '')::uuid, $8, $9, $10,
    $11::jsonb, $12::jsonb, $13::jsonb,
    $14, $15, $16, $17::jsonb, $18, 'active'
  FROM locked_state
  RETURNING
    id::text,
    folder_id::text,
    COALESCE(rfc_message_id, '') AS rfc_message_id,
    subject,
    from_addr,
    from_name,
    COALESCE(received_at, created_at) AS internal_date,
    size,
    COALESCE((flags->>'read')::boolean, false) AS read,
    COALESCE((flags->>'starred')::boolean, false) AS starred,
    COALESCE((flags->>'answered')::boolean, false) AS answered,
    COALESCE((flags->>'forwarded')::boolean, false) AS forwarded,
    COALESCE((flags->>'draft')::boolean, false) AS draft,
    COALESCE((flags->>'imap_deleted')::boolean, false) AS deleted,
    status
),
inserted_uid AS (
  INSERT INTO imap_message_uid (message_id, mailbox_id, user_id, uid, modseq)
  SELECT inserted_message.id::uuid, locked_state.mailbox_id, locked_state.user_id, locked_state.uidnext, locked_state.highest_modseq + 1
  FROM inserted_message, locked_state
  RETURNING uid, modseq
),
bumped_state AS (
  UPDATE imap_mailbox_state
  SET uidnext = uidnext + 1,
      highest_modseq = highest_modseq + 1,
      updated_at = now()
  WHERE mailbox_id = $4::uuid
    AND user_id = $3::uuid
  RETURNING uidvalidity
)
SELECT
  inserted_message.id,
  inserted_message.folder_id,
  inserted_message.rfc_message_id,
  inserted_message.subject,
  inserted_message.from_addr,
  inserted_message.from_name,
  inserted_message.internal_date,
  inserted_message.size,
  inserted_message.read,
  inserted_message.starred,
  inserted_message.answered,
  inserted_message.forwarded,
  inserted_message.draft,
  inserted_message.deleted,
  inserted_message.status,
  inserted_uid.uid,
  inserted_uid.modseq,
  bumped_state.uidvalidity
FROM inserted_message, inserted_uid, bumped_state`

	var row imapMessageRow
	var uid IMAPMessageUID
	var uidValidity uint32
	if err := tx.QueryRowContext(
		ctx,
		insert,
		req.Target.CompanyID,
		req.Target.DomainID,
		req.Target.UserID,
		req.Target.MailboxID,
		emptyToNull(req.Parsed.MessageID),
		emptyToNull(req.Parsed.InReplyTo),
		threadID,
		req.Parsed.Subject,
		req.Parsed.From.Address,
		req.Parsed.From.Name,
		string(toAddrs),
		string(ccAddrs),
		string(bccAddrs),
		req.InternalDate,
		req.Size,
		req.Parsed.HasAttachment,
		string(flags),
		req.StoragePath,
	).Scan(
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
		&row.Deleted,
		&row.Status,
		&uid.UID,
		&uid.ModSeq,
		&uidValidity,
	); err != nil {
		return imapgw.AppendMessageResult{}, fmt.Errorf("insert imap append message: %w", err)
	}
	uid.MessageID = imapgw.MessageID(row.ID)
	uid.MailboxID = imapgw.MailboxID(row.MailboxID)
	if err := imapgw.ValidateMessageUID(uid); err != nil {
		return imapgw.AppendMessageResult{}, err
	}
	sequenceNumber, err := imapSequenceNumberForUID(ctx, tx, req.Target.UserID, req.Target.MailboxID, uid.UID)
	if err != nil {
		return imapgw.AppendMessageResult{}, err
	}

	if err := r.insertStoredOutbox(ctx, tx, row.ID, row.MailboxID, smtpd.ReceivedMessage{
		EnvelopeFrom: strings.TrimSpace(req.Parsed.From.Address),
		Mailbox: smtpd.Mailbox{
			CompanyID: req.Target.CompanyID,
			DomainID:  req.Target.DomainID,
			UserID:    req.Target.UserID,
			Address:   req.Target.Address,
		},
		StoragePath: req.StoragePath,
		Parsed:      req.Parsed,
		ReceivedAt:  req.InternalDate,
		Size:        req.Size,
	}); err != nil {
		return imapgw.AppendMessageResult{}, err
	}

	if err := tx.Commit(); err != nil {
		return imapgw.AppendMessageResult{}, fmt.Errorf("commit imap append transaction: %w", err)
	}
	summary := imapMessageFromRow(row, uid)
	summary.SequenceNumber = sequenceNumber
	return imapgw.AppendMessageResult{
		Summary:     summary,
		UIDValidity: uidValidity,
	}, nil
}

func imapFlagsJSON(flags imapgw.MessageFlags) ([]byte, error) {
	raw, err := json.Marshal(map[string]bool{
		"read":         flags.Read,
		"starred":      flags.Starred,
		"answered":     flags.Answered,
		"forwarded":    flags.Forwarded,
		"draft":        flags.Draft,
		"imap_deleted": flags.Deleted,
	})
	if err != nil {
		return nil, fmt.Errorf("marshal imap append flags: %w", err)
	}
	return raw, nil
}
