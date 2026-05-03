package smtpd

import (
	"strings"
	"testing"

	gosmtp "github.com/emersion/go-smtp"

	"github.com/gogomail/gogomail/internal/storage"
)

func newStatusReceiverSession(t *testing.T, opts ReceiverOptions) *session {
	t.Helper()
	if opts.Store == nil {
		opts.Store = storage.NewLocalStore(t.TempDir())
	}
	if opts.Resolver == nil {
		opts.Resolver = StaticResolver{
			"jangwon@example.com": {CompanyID: "c", DomainID: "d", UserID: "u", Address: "jangwon@example.com"},
		}
	}
	sess, err := NewReceiver(opts).NewSession(nil)
	if err != nil {
		t.Fatalf("NewSession returned error: %v", err)
	}
	return sess.(*session)
}

func TestReceiverRcptBeforeMailReturns503(t *testing.T) {
	t.Parallel()

	session := newStatusReceiverSession(t, ReceiverOptions{})
	requireSMTPStatus(t, session.Rcpt("jangwon@example.com", nil), 503, gosmtp.EnhancedCode{5, 5, 1})
}

func TestReceiverDataBeforeMailReturns503(t *testing.T) {
	t.Parallel()

	session := newStatusReceiverSession(t, ReceiverOptions{})
	requireSMTPStatus(t, session.Data(strings.NewReader("Subject: nope\r\n\r\nbody")), 503, gosmtp.EnhancedCode{5, 5, 1})
}

func TestReceiverDataBeforeRcptReturns503(t *testing.T) {
	t.Parallel()

	session := newStatusReceiverSession(t, ReceiverOptions{})
	if err := session.Mail("sender@example.net", nil); err != nil {
		t.Fatalf("Mail returned error: %v", err)
	}
	requireSMTPStatus(t, session.Data(strings.NewReader("Subject: nope\r\n\r\nbody")), 503, gosmtp.EnhancedCode{5, 5, 1})
}

func TestReceiverUnknownRecipientReturns550(t *testing.T) {
	t.Parallel()

	session := newStatusReceiverSession(t, ReceiverOptions{Resolver: StaticResolver{}})
	if err := session.Mail("sender@example.net", nil); err != nil {
		t.Fatalf("Mail returned error: %v", err)
	}
	requireSMTPStatus(t, session.Rcpt("missing@example.com", nil), 550, gosmtp.EnhancedCode{5, 1, 1})
}
