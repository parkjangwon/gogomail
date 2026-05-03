package smtpd

import (
	"strings"
	"testing"

	"github.com/emersion/go-sasl"
	gosmtp "github.com/emersion/go-smtp"

	"github.com/gogomail/gogomail/internal/storage"
)

func TestSubmissionRcptBeforeMailReturns503(t *testing.T) {
	t.Parallel()

	session := newAuthenticatedSubmissionSession(t, &submissionRecorder{}, storage.NewLocalStore(t.TempDir()))
	requireSMTPStatus(t, session.Rcpt("outside@example.net", nil), 503, gosmtp.EnhancedCode{5, 5, 1})
}

func TestSubmissionDataBeforeMailReturns503(t *testing.T) {
	t.Parallel()

	session := newAuthenticatedSubmissionSession(t, &submissionRecorder{}, storage.NewLocalStore(t.TempDir()))
	requireSMTPStatus(t, session.Data(strings.NewReader("Subject: nope\r\n\r\nbody")), 503, gosmtp.EnhancedCode{5, 5, 1})
}

func TestSubmissionDataBeforeRcptReturns503(t *testing.T) {
	t.Parallel()

	session := newAuthenticatedSubmissionSession(t, &submissionRecorder{}, storage.NewLocalStore(t.TempDir()))
	if err := session.Mail("jangwon@example.com", nil); err != nil {
		t.Fatalf("Mail returned error: %v", err)
	}
	requireSMTPStatus(t, session.Data(strings.NewReader("Subject: nope\r\n\r\nbody")), 503, gosmtp.EnhancedCode{5, 5, 1})
}

func TestSubmissionEnvelopeMismatchReturns550(t *testing.T) {
	t.Parallel()

	session := newAuthenticatedSubmissionSession(t, &submissionRecorder{}, storage.NewLocalStore(t.TempDir()))
	requireSMTPStatus(t, session.Mail("other@example.com", nil), 550, gosmtp.EnhancedCode{5, 7, 1})
}

func TestSubmissionRecipientLimitReturns452(t *testing.T) {
	t.Parallel()

	receiver := NewSubmissionReceiver(SubmissionOptions{
		Store:         storage.NewLocalStore(t.TempDir()),
		Authenticator: submissionAuthenticator{username: "jangwon@example.com", password: "pass"},
		Recorder:      &submissionRecorder{},
		Policy:        ReceivePolicy{MaxRecipientsPerMessage: 1},
	})
	session, err := receiver.NewSession(nil)
	if err != nil {
		t.Fatalf("NewSession returned error: %v", err)
	}
	submission := session.(*submissionSession)
	server, err := submission.Auth(sasl.Plain)
	if err != nil {
		t.Fatalf("Auth returned error: %v", err)
	}
	if _, done, err := server.Next([]byte("\x00jangwon@example.com\x00pass")); err != nil {
		t.Fatalf("AUTH PLAIN returned error: %v", err)
	} else if !done {
		t.Fatal("AUTH PLAIN did not complete")
	}
	if err := submission.Mail("jangwon@example.com", nil); err != nil {
		t.Fatalf("Mail returned error: %v", err)
	}
	if err := submission.Rcpt("one@example.net", nil); err != nil {
		t.Fatalf("first Rcpt returned error: %v", err)
	}
	requireSMTPStatus(t, submission.Rcpt("two@example.net", nil), 452, gosmtp.EnhancedCode{4, 5, 3})
}

func TestSubmissionMessageSizeLimitReturns552(t *testing.T) {
	t.Parallel()

	receiver := NewSubmissionReceiver(SubmissionOptions{
		Store:         storage.NewLocalStore(t.TempDir()),
		Authenticator: submissionAuthenticator{username: "jangwon@example.com", password: "pass"},
		Recorder:      &submissionRecorder{},
		Policy:        ReceivePolicy{MaxMessageBytes: 8},
	})
	session, err := receiver.NewSession(nil)
	if err != nil {
		t.Fatalf("NewSession returned error: %v", err)
	}
	submission := session.(*submissionSession)
	server, err := submission.Auth(sasl.Plain)
	if err != nil {
		t.Fatalf("Auth returned error: %v", err)
	}
	if _, done, err := server.Next([]byte("\x00jangwon@example.com\x00pass")); err != nil {
		t.Fatalf("AUTH PLAIN returned error: %v", err)
	} else if !done {
		t.Fatal("AUTH PLAIN did not complete")
	}
	if err := submission.Mail("jangwon@example.com", nil); err != nil {
		t.Fatalf("Mail returned error: %v", err)
	}
	if err := submission.Rcpt("one@example.net", nil); err != nil {
		t.Fatalf("Rcpt returned error: %v", err)
	}
	requireSMTPStatus(t, submission.Data(strings.NewReader("Subject: too large\r\n\r\nbody")), 552, gosmtp.EnhancedCode{5, 3, 4})
}

func TestSubmissionAnnouncedMessageSizeLimitReturns552AtMail(t *testing.T) {
	t.Parallel()

	receiver := NewSubmissionReceiver(SubmissionOptions{
		Store:         storage.NewLocalStore(t.TempDir()),
		Authenticator: submissionAuthenticator{username: "jangwon@example.com", password: "pass"},
		Recorder:      &submissionRecorder{},
		Policy:        ReceivePolicy{MaxMessageBytes: 8},
	})
	session, err := receiver.NewSession(nil)
	if err != nil {
		t.Fatalf("NewSession returned error: %v", err)
	}
	submission := session.(*submissionSession)
	server, err := submission.Auth(sasl.Plain)
	if err != nil {
		t.Fatalf("Auth returned error: %v", err)
	}
	if _, done, err := server.Next([]byte("\x00jangwon@example.com\x00pass")); err != nil {
		t.Fatalf("AUTH PLAIN returned error: %v", err)
	} else if !done {
		t.Fatal("AUTH PLAIN did not complete")
	}
	requireSMTPStatus(t, submission.Mail("jangwon@example.com", &gosmtp.MailOptions{Size: 9}), 552, gosmtp.EnhancedCode{5, 3, 4})
}

func TestSubmissionFailedSecondMailClearsEnvelope(t *testing.T) {
	t.Parallel()

	receiver := NewSubmissionReceiver(SubmissionOptions{
		Store:         storage.NewLocalStore(t.TempDir()),
		Authenticator: submissionAuthenticator{username: "jangwon@example.com", password: "pass"},
		Recorder:      &submissionRecorder{},
		Policy:        ReceivePolicy{MaxMessageBytes: 8},
	})
	session, err := receiver.NewSession(nil)
	if err != nil {
		t.Fatalf("NewSession returned error: %v", err)
	}
	submission := session.(*submissionSession)
	server, err := submission.Auth(sasl.Plain)
	if err != nil {
		t.Fatalf("Auth returned error: %v", err)
	}
	if _, done, err := server.Next([]byte("\x00jangwon@example.com\x00pass")); err != nil {
		t.Fatalf("AUTH PLAIN returned error: %v", err)
	} else if !done {
		t.Fatal("AUTH PLAIN did not complete")
	}
	if err := submission.Mail("jangwon@example.com", nil); err != nil {
		t.Fatalf("Mail returned error: %v", err)
	}
	if err := submission.Rcpt("one@example.net", nil); err != nil {
		t.Fatalf("Rcpt returned error: %v", err)
	}
	requireSMTPStatus(t, submission.Mail("jangwon@example.com", &gosmtp.MailOptions{Size: 9}), 552, gosmtp.EnhancedCode{5, 3, 4})
	requireSMTPStatus(t, submission.Data(strings.NewReader("Subject: stale\r\n\r\nbody")), 503, gosmtp.EnhancedCode{5, 5, 1})
}
