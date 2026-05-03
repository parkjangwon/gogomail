package smtpd

import (
	"strings"
	"testing"

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
