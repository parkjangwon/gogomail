package mailservice

import (
	"context"
	"fmt"

	"github.com/gogomail/gogomail/internal/imapgw"
)

type IMAPStoreAdapter struct {
	service *Service
}

var _ imapgw.Store = IMAPStoreAdapter{}
var _ imapgw.MailboxSessionStore = IMAPStoreAdapter{}

func NewIMAPStoreAdapter(service *Service) IMAPStoreAdapter {
	return IMAPStoreAdapter{service: service}
}

func (a IMAPStoreAdapter) ListMailboxes(ctx context.Context, req imapgw.ListMailboxesRequest) ([]imapgw.Mailbox, error) {
	return a.service.ListIMAPMailboxes(ctx, req)
}

func (a IMAPStoreAdapter) GetMailbox(ctx context.Context, userID imapgw.UserID, mailboxID imapgw.MailboxID) (imapgw.Mailbox, error) {
	return a.service.GetIMAPMailbox(ctx, userID, mailboxID)
}

func (a IMAPStoreAdapter) CreateMailbox(ctx context.Context, userID imapgw.UserID, mailboxID imapgw.MailboxID) (imapgw.Mailbox, error) {
	return a.service.CreateIMAPMailbox(ctx, userID, mailboxID)
}

func (a IMAPStoreAdapter) DeleteMailbox(ctx context.Context, userID imapgw.UserID, mailboxID imapgw.MailboxID) error {
	return a.service.DeleteIMAPMailbox(ctx, userID, mailboxID)
}

func (a IMAPStoreAdapter) RenameMailbox(ctx context.Context, userID imapgw.UserID, mailboxID imapgw.MailboxID, newMailboxID imapgw.MailboxID) (imapgw.Mailbox, error) {
	return a.service.RenameIMAPMailbox(ctx, userID, mailboxID, newMailboxID)
}

func (a IMAPStoreAdapter) ListMessages(ctx context.Context, req imapgw.ListMessagesRequest) ([]imapgw.MessageSummary, error) {
	return a.service.ListIMAPMessages(ctx, req)
}

func (a IMAPStoreAdapter) FetchMessage(ctx context.Context, req imapgw.FetchMessageRequest) (imapgw.Message, error) {
	return a.service.FetchIMAPMessage(ctx, req)
}

func (a IMAPStoreAdapter) StoreFlags(ctx context.Context, req imapgw.StoreFlagsRequest) ([]imapgw.MessageSummary, error) {
	return a.service.StoreIMAPFlags(ctx, req)
}

func (a IMAPStoreAdapter) SelectMailbox(ctx context.Context, req imapgw.SelectMailboxRequest) (imapgw.MailboxState, error) {
	mailbox, err := a.service.GetIMAPMailbox(ctx, req.UserID, req.MailboxID)
	if err != nil {
		return imapgw.MailboxState{}, err
	}
	return imapgw.MailboxState{
		Mailbox: mailbox,
		PermanentFlags: []string{
			imapgw.FlagSeen,
			imapgw.FlagFlagged,
			imapgw.FlagAnswered,
			imapgw.FlagDraft,
			imapgw.FlagDeleted,
		},
	}, nil
}

func (a IMAPStoreAdapter) CopyMessages(ctx context.Context, req imapgw.CopyMessagesRequest) ([]imapgw.MessageSummary, error) {
	return a.service.CopyIMAPMessages(ctx, req)
}

func (a IMAPStoreAdapter) MoveMessages(context.Context, imapgw.MoveMessagesRequest) ([]imapgw.MessageSummary, error) {
	return nil, fmt.Errorf("%w: MOVE is deferred until IMAP-safe mailbox mutation semantics are implemented", imapgw.ErrUnsupportedMailboxMutation)
}

func (a IMAPStoreAdapter) Expunge(context.Context, imapgw.ExpungeRequest) ([]imapgw.UID, error) {
	return nil, fmt.Errorf("%w: EXPUNGE is deferred until IMAP-safe delete semantics are implemented", imapgw.ErrUnsupportedMailboxMutation)
}

func (a IMAPStoreAdapter) Subscribe(ctx context.Context, userID imapgw.UserID, mailboxID imapgw.MailboxID) (<-chan imapgw.MailboxEvent, func(), error) {
	return a.service.SubscribeIMAPMailbox(ctx, userID, mailboxID)
}
