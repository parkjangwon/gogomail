package smtpd

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	gosmtp "github.com/emersion/go-smtp"
	"github.com/gogomail/gogomail/internal/mail"
	"github.com/gogomail/gogomail/internal/storage"
)

type quotaFullRecorder struct{}

func (r *quotaFullRecorder) Record(_ context.Context, _ ReceivedMessage) (string, error) {
	return "", fmt.Errorf("store failed: %w", mail.ErrMailboxFull)
}

func TestSessionReturns452WhenRecorderReportsMailboxFull(t *testing.T) {
	t.Parallel()

	store := storage.NewLocalStore(t.TempDir())
	receiver := NewReceiver(ReceiverOptions{
		Store: store,
		Resolver: StaticResolver{
			"user@example.com": {CompanyID: "c", DomainID: "d", UserID: "u", Address: "user@example.com"},
		},
		Recorder:    &quotaFullRecorder{},
		IDGenerator: func() string { return "quota-test-id" },
		Clock:       func() time.Time { return time.Date(2026, 5, 4, 0, 0, 0, 0, time.UTC) },
	})

	session, err := receiver.NewSession(nil)
	if err != nil {
		t.Fatalf("NewSession: %v", err)
	}
	if err := session.Mail("sender@other.com", nil); err != nil {
		t.Fatalf("Mail: %v", err)
	}
	if err := session.Rcpt("user@example.com", nil); err != nil {
		t.Fatalf("Rcpt: %v", err)
	}

	raw := "From: sender@other.com\r\nTo: user@example.com\r\nSubject: hi\r\n\r\nbody"
	dataErr := session.Data(strings.NewReader(raw))
	if dataErr == nil {
		t.Fatal("Data: want error for mailbox full, got nil")
	}

	var smtpErr *gosmtp.SMTPError
	if !errors.As(dataErr, &smtpErr) {
		t.Fatalf("Data error is not *gosmtp.SMTPError: %T %v", dataErr, dataErr)
	}
	if smtpErr.Code != 452 {
		t.Fatalf("SMTP error code = %d, want 452", smtpErr.Code)
	}
	if _, err := store.Get(context.Background(), "mailstore/c/d/u/maildir/2026/05/quota-test-id.eml"); err == nil {
		t.Fatal("stored object remained after mailbox full recorder failure")
	}
}
