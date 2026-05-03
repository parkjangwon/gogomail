package smtpd

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/emersion/go-sasl"
	gosmtp "github.com/emersion/go-smtp"
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

func TestSessionRecordsParsedMessageMetadata(t *testing.T) {
	t.Parallel()

	recorder := &recordingRecorder{}
	receiver := NewReceiver(ReceiverOptions{
		Store: storage.NewLocalStore(t.TempDir()),
		Resolver: StaticResolver{
			"jangwon@example.com": {
				CompanyID: "company-1",
				DomainID:  "domain-1",
				UserID:    "user-1",
				Address:   "jangwon@example.com",
			},
		},
		Recorder:    recorder,
		IDGenerator: func() string { return "018e9b3a-record" },
		Clock:       func() time.Time { return time.Date(2026, 5, 3, 9, 30, 0, 0, time.UTC) },
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

	raw := strings.Join([]string{
		"Message-ID: <record@example.com>",
		"From: Sender <sender@example.net>",
		"To: JangWon <jangwon@example.com>",
		"Subject: record me",
		"Content-Type: text/plain; charset=utf-8",
		"",
		"body",
	}, "\r\n")
	if err := session.Data(strings.NewReader(raw)); err != nil {
		t.Fatalf("Data returned error: %v", err)
	}

	if len(recorder.messages) != 1 {
		t.Fatalf("recorded messages = %d, want 1", len(recorder.messages))
	}
	recorded := recorder.messages[0]
	if recorded.EnvelopeFrom != "sender@example.net" {
		t.Fatalf("EnvelopeFrom = %q", recorded.EnvelopeFrom)
	}
	if recorded.Mailbox.Address != "jangwon@example.com" {
		t.Fatalf("Mailbox = %+v", recorded.Mailbox)
	}
	if recorded.StoragePath != "mailstore/company-1/domain-1/user-1/maildir/2026/05/018e9b3a-record.eml" {
		t.Fatalf("StoragePath = %q", recorded.StoragePath)
	}
	if recorded.Parsed.MessageID != "<record@example.com>" {
		t.Fatalf("MessageID = %q", recorded.Parsed.MessageID)
	}
	if recorded.Parsed.Subject != "record me" {
		t.Fatalf("Subject = %q", recorded.Parsed.Subject)
	}
	if !recorded.ReceivedAt.Equal(time.Date(2026, 5, 3, 9, 30, 0, 0, time.UTC)) {
		t.Fatalf("ReceivedAt = %s", recorded.ReceivedAt)
	}
	if recorded.Size == 0 {
		t.Fatal("Size = 0, want stored message size")
	}
}

func TestSessionPrependsReceivedHeaderWhenConfigured(t *testing.T) {
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
		AddReceivedHeader: true,
		ReceivedDomain:    "mx.example.com",
		IDGenerator:       func() string { return "received-id" },
		Clock:             func() time.Time { return time.Date(2026, 5, 3, 9, 0, 0, 0, time.UTC) },
	})

	session, err := receiver.NewSession(nil)
	if err != nil {
		t.Fatalf("NewSession returned error: %v", err)
	}
	if err := session.Mail("sender@example.net", nil); err != nil {
		t.Fatalf("Mail returned error: %v", err)
	}
	if err := session.Rcpt("jangwon@example.com", nil); err != nil {
		t.Fatalf("Rcpt returned error: %v", err)
	}

	raw := "From: sender@example.net\r\nTo: jangwon@example.com\r\nSubject: hello\r\n\r\nbody"
	if err := session.Data(strings.NewReader(raw)); err != nil {
		t.Fatalf("Data returned error: %v", err)
	}

	body, err := store.Get(context.Background(), "mailstore/company-1/domain-1/user-1/maildir/2026/05/received-id.eml")
	if err != nil {
		t.Fatalf("stored message not found: %v", err)
	}
	defer body.Close()

	got, err := io.ReadAll(body)
	if err != nil {
		t.Fatalf("ReadAll returned error: %v", err)
	}
	if !strings.HasPrefix(string(got), "Received: from unknown by mx.example.com with ESMTP id received-id; ") {
		t.Fatalf("stored message missing Received header: %q", got)
	}
	if !strings.Contains(string(got), "\r\nFrom: sender@example.net\r\n") {
		t.Fatalf("stored message missing original headers after Received: %q", got)
	}
}

func TestSessionSkipsDuplicateMessageForRecipient(t *testing.T) {
	t.Parallel()

	store := storage.NewLocalStore(t.TempDir())
	recorder := &recordingRecorder{}
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
		Recorder:     recorder,
		Deduplicator: &duplicateDeduplicator{},
		IDGenerator:  func() string { return "duplicate" },
		Clock:        func() time.Time { return time.Date(2026, 5, 3, 9, 30, 0, 0, time.UTC) },
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

	raw := strings.Join([]string{
		"Message-ID: <duplicate@example.com>",
		"From: Sender <sender@example.net>",
		"To: JangWon <jangwon@example.com>",
		"Subject: duplicate",
		"Content-Type: text/plain; charset=utf-8",
		"",
		"body",
	}, "\r\n")
	if err := session.Data(strings.NewReader(raw)); err != nil {
		t.Fatalf("Data returned error: %v", err)
	}

	if len(recorder.messages) != 0 {
		t.Fatalf("recorded messages = %d, want 0", len(recorder.messages))
	}
	if _, err := store.Get(context.Background(), "mailstore/company-1/domain-1/user-1/maildir/2026/05/duplicate.eml"); err == nil {
		t.Fatal("duplicate message was stored")
	}
}

func TestSessionEmitsPipelineHooksInOrder(t *testing.T) {
	t.Parallel()

	var stages []Stage
	receiver := NewReceiver(ReceiverOptions{
		Store: storage.NewLocalStore(t.TempDir()),
		Resolver: StaticResolver{
			"jangwon@example.com": {
				CompanyID: "company-1",
				DomainID:  "domain-1",
				UserID:    "user-1",
				Address:   "jangwon@example.com",
			},
		},
		IDGenerator: func() string { return "hooked" },
		Clock:       func() time.Time { return time.Date(2026, 5, 3, 9, 30, 0, 0, time.UTC) },
		Hooks: []Hook{
			func(_ context.Context, event Event) error {
				stages = append(stages, event.Stage)
				if event.Stage == StageStored && event.StoragePath == "" {
					t.Fatal("StageStored event has empty StoragePath")
				}
				if event.Stage == StageParsed && event.Parsed.Subject != "hook me" {
					t.Fatalf("StageParsed subject = %q", event.Parsed.Subject)
				}
				return nil
			},
		},
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

	raw := strings.Join([]string{
		"Message-ID: <hook@example.com>",
		"From: Sender <sender@example.net>",
		"To: JangWon <jangwon@example.com>",
		"Subject: hook me",
		"Content-Type: text/plain; charset=utf-8",
		"",
		"body",
	}, "\r\n")
	if err := session.Data(strings.NewReader(raw)); err != nil {
		t.Fatalf("Data returned error: %v", err)
	}

	want := []Stage{
		StageBackpressureChecked,
		StageSpooled,
		StageParsed,
		StageDedupChecked,
		StageStored,
		StageRecorded,
	}
	if len(stages) != len(want) {
		t.Fatalf("stages = %v, want %v", stages, want)
	}
	for i := range want {
		if stages[i] != want[i] {
			t.Fatalf("stages = %v, want %v", stages, want)
		}
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

func TestSessionRejectsRecipientsOverPolicyLimit(t *testing.T) {
	t.Parallel()

	receiver := NewReceiver(ReceiverOptions{
		Store: storage.NewLocalStore(t.TempDir()),
		Resolver: StaticResolver{
			"one@example.com": {CompanyID: "c", DomainID: "d", UserID: "u1", Address: "one@example.com"},
			"two@example.com": {CompanyID: "c", DomainID: "d", UserID: "u2", Address: "two@example.com"},
		},
		Policy: ReceivePolicy{MaxRecipientsPerMessage: 1},
	})

	session, err := receiver.NewSession(nil)
	if err != nil {
		t.Fatalf("NewSession returned error: %v", err)
	}
	if err := session.Mail("sender@example.net", nil); err != nil {
		t.Fatalf("Mail returned error: %v", err)
	}
	if err := session.Rcpt("one@example.com", nil); err != nil {
		t.Fatalf("first Rcpt returned error: %v", err)
	}
	if err := session.Rcpt("two@example.com", nil); err == nil {
		t.Fatal("second Rcpt was accepted over policy limit")
	}
}

func TestSessionRejectsRecipientWhenRateLimited(t *testing.T) {
	t.Parallel()

	receiver := NewReceiver(ReceiverOptions{
		Store: storage.NewLocalStore(t.TempDir()),
		Resolver: StaticResolver{
			"one@example.com": {CompanyID: "c", DomainID: "d", UserID: "u1", Address: "one@example.com"},
		},
		RateLimiter: denyRateLimiter{},
	})

	session, err := receiver.NewSession(nil)
	if err != nil {
		t.Fatalf("NewSession returned error: %v", err)
	}
	if err := session.Mail("sender@example.net", nil); err != nil {
		t.Fatalf("Mail returned error: %v", err)
	}
	if err := session.Rcpt("one@example.com", nil); err == nil {
		t.Fatal("Rcpt accepted recipient while rate limited")
	}
}

func TestSessionRejectsDataWhenBackpressureActive(t *testing.T) {
	t.Parallel()

	store := storage.NewLocalStore(t.TempDir())
	receiver := NewReceiver(ReceiverOptions{
		Store: store,
		Resolver: StaticResolver{
			"one@example.com": {CompanyID: "c", DomainID: "d", UserID: "u1", Address: "one@example.com"},
		},
		Backpressure: rejectBackpressure{},
		IDGenerator:  func() string { return "backpressure" },
		Clock:        func() time.Time { return time.Date(2026, 5, 3, 9, 30, 0, 0, time.UTC) },
	})

	session, err := receiver.NewSession(nil)
	if err != nil {
		t.Fatalf("NewSession returned error: %v", err)
	}
	if err := session.Mail("sender@example.net", nil); err != nil {
		t.Fatalf("Mail returned error: %v", err)
	}
	if err := session.Rcpt("one@example.com", nil); err != nil {
		t.Fatalf("Rcpt returned error: %v", err)
	}

	err = session.Data(strings.NewReader("Message-ID: <bp@example.com>\r\nSubject: bp\r\n\r\nbody"))
	if err == nil {
		t.Fatal("Data accepted while backpressure was active")
	}
	if _, err := store.Get(context.Background(), "mailstore/c/d/u1/maildir/2026/05/backpressure.eml"); err == nil {
		t.Fatal("message was stored while backpressure was active")
	}
}

func TestSessionRejectsMessageLargerThanLimit(t *testing.T) {
	t.Parallel()

	store := storage.NewLocalStore(t.TempDir())
	receiver := NewReceiver(ReceiverOptions{
		Store:           store,
		MaxMessageBytes: 32,
		Resolver: StaticResolver{
			"jangwon@example.com": {CompanyID: "c", DomainID: "d", UserID: "u", Address: "jangwon@example.com"},
		},
		IDGenerator: func() string { return "too-large" },
		Clock:       func() time.Time { return time.Date(2026, 5, 3, 0, 0, 0, 0, time.UTC) },
	})

	session, err := receiver.NewSession(nil)
	if err != nil {
		t.Fatalf("NewSession returned error: %v", err)
	}
	if err := session.Mail("sender@example.net", nil); err != nil {
		t.Fatalf("Mail returned error: %v", err)
	}
	if err := session.Rcpt("jangwon@example.com", nil); err != nil {
		t.Fatalf("Rcpt returned error: %v", err)
	}

	err = session.Data(strings.NewReader("From: sender@example.net\r\nTo: jangwon@example.com\r\nSubject: too large\r\n\r\nbody"))
	if err == nil {
		t.Fatal("Data accepted message larger than limit")
	}

	if _, err := store.Get(context.Background(), "mailstore/c/d/u/maildir/2026/05/too-large.eml"); err == nil {
		t.Fatal("oversized message was stored")
	}
}

func TestSessionRequiresAuthWhenConfigured(t *testing.T) {
	t.Parallel()

	receiver := NewReceiver(ReceiverOptions{
		Store: storage.NewLocalStore(t.TempDir()),
		Resolver: StaticResolver{
			"jangwon@example.com": {CompanyID: "c", DomainID: "d", UserID: "u", Address: "jangwon@example.com"},
		},
		Authenticator: plainAuthenticator{username: "user", password: "pass"},
		RequireAuth:   true,
	})

	session, err := receiver.NewSession(nil)
	if err != nil {
		t.Fatalf("NewSession returned error: %v", err)
	}
	if err := session.Mail("sender@example.net", nil); err == nil {
		t.Fatal("Mail accepted unauthenticated session")
	}
	authSession, ok := session.(interface {
		Auth(string) (sasl.Server, error)
	})
	if !ok {
		t.Fatal("session does not implement AUTH")
	}
	server, err := authSession.Auth(sasl.Plain)
	if err != nil {
		t.Fatalf("Auth returned error: %v", err)
	}
	_, done, err := server.Next([]byte("\x00user\x00pass"))
	if err != nil {
		t.Fatalf("AUTH PLAIN returned error: %v", err)
	}
	if !done {
		t.Fatal("AUTH PLAIN did not complete")
	}
	if err := session.Mail("sender@example.net", nil); err != nil {
		t.Fatalf("Mail after auth returned error: %v", err)
	}
}

func TestSessionRejectsSMTPUTF8UntilExplicitlySupported(t *testing.T) {
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
	if err := session.Mail("jangwon@example.com", &gosmtp.MailOptions{UTF8: true}); err == nil {
		t.Fatal("Mail accepted SMTPUTF8 before support is enabled")
	}
}

type plainAuthenticator struct {
	username string
	password string
}

func (a plainAuthenticator) AuthenticatePlain(_ context.Context, _ string, username string, password string) error {
	if username != a.username || password != a.password {
		return errAuthTestFailed
	}
	return nil
}

var errAuthTestFailed = errors.New("auth failed")

type recordingRecorder struct {
	messages []ReceivedMessage
}

func (r *recordingRecorder) Record(_ context.Context, msg ReceivedMessage) error {
	r.messages = append(r.messages, msg)
	return nil
}

type duplicateDeduplicator struct{}

func (duplicateDeduplicator) CheckAndSet(context.Context, DedupKey) (bool, error) {
	return false, nil
}

type denyRateLimiter struct{}

func (denyRateLimiter) Allow(context.Context, RateLimitKey) (bool, error) {
	return false, nil
}

type rejectBackpressure struct{}

func (rejectBackpressure) Accept(context.Context) (bool, error) {
	return false, nil
}
