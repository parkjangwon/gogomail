package message

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"sort"
	"strings"
	"time"

	gomail "github.com/emersion/go-message/mail"
)

type Address struct {
	Name    string
	Address string
}

type Attachment struct {
	Filename string
}

type ParseOptions struct {
	MaxTextBodyBytes int64
	SkipTextBody     bool
	MaxAttachments   int
	MaxParts         int
}

type ParsedMessage struct {
	MessageID            string
	InReplyTo            string
	References           []string
	Subject              string
	From                 Address
	To                   []Address
	Cc                   []Address
	Bcc                  []Address
	Date                 time.Time
	TextBody             string
	TextBodyTruncated    bool
	HasAttachment        bool
	AttachmentsTruncated bool
	PartsTruncated       bool
	Attachments          []Attachment
}

func ParseEML(r io.Reader) (ParsedMessage, error) {
	return ParseEMLWithOptions(r, ParseOptions{})
}

func ParseEMLWithOptions(r io.Reader, opts ParseOptions) (ParsedMessage, error) {
	opts = normalizeParseOptions(opts)

	reader, err := gomail.CreateReader(r)
	if err != nil {
		return ParsedMessage{}, fmt.Errorf("create mail reader: %w", err)
	}
	defer reader.Close()

	parsed := ParsedMessage{}

	if parsed.MessageID, err = reader.Header.MessageID(); err != nil {
		parsed.MessageID = ""
	} else {
		parsed.MessageID = normalizeMessageID(parsed.MessageID)
	}
	parsed.InReplyTo = firstMessageID(reader.Header, "In-Reply-To")
	parsed.References = messageIDList(reader.Header, "References")
	if parsed.Subject, err = reader.Header.Subject(); err != nil {
		parsed.Subject = ""
	}
	if parsed.Date, err = reader.Header.Date(); err != nil {
		parsed.Date = time.Time{}
	}
	if parsed.From, err = firstAddress(reader.Header, "From"); err != nil {
		parsed.From = Address{}
	}
	parsed.To = addressList(reader.Header, "To")
	parsed.Cc = addressList(reader.Header, "Cc")
	parsed.Bcc = addressList(reader.Header, "Bcc")

	partsSeen := 0
	for {
		part, err := reader.NextPart()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return ParsedMessage{}, fmt.Errorf("read mail part: %w", err)
		}
		partsSeen++
		if partsSeen > opts.MaxParts {
			parsed.PartsTruncated = true
			break
		}

		switch header := part.Header.(type) {
		case *gomail.InlineHeader:
			filename, isInlineAttachment := inlineAttachmentMetadata(header)
			if isInlineAttachment {
				recordAttachment(&parsed, opts, filename)
			}
			if !isInlineAttachment && !opts.SkipTextBody && parsed.TextBody == "" && isTextPlain(header) {
				body, truncated, err := readLimitedText(part.Body, opts.MaxTextBodyBytes)
				if err != nil {
					return ParsedMessage{}, fmt.Errorf("read text body: %w", err)
				}
				parsed.TextBody = strings.TrimRight(body, "\r\n")
				parsed.TextBodyTruncated = truncated
			}
		case *gomail.AttachmentHeader:
			filename, err := header.Filename()
			if err != nil {
				filename = ""
			}
			recordAttachment(&parsed, opts, filename)
		}
	}

	return parsed, nil
}

func recordAttachment(parsed *ParsedMessage, opts ParseOptions, filename string) {
	parsed.HasAttachment = true
	if len(parsed.Attachments) < opts.MaxAttachments {
		parsed.Attachments = append(parsed.Attachments, Attachment{Filename: filename})
	} else {
		parsed.AttachmentsTruncated = true
	}
}

func inlineAttachmentMetadata(header *gomail.InlineHeader) (string, bool) {
	_, dispositionParams, dispositionErr := header.ContentDisposition()
	if dispositionErr == nil {
		if filename := strings.TrimSpace(dispositionParams["filename"]); filename != "" {
			return filename, true
		}
	}
	contentType, contentParams, contentErr := header.ContentType()
	if contentErr != nil {
		return "", false
	}
	if filename := strings.TrimSpace(contentParams["name"]); filename != "" {
		return filename, true
	}
	return "", !strings.HasPrefix(strings.ToLower(contentType), "text/")
}

func firstMessageID(header gomail.Header, key string) string {
	ids := messageIDList(header, key)
	if len(ids) == 0 {
		return ""
	}
	return ids[len(ids)-1]
}

func messageIDList(header gomail.Header, key string) []string {
	ids, err := header.MsgIDList(key)
	if err != nil {
		return nil
	}
	out := make([]string, 0, len(ids))
	for _, id := range ids {
		id = normalizeMessageID(id)
		if id != "" {
			out = append(out, id)
		}
	}
	return out
}

func normalizeParseOptions(opts ParseOptions) ParseOptions {
	if opts.MaxTextBodyBytes <= 0 {
		opts.MaxTextBodyBytes = 1 << 20
	}
	if opts.MaxAttachments <= 0 {
		opts.MaxAttachments = 1000
	}
	if opts.MaxParts <= 0 {
		opts.MaxParts = 10000
	}
	return opts
}

func readLimitedText(r io.Reader, maxBytes int64) (string, bool, error) {
	if maxBytes <= 0 {
		maxBytes = 1 << 20
	}
	body, err := io.ReadAll(io.LimitReader(r, maxBytes+1))
	if err != nil {
		return "", false, err
	}
	truncated := int64(len(body)) > maxBytes
	if truncated {
		body = body[:maxBytes]
	}
	return string(body), truncated, nil
}

func firstAddress(header gomail.Header, key string) (Address, error) {
	addrs, err := header.AddressList(key)
	if err != nil || len(addrs) == 0 {
		return Address{}, err
	}
	return convertAddress(addrs[0]), nil
}

func addressList(header gomail.Header, key string) []Address {
	addrs, err := header.AddressList(key)
	if err != nil {
		return nil
	}
	result := make([]Address, 0, len(addrs))
	for _, addr := range addrs {
		result = append(result, convertAddress(addr))
	}
	return result
}

func convertAddress(addr *gomail.Address) Address {
	if addr == nil {
		return Address{}
	}
	return Address{Name: addr.Name, Address: strings.ToLower(addr.Address)}
}

func isTextPlain(header *gomail.InlineHeader) bool {
	contentType, _, err := header.ContentType()
	if err != nil {
		return true
	}
	return strings.EqualFold(contentType, "text/plain")
}

func normalizeMessageID(messageID string) string {
	messageID = strings.TrimSpace(messageID)
	if messageID == "" {
		return ""
	}
	if strings.HasPrefix(messageID, "<") && strings.HasSuffix(messageID, ">") {
		return messageID
	}
	return "<" + messageID + ">"
}

func FallbackMessageID(envelopeFrom string, recipients []string, date time.Time, subject string) string {
	parts := make([]string, 0, 4+len(recipients))
	parts = append(parts, strings.ToLower(strings.TrimSpace(envelopeFrom)))
	normalizedRecipients := append([]string(nil), recipients...)
	for i := range normalizedRecipients {
		normalizedRecipients[i] = strings.ToLower(strings.TrimSpace(normalizedRecipients[i]))
	}
	sort.Strings(normalizedRecipients)
	parts = append(parts, normalizedRecipients...)
	if !date.IsZero() {
		parts = append(parts, date.UTC().Format(time.RFC3339Nano))
	}
	parts = append(parts, strings.TrimSpace(subject))

	sum := sha256.Sum256([]byte(strings.Join(parts, "\x00")))
	return "<missing-" + hex.EncodeToString(sum[:16]) + "@gogomail.local>"
}
