package maildb

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/gogomail/gogomail/internal/imapgw"
)

func (r *Repository) ListSubscribedIMAPMailboxes(ctx context.Context, userID string) ([]imapgw.MailboxSubscription, error) {
	if r.db == nil {
		return nil, fmt.Errorf("database handle is required")
	}
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return nil, fmt.Errorf("user_id is required")
	}

	rows, err := r.db.QueryContext(ctx, `
SELECT mailbox_name, canonical_name
FROM imap_mailbox_subscriptions
WHERE user_id = $1::uuid
ORDER BY mailbox_name`, userID)
	if err != nil {
		return nil, fmt.Errorf("list imap mailbox subscriptions: %w", err)
	}
	defer rows.Close()

	type storedSubscription struct {
		name      string
		canonical string
	}
	var stored []storedSubscription
	for rows.Next() {
		var sub storedSubscription
		if err := rows.Scan(&sub.name, &sub.canonical); err != nil {
			return nil, fmt.Errorf("scan imap mailbox subscription: %w", err)
		}
		stored = append(stored, sub)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate imap mailbox subscriptions: %w", err)
	}
	if len(stored) == 0 {
		return nil, nil
	}

	mailboxes, err := r.ListIMAPMailboxes(ctx, userID)
	if err != nil {
		return nil, err
	}
	byCanonical := make(map[string]imapgw.Mailbox, len(mailboxes))
	for _, mailbox := range mailboxes {
		for _, name := range []string{
			imapgw.MailboxPath(mailbox),
			imapgw.MailboxDisplayName(mailbox),
			string(mailbox.ID),
		} {
			if canonical := canonicalIMAPSubscriptionName(name); canonical != "" {
				byCanonical[canonical] = mailbox
			}
		}
	}

	out := make([]imapgw.MailboxSubscription, 0, len(stored))
	for _, sub := range stored {
		item := imapgw.MailboxSubscription{Name: sub.name}
		if mailbox, ok := byCanonical[sub.canonical]; ok {
			item.Mailbox = mailbox
			item.Exists = true
		}
		out = append(out, item)
	}
	return out, nil
}

func (r *Repository) SubscribeIMAPMailbox(ctx context.Context, userID string, mailboxID string) (imapgw.MailboxSubscription, error) {
	if r.db == nil {
		return imapgw.MailboxSubscription{}, fmt.Errorf("database handle is required")
	}
	userID = strings.TrimSpace(userID)
	mailboxID = strings.TrimSpace(mailboxID)
	if userID == "" {
		return imapgw.MailboxSubscription{}, fmt.Errorf("user_id is required")
	}
	if mailboxID == "" {
		return imapgw.MailboxSubscription{}, fmt.Errorf("mailbox_id is required")
	}
	mailbox, err := r.GetIMAPMailbox(ctx, userID, mailboxID)
	exists := true
	name := ""
	if err != nil {
		if !isIMAPMailboxNotFound(err) {
			return imapgw.MailboxSubscription{}, err
		}
		exists = false
		name = strings.Trim(strings.TrimSpace(mailboxID), `"`)
	} else {
		name = imapSubscriptionDisplayName(mailbox)
	}
	canonical := canonicalIMAPSubscriptionName(name)
	if canonical == "" {
		return imapgw.MailboxSubscription{}, fmt.Errorf("imap mailbox subscription name is required")
	}
	if _, err := r.db.ExecContext(ctx, `
INSERT INTO imap_mailbox_subscriptions (user_id, mailbox_name, canonical_name)
VALUES ($1::uuid, $2, $3)
ON CONFLICT (user_id, canonical_name)
DO UPDATE SET mailbox_name = EXCLUDED.mailbox_name`, userID, name, canonical); err != nil {
		return imapgw.MailboxSubscription{}, fmt.Errorf("subscribe imap mailbox: %w", err)
	}
	return imapgw.MailboxSubscription{Name: name, Mailbox: mailbox, Exists: exists}, nil
}

func (r *Repository) UnsubscribeIMAPMailbox(ctx context.Context, userID string, mailboxID string) error {
	if r.db == nil {
		return fmt.Errorf("database handle is required")
	}
	userID = strings.TrimSpace(userID)
	mailboxID = strings.TrimSpace(mailboxID)
	if userID == "" {
		return fmt.Errorf("user_id is required")
	}
	if mailboxID == "" {
		return fmt.Errorf("mailbox_id is required")
	}

	name := mailboxID
	if mailbox, err := r.GetIMAPMailbox(ctx, userID, mailboxID); err == nil {
		name = imapSubscriptionDisplayName(mailbox)
	} else if !isIMAPMailboxNotFound(err) {
		return err
	}
	canonical := canonicalIMAPSubscriptionName(name)
	if canonical == "" {
		return fmt.Errorf("imap mailbox subscription name is required")
	}
	if _, err := r.db.ExecContext(ctx, `
DELETE FROM imap_mailbox_subscriptions
WHERE user_id = $1::uuid
  AND canonical_name = $2`, userID, canonical); err != nil {
		return fmt.Errorf("unsubscribe imap mailbox: %w", err)
	}
	return nil
}

func imapSubscriptionDisplayName(mailbox imapgw.Mailbox) string {
	if name := imapgw.MailboxPath(mailbox); name != "" {
		return name
	}
	if name := imapgw.MailboxDisplayName(mailbox); name != "" {
		return name
	}
	return strings.TrimSpace(string(mailbox.ID))
}

func canonicalIMAPSubscriptionName(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

func isIMAPMailboxNotFound(err error) bool {
	if err == nil {
		return false
	}
	if err == sql.ErrNoRows {
		return true
	}
	return errors.Is(err, imapgw.ErrMailboxNotFound)
}
