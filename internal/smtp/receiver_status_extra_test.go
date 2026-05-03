package smtpd

import (
	"context"
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

func TestReceiverRecipientLimitReturns452(t *testing.T) {
	t.Parallel()

	session := newStatusReceiverSession(t, ReceiverOptions{
		Resolver: StaticResolver{
			"one@example.com": {CompanyID: "c", DomainID: "d", UserID: "u1", Address: "one@example.com"},
			"two@example.com": {CompanyID: "c", DomainID: "d", UserID: "u2", Address: "two@example.com"},
		},
		Policy: ReceivePolicy{MaxRecipientsPerMessage: 1},
	})
	if err := session.Mail("sender@example.net", nil); err != nil {
		t.Fatalf("Mail returned error: %v", err)
	}
	if err := session.Rcpt("one@example.com", nil); err != nil {
		t.Fatalf("first Rcpt returned error: %v", err)
	}
	requireSMTPStatus(t, session.Rcpt("two@example.com", nil), 452, gosmtp.EnhancedCode{4, 5, 3})
}

func TestReceiverRateLimitReturns451(t *testing.T) {
	t.Parallel()

	session := newStatusReceiverSession(t, ReceiverOptions{RateLimiter: denyingRateLimiter{}})
	if err := session.Mail("sender@example.net", nil); err != nil {
		t.Fatalf("Mail returned error: %v", err)
	}
	requireSMTPStatus(t, session.Rcpt("jangwon@example.com", nil), 451, gosmtp.EnhancedCode{4, 7, 1})
}

func TestReceiverBackpressureReturns421(t *testing.T) {
	t.Parallel()

	session := newStatusReceiverSession(t, ReceiverOptions{Backpressure: closedBackpressure{}})
	if err := session.Mail("sender@example.net", nil); err != nil {
		t.Fatalf("Mail returned error: %v", err)
	}
	if err := session.Rcpt("jangwon@example.com", nil); err != nil {
		t.Fatalf("Rcpt returned error: %v", err)
	}
	requireSMTPStatus(t, session.Data(strings.NewReader("Subject: busy\r\n\r\nbody")), 421, gosmtp.EnhancedCode{4, 3, 2})
}

func TestReceiverMessageSizeLimitReturns552(t *testing.T) {
	t.Parallel()

	session := newStatusReceiverSession(t, ReceiverOptions{Policy: ReceivePolicy{MaxMessageBytes: 8}})
	if err := session.Mail("sender@example.net", nil); err != nil {
		t.Fatalf("Mail returned error: %v", err)
	}
	if err := session.Rcpt("jangwon@example.com", nil); err != nil {
		t.Fatalf("Rcpt returned error: %v", err)
	}
	requireSMTPStatus(t, session.Data(strings.NewReader("Subject: too large\r\n\r\nbody")), 552, gosmtp.EnhancedCode{5, 3, 4})
}

func TestReceiverAnnouncedMessageSizeLimitReturns552AtMail(t *testing.T) {
	t.Parallel()

	session := newStatusReceiverSession(t, ReceiverOptions{Policy: ReceivePolicy{MaxMessageBytes: 8}})
	requireSMTPStatus(t, session.Mail("sender@example.net", &gosmtp.MailOptions{Size: 9}), 552, gosmtp.EnhancedCode{5, 3, 4})
}

type closedBackpressure struct{}

func (closedBackpressure) Accept(context.Context) (bool, error) {
	return false, nil
}

type denyingRateLimiter struct{}

func (denyingRateLimiter) Allow(context.Context, RateLimitKey) (bool, error) {
	return false, nil
}
