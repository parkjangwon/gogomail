package imapgw

import (
	"context"
	"errors"
)

var ErrUnsupportedMailboxMutation = errors.New("imap mailbox mutation is not supported")

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
	Type      MailboxEventType
	UserID    UserID
	MailboxID MailboxID
	UID       UID
	Messages  uint32
}

type MailboxSessionStore interface {
	SelectMailbox(ctx context.Context, req SelectMailboxRequest) (MailboxState, error)
	CopyMessages(ctx context.Context, req CopyMessagesRequest) ([]MessageSummary, error)
	MoveMessages(ctx context.Context, req MoveMessagesRequest) ([]MessageSummary, error)
	Expunge(ctx context.Context, req ExpungeRequest) ([]UID, error)
	Subscribe(ctx context.Context, userID UserID, mailboxID MailboxID) (<-chan MailboxEvent, func(), error)
}

type Backend interface {
	Authenticator
	Store
	MailboxSessionStore
}
