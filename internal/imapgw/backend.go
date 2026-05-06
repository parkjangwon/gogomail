package imapgw

import (
	"context"
	"errors"
)

var ErrUnsupportedMailboxMutation = errors.New("imap mailbox mutation is not supported")
var ErrUnsupportedAppend = errors.New("imap append is not supported")
var ErrMailboxNotFound = errors.New("imap mailbox not found")
var ErrOverQuota = errors.New("imap append is over quota")

type Session struct {
	UserID      UserID
	Username    string
	DomainID    string
	DisplayName string
}

type Authenticator interface {
	Authenticate(ctx context.Context, username string, password string) (Session, error)
}

type MailboxState struct {
	Mailbox
	PermanentFlags []string
	UIDNotSticky   bool
}

type SelectMailboxRequest struct {
	UserID    UserID
	MailboxID MailboxID
	ReadOnly  bool
}

type MoveMessagesRequest struct {
	UserID          UserID
	SourceMailboxID MailboxID
	DestMailboxID   MailboxID
	UIDs            []UID
}

type MoveMessageResult struct {
	Source              MessageSummary
	Destination         MessageSummary
	SourceHighestModSeq uint64
}

type CopyMessageResult struct {
	SourceUID   UID
	Destination MessageSummary
}

type CopyMessagesRequest struct {
	UserID          UserID
	SourceMailboxID MailboxID
	DestMailboxID   MailboxID
	UIDs            []UID
}

type ExpungeRequest struct {
	UserID    UserID
	MailboxID MailboxID
	UIDs      []UID
}

type MailboxEventType string

const (
	MailboxEventExists  MailboxEventType = "exists"
	MailboxEventExpunge MailboxEventType = "expunge"
	MailboxEventFlags   MailboxEventType = "flags"
)

type MailboxEvent struct {
	Type           MailboxEventType
	UserID         UserID
	MailboxID      MailboxID
	UID            UID
	SequenceNumber uint32
	Messages       uint32
}

type MailboxSessionStore interface {
	SelectMailbox(ctx context.Context, req SelectMailboxRequest) (MailboxState, error)
	CopyMessages(ctx context.Context, req CopyMessagesRequest) ([]CopyMessageResult, error)
	MoveMessages(ctx context.Context, req MoveMessagesRequest) ([]MoveMessageResult, error)
	Expunge(ctx context.Context, req ExpungeRequest) ([]MessageSummary, error)
	Subscribe(ctx context.Context, userID UserID, mailboxID MailboxID) (<-chan MailboxEvent, func(), error)
}

type Backend interface {
	Authenticator
	Store
	MailboxSessionStore
}
