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

	raw := "Message-ID: <raw@example.net>\r\nFrom: sender@example.net\r\nTo: jangwon@example.com\r\nSubject: hello\r\n\r\nbody"
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

func TestSessionAcceptsNullReversePathForBounceMail(t *testing.T) {
	t.Parallel()

	recorder := &recordingRecorder{}
	receiver := NewReceiver(ReceiverOptions{
		Store: storage.NewLocalStore(t.TempDir()),
		Resolver: StaticResolver{
			"postmaster@example.com": {
				CompanyID: "company-1",
				DomainID:  "domain-1",
				UserID:    "user-1",
				Address:   "postmaster@example.com",
			},
		},
		Recorder:    recorder,
		IDGenerator: func() string { return "null-reverse-id" },
		Clock:       func() time.Time { return time.Date(2026, 5, 3, 0, 0, 0, 0, time.UTC) },
	})

	session, err := receiver.NewSession(nil)
	if err != nil {
		t.Fatalf("NewSession returned error: %v", err)
	}
	if err := session.Mail("", nil); err != nil {
		t.Fatalf("Mail null reverse-path returned error: %v", err)
	}
	if err := session.Rcpt("postmaster@example.com", nil); err != nil {
		t.Fatalf("Rcpt returned error: %v", err)
	}
	raw := "Message-ID: <bounce@example.net>\r\nFrom: MAILER-DAEMON@example.net\r\nTo: postmaster@example.com\r\nSubject: bounce\r\n\r\nbody"
	if err := session.Data(strings.NewReader(raw)); err != nil {
		t.Fatalf("Data returned error: %v", err)
	}

	if len(recorder.messages) != 1 {
		t.Fatalf("recorded messages = %d, want 1", len(recorder.messages))
	}
	if recorder.messages[0].EnvelopeFrom != "" {
		t.Fatalf("EnvelopeFrom = %q, want null reverse-path", recorder.messages[0].EnvelopeFrom)
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

func TestBuildStoragePathSanitizesPathSegments(t *testing.T) {
	t.Parallel()

	got := BuildStoragePath(Mailbox{
		CompanyID: "../company",
		DomainID:  "domain/one",
		UserID:    "user one",
		Address:   "jangwon@example.com",
	}, "../message/id", time.Date(2026, 5, 3, 9, 0, 0, 0, time.UTC))

	want := "mailstore/company/domain-one/user-one/maildir/2026/05/message-id.eml"
	if got != want {
		t.Fatalf("BuildStoragePath = %q, want %q", got, want)
	}
}

func TestSessionGeneratesFallbackMessageIDWhenMissing(t *testing.T) {
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
		Recorder:    recorder,
		IDGenerator: func() string { return "fallback-storage-id" },
		Clock:       func() time.Time { return time.Date(2026, 5, 3, 9, 0, 0, 0, time.UTC) },
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

	raw := strings.Join([]string{
		"From: sender@example.net",
		"To: jangwon@example.com",
		"Subject: no message id",
		"",
		"body",
	}, "\r\n")
	if err := session.Data(strings.NewReader(raw)); err != nil {
		t.Fatalf("Data returned error: %v", err)
	}
	if len(recorder.messages) != 1 {
		t.Fatalf("recorded messages = %d, want 1", len(recorder.messages))
	}
	if !strings.HasPrefix(recorder.messages[0].Parsed.MessageID, "<missing-") {
		t.Fatalf("fallback MessageID = %q", recorder.messages[0].Parsed.MessageID)
	}

	body, err := store.Get(context.Background(), "mailstore/company-1/domain-1/user-1/maildir/2026/05/fallback-storage-id.eml")
	if err != nil {
		t.Fatalf("stored message not found: %v", err)
	}
	defer body.Close()
	got, err := io.ReadAll(body)
	if err != nil {
		t.Fatalf("ReadAll returned error: %v", err)
	}
	if !strings.HasPrefix(string(got), "Message-ID: <missing-") {
		t.Fatalf("stored message missing fallback Message-ID header: %q", got)
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

func TestSessionDeletesStoredObjectWhenStoredHookFails(t *testing.T) {
	t.Parallel()

	store := storage.NewLocalStore(t.TempDir())
	receiver := NewReceiver(ReceiverOptions{
		Store: store,
		Resolver: StaticResolver{
			"jangwon@example.com": {CompanyID: "c", DomainID: "d", UserID: "u", Address: "jangwon@example.com"},
		},
		IDGenerator: func() string { return "stored-hook-fail" },
		Clock:       func() time.Time { return time.Date(2026, 5, 3, 9, 30, 0, 0, time.UTC) },
		Hooks: []Hook{func(_ context.Context, event Event) error {
			if event.Stage == StageStored {
				return errors.New("stored hook failed")
			}
			return nil
		}},
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
	raw := "Message-ID: <hook-fail@example.net>\r\nFrom: sender@example.net\r\nTo: jangwon@example.com\r\nSubject: hook fail\r\n\r\nbody"
	if err := session.Data(strings.NewReader(raw)); err == nil {
		t.Fatal("Data succeeded despite stored hook failure")
	}
	if _, err := store.Get(context.Background(), "mailstore/c/d/u/maildir/2026/05/stored-hook-fail.eml"); err == nil {
		t.Fatal("stored object remained after stored hook failure")
	}
}

func TestSessionDeletesStoredObjectWhenRecorderFails(t *testing.T) {
	t.Parallel()

	store := storage.NewLocalStore(t.TempDir())
	receiver := NewReceiver(ReceiverOptions{
		Store: store,
		Resolver: StaticResolver{
			"jangwon@example.com": {CompanyID: "c", DomainID: "d", UserID: "u", Address: "jangwon@example.com"},
		},
		Recorder:    failingRecorder{err: errors.New("record failed")},
		IDGenerator: func() string { return "record-fail" },
		Clock:       func() time.Time { return time.Date(2026, 5, 3, 9, 30, 0, 0, time.UTC) },
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
	raw := "Message-ID: <record-fail@example.net>\r\nFrom: sender@example.net\r\nTo: jangwon@example.com\r\nSubject: record fail\r\n\r\nbody"
	if err := session.Data(strings.NewReader(raw)); err == nil {
		t.Fatal("Data succeeded despite recorder failure")
	}
	if _, err := store.Get(context.Background(), "mailstore/c/d/u/maildir/2026/05/record-fail.eml"); err == nil {
		t.Fatal("stored object remained after recorder failure")
	}
}

func TestSessionRunsAuthenticationVerifierAfterParse(t *testing.T) {
	t.Parallel()

	store := storage.NewLocalStore(t.TempDir())
	recorder := &recordingRecorder{}
	verifier := &recordingAuthVerifier{results: AuthenticationResults{
		AuthservID: "mx.example.com",
		SPF:        AuthCheckResult{Result: AuthResultPass, Reason: "mx matched", Identifier: "sender@example.net"},
		DKIM:       AuthCheckResult{Result: AuthResultPass, Reason: "signature valid", Domain: "example.net", Identifier: "@example.net"},
		DMARC:      AuthCheckResult{Result: AuthResultPass, Reason: "aligned", Domain: "example.net"},
	}}
	var stages []Stage
	receiver := NewReceiver(ReceiverOptions{
		Store: store,
		Resolver: StaticResolver{
			"jangwon@example.com": {CompanyID: "c", DomainID: "d", UserID: "u", Address: "jangwon@example.com"},
		},
		Recorder:     recorder,
		AuthVerifier: verifier,
		IDGenerator:  func() string { return "auth-results-id" },
		Clock:        func() time.Time { return time.Date(2026, 5, 3, 9, 0, 0, 0, time.UTC) },
		Hooks: []Hook{
			func(_ context.Context, event Event) error {
				stages = append(stages, event.Stage)
				if event.Stage == StageAuthenticationChecked && event.Authentication.SPF.Result != AuthResultPass {
					t.Fatalf("authentication event = %+v, want SPF pass", event.Authentication)
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
	if err := session.Rcpt("jangwon@example.com", nil); err != nil {
		t.Fatalf("Rcpt returned error: %v", err)
	}
	raw := "Message-ID: <auth@example.net>\r\nFrom: sender@example.net\r\nTo: jangwon@example.com\r\nSubject: auth\r\n\r\nbody"
	if err := session.Data(strings.NewReader(raw)); err != nil {
		t.Fatalf("Data returned error: %v", err)
	}

	want := []Stage{
		StageBackpressureChecked,
		StageSpooled,
		StageParsed,
		StageAuthenticationChecked,
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
	if len(recorder.messages) != 1 {
		t.Fatalf("recorded messages = %d, want 1", len(recorder.messages))
	}
	if recorder.messages[0].Authentication.DMARC.Result != AuthResultPass {
		t.Fatalf("recorded auth = %+v, want DMARC pass", recorder.messages[0].Authentication)
	}
	if verifier.request.EnvelopeFrom != "sender@example.net" || verifier.request.Parsed.MessageID != "<auth@example.net>" {
		t.Fatalf("auth verifier request = %+v", verifier.request)
	}
	if verifier.request.RawMessage == nil {
		t.Fatal("auth verifier raw message is nil")
	}
	body, err := store.Get(context.Background(), "mailstore/c/d/u/maildir/2026/05/auth-results-id.eml")
	if err != nil {
		t.Fatalf("stored message not found: %v", err)
	}
	defer body.Close()
	got, err := io.ReadAll(body)
	if err != nil {
		t.Fatalf("ReadAll returned error: %v", err)
	}
	if !strings.Contains(string(got), "Authentication-Results: mx.example.com;") {
		t.Fatalf("stored message missing Authentication-Results header: %q", string(got))
	}
}

func TestSessionObservesSMTPMetrics(t *testing.T) {
	t.Parallel()

	metrics := &recordingMetrics{}
	receiver := NewReceiver(ReceiverOptions{
		Store: storage.NewLocalStore(t.TempDir()),
		Resolver: StaticResolver{
			"jangwon@example.com": {CompanyID: "c", DomainID: "d", UserID: "u", Address: "jangwon@example.com"},
		},
		Metrics:     metrics,
		IDGenerator: func() string { return "metrics-id" },
		Clock:       func() time.Time { return time.Date(2026, 5, 3, 9, 0, 0, 0, time.UTC) },
	})

	session, err := receiver.NewSession(nil)
	if err != nil {
		t.Fatalf("NewSession returned error: %v", err)
	}
	if err := session.Rcpt("jangwon@example.com", nil); err == nil {
		t.Fatal("Rcpt accepted before Mail")
	}
	if err := session.Mail("sender@example.net", nil); err != nil {
		t.Fatalf("Mail returned error: %v", err)
	}
	if err := session.Rcpt("jangwon@example.com", nil); err != nil {
		t.Fatalf("Rcpt returned error: %v", err)
	}
	raw := "Message-ID: <metrics@example.net>\r\nFrom: sender@example.net\r\nTo: jangwon@example.com\r\nSubject: metrics\r\n\r\nbody"
	if err := session.Data(strings.NewReader(raw)); err != nil {
		t.Fatalf("Data returned error: %v", err)
	}

	if !metrics.has(StageRcpt, MetricRejected) {
		t.Fatalf("metrics = %+v, want rejected rcpt event", metrics.events)
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
	if last.Size == 0 || len(last.Recipients) != 1 || last.Recipients[0] != "jangwon@example.com" {
		t.Fatalf("last metric = %+v, want size and recipient", last)
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

func TestSessionRequiresMailBeforeRcpt(t *testing.T) {
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
	if err := session.Rcpt("jangwon@example.com", nil); err == nil {
		t.Fatal("Rcpt accepted before Mail")
	}
}

func TestSessionMailResetsPreviousRecipients(t *testing.T) {
	t.Parallel()

	recorder := &recordingRecorder{}
	receiver := NewReceiver(ReceiverOptions{
		Store: storage.NewLocalStore(t.TempDir()),
		Resolver: StaticResolver{
			"one@example.com": {CompanyID: "c", DomainID: "d", UserID: "one", Address: "one@example.com"},
			"two@example.com": {CompanyID: "c", DomainID: "d", UserID: "two", Address: "two@example.com"},
		},
		Recorder:    recorder,
		IDGenerator: func() string { return "mail-reset" },
	})

	session, err := receiver.NewSession(nil)
	if err != nil {
		t.Fatalf("NewSession returned error: %v", err)
	}
	if err := session.Mail("sender@example.net", nil); err != nil {
		t.Fatalf("first Mail returned error: %v", err)
	}
	if err := session.Rcpt("one@example.com", nil); err != nil {
		t.Fatalf("first Rcpt returned error: %v", err)
	}
	if err := session.Mail("sender@example.net", nil); err != nil {
		t.Fatalf("second Mail returned error: %v", err)
	}
	if err := session.Rcpt("two@example.com", nil); err != nil {
		t.Fatalf("second Rcpt returned error: %v", err)
	}
	raw := "Message-ID: <mail-reset@example.net>\r\nFrom: sender@example.net\r\nTo: two@example.com\r\nSubject: reset\r\n\r\nbody"
	if err := session.Data(strings.NewReader(raw)); err != nil {
		t.Fatalf("Data returned error: %v", err)
	}
	if len(recorder.messages) != 1 || recorder.messages[0].Mailbox.Address != "two@example.com" {
		t.Fatalf("recorded messages = %+v, want only second transaction recipient", recorder.messages)
	}
}

func TestSessionResetsEnvelopeAfterSuccessfulData(t *testing.T) {
	t.Parallel()

	receiver := NewReceiver(ReceiverOptions{
		Store: storage.NewLocalStore(t.TempDir()),
		Resolver: StaticResolver{
			"jangwon@example.com": {CompanyID: "c", DomainID: "d", UserID: "u", Address: "jangwon@example.com"},
		},
		IDGenerator: func() string { return "reset-envelope" },
		Clock:       func() time.Time { return time.Date(2026, 5, 3, 9, 0, 0, 0, time.UTC) },
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
	raw := "Message-ID: <reset@example.net>\r\nFrom: sender@example.net\r\nTo: jangwon@example.com\r\nSubject: reset\r\n\r\nbody"
	if err := session.Data(strings.NewReader(raw)); err != nil {
		t.Fatalf("Data returned error: %v", err)
	}
	if err := session.Data(strings.NewReader(raw)); err == nil {
		t.Fatal("Data accepted after successful transaction without new Mail/Rcpt")
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

func TestSessionRejectsRecipientsOverAnyRecipientDomainPolicyLimit(t *testing.T) {
	t.Parallel()

	lookup := &mapDomainPolicyLookup{
		policies: map[string]InboundDomainPolicy{
			"d1": {InboundMode: "enforce", MaxRecipientsPerMessage: 100},
			"d2": {InboundMode: "enforce", MaxRecipientsPerMessage: 1},
		},
	}
	receiver := NewReceiver(ReceiverOptions{
		Store: storage.NewLocalStore(t.TempDir()),
		Resolver: StaticResolver{
			"one@example.com": {CompanyID: "c", DomainID: "d1", UserID: "u1", Address: "one@example.com"},
			"two@example.net": {CompanyID: "c", DomainID: "d2", UserID: "u2", Address: "two@example.net"},
		},
		Policy:             ReceivePolicy{MaxRecipientsPerMessage: 100},
		DomainPolicyLookup: lookup,
	})

	session, err := receiver.NewSession(nil)
	if err != nil {
		t.Fatalf("NewSession returned error: %v", err)
	}
	if err := session.Mail("sender@example.org", nil); err != nil {
		t.Fatalf("Mail returned error: %v", err)
	}
	if err := session.Rcpt("one@example.com", nil); err != nil {
		t.Fatalf("first Rcpt returned error: %v", err)
	}
	if err := session.Rcpt("two@example.net", nil); err == nil {
		t.Fatal("second Rcpt was accepted over second recipient domain policy limit")
	}
	if !lookup.seen("d1") || !lookup.seen("d2") {
		t.Fatalf("domain policy lookup calls = %v, want both domains", lookup.calls)
	}
}

func TestSessionAppliesStrictestMixedDomainMessageSizeLimit(t *testing.T) {
	t.Parallel()

	lookup := &mapDomainPolicyLookup{
		policies: map[string]InboundDomainPolicy{
			"d1": {InboundMode: "enforce", MaxMessageBytes: 1024},
			"d2": {InboundMode: "enforce", MaxMessageBytes: 32},
		},
	}
	receiver := NewReceiver(ReceiverOptions{
		Store: storage.NewLocalStore(t.TempDir()),
		Resolver: StaticResolver{
			"one@example.com": {CompanyID: "c", DomainID: "d1", UserID: "u1", Address: "one@example.com"},
			"two@example.net": {CompanyID: "c", DomainID: "d2", UserID: "u2", Address: "two@example.net"},
		},
		Policy:             ReceivePolicy{MaxRecipientsPerMessage: 100, MaxMessageBytes: 1024},
		DomainPolicyLookup: lookup,
	})

	session, err := receiver.NewSession(nil)
	if err != nil {
		t.Fatalf("NewSession returned error: %v", err)
	}
	if err := session.Mail("sender@example.org", nil); err != nil {
		t.Fatalf("Mail returned error: %v", err)
	}
	if err := session.Rcpt("one@example.com", nil); err != nil {
		t.Fatalf("first Rcpt returned error: %v", err)
	}
	if err := session.Rcpt("two@example.net", nil); err != nil {
		t.Fatalf("second Rcpt returned error: %v", err)
	}

	requireSMTPStatus(t, session.Data(strings.NewReader("Subject: too large for d2\r\n\r\nbody")), 552, gosmtp.EnhancedCode{5, 3, 4})
	if !lookup.seen("d1") || !lookup.seen("d2") {
		t.Fatalf("domain policy lookup calls = %v, want both domains", lookup.calls)
	}
}

func TestSessionResetsMixedDomainPolicyAfterFailedData(t *testing.T) {
	t.Parallel()

	lookup := &mapDomainPolicyLookup{
		policies: map[string]InboundDomainPolicy{
			"d1": {InboundMode: "enforce", MaxMessageBytes: 1024},
			"d2": {InboundMode: "enforce", MaxMessageBytes: 32},
		},
	}
	recorder := &recordingRecorder{}
	receiver := NewReceiver(ReceiverOptions{
		Store: storage.NewLocalStore(t.TempDir()),
		Resolver: StaticResolver{
			"one@example.com": {CompanyID: "c", DomainID: "d1", UserID: "u1", Address: "one@example.com"},
			"two@example.net": {CompanyID: "c", DomainID: "d2", UserID: "u2", Address: "two@example.net"},
		},
		Policy:             ReceivePolicy{MaxRecipientsPerMessage: 100, MaxMessageBytes: 1024},
		DomainPolicyLookup: lookup,
		Recorder:           recorder,
	})

	session, err := receiver.NewSession(nil)
	if err != nil {
		t.Fatalf("NewSession returned error: %v", err)
	}
	if err := session.Mail("sender@example.org", nil); err != nil {
		t.Fatalf("first Mail returned error: %v", err)
	}
	if err := session.Rcpt("one@example.com", nil); err != nil {
		t.Fatalf("first Rcpt returned error: %v", err)
	}
	if err := session.Rcpt("two@example.net", nil); err != nil {
		t.Fatalf("second Rcpt returned error: %v", err)
	}
	raw := "Message-ID: <mixed-too-large@example.org>\r\nSubject: too large for d2\r\n\r\nbody"
	requireSMTPStatus(t, session.Data(strings.NewReader(raw)), 552, gosmtp.EnhancedCode{5, 3, 4})

	if err := session.Mail("sender@example.org", nil); err != nil {
		t.Fatalf("second Mail returned error: %v", err)
	}
	if err := session.Rcpt("one@example.com", nil); err != nil {
		t.Fatalf("third Rcpt returned error: %v", err)
	}
	if err := session.Data(strings.NewReader(raw)); err != nil {
		t.Fatalf("d1-only Data after failed mixed-domain DATA returned error: %v", err)
	}
	if len(recorder.messages) != 1 {
		t.Fatalf("recorded messages = %d, want 1", len(recorder.messages))
	}
	if recorder.messages[0].Mailbox.Address != "one@example.com" {
		t.Fatalf("recorded mailbox = %q, want d1-only recipient", recorder.messages[0].Mailbox.Address)
	}
}

func TestSessionResetsMixedDomainPolicyAfterRSET(t *testing.T) {
	t.Parallel()

	lookup := &mapDomainPolicyLookup{
		policies: map[string]InboundDomainPolicy{
			"d1": {InboundMode: "enforce", MaxMessageBytes: 1024},
			"d2": {InboundMode: "enforce", MaxMessageBytes: 32},
		},
	}
	recorder := &recordingRecorder{}
	receiver := NewReceiver(ReceiverOptions{
		Store: storage.NewLocalStore(t.TempDir()),
		Resolver: StaticResolver{
			"one@example.com": {CompanyID: "c", DomainID: "d1", UserID: "u1", Address: "one@example.com"},
			"two@example.net": {CompanyID: "c", DomainID: "d2", UserID: "u2", Address: "two@example.net"},
		},
		Policy:             ReceivePolicy{MaxRecipientsPerMessage: 100, MaxMessageBytes: 1024},
		DomainPolicyLookup: lookup,
		Recorder:           recorder,
	})

	session, err := receiver.NewSession(nil)
	if err != nil {
		t.Fatalf("NewSession returned error: %v", err)
	}
	if err := session.Mail("sender@example.org", nil); err != nil {
		t.Fatalf("first Mail returned error: %v", err)
	}
	if err := session.Rcpt("one@example.com", nil); err != nil {
		t.Fatalf("first Rcpt returned error: %v", err)
	}
	if err := session.Rcpt("two@example.net", nil); err != nil {
		t.Fatalf("second Rcpt returned error: %v", err)
	}
	session.Reset()

	if err := session.Mail("sender@example.org", nil); err != nil {
		t.Fatalf("second Mail returned error: %v", err)
	}
	if err := session.Rcpt("one@example.com", nil); err != nil {
		t.Fatalf("third Rcpt returned error: %v", err)
	}
	raw := "Message-ID: <rset-reset@example.org>\r\nSubject: larger than d2\r\n\r\nbody"
	if err := session.Data(strings.NewReader(raw)); err != nil {
		t.Fatalf("d1-only Data after RSET returned error: %v", err)
	}
	if len(recorder.messages) != 1 {
		t.Fatalf("recorded messages = %d, want 1", len(recorder.messages))
	}
	if recorder.messages[0].Mailbox.Address != "one@example.com" {
		t.Fatalf("recorded mailbox = %q, want d1-only recipient", recorder.messages[0].Mailbox.Address)
	}
}

func TestSessionRejectsRcptWhenDomainPolicyLookupFails(t *testing.T) {
	t.Parallel()

	receiver := NewReceiver(ReceiverOptions{
		Store: storage.NewLocalStore(t.TempDir()),
		Resolver: StaticResolver{
			"one@example.com": {CompanyID: "c", DomainID: "d1", UserID: "u1", Address: "one@example.com"},
		},
		DomainPolicyLookup: &mapDomainPolicyLookup{err: errors.New("database unavailable")},
	})

	session, err := receiver.NewSession(nil)
	if err != nil {
		t.Fatalf("NewSession returned error: %v", err)
	}
	if err := session.Mail("sender@example.org", nil); err != nil {
		t.Fatalf("Mail returned error: %v", err)
	}
	requireSMTPStatus(t, session.Rcpt("one@example.com", nil), 451, gosmtp.EnhancedCode{4, 7, 1})
}

func TestSessionDomainPolicyLookupFailureDoesNotPoisonAcceptedRecipients(t *testing.T) {
	t.Parallel()

	lookup := &mapDomainPolicyLookup{
		policies: map[string]InboundDomainPolicy{
			"d1": {InboundMode: "enforce", MaxMessageBytes: 1024},
		},
		errs: map[string]error{
			"d2": errors.New("database unavailable"),
		},
	}
	recorder := &recordingRecorder{}
	receiver := NewReceiver(ReceiverOptions{
		Store: storage.NewLocalStore(t.TempDir()),
		Resolver: StaticResolver{
			"one@example.com": {CompanyID: "c", DomainID: "d1", UserID: "u1", Address: "one@example.com"},
			"two@example.net": {CompanyID: "c", DomainID: "d2", UserID: "u2", Address: "two@example.net"},
		},
		Policy:             ReceivePolicy{MaxRecipientsPerMessage: 100, MaxMessageBytes: 1024},
		DomainPolicyLookup: lookup,
		Recorder:           recorder,
	})

	session, err := receiver.NewSession(nil)
	if err != nil {
		t.Fatalf("NewSession returned error: %v", err)
	}
	if err := session.Mail("sender@example.org", nil); err != nil {
		t.Fatalf("Mail returned error: %v", err)
	}
	if err := session.Rcpt("one@example.com", nil); err != nil {
		t.Fatalf("first Rcpt returned error: %v", err)
	}
	requireSMTPStatus(t, session.Rcpt("two@example.net", nil), 451, gosmtp.EnhancedCode{4, 7, 1})

	raw := "Message-ID: <one@example.org>\r\nSubject: accepted\r\n\r\nbody"
	if err := session.Data(strings.NewReader(raw)); err != nil {
		t.Fatalf("Data after failed second-domain policy lookup returned error: %v", err)
	}
	if len(recorder.messages) != 1 {
		t.Fatalf("recorded messages = %d, want 1", len(recorder.messages))
	}
	if recorder.messages[0].Mailbox.Address != "one@example.com" {
		t.Fatalf("recorded mailbox = %q, want accepted first recipient", recorder.messages[0].Mailbox.Address)
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
	if err := session.Logout(); err != nil {
		t.Fatalf("Logout returned error: %v", err)
	}
	if err := session.Mail("sender@example.net", nil); err == nil {
		t.Fatal("Mail accepted after logout without re-authentication")
	}
}

func TestSessionObservesAndEmitsAuth(t *testing.T) {
	t.Parallel()

	metrics := &recordingMetrics{}
	var stages []Stage
	receiver := NewReceiver(ReceiverOptions{
		Store: storage.NewLocalStore(t.TempDir()),
		Resolver: StaticResolver{
			"user@example.com": {CompanyID: "c", DomainID: "d", UserID: "u", Address: "user@example.com"},
		},
		Authenticator: plainAuthenticator{username: "user", password: "pass"},
		RequireAuth:   true,
		Metrics:       metrics,
		Hooks: []Hook{func(_ context.Context, event Event) error {
			stages = append(stages, event.Stage)
			return nil
		}},
	})

	session, err := receiver.NewSession(nil)
	if err != nil {
		t.Fatalf("NewSession returned error: %v", err)
	}
	authSession := session.(interface {
		Auth(string) (sasl.Server, error)
	})
	server, err := authSession.Auth(sasl.Plain)
	if err != nil {
		t.Fatalf("Auth returned error: %v", err)
	}
	if _, done, err := server.Next([]byte("\x00user\x00pass")); err != nil {
		t.Fatalf("AUTH PLAIN returned error: %v", err)
	} else if !done {
		t.Fatal("AUTH PLAIN did not complete")
	}

	if len(stages) != 1 || stages[0] != StageAuthenticated {
		t.Fatalf("stages = %v, want authenticated", stages)
	}
	if !metrics.has(StageAuthenticated, MetricAccepted) {
		t.Fatalf("metrics = %+v, want accepted auth metric", metrics.events)
	}
}

func TestSessionRejectsRepeatedAuth(t *testing.T) {
	t.Parallel()

	receiver := NewReceiver(ReceiverOptions{
		Store: storage.NewLocalStore(t.TempDir()),
		Resolver: StaticResolver{
			"user@example.com": {CompanyID: "c", DomainID: "d", UserID: "u", Address: "user@example.com"},
		},
		Authenticator: plainAuthenticator{username: "user", password: "pass"},
	})

	session, err := receiver.NewSession(nil)
	if err != nil {
		t.Fatalf("NewSession returned error: %v", err)
	}
	authSession := session.(interface {
		Auth(string) (sasl.Server, error)
	})
	server, err := authSession.Auth(sasl.Plain)
	if err != nil {
		t.Fatalf("Auth returned error: %v", err)
	}
	if _, done, err := server.Next([]byte("\x00user\x00pass")); err != nil {
		t.Fatalf("AUTH PLAIN returned error: %v", err)
	} else if !done {
		t.Fatal("AUTH PLAIN did not complete")
	}
	if _, err := authSession.Auth(sasl.Plain); err == nil {
		t.Fatal("second AUTH was accepted")
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

func TestSessionRejectsInternationalizedRecipientWithoutSMTPUTF8Transaction(t *testing.T) {
	t.Parallel()

	receiver := NewReceiver(ReceiverOptions{
		Store:           storage.NewLocalStore(t.TempDir()),
		SupportSMTPUTF8: true,
		Resolver: StaticResolver{
			"장원@example.com": {CompanyID: "c", DomainID: "d", UserID: "u", Address: "장원@example.com"},
		},
	})

	session, err := receiver.NewSession(nil)
	if err != nil {
		t.Fatalf("NewSession returned error: %v", err)
	}
	if err := session.Mail("sender@example.net", nil); err != nil {
		t.Fatalf("Mail returned error: %v", err)
	}
	err = session.Rcpt("장원@example.com", nil)
	if err == nil {
		t.Fatal("Rcpt accepted internationalized address without SMTPUTF8 transaction")
	}
	var smtpErr *gosmtp.SMTPError
	if !errors.As(err, &smtpErr) || smtpErr.Code != 553 {
		t.Fatalf("Rcpt error = %v, want SMTP 553", err)
	}
}

func TestSessionAcceptsInternationalizedRecipientWithSMTPUTF8Transaction(t *testing.T) {
	t.Parallel()

	receiver := NewReceiver(ReceiverOptions{
		Store:           storage.NewLocalStore(t.TempDir()),
		SupportSMTPUTF8: true,
		Resolver: StaticResolver{
			"장원@example.com": {CompanyID: "c", DomainID: "d", UserID: "u", Address: "장원@example.com"},
		},
		IDGenerator: func() string { return "smtputf8-id" },
		Clock:       func() time.Time { return time.Date(2026, 5, 3, 9, 0, 0, 0, time.UTC) },
	})

	session, err := receiver.NewSession(nil)
	if err != nil {
		t.Fatalf("NewSession returned error: %v", err)
	}
	if err := session.Mail("sender@example.net", &gosmtp.MailOptions{UTF8: true}); err != nil {
		t.Fatalf("Mail returned error: %v", err)
	}
	if err := session.Rcpt("장원@example.com", nil); err != nil {
		t.Fatalf("Rcpt returned error: %v", err)
	}
	raw := "Message-ID: <smtputf8@example.net>\r\nFrom: sender@example.net\r\nTo: 장원@example.com\r\nSubject: utf8\r\n\r\nbody"
	if err := session.Data(strings.NewReader(raw)); err != nil {
		t.Fatalf("Data returned error: %v", err)
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

type failingRecorder struct {
	err error
}

func (r failingRecorder) Record(context.Context, ReceivedMessage) error {
	return r.err
}

func TestSessionSkipsAuthenticationHookWhenVerifierDisabled(t *testing.T) {
	t.Parallel()

	var stages []Stage
	receiver := NewReceiver(ReceiverOptions{
		Store: storage.NewLocalStore(t.TempDir()),
		Resolver: StaticResolver{
			"jangwon@example.com": {CompanyID: "c", DomainID: "d", UserID: "u", Address: "jangwon@example.com"},
		},
		IDGenerator: func() string { return "auth-disabled-id" },
		Clock:       func() time.Time { return time.Date(2026, 5, 3, 9, 0, 0, 0, time.UTC) },
		Hooks: []Hook{
			func(_ context.Context, event Event) error {
				stages = append(stages, event.Stage)
				if event.Stage == StageAuthenticationChecked {
					t.Fatal("authentication hook ran without an auth verifier")
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
	if err := session.Rcpt("jangwon@example.com", nil); err != nil {
		t.Fatalf("Rcpt returned error: %v", err)
	}
	raw := "Message-ID: <disabled@example.net>\r\nFrom: sender@example.net\r\nTo: jangwon@example.com\r\nSubject: disabled\r\n\r\nbody"
	if err := session.Data(strings.NewReader(raw)); err != nil {
		t.Fatalf("Data returned error: %v", err)
	}
	if len(stages) == 0 {
		t.Fatal("no hooks ran")
	}
}

func TestSessionPreservesDSNOptionsForRecorderAndHooks(t *testing.T) {
	t.Parallel()

	recorder := &recordingRecorder{}
	var recordedEvent Event
	receiver := NewReceiver(ReceiverOptions{
		Store: storage.NewLocalStore(t.TempDir()),
		Resolver: StaticResolver{
			"jangwon@example.com": {CompanyID: "c", DomainID: "d", UserID: "u", Address: "jangwon@example.com"},
		},
		Recorder:    recorder,
		SupportDSN:  true,
		IDGenerator: func() string { return "dsn-options-id" },
		Clock:       func() time.Time { return time.Date(2026, 5, 3, 9, 0, 0, 0, time.UTC) },
		Hooks: []Hook{
			func(_ context.Context, event Event) error {
				if event.Stage == StageRecorded {
					recordedEvent = event
				}
				return nil
			},
		},
	})

	session, err := receiver.NewSession(nil)
	if err != nil {
		t.Fatalf("NewSession returned error: %v", err)
	}
	if err := session.Mail("sender@example.net", &gosmtp.MailOptions{
		Return:     gosmtp.DSNReturnFull,
		EnvelopeID: "env-123",
	}); err != nil {
		t.Fatalf("Mail returned error: %v", err)
	}
	if err := session.Rcpt("jangwon@example.com", &gosmtp.RcptOptions{
		Notify:            []gosmtp.DSNNotify{gosmtp.DSNNotifyFailure, gosmtp.DSNNotifyDelayed},
		OriginalRecipient: "rfc822;alias@example.com",
	}); err != nil {
		t.Fatalf("Rcpt returned error: %v", err)
	}
	raw := "Message-ID: <dsn-options@example.net>\r\nFrom: sender@example.net\r\nTo: jangwon@example.com\r\nSubject: dsn\r\n\r\nbody"
	if err := session.Data(strings.NewReader(raw)); err != nil {
		t.Fatalf("Data returned error: %v", err)
	}

	if len(recorder.messages) != 1 {
		t.Fatalf("recorded messages = %d, want 1", len(recorder.messages))
	}
	for name, got := range map[string]DSNOptions{
		"recorder": recorder.messages[0].DSN,
		"hook":     recordedEvent.DSN,
	} {
		if got.Return != "FULL" || got.EnvelopeID != "env-123" {
			t.Fatalf("%s DSN envelope = %+v", name, got)
		}
		if len(got.Recipients) != 1 {
			t.Fatalf("%s DSN recipients = %+v", name, got.Recipients)
		}
		recipient := got.Recipients[0]
		if recipient.Address != "jangwon@example.com" {
			t.Fatalf("%s DSN recipient address = %q", name, recipient.Address)
		}
		if strings.Join(recipient.Notify, ",") != "FAILURE,DELAY" {
			t.Fatalf("%s DSN notify = %v", name, recipient.Notify)
		}
		if recipient.OriginalRecipient != "rfc822;alias@example.com" {
			t.Fatalf("%s DSN original recipient = %q", name, recipient.OriginalRecipient)
		}
	}
}

type recordingAuthVerifier struct {
	results AuthenticationResults
	request AuthenticationRequest
}

func (v *recordingAuthVerifier) VerifyAuthentication(_ context.Context, req AuthenticationRequest) (AuthenticationResults, error) {
	v.request = req
	return v.results, nil
}

type recordingMetrics struct {
	events []MetricEvent
}

func (m *recordingMetrics) ObserveSMTP(_ context.Context, event MetricEvent) {
	m.events = append(m.events, event)
}

func (m *recordingMetrics) has(stage Stage, result MetricResult) bool {
	for _, event := range m.events {
		if event.Stage == stage && event.Result == result {
			return true
		}
	}
	return false
}

type duplicateDeduplicator struct{}

func (duplicateDeduplicator) CheckAndSet(context.Context, DedupKey) (bool, error) {
	return false, nil
}

type mapDomainPolicyLookup struct {
	policies map[string]InboundDomainPolicy
	calls    []string
	err      error
	errs     map[string]error
}

func (l *mapDomainPolicyLookup) InboundDomainPolicy(_ context.Context, domainID string) (InboundDomainPolicy, error) {
	l.calls = append(l.calls, domainID)
	if l.err != nil {
		return InboundDomainPolicy{}, l.err
	}
	if err := l.errs[domainID]; err != nil {
		return InboundDomainPolicy{}, err
	}
	return l.policies[domainID], nil
}

func (l *mapDomainPolicyLookup) seen(domainID string) bool {
	for _, call := range l.calls {
		if call == domainID {
			return true
		}
	}
	return false
}

type denyRateLimiter struct{}

func (denyRateLimiter) Allow(context.Context, RateLimitKey) (bool, error) {
	return false, nil
}

type rejectBackpressure struct{}

func (rejectBackpressure) Accept(context.Context) (bool, error) {
	return false, nil
}
