package smtpd

import (
	"context"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/emersion/go-sasl"
	gosmtp "github.com/emersion/go-smtp"
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

func TestSubmissionRejectsRepeatedAuth(t *testing.T) {
	t.Parallel()

	session := newAuthenticatedSubmissionSession(t, &submissionRecorder{}, storage.NewLocalStore(t.TempDir()))
	if _, err := session.Auth(sasl.Plain); err == nil {
		t.Fatal("second AUTH was accepted")
	}
}

func TestSubmissionRejectsEnvelopeFromMismatch(t *testing.T) {
	t.Parallel()

	session := newAuthenticatedSubmissionSession(t, &submissionRecorder{}, storage.NewLocalStore(t.TempDir()))

	if err := session.Mail("other@example.com", nil); err == nil {
		t.Fatal("Mail accepted envelope sender that does not belong to authenticated user")
	}
}

func TestSubmissionRejectsSMTPUTF8UntilExplicitlySupported(t *testing.T) {
	t.Parallel()

	session := newAuthenticatedSubmissionSession(t, &submissionRecorder{}, storage.NewLocalStore(t.TempDir()))

	if err := session.Mail("jangwon@example.com", &gosmtp.MailOptions{UTF8: true}); err == nil {
		t.Fatal("Mail accepted SMTPUTF8 before support is enabled")
	}
}

func TestSubmissionRequiresMailBeforeRcpt(t *testing.T) {
	t.Parallel()

	session := newAuthenticatedSubmissionSession(t, &submissionRecorder{}, storage.NewLocalStore(t.TempDir()))

	if err := session.Rcpt("outside@example.net", nil); err == nil {
		t.Fatal("Rcpt accepted before Mail")
	}
}

func TestSubmissionRejectsRecipientsOverPolicyLimit(t *testing.T) {
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
	if err := submission.Rcpt("two@example.net", nil); err == nil {
		t.Fatal("second Rcpt accepted over policy limit")
	}
}

func TestSubmissionResetsEnvelopeAfterSuccessfulData(t *testing.T) {
	t.Parallel()

	session := newAuthenticatedSubmissionSession(t, &submissionRecorder{}, storage.NewLocalStore(t.TempDir()))

	if err := session.Mail("jangwon@example.com", nil); err != nil {
		t.Fatalf("Mail returned error: %v", err)
	}
	if err := session.Rcpt("outside@example.net", nil); err != nil {
		t.Fatalf("Rcpt returned error: %v", err)
	}
	raw := "Message-ID: <submission-reset@example.com>\r\nFrom: Jang Won <jangwon@example.com>\r\nTo: Outside <outside@example.net>\r\nSubject: reset\r\n\r\nbody"
	if err := session.Data(strings.NewReader(raw)); err != nil {
		t.Fatalf("Data returned error: %v", err)
	}
	if err := session.Data(strings.NewReader(raw)); err == nil {
		t.Fatal("Data accepted after successful transaction without new Mail/Rcpt")
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

func TestSubmissionPrependsReceivedHeaderWhenConfigured(t *testing.T) {
	t.Parallel()

	store := storage.NewLocalStore(t.TempDir())
	recorder := &submissionRecorder{}
	receiver := NewSubmissionReceiver(SubmissionOptions{
		Store:             store,
		Authenticator:     submissionAuthenticator{username: "jangwon@example.com", password: "pass"},
		Recorder:          recorder,
		AddReceivedHeader: true,
		ReceivedDomain:    "submit.example.com",
		IDGenerator:       func() string { return "submission-received-id" },
		Clock:             func() time.Time { return time.Date(2026, 5, 3, 10, 0, 0, 0, time.UTC) },
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
	if err := submission.Rcpt("outside@example.net", nil); err != nil {
		t.Fatalf("Rcpt returned error: %v", err)
	}
	raw := "From: Jang Won <jangwon@example.com>\r\nTo: Outside <outside@example.net>\r\nSubject: submitted\r\n\r\nbody"
	if err := submission.Data(strings.NewReader(raw)); err != nil {
		t.Fatalf("Data returned error: %v", err)
	}

	body, err := store.Get(context.Background(), "mailstore/company-1/domain-1/user-1/maildir/2026/05/submission-received-id.eml")
	if err != nil {
		t.Fatalf("stored submitted message not found: %v", err)
	}
	defer body.Close()
	got, err := io.ReadAll(body)
	if err != nil {
		t.Fatalf("ReadAll returned error: %v", err)
	}
	if !strings.HasPrefix(string(got), "Received: from unknown by submit.example.com with ESMTPA id submission-received-id; ") {
		t.Fatalf("stored submission missing Received header: %q", got)
	}
	if !strings.Contains(string(got), "\r\nMessage-ID: <submission-received-id@example.com>\r\n") {
		t.Fatalf("stored submission missing generated Message-ID header: %q", got)
	}
	if recorder.messages[0].Parsed.MessageID != "<submission-received-id@example.com>" {
		t.Fatalf("recorded MessageID = %q", recorder.messages[0].Parsed.MessageID)
	}
}

func TestSubmissionEmitsPipelineHooksInOrder(t *testing.T) {
	t.Parallel()

	var stages []Stage
	store := storage.NewLocalStore(t.TempDir())
	recorder := &submissionRecorder{}
	receiver := NewSubmissionReceiver(SubmissionOptions{
		Store:         store,
		Authenticator: submissionAuthenticator{username: "jangwon@example.com", password: "pass"},
		Recorder:      recorder,
		IDGenerator:   func() string { return "hooked-submission" },
		Clock:         func() time.Time { return time.Date(2026, 5, 3, 11, 0, 0, 0, time.UTC) },
		Hooks: []Hook{
			func(_ context.Context, event Event) error {
				stages = append(stages, event.Stage)
				if event.Stage == StageAuthenticated && event.SubmissionUser.UserID != "user-1" {
					t.Fatalf("authenticated user = %+v", event.SubmissionUser)
				}
				if event.Stage == StageParsed && event.Parsed.Subject != "hook submission" {
					t.Fatalf("parsed subject = %q", event.Parsed.Subject)
				}
				if event.Stage == StageStored && event.StoragePath == "" {
					t.Fatal("stored event has empty storage path")
				}
				return nil
			},
		},
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
	if err := submission.Rcpt("outside@example.net", nil); err != nil {
		t.Fatalf("Rcpt returned error: %v", err)
	}
	raw := strings.Join([]string{
		"Message-ID: <hook-submission@example.com>",
		"From: Jang Won <jangwon@example.com>",
		"To: Outside <outside@example.net>",
		"Subject: hook submission",
		"Content-Type: text/plain; charset=utf-8",
		"",
		"body",
	}, "\r\n")
	if err := submission.Data(strings.NewReader(raw)); err != nil {
		t.Fatalf("Data returned error: %v", err)
	}

	want := []Stage{
		StageAuthenticated,
		StageMailFrom,
		StageRcpt,
		StageSpooled,
		StageParsed,
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

func TestSubmissionObservesSMTPMetrics(t *testing.T) {
	t.Parallel()

	metrics := &recordingMetrics{}
	store := storage.NewLocalStore(t.TempDir())
	recorder := &submissionRecorder{}
	receiver := NewSubmissionReceiver(SubmissionOptions{
		Store:         store,
		Authenticator: submissionAuthenticator{username: "jangwon@example.com", password: "pass"},
		Recorder:      recorder,
		Metrics:       metrics,
		IDGenerator:   func() string { return "submission-metrics-id" },
		Clock:         func() time.Time { return time.Date(2026, 5, 3, 11, 0, 0, 0, time.UTC) },
	})

	session, err := receiver.NewSession(nil)
	if err != nil {
		t.Fatalf("NewSession returned error: %v", err)
	}
	submission := session.(*submissionSession)
	if err := submission.Mail("jangwon@example.com", nil); err == nil {
		t.Fatal("Mail accepted before AUTH")
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
	if err := submission.Mail("jangwon@example.com", nil); err != nil {
		t.Fatalf("Mail returned error: %v", err)
	}
	if err := submission.Rcpt("outside@example.net", nil); err != nil {
		t.Fatalf("Rcpt returned error: %v", err)
	}
	raw := "Message-ID: <submission-metrics@example.com>\r\nFrom: Jang Won <jangwon@example.com>\r\nTo: Outside <outside@example.net>\r\nSubject: metrics\r\n\r\nbody"
	if err := submission.Data(strings.NewReader(raw)); err != nil {
		t.Fatalf("Data returned error: %v", err)
	}

	if !metrics.has(StageMailFrom, MetricRejected) {
		t.Fatalf("metrics = %+v, want rejected mail event", metrics.events)
	}
	if !metrics.has(StageAuthenticated, MetricAccepted) {
		t.Fatalf("metrics = %+v, want accepted auth event", metrics.events)
	}
	if !metrics.has(StageMailFrom, MetricAccepted) {
		t.Fatalf("metrics = %+v, want accepted mail event", metrics.events)
	}
	if !metrics.has(StageRcpt, MetricAccepted) {
		t.Fatalf("metrics = %+v, want accepted rcpt event", metrics.events)
	}
	if !metrics.has(StageRecorded, MetricAccepted) {
		t.Fatalf("metrics = %+v, want accepted data/recorded event", metrics.events)
	}
	last := metrics.events[len(metrics.events)-1]
	if last.Size == 0 || len(last.Recipients) != 1 || last.Recipients[0] != "outside@example.net" {
		t.Fatalf("last metric = %+v, want size and submitted recipient", last)
	}
}

func TestSubmissionPreservesDSNOptions(t *testing.T) {
	t.Parallel()

	recorder := &submissionRecorder{}
	store := storage.NewLocalStore(t.TempDir())
	submission := newAuthenticatedSubmissionSession(t, recorder, store)
	submission.receiver.supportDSN = true

	if err := submission.Mail("jangwon@example.com", &gosmtp.MailOptions{
		Return:     gosmtp.DSNReturnHeaders,
		EnvelopeID: "submitted-env",
	}); err != nil {
		t.Fatalf("Mail returned error: %v", err)
	}
	if err := submission.Rcpt("Outside@Example.NET", &gosmtp.RcptOptions{
		Notify:            []gosmtp.DSNNotify{gosmtp.DSNNotifySuccess, gosmtp.DSNNotifyFailure},
		OriginalRecipient: "rfc822;team@example.net",
	}); err != nil {
		t.Fatalf("Rcpt returned error: %v", err)
	}
	raw := "Message-ID: <submitted-dsn@example.com>\r\nFrom: Jang Won <jangwon@example.com>\r\nTo: Outside <outside@example.net>\r\nSubject: dsn\r\n\r\nbody"
	if err := submission.Data(strings.NewReader(raw)); err != nil {
		t.Fatalf("Data returned error: %v", err)
	}

	if len(recorder.messages) != 1 {
		t.Fatalf("submitted messages = %d, want 1", len(recorder.messages))
	}
	got := recorder.messages[0].DSN
	if got.Return != "HDRS" || got.EnvelopeID != "submitted-env" {
		t.Fatalf("DSN envelope = %+v", got)
	}
	if len(got.Recipients) != 1 {
		t.Fatalf("DSN recipients = %+v", got.Recipients)
	}
	recipient := got.Recipients[0]
	if recipient.Address != "outside@example.net" {
		t.Fatalf("recipient address = %q", recipient.Address)
	}
	if strings.Join(recipient.Notify, ",") != "SUCCESS,FAILURE" {
		t.Fatalf("notify = %v", recipient.Notify)
	}
	if recipient.OriginalRecipient != "rfc822;team@example.net" {
		t.Fatalf("original recipient = %q", recipient.OriginalRecipient)
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
