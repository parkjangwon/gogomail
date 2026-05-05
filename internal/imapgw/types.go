package imapgw

import (
	"context"
	"io"
	"time"
)

// UID is an RFC 3501 message UID. Adapters should not expose zero as a valid UID.
type UID uint32

type MailboxID string
type MessageID string
type UserID string

type Mailbox struct {
	ID          MailboxID
	ParentID    MailboxID
	Name        string
	FullPath    string
	SystemType  string
	UIDValidity uint32
	UIDNext     UID
	Messages    uint32
	Recent      uint32
	Unseen      uint32
}

type MessageSummary struct {
	ID             MessageID
	MailboxID      MailboxID
	UID            UID
	SequenceNumber uint32
	Envelope       Envelope
	Flags          MessageFlags
	InternalDate   time.Time
	Size           int64
}

type Message struct {
	Summary MessageSummary
	Body    io.ReadCloser
}

type Envelope struct {
	MessageID string
	Subject   string
	From      []Address
	To        []Address
	Cc        []Address
	Bcc       []Address
	Date      time.Time
}

type Address struct {
	Name    string
	Mailbox string
	Host    string
}

type ListMailboxesRequest struct {
	UserID UserID
}

type ListMessagesRequest struct {
	UserID    UserID
	MailboxID MailboxID
	Limit     int
	AfterUID  UID
}

type FetchMessageRequest struct {
	UserID    UserID
	MailboxID MailboxID
	UID       UID
}

type StoreFlagsRequest struct {
	UserID    UserID
	MailboxID MailboxID
	UIDs      []UID
	Flags     MessageFlags
	Mode      StoreFlagsMode
}

type StoreFlagsMode string

const (
	StoreFlagsReplace StoreFlagsMode = "replace"
	StoreFlagsAdd     StoreFlagsMode = "add"
	StoreFlagsRemove  StoreFlagsMode = "remove"
)

type MailboxStore interface {
	ListMailboxes(ctx context.Context, req ListMailboxesRequest) ([]Mailbox, error)
	GetMailbox(ctx context.Context, userID UserID, mailboxID MailboxID) (Mailbox, error)
	CreateMailbox(ctx context.Context, userID UserID, mailboxID MailboxID) (Mailbox, error)
}

type MessageStore interface {
	ListMessages(ctx context.Context, req ListMessagesRequest) ([]MessageSummary, error)
	FetchMessage(ctx context.Context, req FetchMessageRequest) (Message, error)
	StoreFlags(ctx context.Context, req StoreFlagsRequest) ([]MessageSummary, error)
}

type Store interface {
	MailboxStore
	MessageStore
}
