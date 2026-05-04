package outbound

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"mime"
	"mime/quotedprintable"
	"net/mail"
	"strings"
	"time"
)

type Address struct {
	Name  string
	Email string
}

type TextMessage struct {
	From       Address
	To         []Address
	Cc         []Address
	Bcc        []Address
	Subject    string
	TextBody   string
	MessageID  string
	InReplyTo  string
	References []string
	Date       time.Time
}

type ComposedMessage struct {
	Raw       []byte
	MessageID string
	Size      int64
}

func ComposeText(msg TextMessage) (ComposedMessage, error) {
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
	}
	if !strings.HasPrefix(msg.MessageID, "<") {
		msg.MessageID = "<" + msg.MessageID + ">"
	}
	if !strings.HasSuffix(msg.MessageID, ">") {
		msg.MessageID += ">"
	}

	var buf bytes.Buffer
	writeHeader(&buf, "From", formatAddress(msg.From))
	writeHeader(&buf, "To", formatAddresses(msg.To))
	if len(msg.Cc) > 0 {
		writeHeader(&buf, "Cc", formatAddresses(msg.Cc))
	}
	writeHeader(&buf, "Subject", mime.QEncoding.Encode("utf-8", msg.Subject))
	writeHeader(&buf, "Date", msg.Date.Format(time.RFC1123Z))
	writeHeader(&buf, "Message-ID", msg.MessageID)
	if inReplyTo := normalizeHeaderMessageID(msg.InReplyTo); inReplyTo != "" {
		writeHeader(&buf, "In-Reply-To", inReplyTo)
	}
	if references := formatReferencesHeader(msg.References); references != "" {
		writeHeader(&buf, "References", references)
	}
	writeHeader(&buf, "MIME-Version", "1.0")
	writeHeader(&buf, "Content-Type", `text/plain; charset="utf-8"`)
	writeHeader(&buf, "Content-Transfer-Encoding", "quoted-printable")
	buf.WriteString("\r\n")

	body := quotedprintable.NewWriter(&buf)
	if _, err := body.Write([]byte(normalizeCRLF(msg.TextBody))); err != nil {
		return ComposedMessage{}, fmt.Errorf("write quoted-printable body: %w", err)
	}
	if err := body.Close(); err != nil {
		return ComposedMessage{}, fmt.Errorf("close quoted-printable body: %w", err)
	}
	buf.WriteString("\r\n")

	raw := buf.Bytes()
	return ComposedMessage{
		Raw:       raw,
		MessageID: msg.MessageID,
		Size:      int64(len(raw)),
	}, nil
}

func normalizeHeaderMessageID(value string) string {
	value = strings.TrimSpace(value)
	if value == "" || strings.ContainsAny(value, "\r\n") {
		return ""
	}
	if !strings.HasPrefix(value, "<") {
		value = "<" + value
	}
	if !strings.HasSuffix(value, ">") {
		value += ">"
	}
	return value
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

func writeHeader(buf *bytes.Buffer, key string, value string) {
	buf.WriteString(key)
	buf.WriteString(": ")
	buf.WriteString(value)
	buf.WriteString("\r\n")
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
