package jmap

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strconv"
)

// EmailStateFor returns opaque state for Email/Thread objects.
// Uses MAX(highest_modseq) across all IMAP mailboxes for the user.
func EmailStateFor(ctx context.Context, db *sql.DB, userID string) (string, error) {
	var maxSeq int64
	err := db.QueryRowContext(ctx,
		`SELECT COALESCE(MAX(highest_modseq), 0) FROM imap_mailbox_state WHERE user_id = $1`,
		userID,
	).Scan(&maxSeq)
	if err != nil {
		return "0", fmt.Errorf("email state: %w", err)
	}
	return strconv.FormatInt(maxSeq, 10), nil
}

// MailboxStateFor returns opaque state for Mailbox objects.
// Uses session_version from the users table.
func MailboxStateFor(ctx context.Context, db *sql.DB, userID string) (string, error) {
	var ver int64
	err := db.QueryRowContext(ctx,
		`SELECT session_version FROM users WHERE id = $1`, userID,
	).Scan(&ver)
	if errors.Is(err, sql.ErrNoRows) {
		return "0", nil
	}
	if err != nil {
		return "0", fmt.Errorf("mailbox state: %w", err)
	}
	return strconv.FormatInt(ver, 10), nil
}

// ParseModSeqState parses a state string back to int64; returns 0 on error.
func ParseModSeqState(state string) int64 {
	v, _ := strconv.ParseInt(state, 10, 64)
	return v
}
