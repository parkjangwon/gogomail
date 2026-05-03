package smtpd

import (
	"context"
	"io"
	"testing"

	gosmtp "github.com/emersion/go-smtp"
)

func TestNewSMTPServerPropagatesExtensionToggles(t *testing.T) {
	t.Parallel()

	server := newSMTPServer(extensionBackend{}, ServerOptions{
		Addr:             ":2525",
		EnableSMTPUTF8:   true,
		EnableRequireTLS: true,
		EnableDSN:        true,
		EnableBinaryMIME: true,
	})

	if !server.EnableSMTPUTF8 {
		t.Fatal("EnableSMTPUTF8 was not propagated")
	}
	if !server.EnableREQUIRETLS {
		t.Fatal("EnableREQUIRETLS was not propagated")
	}
	if !server.EnableDSN {
		t.Fatal("EnableDSN was not propagated")
	}
	if !server.EnableBINARYMIME {
		t.Fatal("EnableBINARYMIME was not propagated")
	}
}

type extensionBackend struct{}

func (extensionBackend) NewSession(_ *gosmtp.Conn) (gosmtp.Session, error) {
	return extensionSession{}, nil
}

type extensionSession struct{}

func (extensionSession) Mail(string, *gosmtp.MailOptions) error { return nil }
func (extensionSession) Rcpt(string, *gosmtp.RcptOptions) error { return nil }
func (extensionSession) Data(io.Reader) error                   { return nil }
func (extensionSession) Reset()                                 {}
func (extensionSession) Logout() error                          { return nil }
func (extensionSession) AuthPlain(string, string) error         { return nil }
func (extensionSession) LMTPData(io.Reader, gosmtp.StatusCollector) error {
	return nil
}
func (extensionSession) AuthMechanisms() []string { return nil }
func (extensionSession) Auth(context.Context, string) (gosmtp.AuthSession, error) {
	return nil, gosmtp.ErrAuthUnsupported
}
