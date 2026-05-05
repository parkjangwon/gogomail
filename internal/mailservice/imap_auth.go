package mailservice

import (
	"context"
	"fmt"
	"strings"

	"github.com/gogomail/gogomail/internal/imapgw"
	smtpd "github.com/gogomail/gogomail/internal/smtp"
)

type IMAPAuthenticatorAdapter struct {
	authenticator smtpd.SubmissionAuthenticator
}

var _ imapgw.Authenticator = IMAPAuthenticatorAdapter{}

func NewIMAPAuthenticatorAdapter(authenticator smtpd.SubmissionAuthenticator) IMAPAuthenticatorAdapter {
	return IMAPAuthenticatorAdapter{authenticator: authenticator}
}

func (a IMAPAuthenticatorAdapter) Authenticate(ctx context.Context, username string, password string) (imapgw.Session, error) {
	if a.authenticator == nil {
		return imapgw.Session{}, fmt.Errorf("imap authenticator is required")
	}
	username = strings.TrimSpace(username)
	if username == "" || strings.ContainsAny(username, "\r\n") {
		return imapgw.Session{}, fmt.Errorf("imap username is invalid")
	}
	if strings.ContainsAny(password, "\r\n") {
		return imapgw.Session{}, fmt.Errorf("imap password is invalid")
	}
	user, err := a.authenticator.AuthenticatePlain(ctx, "", username, password)
	if err != nil {
		return imapgw.Session{}, err
	}
	return imapgw.Session{
		UserID:      imapgw.UserID(strings.TrimSpace(user.UserID)),
		Username:    strings.TrimSpace(firstNonEmpty(user.Address, username)),
		DomainID:    strings.TrimSpace(user.DomainID),
		DisplayName: strings.TrimSpace(user.DisplayName),
	}, nil
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}
