package message

import (
	"strings"
	"testing"
)

func TestParseMIMEStructureReadsMultipartMetadata(t *testing.T) {
	t.Parallel()

	raw := strings.Join([]string{
		"Subject: MIME",
		"Content-Type: multipart/mixed; boundary=frontier",
		"",
		"--frontier",
		"Content-Type: text/plain; charset=utf-8",
		"Content-Transfer-Encoding: quoted-printable",
		"",
		"hello",
		"world",
		"--frontier",
		"Content-Type: application/pdf; name=\"report.pdf\"",
		"Content-Transfer-Encoding: base64",
		"Content-Disposition: attachment; filename=\"report.pdf\"",
		"",
		"UEZGREFUQQ==",
		"--frontier--",
		"",
	}, "\r\n")

	parsed, err := ParseMIMEStructure(strings.NewReader(raw), MIMEStructureOptions{})
	if err != nil {
		t.Fatalf("ParseMIMEStructure returned error: %v", err)
	}
	if parsed.PartsTruncated {
		t.Fatal("PartsTruncated = true, want false")
	}
	root := parsed.Root
	if root.MediaType != "MULTIPART" || root.MediaSubtype != "MIXED" || root.Params["boundary"] != "frontier" {
		t.Fatalf("root = %+v, want multipart/mixed boundary", root)
	}
	if len(root.Parts) != 2 {
		t.Fatalf("root parts = %d, want 2", len(root.Parts))
	}
	text := root.Parts[0]
	if text.MediaType != "TEXT" || text.MediaSubtype != "PLAIN" || text.Params["charset"] != "utf-8" || text.Encoding != "QUOTED-PRINTABLE" {
		t.Fatalf("text part = %+v", text)
	}
	if text.Size != int64(len("hello\r\nworld")) || text.Lines != 2 {
		t.Fatalf("text size/lines = %d/%d, want body bytes and two lines", text.Size, text.Lines)
	}
	attachment := root.Parts[1]
	if attachment.MediaType != "APPLICATION" || attachment.MediaSubtype != "PDF" || attachment.Encoding != "BASE64" {
		t.Fatalf("attachment part = %+v", attachment)
	}
	if attachment.Disposition != "ATTACHMENT" || attachment.DispositionParams["filename"] != "report.pdf" || attachment.Params["name"] != "report.pdf" {
		t.Fatalf("attachment metadata = %+v", attachment)
	}
	if attachment.Size != int64(len("UEZGREFUQQ==")) {
		t.Fatalf("attachment size = %d, want encoded body size", attachment.Size)
	}
	if mimePartRetainsPayload(root, "UEZGREFUQQ==") {
		t.Fatal("MIME structure retained attachment payload")
	}
}

func TestParseMIMEStructureReadsNestedMultipartOrder(t *testing.T) {
	t.Parallel()

	raw := strings.Join([]string{
		"Content-Type: multipart/mixed; boundary=mixed",
		"",
		"--mixed",
		"Content-Type: multipart/alternative; boundary=alt",
		"",
		"--alt",
		"Content-Type: text/plain",
		"",
		"plain",
		"--alt",
		"Content-Type: text/html; charset=utf-8",
		"",
		"<p>html</p>",
		"--alt--",
		"--mixed",
		"Content-Type: image/png",
		"Content-Disposition: inline",
		"",
		"PNGDATA",
		"--mixed--",
		"",
	}, "\r\n")

	parsed, err := ParseMIMEStructure(strings.NewReader(raw), MIMEStructureOptions{})
	if err != nil {
		t.Fatalf("ParseMIMEStructure returned error: %v", err)
	}
	if len(parsed.Root.Parts) != 2 {
		t.Fatalf("root parts = %d, want 2", len(parsed.Root.Parts))
	}
	alternative := parsed.Root.Parts[0]
	if alternative.MediaType != "MULTIPART" || alternative.MediaSubtype != "ALTERNATIVE" {
		t.Fatalf("first child = %+v, want multipart/alternative", alternative)
	}
	if len(alternative.Parts) != 2 || alternative.Parts[0].MediaSubtype != "PLAIN" || alternative.Parts[1].MediaSubtype != "HTML" {
		t.Fatalf("alternative children = %+v", alternative.Parts)
	}
	image := parsed.Root.Parts[1]
	if image.MediaType != "IMAGE" || image.MediaSubtype != "PNG" || image.Disposition != "INLINE" {
		t.Fatalf("image child = %+v", image)
	}
}

func TestParseMIMEStructureCountsMessageRFC822Lines(t *testing.T) {
	t.Parallel()

	raw := strings.Join([]string{
		"Content-Type: message/rfc822",
		"Content-Transfer-Encoding: 7bit",
		"",
		"Subject: Nested",
		"",
		"line one",
		"line two",
	}, "\r\n")

	parsed, err := ParseMIMEStructure(strings.NewReader(raw), MIMEStructureOptions{})
	if err != nil {
		t.Fatalf("ParseMIMEStructure returned error: %v", err)
	}
	root := parsed.Root
	if root.MediaType != "MESSAGE" || root.MediaSubtype != "RFC822" {
		t.Fatalf("root = %+v, want message/rfc822", root)
	}
	body := "Subject: Nested\r\n\r\nline one\r\nline two"
	if root.Size != int64(len(body)) || root.Lines != 4 {
		t.Fatalf("message/rfc822 size/lines = %d/%d, want %d/4", root.Size, root.Lines, len(body))
	}
	if len(root.Parts) != 1 {
		t.Fatalf("message/rfc822 child parts = %d, want 1", len(root.Parts))
	}
	child := root.Parts[0]
	if child.MediaType != "TEXT" || child.MediaSubtype != "PLAIN" {
		t.Fatalf("message/rfc822 child = %+v, want default text/plain", child)
	}
	if child.Size != int64(len("line one\r\nline two")) || child.Lines != 2 {
		t.Fatalf("message/rfc822 child size/lines = %d/%d, want nested body bytes and two lines", child.Size, child.Lines)
	}
	if root.Envelope.Subject != "Nested" {
		t.Fatalf("message/rfc822 envelope subject = %q, want Nested", root.Envelope.Subject)
	}
}

func TestParseMIMEStructureLimitsPartCount(t *testing.T) {
	t.Parallel()

	raw := strings.Join([]string{
		"Content-Type: multipart/mixed; boundary=frontier",
		"",
		"--frontier",
		"Content-Type: text/plain",
		"",
		"one",
		"--frontier",
		"Content-Type: text/plain",
		"",
		"two",
		"--frontier--",
		"",
	}, "\r\n")
	parsed, err := ParseMIMEStructure(strings.NewReader(raw), MIMEStructureOptions{MaxParts: 2})
	if err != nil {
		t.Fatalf("ParseMIMEStructure returned error: %v", err)
	}
	if !parsed.PartsTruncated {
		t.Fatal("PartsTruncated = false, want true")
	}
	if len(parsed.Root.Parts) != 1 {
		t.Fatalf("retained child count = %d, want one child before limit", len(parsed.Root.Parts))
	}
}

func mimePartRetainsPayload(part MIMEPart, payload string) bool {
	if payload == "" {
		return false
	}
	if strings.Contains(part.ContentID, payload) || strings.Contains(part.Description, payload) {
		return true
	}
	for _, value := range part.Params {
		if strings.Contains(value, payload) {
			return true
		}
	}
	for _, value := range part.DispositionParams {
		if strings.Contains(value, payload) {
			return true
		}
	}
	for _, child := range part.Parts {
		if mimePartRetainsPayload(child, payload) {
			return true
		}
	}
	return false
}
