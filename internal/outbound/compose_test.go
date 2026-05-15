package outbound

import (
	"bytes"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/gogomail/gogomail/internal/message"
)

func TestComposeTextBuildsParseableRFC5322Message(t *testing.T) {
	t.Parallel()

	composed, err := ComposeText(TextMessage{
		From:     Address{Name: "Sender", Email: "sender@example.com"},
		To:       []Address{{Name: "User", Email: "user@example.com"}},
		Subject:  "hello 안녕",
		TextBody: "line1\nline2",
		Date:     time.Date(2026, 5, 3, 9, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatalf("ComposeText returned error: %v", err)
	}
	if !bytes.Contains(composed.Raw, []byte("\r\n")) {
		t.Fatal("composed message does not use CRLF line endings")
	}
	if !strings.HasPrefix(composed.MessageID, "<") || !strings.HasSuffix(composed.MessageID, ">") {
		t.Fatalf("MessageID = %q, want angle-address form", composed.MessageID)
	}

	parsed, err := message.ParseEML(bytes.NewReader(composed.Raw))
	if err != nil {
		t.Fatalf("ParseEML returned error: %v", err)
	}
	if parsed.Subject != "hello 안녕" {
		t.Fatalf("Subject = %q", parsed.Subject)
	}
	if !strings.Contains(parsed.TextBody, "line1") || !strings.Contains(parsed.TextBody, "line2") {
		t.Fatalf("TextBody = %q", parsed.TextBody)
	}
}

func TestComposeTextBuildsMultipartMixedWithAlternativeAndAttachments(t *testing.T) {
	t.Parallel()

	composed, err := ComposeText(TextMessage{
		From:     Address{Name: "Sender", Email: "sender@example.com"},
		To:       []Address{{Email: "user@example.com"}},
		Subject:  "hello",
		TextBody: "plain body",
		HTMLBody: "<p>rich body</p>",
		Attachments: []Attachment{{
			Filename: "report.pdf",
			MIMEType: "application/pdf",
			Open: func() (io.ReadCloser, error) {
				return io.NopCloser(strings.NewReader("PDFDATA")), nil
			},
		}},
	})
	if err != nil {
		t.Fatalf("ComposeText returned error: %v", err)
	}

	parsed, err := message.ParseEML(bytes.NewReader(composed.Raw))
	if err != nil {
		t.Fatalf("ParseEML returned error: %v", err)
	}
	if parsed.HTMLBody != "<p>rich body</p>" {
		t.Fatalf("HTMLBody = %q", parsed.HTMLBody)
	}
	if !parsed.HasAttachment || len(parsed.Attachments) != 1 || parsed.Attachments[0].Filename != "report.pdf" {
		t.Fatalf("Attachments = %+v", parsed.Attachments)
	}

	structure, err := message.ParseMIMEStructure(bytes.NewReader(composed.Raw), message.MIMEStructureOptions{})
	if err != nil {
		t.Fatalf("ParseMIMEStructure returned error: %v", err)
	}
	if structure.Root.MediaType != "MULTIPART" || structure.Root.MediaSubtype != "MIXED" {
		t.Fatalf("root = %+v, want multipart/mixed", structure.Root)
	}
	if len(structure.Root.Parts) != 2 {
		t.Fatalf("root parts = %d, want 2", len(structure.Root.Parts))
	}
	if structure.Root.Parts[0].MediaType != "MULTIPART" || structure.Root.Parts[0].MediaSubtype != "ALTERNATIVE" {
		t.Fatalf("body part = %+v, want multipart/alternative", structure.Root.Parts[0])
	}
	if len(structure.Root.Parts[0].Parts) != 2 || structure.Root.Parts[0].Parts[1].MediaSubtype != "HTML" {
		t.Fatalf("alternative parts = %+v", structure.Root.Parts[0].Parts)
	}
	if structure.Root.Parts[1].Disposition != "ATTACHMENT" || structure.Root.Parts[1].DispositionParams["filename"] != "report.pdf" {
		t.Fatalf("attachment part = %+v", structure.Root.Parts[1])
	}
}

func TestComposeTextRejectsMissingRecipients(t *testing.T) {
	t.Parallel()

	_, err := ComposeText(TextMessage{
		From:    Address{Email: "sender@example.com"},
		Subject: "hello",
	})
	if err == nil {
		t.Fatal("ComposeText accepted missing recipients")
	}
}

func TestComposeTextRejectsInvalidRecipientAddress(t *testing.T) {
	t.Parallel()

	_, err := ComposeText(TextMessage{
		From:     Address{Email: "sender@example.com"},
		To:       []Address{{Email: "not an address"}},
		Subject:  "hello",
		TextBody: "body",
	})
	if err == nil {
		t.Fatal("ComposeText accepted invalid recipient address")
	}
}

func TestComposeTextRejectsHeaderInjectionValues(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		msg  TextMessage
	}{
		{
			name: "subject",
			msg: TextMessage{
				From:    Address{Email: "sender@example.com"},
				To:      []Address{{Email: "user@example.com"}},
				Subject: "hello\r\nBcc: victim@example.net",
			},
		},
		{
			name: "display name",
			msg: TextMessage{
				From: Address{Name: "Sender\nInjected: yes", Email: "sender@example.com"},
				To:   []Address{{Email: "user@example.com"}},
			},
		},
		{
			name: "message id",
			msg: TextMessage{
				From:      Address{Email: "sender@example.com"},
				To:        []Address{{Email: "user@example.com"}},
				MessageID: "safe@example.com\r\nX-Injected: yes",
			},
		},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if _, err := ComposeText(tt.msg); err == nil {
				t.Fatal("ComposeText accepted newline-bearing header value")
			}
		})
	}
}
