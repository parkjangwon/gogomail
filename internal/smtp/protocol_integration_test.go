package smtpd

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/pem"
	"io"
	"math/big"
	"net"
	"net/smtp"
	"net/textproto"
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

func TestSMTPProtocolSubmissionAuthAfterSTARTTLS(t *testing.T) {
	t.Parallel()

	store := storage.NewLocalStore(t.TempDir())
	recorder := &submissionRecorder{}
	receiver := NewSubmissionReceiver(SubmissionOptions{
		Store:         store,
		Authenticator: submissionAuthenticator{username: "jangwon@example.com", password: "pass"},
		Recorder:      recorder,
		IDGenerator:   func() string { return "protocol-starttls-submission-id" },
		Clock:         func() time.Time { return time.Date(2026, 5, 4, 12, 0, 0, 0, time.UTC) },
	})
	addr, shutdown := startProtocolTestServer(t, receiver, ServerOptions{
		Domain:            "submit.example.com",
		TLSConfig:         testServerTLSConfig(t),
		AllowInsecureAuth: false,
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
	if ok, _ := client.Extension("STARTTLS"); !ok {
		t.Fatal("STARTTLS extension not advertised")
	}
	if err := client.StartTLS(&tls.Config{InsecureSkipVerify: true, MinVersion: tls.VersionTLS12}); err != nil {
		t.Fatalf("StartTLS returned error: %v", err)
	}
	authLine := base64.StdEncoding.EncodeToString([]byte("\x00jangwon@example.com\x00pass"))
	if err := protocolCommand(client, 235, "AUTH PLAIN "+authLine); err != nil {
		t.Fatalf("AUTH PLAIN after STARTTLS returned error: %v", err)
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
	raw := "Message-ID: <protocol-starttls-submission@example.com>\r\nFrom: jangwon@example.com\r\nTo: outside@example.net\r\nSubject: starttls submission\r\n\r\nbody\r\n"
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
}

func TestSMTPProtocolSubmissionRejectsAuthBeforeSTARTTLS(t *testing.T) {
	t.Parallel()

	receiver := NewSubmissionReceiver(SubmissionOptions{
		Store:         storage.NewLocalStore(t.TempDir()),
		Authenticator: submissionAuthenticator{username: "jangwon@example.com", password: "pass"},
		Recorder:      &submissionRecorder{},
	})
	addr, shutdown := startProtocolTestServer(t, receiver, ServerOptions{
		Domain:            "submit.example.com",
		TLSConfig:         testServerTLSConfig(t),
		AllowInsecureAuth: false,
	})
	defer shutdown()

	conn, err := net.Dial("tcp", addr)
	if err != nil {
		t.Fatalf("Dial returned error: %v", err)
	}
	defer conn.Close()
	text := textproto.NewConn(conn)
	defer text.Close()
	if _, _, err := text.ReadResponse(220); err != nil {
		t.Fatalf("banner ReadResponse returned error: %v", err)
	}
	if err := rawProtocolCommand(text, 250, "EHLO client.example.net"); err != nil {
		t.Fatalf("EHLO returned error: %v", err)
	}
	authLine := base64.StdEncoding.EncodeToString([]byte("\x00jangwon@example.com\x00pass"))
	code, msg, err := rawProtocolCommandCode(text, "AUTH PLAIN "+authLine)
	if err != nil {
		t.Fatalf("AUTH before STARTTLS returned transport error: %v", err)
	}
	if code < 500 || code > 599 {
		t.Fatalf("AUTH before STARTTLS code = %d %q, want 5xx", code, msg)
	}
	if err := rawProtocolCommand(text, 221, "QUIT"); err != nil {
		t.Fatalf("QUIT returned error: %v", err)
	}
}

func TestSMTPProtocolRejectsUnsupportedDSNMailOptions(t *testing.T) {
	t.Parallel()

	receiver := NewReceiver(ReceiverOptions{
		Store: storage.NewLocalStore(t.TempDir()),
		Resolver: StaticResolver{
			"user@example.com": {CompanyID: "company-1", DomainID: "domain-1", UserID: "user-1", Address: "user@example.com"},
		},
	})
	addr, shutdown := startProtocolTestServer(t, receiver, ServerOptions{Domain: "mx.example.com"})
	defer shutdown()

	conn, err := net.Dial("tcp", addr)
	if err != nil {
		t.Fatalf("Dial returned error: %v", err)
	}
	defer conn.Close()
	text := textproto.NewConn(conn)
	defer text.Close()
	if _, _, err := text.ReadResponse(220); err != nil {
		t.Fatalf("banner ReadResponse returned error: %v", err)
	}
	if err := rawProtocolCommand(text, 250, "EHLO client.example.net"); err != nil {
		t.Fatalf("EHLO returned error: %v", err)
	}
	code, msg, err := rawProtocolCommandCode(text, "MAIL FROM:<sender@example.net> RET=HDRS")
	if err != nil {
		t.Fatalf("MAIL FROM with unsupported DSN returned transport error: %v", err)
	}
	if code < 500 || code > 599 {
		t.Fatalf("MAIL FROM unsupported DSN code = %d %q, want 5xx", code, msg)
	}
	if err := rawProtocolCommand(text, 221, "QUIT"); err != nil {
		t.Fatalf("QUIT returned error: %v", err)
	}
}

func TestSMTPProtocolPreservesWireDSNOptions(t *testing.T) {
	t.Parallel()

	recorder := &recordingRecorder{}
	receiver := NewReceiver(ReceiverOptions{
		Store: storage.NewLocalStore(t.TempDir()),
		Resolver: StaticResolver{
			"user@example.com": {CompanyID: "company-1", DomainID: "domain-1", UserID: "user-1", Address: "user@example.com"},
		},
		Recorder:   recorder,
		SupportDSN: true,
	})
	addr, shutdown := startProtocolTestServer(t, receiver, ServerOptions{
		Domain:    "mx.example.com",
		EnableDSN: true,
	})
	defer shutdown()

	conn, err := net.Dial("tcp", addr)
	if err != nil {
		t.Fatalf("Dial returned error: %v", err)
	}
	defer conn.Close()
	text := textproto.NewConn(conn)
	defer text.Close()
	if _, _, err := text.ReadResponse(220); err != nil {
		t.Fatalf("banner ReadResponse returned error: %v", err)
	}
	if err := rawProtocolCommand(text, 250, "EHLO client.example.net"); err != nil {
		t.Fatalf("EHLO returned error: %v", err)
	}
	if err := rawProtocolCommand(text, 250, "MAIL FROM:<sender@example.net> RET=HDRS ENVID=env+2D42"); err != nil {
		t.Fatalf("MAIL FROM with DSN options returned error: %v", err)
	}
	if err := rawProtocolCommand(text, 250, "RCPT TO:<user@example.com> NOTIFY=SUCCESS,FAILURE ORCPT=rfc822;user+40example.com"); err != nil {
		t.Fatalf("RCPT TO with DSN options returned error: %v", err)
	}
	if err := rawProtocolCommand(text, 354, "DATA"); err != nil {
		t.Fatalf("DATA returned error: %v", err)
	}
	writer := text.DotWriter()
	raw := "Message-ID: <dsn-wire@example.net>\r\nFrom: sender@example.net\r\nTo: user@example.com\r\nSubject: dsn wire\r\n\r\nbody\r\n"
	if _, err := io.WriteString(writer, raw); err != nil {
		t.Fatalf("write DATA returned error: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close DATA returned error: %v", err)
	}
	if _, msg, err := text.ReadResponse(250); err != nil {
		t.Fatalf("DATA completion returned %q, %v", msg, err)
	}
	if err := rawProtocolCommand(text, 221, "QUIT"); err != nil {
		t.Fatalf("QUIT returned error: %v", err)
	}

	if len(recorder.messages) != 1 {
		t.Fatalf("recorded messages = %d, want 1", len(recorder.messages))
	}
	got := recorder.messages[0].DSN
	if got.Return != "HDRS" || got.EnvelopeID != "env-42" {
		t.Fatalf("DSN envelope = %+v, want RET/ENVID from wire", got)
	}
	if len(got.Recipients) != 1 {
		t.Fatalf("DSN recipients = %+v, want one recipient", got.Recipients)
	}
	recipient := got.Recipients[0]
	if recipient.Address != "user@example.com" || strings.Join(recipient.Notify, ",") != "SUCCESS,FAILURE" || recipient.OriginalRecipient != "RFC822;user@example.com" {
		t.Fatalf("DSN recipient = %+v, want wire NOTIFY/ORCPT", recipient)
	}
}

func TestSMTPProtocolRejectsUnsupportedMailExtensions(t *testing.T) {
	t.Parallel()

	for _, command := range []string{
		"MAIL FROM:<sender@example.net> REQUIRETLS",
		"MAIL FROM:<sender@example.net> BODY=BINARYMIME",
	} {
		command := command
		t.Run(command, func(t *testing.T) {
			t.Parallel()

			receiver := NewReceiver(ReceiverOptions{
				Store: storage.NewLocalStore(t.TempDir()),
				Resolver: StaticResolver{
					"user@example.com": {CompanyID: "company-1", DomainID: "domain-1", UserID: "user-1", Address: "user@example.com"},
				},
			})
			addr, shutdown := startProtocolTestServer(t, receiver, ServerOptions{Domain: "mx.example.com"})
			defer shutdown()

			conn, err := net.Dial("tcp", addr)
			if err != nil {
				t.Fatalf("Dial returned error: %v", err)
			}
			defer conn.Close()
			text := textproto.NewConn(conn)
			defer text.Close()
			if _, _, err := text.ReadResponse(220); err != nil {
				t.Fatalf("banner ReadResponse returned error: %v", err)
			}
			if err := rawProtocolCommand(text, 250, "EHLO client.example.net"); err != nil {
				t.Fatalf("EHLO returned error: %v", err)
			}
			code, msg, err := rawProtocolCommandCode(text, command)
			if err != nil {
				t.Fatalf("%s returned transport error: %v", command, err)
			}
			if code < 500 || code > 599 {
				t.Fatalf("%s code = %d %q, want 5xx", command, code, msg)
			}
		})
	}
}

func TestSMTPProtocolDoesNotAdvertiseDisabledExtensions(t *testing.T) {
	t.Parallel()

	receiver := NewReceiver(ReceiverOptions{
		Store: storage.NewLocalStore(t.TempDir()),
		Resolver: StaticResolver{
			"user@example.com": {CompanyID: "company-1", DomainID: "domain-1", UserID: "user-1", Address: "user@example.com"},
		},
	})
	addr, shutdown := startProtocolTestServer(t, receiver, ServerOptions{Domain: "mx.example.com"})
	defer shutdown()

	conn, err := net.Dial("tcp", addr)
	if err != nil {
		t.Fatalf("Dial returned error: %v", err)
	}
	defer conn.Close()
	text := textproto.NewConn(conn)
	defer text.Close()
	if _, _, err := text.ReadResponse(220); err != nil {
		t.Fatalf("banner ReadResponse returned error: %v", err)
	}
	code, msg, err := rawProtocolCommandCode(text, "EHLO client.example.net")
	if err != nil {
		t.Fatalf("EHLO returned error: %v", err)
	}
	if code != 250 {
		t.Fatalf("EHLO code = %d, want 250", code)
	}
	for _, disabled := range []string{"DSN", "SMTPUTF8", "REQUIRETLS", "BINARYMIME"} {
		if strings.Contains(msg, disabled) {
			t.Fatalf("EHLO advertised disabled extension %s in:\n%s", disabled, msg)
		}
	}
	if err := rawProtocolCommand(text, 221, "QUIT"); err != nil {
		t.Fatalf("QUIT returned error: %v", err)
	}
}

func TestSMTPProtocolAdvertisesEnabledExtensions(t *testing.T) {
	t.Parallel()

	receiver := NewReceiver(ReceiverOptions{
		Store: storage.NewLocalStore(t.TempDir()),
		Resolver: StaticResolver{
			"user@example.com": {CompanyID: "company-1", DomainID: "domain-1", UserID: "user-1", Address: "user@example.com"},
		},
		SupportDSN:        true,
		SupportSMTPUTF8:   true,
		SupportRequireTLS: true,
		SupportBinaryMIME: true,
	})
	addr, shutdown := startProtocolTestServer(t, receiver, ServerOptions{
		Domain:           "mx.example.com",
		EnableDSN:        true,
		EnableSMTPUTF8:   true,
		EnableRequireTLS: true,
		EnableBinaryMIME: true,
	})
	defer shutdown()

	conn, err := net.Dial("tcp", addr)
	if err != nil {
		t.Fatalf("Dial returned error: %v", err)
	}
	defer conn.Close()
	text := textproto.NewConn(conn)
	defer text.Close()
	if _, _, err := text.ReadResponse(220); err != nil {
		t.Fatalf("banner ReadResponse returned error: %v", err)
	}
	code, msg, err := rawProtocolCommandCode(text, "EHLO client.example.net")
	if err != nil {
		t.Fatalf("EHLO returned error: %v", err)
	}
	if code != 250 {
		t.Fatalf("EHLO code = %d, want 250", code)
	}
	for _, enabled := range []string{"DSN", "SMTPUTF8", "BINARYMIME"} {
		if !strings.Contains(msg, enabled) {
			t.Fatalf("EHLO did not advertise enabled extension %s in:\n%s", enabled, msg)
		}
	}
	if err := rawProtocolCommand(text, 221, "QUIT"); err != nil {
		t.Fatalf("QUIT returned error: %v", err)
	}
}

func TestSMTPProtocolRejectsOversizedDeclaredSize(t *testing.T) {
	t.Parallel()

	receiver := NewReceiver(ReceiverOptions{
		Store:           storage.NewLocalStore(t.TempDir()),
		Resolver:        StaticResolver{"user@example.com": {CompanyID: "company-1", DomainID: "domain-1", UserID: "user-1", Address: "user@example.com"}},
		MaxMessageBytes: 10,
	})
	addr, shutdown := startProtocolTestServer(t, receiver, ServerOptions{Domain: "mx.example.com"})
	defer shutdown()

	conn, err := net.Dial("tcp", addr)
	if err != nil {
		t.Fatalf("Dial returned error: %v", err)
	}
	defer conn.Close()
	text := textproto.NewConn(conn)
	defer text.Close()
	if _, _, err := text.ReadResponse(220); err != nil {
		t.Fatalf("banner ReadResponse returned error: %v", err)
	}
	if err := rawProtocolCommand(text, 250, "EHLO client.example.net"); err != nil {
		t.Fatalf("EHLO returned error: %v", err)
	}
	code, msg, err := rawProtocolCommandCode(text, "MAIL FROM:<sender@example.net> SIZE=11")
	if err != nil {
		t.Fatalf("MAIL FROM with oversized SIZE returned transport error: %v", err)
	}
	if code != 552 {
		t.Fatalf("MAIL FROM oversized SIZE code = %d %q, want 552", code, msg)
	}
	if err := rawProtocolCommand(text, 221, "QUIT"); err != nil {
		t.Fatalf("QUIT returned error: %v", err)
	}
}

func TestSMTPProtocolImplicitTLSAcceptsMessage(t *testing.T) {
	t.Parallel()

	store := storage.NewLocalStore(t.TempDir())
	receiver := NewReceiver(ReceiverOptions{
		Store: store,
		Resolver: StaticResolver{
			"user@example.com": {CompanyID: "company-1", DomainID: "domain-1", UserID: "user-1", Address: "user@example.com"},
		},
		IDGenerator: func() string { return "protocol-smtps-id" },
		Clock:       func() time.Time { return time.Date(2026, 5, 4, 11, 0, 0, 0, time.UTC) },
	})
	addr, shutdown := startImplicitTLSProtocolTestServer(t, receiver, ServerOptions{
		Domain:    "mx.example.com",
		TLSConfig: testServerTLSConfig(t),
	})
	defer shutdown()

	conn, err := tls.Dial("tcp", addr, &tls.Config{InsecureSkipVerify: true, MinVersion: tls.VersionTLS12})
	if err != nil {
		t.Fatalf("TLS Dial returned error: %v", err)
	}
	client, err := smtp.NewClient(conn, "mx.example.com")
	if err != nil {
		t.Fatalf("NewClient returned error: %v", err)
	}
	defer client.Close()
	if err := client.Hello("client.example.net"); err != nil {
		t.Fatalf("Hello returned error: %v", err)
	}
	if err := client.Mail("sender@example.net"); err != nil {
		t.Fatalf("Mail returned error: %v", err)
	}
	if err := client.Rcpt("user@example.com"); err != nil {
		t.Fatalf("Rcpt returned error: %v", err)
	}
	writer, err := client.Data()
	if err != nil {
		t.Fatalf("Data returned error: %v", err)
	}
	raw := "Message-ID: <protocol-smtps@example.net>\r\nFrom: sender@example.net\r\nTo: user@example.com\r\nSubject: smtps\r\n\r\nbody\r\n"
	if _, err := io.WriteString(writer, raw); err != nil {
		t.Fatalf("write DATA returned error: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close DATA returned error: %v", err)
	}
	if err := client.Quit(); err != nil {
		t.Fatalf("Quit returned error: %v", err)
	}

	body, err := store.Get(context.Background(), "mailstore/company-1/domain-1/user-1/maildir/2026/05/protocol-smtps-id.eml")
	if err != nil {
		t.Fatalf("stored SMTPS message not found: %v", err)
	}
	defer body.Close()
	got, err := io.ReadAll(body)
	if err != nil {
		t.Fatalf("ReadAll returned error: %v", err)
	}
	if string(got) != raw {
		t.Fatalf("stored SMTPS message = %q, want raw protocol payload", got)
	}
}

func startImplicitTLSProtocolTestServer(t *testing.T, backend gosmtp.Backend, opts ServerOptions) (string, func()) {
	t.Helper()

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Listen returned error: %v", err)
	}
	opts.Addr = listener.Addr().String()
	opts.Backend = backend
	opts.ImplicitTLS = true
	if strings.TrimSpace(opts.Domain) == "" {
		opts.Domain = "localhost"
	}
	server := newSMTPServer(backend, opts)
	errCh := make(chan error, 1)
	go func() {
		errCh <- server.Serve(tls.NewListener(listener, server.TLSConfig))
	}()
	shutdown := func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = server.Shutdown(ctx)
		if err := <-errCh; err != nil && !strings.Contains(err.Error(), "server closed") {
			t.Fatalf("implicit TLS SMTP test server returned error: %v", err)
		}
	}
	return listener.Addr().String(), shutdown
}

func testServerTLSConfig(t *testing.T) *tls.Config {
	t.Helper()

	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("GenerateKey returned error: %v", err)
	}
	template := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "mx.example.com"},
		DNSNames:     []string{"mx.example.com"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Hour),
		KeyUsage:     x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}
	certDER, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		t.Fatalf("CreateCertificate returned error: %v", err)
	}
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER})
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)})
	cert, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		t.Fatalf("X509KeyPair returned error: %v", err)
	}
	return &tls.Config{Certificates: []tls.Certificate{cert}, MinVersion: tls.VersionTLS12}
}

func rawProtocolCommand(text *textproto.Conn, expect int, command string) error {
	_, _, err := rawProtocolCommandCodeExpect(text, expect, command)
	return err
}

func rawProtocolCommandCode(text *textproto.Conn, command string) (int, string, error) {
	return rawProtocolCommandCodeExpect(text, -1, command)
}

func rawProtocolCommandCodeExpect(text *textproto.Conn, expect int, command string) (int, string, error) {
	id, err := text.Cmd("%s", command)
	if err != nil {
		return 0, "", err
	}
	text.StartResponse(id)
	defer text.EndResponse(id)
	return text.ReadResponse(expect)
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
