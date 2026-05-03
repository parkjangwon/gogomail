package smtpd

import (
	"context"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/gogomail/gogomail/internal/storage"
)

func TestSessionStoresRawMessageForAcceptedRecipient(t *testing.T) {
	t.Parallel()

	store := storage.NewLocalStore(t.TempDir())
	receiver := NewReceiver(ReceiverOptions{
		Store: store,
		Resolver: StaticResolver{
			"jangwon@example.com": {
				CompanyID: "company-1",
				DomainID:  "domain-1",
				UserID:    "user-1",
				Address:   "jangwon@example.com",
			},
		},
		IDGenerator: func() string { return "018e9b3a-test" },
		Clock:       func() time.Time { return time.Date(2026, 5, 3, 0, 0, 0, 0, time.UTC) },
	})

	session, err := receiver.NewSession(nil)
	if err != nil {
		t.Fatalf("NewSession returned error: %v", err)
	}

	if err := session.Mail("sender@example.net", nil); err != nil {
		t.Fatalf("Mail returned error: %v", err)
	}
	if err := session.Rcpt("JangWon@Example.COM", nil); err != nil {
		t.Fatalf("Rcpt returned error: %v", err)
	}

	raw := "From: sender@example.net\r\nTo: jangwon@example.com\r\nSubject: hello\r\n\r\nbody"
	if err := session.Data(strings.NewReader(raw)); err != nil {
		t.Fatalf("Data returned error: %v", err)
	}

	body, err := store.Get(context.Background(), "mailstore/company-1/domain-1/user-1/maildir/2026/05/018e9b3a-test.eml")
	if err != nil {
		t.Fatalf("stored message not found: %v", err)
	}
	defer body.Close()

	got, err := io.ReadAll(body)
	if err != nil {
		t.Fatalf("ReadAll returned error: %v", err)
	}
	if string(got) != raw {
		t.Fatalf("stored message = %q, want raw input", string(got))
	}
}

func TestSessionRejectsUnknownRecipient(t *testing.T) {
	t.Parallel()

	receiver := NewReceiver(ReceiverOptions{
		Store:    storage.NewLocalStore(t.TempDir()),
		Resolver: StaticResolver{},
	})

	session, err := receiver.NewSession(nil)
	if err != nil {
		t.Fatalf("NewSession returned error: %v", err)
	}
	if err := session.Mail("sender@example.net", nil); err != nil {
		t.Fatalf("Mail returned error: %v", err)
	}
	if err := session.Rcpt("missing@example.com", nil); err == nil {
		t.Fatal("Rcpt accepted unknown recipient")
	}
}

func TestSessionRequiresRecipientBeforeData(t *testing.T) {
	t.Parallel()

	receiver := NewReceiver(ReceiverOptions{
		Store: storage.NewLocalStore(t.TempDir()),
		Resolver: StaticResolver{
			"jangwon@example.com": {CompanyID: "c", DomainID: "d", UserID: "u", Address: "jangwon@example.com"},
		},
	})

	session, err := receiver.NewSession(nil)
	if err != nil {
		t.Fatalf("NewSession returned error: %v", err)
	}
	if err := session.Mail("sender@example.net", nil); err != nil {
		t.Fatalf("Mail returned error: %v", err)
	}
	if err := session.Data(strings.NewReader("Subject: nope\r\n\r\nbody")); err == nil {
		t.Fatal("Data accepted message without recipients")
	}
}
