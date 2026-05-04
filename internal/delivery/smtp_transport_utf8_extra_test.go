package delivery

import (
	"bufio"
	"errors"
	"fmt"
	"net"
	"net/smtp"
	"strings"
	"testing"

	"github.com/gogomail/gogomail/internal/outbound"
)

func TestJobNeedsUTF8FalseForASCIIAddresses(t *testing.T) {
	t.Parallel()

	job := Job{QueuedMessage: QueuedMessage{
		Event:     "mail.queued",
		MessageID: "test-id",
		From:      outbound.Address{Email: "sender@example.com"},
		To:        []outbound.Address{{Email: "recipient@example.net"}},
	}}
	if jobNeedsUTF8(job) {
		t.Error("jobNeedsUTF8 returned true for pure-ASCII addresses")
	}
}

func TestJobNeedsUTF8TrueForInternationalizedSender(t *testing.T) {
	t.Parallel()

	job := Job{QueuedMessage: QueuedMessage{
		Event:     "mail.queued",
		MessageID: "test-id",
		From:      outbound.Address{Email: "발신자@example.com"},
		To:        []outbound.Address{{Email: "recipient@example.net"}},
	}}
	if !jobNeedsUTF8(job) {
		t.Error("jobNeedsUTF8 returned false for non-ASCII sender")
	}
}

func TestJobNeedsUTF8TrueForInternationalizedRecipient(t *testing.T) {
	t.Parallel()

	job := Job{QueuedMessage: QueuedMessage{
		Event:     "mail.queued",
		MessageID: "test-id",
		From:      outbound.Address{Email: "sender@example.com"},
		To:        []outbound.Address{{Email: "수신자@example.net"}},
	}}
	if !jobNeedsUTF8(job) {
		t.Error("jobNeedsUTF8 returned false for non-ASCII recipient")
	}
}

func TestContainsNonASCIIByte(t *testing.T) {
	t.Parallel()

	cases := []struct {
		input string
		want  bool
	}{
		{"", false},
		{"ascii@example.com", false},
		{"한글@example.com", true},
		{"user@xn--p1ai", false},
	}
	for _, tc := range cases {
		got := containsNonASCIIByte(tc.input)
		if got != tc.want {
			t.Errorf("containsNonASCIIByte(%q) = %v, want %v", tc.input, got, tc.want)
		}
	}
}

func TestSMTPMailRejectsUTF8WhenPeerDoesNotAdvertiseSMTPUTF8(t *testing.T) {
	t.Parallel()

	client, commandSeen, cleanup := smtpClientWithExtensions(t, "250 mx.example.net\r\n")
	defer cleanup()

	job := Job{QueuedMessage: QueuedMessage{
		From: outbound.Address{Email: "발신자@example.com"},
		To:   []outbound.Address{{Email: "recipient@example.net"}},
	}}
	err := smtpMail(client, job)
	if err == nil {
		t.Fatal("smtpMail accepted UTF8 sender without SMTPUTF8 extension")
	}
	var smtpErr *SMTPStatusError
	if !errors.As(err, &smtpErr) || smtpErr.Code != 553 || !strings.Contains(smtpErr.Message, "5.6.7") {
		t.Fatalf("error = %#v, want SMTP 553 5.6.7", err)
	}
	select {
	case line := <-commandSeen:
		t.Fatalf("server saw unexpected command %q", line)
	default:
	}
}

func TestSMTPRcptRejectsUTF8WhenPeerDoesNotAdvertiseSMTPUTF8(t *testing.T) {
	t.Parallel()

	client, commandSeen, cleanup := smtpClientWithExtensions(t, "250 mx.example.net\r\n")
	defer cleanup()

	job := Job{QueuedMessage: QueuedMessage{
		From: outbound.Address{Email: "sender@example.com"},
	}}
	err := smtpRcpt(client, job, outbound.Address{Email: "수신자@example.net"})
	if err == nil {
		t.Fatal("smtpRcpt accepted UTF8 recipient without SMTPUTF8 extension")
	}
	var smtpErr *SMTPStatusError
	if !errors.As(err, &smtpErr) || smtpErr.Code != 553 || !strings.Contains(smtpErr.Message, "5.6.7") {
		t.Fatalf("error = %#v, want SMTP 553 5.6.7", err)
	}
	select {
	case line := <-commandSeen:
		t.Fatalf("server saw unexpected command %q", line)
	default:
	}
}

func TestSMTPMailAddsSMTPUTF8WhenPeerAdvertisesExtension(t *testing.T) {
	t.Parallel()

	client, commandSeen, cleanup := smtpClientWithExtensions(t, "250-mx.example.net\r\n250 SMTPUTF8\r\n")
	defer cleanup()

	job := Job{QueuedMessage: QueuedMessage{
		From: outbound.Address{Email: "발신자@example.com"},
		To:   []outbound.Address{{Email: "recipient@example.net"}},
	}}
	if err := smtpMail(client, job); err != nil {
		t.Fatalf("smtpMail returned error: %v", err)
	}
	line := <-commandSeen
	if !strings.Contains(line, "SMTPUTF8") {
		t.Fatalf("MAIL line = %q, want SMTPUTF8", line)
	}
}

func smtpClientWithExtensions(t *testing.T, ehloResponse string) (*smtp.Client, <-chan string, func()) {
	t.Helper()

	clientConn, serverConn := net.Pipe()
	commandSeen := make(chan string, 1)
	errs := make(chan error, 1)
	go func() {
		reader := bufio.NewReader(serverConn)
		if _, err := fmt.Fprintf(serverConn, "220 mx.example.net ESMTP\r\n"); err != nil {
			errs <- err
			return
		}
		if line, err := reader.ReadString('\n'); err != nil || !strings.HasPrefix(line, "EHLO ") {
			errs <- fmt.Errorf("EHLO line = %q, err = %v", line, err)
			return
		}
		if _, err := fmt.Fprint(serverConn, ehloResponse); err != nil {
			errs <- err
			return
		}
		line, err := reader.ReadString('\n')
		if err != nil {
			errs <- nil
			return
		}
		commandSeen <- strings.TrimRight(line, "\r\n")
		_, _ = fmt.Fprintf(serverConn, "250 ok\r\n")
		errs <- nil
	}()

	client, err := smtp.NewClient(clientConn, "mx.example.net")
	if err != nil {
		t.Fatalf("NewClient returned error: %v", err)
	}
	if err := client.Hello("sender.example.com"); err != nil {
		t.Fatalf("Hello returned error: %v", err)
	}
	cleanup := func() {
		_ = client.Close()
		_ = clientConn.Close()
		_ = serverConn.Close()
		if err := <-errs; err != nil {
			t.Fatalf("fake SMTP server error: %v", err)
		}
	}
	return client, commandSeen, cleanup
}
