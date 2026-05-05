package mailservice

import (
	"github.com/gogomail/gogomail/internal/imapgw"
	smtpd "github.com/gogomail/gogomail/internal/smtp"
)

type IMAPBackendAdapter struct {
	IMAPAuthenticatorAdapter
	IMAPStoreAdapter
}

var _ imapgw.Backend = IMAPBackendAdapter{}

func NewIMAPBackendAdapter(authenticator smtpd.SubmissionAuthenticator, service *Service) IMAPBackendAdapter {
	return IMAPBackendAdapter{
		IMAPAuthenticatorAdapter: NewIMAPAuthenticatorAdapter(authenticator),
		IMAPStoreAdapter:         NewIMAPStoreAdapter(service),
	}
}
