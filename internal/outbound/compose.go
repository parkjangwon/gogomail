package outbound

import (
	"bytes"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"mime/quotedprintable"
	"net/mail"
	"net/textproto"
	"strings"
	"time"
	"unicode/utf8"
)

const maxHeaderLineBytes = 998

type Address struct {
	Name  string
	Email string
}

type TextMessage struct {
	From        Address
	To          []Address
	Cc          []Address
	Bcc         []Address
	Subject     string
	TextBody    string
	HTMLBody    string
	Attachments []Attachment
	MessageID   string
	InReplyTo   string
	References  []string
	Date        time.Time
}

type Attachment struct {
	Filename string
	MIMEType string
	Open     func() (io.ReadCloser, error)
}

type ComposedMessage struct {
	Raw       []byte
	MessageID string
	Size      int64
}

func ComposeText(msg TextMessage) (ComposedMessage, error) {
	var buf bytes.Buffer
	composed, err := ComposeTextToWriter(&buf, msg)
	if err != nil {
		return ComposedMessage{}, err
	}
	composed.Raw = append([]byte(nil), buf.Bytes()...)
	return composed, nil
}

func ComposeTextToWriter(w io.Writer, msg TextMessage) (ComposedMessage, error) {
	if strings.TrimSpace(msg.From.Email) == "" {
		return ComposedMessage{}, fmt.Errorf("from address is required")
	}
	if err := validateAddress(msg.From); err != nil {
		return ComposedMessage{}, fmt.Errorf("invalid from address: %w", err)
	}
	if len(msg.To)+len(msg.Cc)+len(msg.Bcc) == 0 {
		return ComposedMessage{}, fmt.Errorf("at least one recipient is required")
	}
	if err := validateAddresses("to", msg.To); err != nil {
		return ComposedMessage{}, err
	}
	if err := validateAddresses("cc", msg.Cc); err != nil {
		return ComposedMessage{}, err
	}
	if err := validateAddresses("bcc", msg.Bcc); err != nil {
		return ComposedMessage{}, err
	}
	if err := validateHeaderValue("subject", msg.Subject); err != nil {
		return ComposedMessage{}, err
	}
	if err := validateHeaderValue("message_id", msg.MessageID); err != nil {
		return ComposedMessage{}, err
	}
	if msg.Date.IsZero() {
		msg.Date = time.Now().UTC()
	} else {
		msg.Date = msg.Date.UTC()
	}
	if strings.TrimSpace(msg.MessageID) == "" {
		msg.MessageID = GenerateMessageID(domainFromAddress(msg.From.Email))
	} else {
		normalized := normalizeHeaderMessageID(msg.MessageID)
		if normalized == "" {
			return ComposedMessage{}, fmt.Errorf("message_id must be a valid RFC 5322 message id")
		}
		msg.MessageID = normalized
	}

	cw := &countingWriter{w: w}
	if err := writeHeader(cw, "From", formatAddress(msg.From)); err != nil {
		return ComposedMessage{}, err
	}
	if err := writeHeader(cw, "To", formatAddresses(msg.To)); err != nil {
		return ComposedMessage{}, err
	}
	if len(msg.Cc) > 0 {
		if err := writeHeader(cw, "Cc", formatAddresses(msg.Cc)); err != nil {
			return ComposedMessage{}, err
		}
	}
	if err := writeHeader(cw, "Subject", mime.QEncoding.Encode("utf-8", msg.Subject)); err != nil {
		return ComposedMessage{}, err
	}
	if err := writeHeader(cw, "Date", msg.Date.Format(time.RFC1123Z)); err != nil {
		return ComposedMessage{}, err
	}
	if err := writeHeader(cw, "Message-ID", msg.MessageID); err != nil {
		return ComposedMessage{}, err
	}
	if inReplyTo := normalizeHeaderMessageID(msg.InReplyTo); inReplyTo != "" {
		if err := writeHeader(cw, "In-Reply-To", inReplyTo); err != nil {
			return ComposedMessage{}, err
		}
	}
	if references := formatReferencesHeader(msg.References); references != "" {
		if err := writeHeader(cw, "References", references); err != nil {
			return ComposedMessage{}, err
		}
	}
	if err := writeHeader(cw, "MIME-Version", "1.0"); err != nil {
		return ComposedMessage{}, err
	}

	if len(msg.Attachments) > 0 {
		if err := writeMultipartMixed(cw, msg); err != nil {
			return ComposedMessage{}, err
		}
	} else if msg.HTMLBody != "" {
		if err := writeAlternativeBody(cw, msg.TextBody, msg.HTMLBody); err != nil {
			return ComposedMessage{}, err
		}
	} else {
		if err := writePlainTextBody(cw, msg.TextBody); err != nil {
			return ComposedMessage{}, err
		}
	}

	return ComposedMessage{
		MessageID: msg.MessageID,
		Size:      cw.n,
	}, nil
}

func writeMultipartMixed(w io.Writer, msg TextMessage) error {
	mw := multipart.NewWriter(w)
	if err := writeHeader(w, "Content-Type", `multipart/mixed; boundary="`+mw.Boundary()+`"`); err != nil {
		return err
	}
	if _, err := io.WriteString(w, "\r\n"); err != nil {
		return fmt.Errorf("write multipart body separator: %w", err)
	}

	if err := writeMessageBodyPart(mw, msg); err != nil {
		return err
	}
	for _, attachment := range msg.Attachments {
		if err := writeAttachmentPart(mw, attachment); err != nil {
			return err
		}
	}
	if err := mw.Close(); err != nil {
		return fmt.Errorf("close multipart writer: %w", err)
	}
	return nil
}

func writePlainTextBody(w io.Writer, textBody string) error {
	if err := writeHeader(w, "Content-Type", `text/plain; charset="utf-8"`); err != nil {
		return err
	}
	if err := writeHeader(w, "Content-Transfer-Encoding", "quoted-printable"); err != nil {
		return err
	}
	if _, err := io.WriteString(w, "\r\n"); err != nil {
		return fmt.Errorf("write body separator: %w", err)
	}

	body := quotedprintable.NewWriter(w)
	if _, err := body.Write([]byte(normalizeCRLF(textBody))); err != nil {
		return fmt.Errorf("write quoted-printable body: %w", err)
	}
	if err := body.Close(); err != nil {
		return fmt.Errorf("close quoted-printable body: %w", err)
	}
	if _, err := io.WriteString(w, "\r\n"); err != nil {
		return fmt.Errorf("write body terminator: %w", err)
	}
	return nil
}

func writeMessageBodyPart(mw *multipart.Writer, msg TextMessage) error {
	if msg.HTMLBody != "" {
		return writeAlternativeMultipartPart(mw, msg.TextBody, msg.HTMLBody)
	}
	return writeTextBodyPart(mw, "text/plain", normalizeCRLF(msg.TextBody))
}

func writeAlternativeBody(w io.Writer, textBody string, htmlBody string) error {
	mw := multipart.NewWriter(w)
	if err := writeHeader(w, "Content-Type", `multipart/alternative; boundary="`+mw.Boundary()+`"`); err != nil {
		return err
	}
	if _, err := io.WriteString(w, "\r\n"); err != nil {
		return fmt.Errorf("write multipart body separator: %w", err)
	}
	if err := writeTextBodyPart(mw, "text/plain", normalizeCRLF(textBody)); err != nil {
		return err
	}
	if err := writeTextBodyPart(mw, "text/html", normalizeCRLF(htmlBody)); err != nil {
		return err
	}
	if err := mw.Close(); err != nil {
		return fmt.Errorf("close multipart writer: %w", err)
	}
	return nil
}

func writeAlternativeMultipartPart(mw *multipart.Writer, textBody string, htmlBody string) error {
	altHeader := make(textproto.MIMEHeader)
	boundaryWriter := multipart.NewWriter(io.Discard)
	boundary := boundaryWriter.Boundary()
	altHeader.Set("Content-Type", `multipart/alternative; boundary="`+boundary+`"`)
	altPart, err := mw.CreatePart(altHeader)
	if err != nil {
		return fmt.Errorf("create multipart/alternative part: %w", err)
	}
	nested := multipart.NewWriter(altPart)
	if err := nested.SetBoundary(boundary); err != nil {
		return fmt.Errorf("set multipart/alternative boundary: %w", err)
	}
	if err := writeTextBodyPart(nested, "text/plain", normalizeCRLF(textBody)); err != nil {
		return err
	}
	if err := writeTextBodyPart(nested, "text/html", normalizeCRLF(htmlBody)); err != nil {
		return err
	}
	if err := nested.Close(); err != nil {
		return fmt.Errorf("close multipart/alternative writer: %w", err)
	}
	return nil
}

func writeTextBodyPart(mw *multipart.Writer, mediaType string, body string) error {
	header := make(textproto.MIMEHeader)
	header.Set("Content-Type", mediaType+`; charset="utf-8"`)
	header.Set("Content-Transfer-Encoding", "quoted-printable")
	part, err := mw.CreatePart(header)
	if err != nil {
		return fmt.Errorf("create %s part: %w", mediaType, err)
	}
	qp := quotedprintable.NewWriter(part)
	if _, err := qp.Write([]byte(body)); err != nil {
		return fmt.Errorf("write %s quoted-printable: %w", mediaType, err)
	}
	if err := qp.Close(); err != nil {
		return fmt.Errorf("close %s quoted-printable: %w", mediaType, err)
	}
	return nil
}

func writeAttachmentPart(mw *multipart.Writer, attachment Attachment) error {
	if attachment.Open == nil {
		return fmt.Errorf("attachment content opener is required")
	}
	content, err := attachment.Open()
	if err != nil {
		return fmt.Errorf("open attachment content: %w", err)
	}
	defer content.Close()
	contentType := strings.TrimSpace(attachment.MIMEType)
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	filename := strings.TrimSpace(attachment.Filename)
	if filename == "" {
		filename = "attachment"
	}
	header := make(textproto.MIMEHeader)
	if formatted := mime.FormatMediaType(contentType, map[string]string{"name": filename}); formatted != "" {
		header.Set("Content-Type", formatted)
	} else {
		header.Set("Content-Type", contentType)
	}
	if formatted := mime.FormatMediaType("attachment", map[string]string{"filename": filename}); formatted != "" {
		header.Set("Content-Disposition", formatted)
	} else {
		header.Set("Content-Disposition", "attachment; filename=\""+strings.ReplaceAll(filename, "\"", "\\\"")+"\"")
	}
	header.Set("Content-Transfer-Encoding", "base64")
	part, err := mw.CreatePart(header)
	if err != nil {
		return fmt.Errorf("create attachment part: %w", err)
	}
	if err := writeBase64MIME(part, content); err != nil {
		return err
	}
	return nil
}

func writeBase64MIME(w io.Writer, src io.Reader) error {
	if src == nil {
		return fmt.Errorf("attachment content is required")
	}
	var input [57]byte
	var output [76]byte
	for {
		n, readErr := src.Read(input[:])
		if n > 0 {
			encodedLen := base64.StdEncoding.EncodedLen(n)
			base64.StdEncoding.Encode(output[:encodedLen], input[:n])
			if _, err := w.Write(output[:encodedLen]); err != nil {
				return fmt.Errorf("write base64 body: %w", err)
			}
			if _, err := io.WriteString(w, "\r\n"); err != nil {
				return fmt.Errorf("write base64 line break: %w", err)
			}
		}
		if readErr == io.EOF {
			return nil
		}
		if readErr != nil {
			return fmt.Errorf("read attachment content: %w", readErr)
		}
	}
}

func normalizeHeaderMessageID(value string) string {
	value = strings.TrimSpace(value)
	if value == "" || strings.ContainsAny(value, "\r\n") {
		return ""
	}
	if strings.HasPrefix(value, "<") || strings.HasSuffix(value, ">") {
		if !strings.HasPrefix(value, "<") || !strings.HasSuffix(value, ">") {
			return ""
		}
		value = strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(value, "<"), ">"))
	}
	if value == "" || strings.ContainsAny(value, " \t<>") || strings.Count(value, "@") != 1 {
		return ""
	}
	local, domain, _ := strings.Cut(value, "@")
	if local == "" || domain == "" {
		return ""
	}
	return "<" + value + ">"
}

func formatReferencesHeader(values []string) string {
	seen := make(map[string]struct{}, len(values))
	refs := make([]string, 0, len(values))
	for _, value := range values {
		value = normalizeHeaderMessageID(value)
		if value == "" {
			continue
		}
		key := strings.ToLower(value)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		refs = append(refs, value)
	}
	return strings.Join(refs, " ")
}

func GenerateMessageID(domain string) string {
	domain = strings.TrimSpace(domain)
	if domain == "" {
		domain = "localhost"
	}
	var random [16]byte
	if _, err := rand.Read(random[:]); err != nil {
		return fmt.Sprintf("<%d@%s>", time.Now().UnixNano(), domain)
	}
	return fmt.Sprintf("<%d.%s@%s>", time.Now().UnixNano(), hex.EncodeToString(random[:]), domain)
}

type countingWriter struct {
	w io.Writer
	n int64
}

func (w *countingWriter) Write(p []byte) (int, error) {
	n, err := w.w.Write(p)
	w.n += int64(n)
	return n, err
}

func writeHeader(w io.Writer, key string, value string) error {
	prefix := key + ": "
	if _, err := io.WriteString(w, prefix); err != nil {
		return err
	}
	if err := writeFoldedHeaderValue(w, value, len(prefix)); err != nil {
		return err
	}
	if _, err := io.WriteString(w, "\r\n"); err != nil {
		return err
	}
	return nil
}

func writeFoldedHeaderValue(w io.Writer, value string, currentLineBytes int) error {
	for len(value) > 0 {
		remaining := maxHeaderLineBytes - currentLineBytes
		if remaining <= 0 {
			if _, err := io.WriteString(w, "\r\n\t"); err != nil {
				return err
			}
			currentLineBytes = 1
			continue
		}
		if len(value) <= remaining {
			_, err := io.WriteString(w, value)
			return err
		}
		splitAt := headerFoldSplit(value, remaining)
		chunk := strings.TrimRight(value[:splitAt], " ")
		if _, err := io.WriteString(w, chunk); err != nil {
			return err
		}
		if _, err := io.WriteString(w, "\r\n\t"); err != nil {
			return err
		}
		value = strings.TrimLeft(value[splitAt:], " ")
		currentLineBytes = 1
	}
	return nil
}

func headerFoldSplit(value string, limit int) int {
	if limit >= len(value) {
		return len(value)
	}
	window := value[:limit]
	if idx := strings.LastIndex(window, ","); idx > 0 {
		return idx + 1
	}
	if idx := strings.LastIndex(window, " "); idx > 0 {
		return idx
	}
	for limit > 0 && !utf8.ValidString(value[:limit]) {
		_, size := utf8.DecodeLastRuneInString(value[:limit])
		limit -= size
	}
	if limit <= 0 {
		return len([]byte(string([]rune(value)[0])))
	}
	return limit
}

func formatAddress(addr Address) string {
	return (&mail.Address{Name: addr.Name, Address: strings.TrimSpace(addr.Email)}).String()
}

func validateAddress(addr Address) error {
	if strings.TrimSpace(addr.Email) == "" {
		return fmt.Errorf("email is required")
	}
	if err := validateHeaderValue("display name", addr.Name); err != nil {
		return err
	}
	if err := validateHeaderValue("email", addr.Email); err != nil {
		return err
	}
	if _, err := mail.ParseAddress(formatAddress(addr)); err != nil {
		return err
	}
	return nil
}

func validateHeaderValue(field string, value string) error {
	if strings.ContainsAny(value, "\r\n") {
		return fmt.Errorf("%s must not contain CR or LF", field)
	}
	return nil
}

func validateAddresses(field string, addrs []Address) error {
	for _, addr := range addrs {
		if err := validateAddress(addr); err != nil {
			return fmt.Errorf("invalid %s address %q: %w", field, addr.Email, err)
		}
	}
	return nil
}

func formatAddresses(addrs []Address) string {
	formatted := make([]string, 0, len(addrs))
	for _, addr := range addrs {
		formatted = append(formatted, formatAddress(addr))
	}
	return strings.Join(formatted, ", ")
}

func domainFromAddress(address string) string {
	_, domain, ok := strings.Cut(strings.TrimSpace(address), "@")
	if !ok {
		return "localhost"
	}
	return domain
}

func normalizeCRLF(value string) string {
	value = strings.ReplaceAll(value, "\r\n", "\n")
	value = strings.ReplaceAll(value, "\r", "\n")
	return strings.ReplaceAll(value, "\n", "\r\n")
}
