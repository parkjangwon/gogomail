package smtpd

import (
	"context"
	"encoding/base64"
	"io"
	"net"
	"net/smtp"
	"strings"
	"testing"
	"time"

	gosmtp "github.com/emersion/go-smtp"
	"github.com/gogomail/gogomail/internal/storage"
)

func TestSMTPProtocolStoresInboundMessage(t *testing.T) {
	t.Parallel()

	store := storage.NewLocalStore(t.TempDir())
	receiver := NewReceiver(ReceiverOptions{
		Store: store,
		Resolver: StaticResolver{
			"user@example.com": {
				CompanyID: "company-1",
				DomainID:  "domain-1",
				UserID:    "user-1",
				Address:   "user@example.com",
			},
		},
		IDGenerator: func() string { return "protocol-inbound-id" },
		Clock:       func() time.Time { return time.Date(2026, 5, 4, 9, 0, 0, 0, time.UTC) },
	})
	addr, shutdown := startProtocolTestServer(t, receiver, ServerOptions{Domain: "mx.example.com"})
	defer shutdown()

	raw := "Message-ID: <protocol-inbound@example.net>\r\nFrom: sender@example.net\r\nTo: User <user@example.com>\r\nSubject: protocol inbound\r\n\r\nbody\r\n"
	if err := smtp.SendMail(addr, nil, "sender@example.net", []string{"user@example.com"}, []byte(raw)); err != nil {
		t.Fatalf("SendMail returned error: %v", err)
	}

	body, err := store.Get(context.Background(), "mailstore/company-1/domain-1/user-1/maildir/2026/05/protocol-inbound-id.eml")
	if err != nil {
		t.Fatalf("stored protocol message not found: %v", err)
	}
	defer body.Close()
	got, err := io.ReadAll(body)
	if err != nil {
		t.Fatalf("ReadAll returned error: %v", err)
	}
	if string(got) != raw {
		t.Fatalf("stored message = %q, want raw protocol payload", got)
	}
}

func TestSMTPProtocolSubmissionAuthStoresMessage(t *testing.T) {
	t.Parallel()

	store := storage.NewLocalStore(t.TempDir())
	recorder := &submissionRecorder{}
	receiver := NewSubmissionReceiver(SubmissionOptions{
		Store:         store,
		Authenticator: submissionAuthenticator{username: "jangwon@example.com", password: "pass"},
		Recorder:      recorder,
		IDGenerator:   func() string { return "protocol-submission-id" },
		Clock:         func() time.Time { return time.Date(2026, 5, 4, 10, 0, 0, 0, time.UTC) },
	})
	addr, shutdown := startProtocolTestServer(t, receiver, ServerOptions{
		Domain:            "submit.example.com",
		AllowInsecureAuth: true,
	})
	defer shutdown()

	client, err := smtp.Dial(addr)
	if err != nil {
		t.Fatalf("Dial returned error: %v", err)
	}
	defer client.Close()
	if err := client.Hello("client.example.net"); err != nil {
		t.Fatalf("Hello returned error: %v", err)
	}
	authLine := base64.StdEncoding.EncodeToString([]byte("\x00jangwon@example.com\x00pass"))
	if err := protocolCommand(client, 235, "AUTH PLAIN "+authLine); err != nil {
		t.Fatalf("AUTH PLAIN returned error: %v", err)
	}
	if err := client.Mail("jangwon@example.com"); err != nil {
		t.Fatalf("Mail returned error: %v", err)
	}
	if err := client.Rcpt("outside@example.net"); err != nil {
		t.Fatalf("Rcpt returned error: %v", err)
	}
	writer, err := client.Data()
	if err != nil {
		t.Fatalf("Data returned error: %v", err)
	}
	raw := "Message-ID: <protocol-submission@example.com>\r\nFrom: Jang Won <jangwon@example.com>\r\nTo: outside@example.net\r\nSubject: protocol submission\r\n\r\nbody\r\n"
	if _, err := io.WriteString(writer, raw); err != nil {
		t.Fatalf("write DATA returned error: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close DATA returned error: %v", err)
	}
	if err := client.Quit(); err != nil {
		t.Fatalf("Quit returned error: %v", err)
	}

	if len(recorder.messages) != 1 {
		t.Fatalf("recorded submissions = %d, want 1", len(recorder.messages))
	}
	recorded := recorder.messages[0]
	if recorded.EnvelopeFrom != "jangwon@example.com" {
		t.Fatalf("EnvelopeFrom = %q", recorded.EnvelopeFrom)
	}
	if len(recorded.Recipients) != 1 || recorded.Recipients[0] != "outside@example.net" {
		t.Fatalf("Recipients = %+v", recorded.Recipients)
	}
	body, err := store.Get(context.Background(), recorded.StoragePath)
	if err != nil {
		t.Fatalf("stored submission not found: %v", err)
	}
	defer body.Close()
	got, err := io.ReadAll(body)
	if err != nil {
		t.Fatalf("ReadAll returned error: %v", err)
	}
	if string(got) != raw {
		t.Fatalf("stored submission = %q, want raw protocol payload", got)
	}
}

func startProtocolTestServer(t *testing.T, backend gosmtp.Backend, opts ServerOptions) (string, func()) {
	t.Helper()

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Listen returned error: %v", err)
	}
	opts.Addr = listener.Addr().String()
	opts.Backend = backend
	if strings.TrimSpace(opts.Domain) == "" {
		opts.Domain = "localhost"
	}
	server := newSMTPServer(backend, opts)
	errCh := make(chan error, 1)
	go func() {
		errCh <- server.Serve(listener)
	}()
	shutdown := func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = server.Shutdown(ctx)
		if err := <-errCh; err != nil && !strings.Contains(err.Error(), "server closed") {
			t.Fatalf("SMTP test server returned error: %v", err)
		}
	}
	return listener.Addr().String(), shutdown
}

func protocolCommand(client *smtp.Client, expect int, command string) error {
	id, err := client.Text.Cmd("%s", command)
	if err != nil {
		return err
	}
	client.Text.StartResponse(id)
	defer client.Text.EndResponse(id)
	_, _, err = client.Text.ReadResponse(expect)
	return err
}
