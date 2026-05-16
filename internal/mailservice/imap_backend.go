package mailservice

import (
	"context"
	"strings"

	"github.com/gogomail/gogomail/internal/imapgw"
	"github.com/gogomail/gogomail/internal/maildb"
	smtpd "github.com/gogomail/gogomail/internal/smtp"
)

type IMAPBackendAdapter struct {
	IMAPAuthenticatorAdapter
	IMAPStoreAdapter
}

var _ imapgw.Backend = IMAPBackendAdapter{}
var _ imapgw.SearchMessageIDSource = IMAPBackendAdapter{}

func NewIMAPBackendAdapter(authenticator smtpd.SubmissionAuthenticator, service *Service) IMAPBackendAdapter {
	return IMAPBackendAdapter{
		IMAPAuthenticatorAdapter: NewIMAPAuthenticatorAdapter(authenticator),
		IMAPStoreAdapter:         NewIMAPStoreAdapter(service),
	}
}

func (a IMAPBackendAdapter) SearchMessageIDs(ctx context.Context, req imapgw.SearchMessagesRequest) ([]imapgw.MessageID, error) {
	query := maildb.MessageSearchQuery{
		UserID:            string(req.UserID),
		Query:             req.Query,
		FolderID:          string(req.MailboxID),
		From:              req.From,
		To:                req.To,
		Cc:                req.Cc,
		Bcc:               req.Bcc,
		Subject:           req.Subject,
		HasAttachment:     req.HasAttachment,
		Since:             req.Since,
		Until:             req.Until,
		Limit:             req.Limit,
		Sort:              maildb.MessageSearchSortRelevance,
		IncludeRank:       false,
		IncludeHighlights: false,
	}
	ids, err := a.service.SearchMessageIDs(ctx, query)
	if err != nil {
		return nil, err
	}
	out := make([]imapgw.MessageID, 0, len(ids))
	for _, id := range ids {
		id = strings.TrimSpace(id)
		if id == "" {
			continue
		}
		out = append(out, imapgw.MessageID(id))
	}
	return out, nil
}
