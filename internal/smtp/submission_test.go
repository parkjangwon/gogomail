package smtpd

import (
	"context"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/emersion/go-sasl"
	"github.com/gogomail/gogomail/internal/storage"
)

func TestSubmissionRequiresAuthBeforeMail(t *testing.T) {
	t.Parallel()

	receiver := NewSubmissionReceiver(SubmissionOptions{
		Store:         storage.NewLocalStore(t.TempDir()),
		Authenticator: submissionAuthenticator{},
		Recorder:      &submissionRecorder{},
	})

	session, err := receiver.NewSession(nil)
	if err != nil {
		t.Fatalf("NewSession returned error: %v", err)
	}
	if err := session.Mail("sender@example.com", nil); err == nil {
		t.Fatal("Mail accepted unauthenticated submission session")
	}
}

func TestSubmissionRejectsEnvelopeFromMismatch(t *testing.T) {
	t.Parallel()

	session := newAuthenticatedSubmissionSession(t, &submissionRecorder{}, storage.NewLocalStore(t.TempDir()))

	if err := session.Mail("other@example.com", nil); err == nil {
		t.Fatal("Mail accepted envelope sender that does not belong to authenticated user")
	}
}

func TestSubmissionStoresAndRecordsSubmittedMessage(t *testing.T) {
	t.Parallel()

	store := storage.NewLocalStore(t.TempDir())
	recorder := &submissionRecorder{}
	session := newAuthenticatedSubmissionSession(t, recorder, store)

	if err := session.Mail("JangWon@Example.COM", nil); err != nil {
		t.Fatalf("Mail returned error: %v", err)
	}
	if err := session.Rcpt("outside@example.net", nil); err != nil {
		t.Fatalf("Rcpt returned error: %v", err)
	}

	raw := strings.Join([]string{
		"Message-ID: <submission@example.com>",
		"From: Jang Won <jangwon@example.com>",
		"To: Outside <outside@example.net>",
		"Subject: submitted 안녕",
		"Content-Type: text/plain; charset=utf-8",
		"",
		"hello",
	}, "\r\n")
	if err := session.Data(strings.NewReader(raw)); err != nil {
		t.Fatalf("Data returned error: %v", err)
	}

	if len(recorder.messages) != 1 {
		t.Fatalf("recorded submitted messages = %d, want 1", len(recorder.messages))
	}
	recorded := recorder.messages[0]
	if recorded.EnvelopeFrom != "jangwon@example.com" {
		t.Fatalf("EnvelopeFrom = %q", recorded.EnvelopeFrom)
	}
	if len(recorded.Recipients) != 1 || recorded.Recipients[0] != "outside@example.net" {
		t.Fatalf("Recipients = %+v", recorded.Recipients)
	}
	if recorded.StoragePath != "mailstore/company-1/domain-1/user-1/maildir/2026/05/submitted-id.eml" {
		t.Fatalf("StoragePath = %q", recorded.StoragePath)
	}
	if recorded.Parsed.MessageID != "<submission@example.com>" {
		t.Fatalf("MessageID = %q", recorded.Parsed.MessageID)
	}
	if recorded.Parsed.Subject != "submitted 안녕" {
		t.Fatalf("Subject = %q", recorded.Parsed.Subject)
	}
	if !recorded.SubmittedAt.Equal(time.Date(2026, 5, 3, 10, 0, 0, 0, time.UTC)) {
		t.Fatalf("SubmittedAt = %s", recorded.SubmittedAt)
	}

	body, err := store.Get(context.Background(), recorded.StoragePath)
	if err != nil {
		t.Fatalf("stored submitted message not found: %v", err)
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

func newAuthenticatedSubmissionSession(t *testing.T, recorder *submissionRecorder, store storage.Store) *submissionSession {
	t.Helper()

	receiver := NewSubmissionReceiver(SubmissionOptions{
		Store:         store,
		Authenticator: submissionAuthenticator{username: "jangwon@example.com", password: "pass"},
		Recorder:      recorder,
		IDGenerator:   func() string { return "submitted-id" },
		Clock:         func() time.Time { return time.Date(2026, 5, 3, 10, 0, 0, 0, time.UTC) },
	})

	session, err := receiver.NewSession(nil)
	if err != nil {
		t.Fatalf("NewSession returned error: %v", err)
	}
	submission, ok := session.(*submissionSession)
	if !ok {
		t.Fatalf("session type = %T, want *submissionSession", session)
	}

	server, err := submission.Auth(sasl.Plain)
	if err != nil {
		t.Fatalf("Auth returned error: %v", err)
	}
	if _, done, err := server.Next([]byte("\x00jangwon@example.com\x00pass")); err != nil {
		t.Fatalf("AUTH PLAIN returned error: %v", err)
	} else if !done {
		t.Fatal("AUTH PLAIN did not complete")
	}
	return submission
}

type submissionAuthenticator struct {
	username string
	password string
}

func (a submissionAuthenticator) AuthenticatePlain(_ context.Context, _ string, username string, password string) (SubmissionUser, error) {
	if a.username != "" && (username != a.username || password != a.password) {
		return SubmissionUser{}, errAuthTestFailed
	}
	return SubmissionUser{
		CompanyID:   "company-1",
		DomainID:    "domain-1",
		UserID:      "user-1",
		Address:     "jangwon@example.com",
		DisplayName: "Jang Won",
	}, nil
}

type submissionRecorder struct {
	messages []SubmittedMessage
}

func (r *submissionRecorder) RecordSubmitted(_ context.Context, msg SubmittedMessage) (string, error) {
	r.messages = append(r.messages, msg)
	return "message-1", nil
}
