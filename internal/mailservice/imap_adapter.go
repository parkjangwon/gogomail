package mailservice

import (
	"context"

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

func (a IMAPStoreAdapter) ListSubscribedMailboxes(ctx context.Context, req imapgw.ListMailboxesRequest) ([]imapgw.MailboxSubscription, error) {
	return a.service.ListSubscribedIMAPMailboxes(ctx, req)
}

func (a IMAPStoreAdapter) GetMailbox(ctx context.Context, userID imapgw.UserID, mailboxID imapgw.MailboxID) (imapgw.Mailbox, error) {
	return a.service.GetIMAPMailbox(ctx, userID, mailboxID)
}

func (a IMAPStoreAdapter) SubscribeMailbox(ctx context.Context, userID imapgw.UserID, mailboxID imapgw.MailboxID) (imapgw.MailboxSubscription, error) {
	return a.service.SubscribeIMAPMailboxName(ctx, userID, mailboxID)
}

func (a IMAPStoreAdapter) UnsubscribeMailbox(ctx context.Context, userID imapgw.UserID, mailboxID imapgw.MailboxID) error {
	return a.service.UnsubscribeIMAPMailboxName(ctx, userID, mailboxID)
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

func (a IMAPStoreAdapter) AppendMessage(ctx context.Context, req imapgw.AppendMessageRequest) (imapgw.AppendMessageResult, error) {
	return a.service.AppendIMAPMessage(ctx, req)
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
			imapgw.FlagForwarded,
			imapgw.FlagDraft,
			imapgw.FlagDeleted,
		},
	}, nil
}

func (a IMAPStoreAdapter) CopyMessages(ctx context.Context, req imapgw.CopyMessagesRequest) ([]imapgw.CopyMessageResult, error) {
	return a.service.CopyIMAPMessages(ctx, req)
}

func (a IMAPStoreAdapter) MoveMessages(ctx context.Context, req imapgw.MoveMessagesRequest) ([]imapgw.MoveMessageResult, error) {
	return a.service.MoveIMAPMessages(ctx, req)
}

func (a IMAPStoreAdapter) Expunge(ctx context.Context, req imapgw.ExpungeRequest) ([]imapgw.MessageSummary, error) {
	return a.service.ExpungeIMAPMessages(ctx, req)
}

func (a IMAPStoreAdapter) Subscribe(ctx context.Context, userID imapgw.UserID, mailboxID imapgw.MailboxID) (<-chan imapgw.MailboxEvent, func(), error) {
	return a.service.SubscribeIMAPMailbox(ctx, userID, mailboxID)
}
