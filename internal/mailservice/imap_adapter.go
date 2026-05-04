package mailservice

import (
	"context"

	"github.com/gogomail/gogomail/internal/imapgw"
)

type IMAPStoreAdapter struct {
	service *Service
}

var _ imapgw.Store = IMAPStoreAdapter{}

func NewIMAPStoreAdapter(service *Service) IMAPStoreAdapter {
	return IMAPStoreAdapter{service: service}
}

func (a IMAPStoreAdapter) ListMailboxes(ctx context.Context, req imapgw.ListMailboxesRequest) ([]imapgw.Mailbox, error) {
	return a.service.ListIMAPMailboxes(ctx, req)
}

func (a IMAPStoreAdapter) GetMailbox(ctx context.Context, userID imapgw.UserID, mailboxID imapgw.MailboxID) (imapgw.Mailbox, error) {
	return a.service.GetIMAPMailbox(ctx, userID, mailboxID)
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
