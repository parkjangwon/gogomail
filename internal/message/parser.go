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
	"unicode/utf8"

	gomessage "github.com/emersion/go-message"
	gomail "github.com/emersion/go-message/mail"
)

const maxAttachmentFilenameBytes = 255
const defaultMaxMetadataBytes = 2048

type Address struct {
	Name    string
	Address string
}

type Attachment struct {
	Filename string
}

type ParseOptions struct {
	MaxTextBodyBytes int64
	MaxHeaderBytes   int64
	SkipTextBody     bool
	MaxAttachments   int
	MaxParts         int
	MaxAddresses     int
	MaxReferences    int
	MaxMetadataBytes int
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
	AddressesTruncated   bool
	ReferencesTruncated  bool
	MetadataTruncated    bool
	Attachments          []Attachment
}

func ParseEML(r io.Reader) (ParsedMessage, error) {
	return ParseEMLWithOptions(r, ParseOptions{})
}

func ParseEMLWithOptions(r io.Reader, opts ParseOptions) (ParsedMessage, error) {
	opts = normalizeParseOptions(opts)

	entity, err := gomessage.ReadWithOptions(r, &gomessage.ReadOptions{MaxHeaderBytes: opts.MaxHeaderBytes})
	if err != nil && !gomessage.IsUnknownCharset(err) {
		return ParsedMessage{}, fmt.Errorf("create mail reader: %w", err)
	}
	reader := gomail.NewReader(entity)
	defer reader.Close()

	parsed := ParsedMessage{}

	if parsed.MessageID, err = reader.Header.MessageID(); err != nil {
		parsed.MessageID = ""
	} else {
		parsed.MessageID, parsed.MetadataTruncated = normalizeMessageIDBounded(parsed.MessageID, opts.MaxMetadataBytes)
	}
	parsed.InReplyTo = firstMessageID(&parsed, reader.Header, "In-Reply-To", opts)
	parsed.References, parsed.ReferencesTruncated = messageIDList(&parsed, reader.Header, "References", opts)
	if parsed.Subject, err = reader.Header.Subject(); err != nil {
		parsed.Subject = ""
	} else {
		parsed.Subject, parsed.MetadataTruncated = sanitizeHeaderMetadata(parsed.Subject, opts.MaxMetadataBytes, parsed.MetadataTruncated)
	}
	if parsed.Date, err = reader.Header.Date(); err != nil {
		parsed.Date = time.Time{}
	}
	if parsed.From, err = firstAddress(&parsed, opts, reader.Header, "From"); err != nil {
		parsed.From = Address{}
	}
	parsed.To = addressList(&parsed, opts, reader.Header, "To")
	parsed.Cc = addressList(&parsed, opts, reader.Header, "Cc")
	parsed.Bcc = addressList(&parsed, opts, reader.Header, "Bcc")

	partsSeen := 0
	for {
		part, err := reader.NextPart()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil && !gomessage.IsUnknownCharset(err) {
			return ParsedMessage{}, fmt.Errorf("read mail part: %w", err)
		}
		if part == nil {
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
		parsed.Attachments = append(parsed.Attachments, Attachment{Filename: sanitizeAttachmentFilename(filename)})
	} else {
		parsed.AttachmentsTruncated = true
	}
}

func sanitizeAttachmentFilename(filename string) string {
	filename = strings.TrimSpace(filename)
	filename = strings.ReplaceAll(filename, "\\", "/")
	if idx := strings.LastIndex(filename, "/"); idx >= 0 {
		filename = filename[idx+1:]
	}
	filename = strings.Map(func(r rune) rune {
		switch r {
		case '\r', '\n', '\t':
			return ' '
		default:
			if r < 0x20 || r == 0x7f {
				return -1
			}
			return r
		}
	}, filename)
	filename = strings.Join(strings.Fields(filename), " ")
	if len(filename) <= maxAttachmentFilenameBytes {
		return filename
	}
	body := []byte(filename[:maxAttachmentFilenameBytes])
	for len(body) > 0 && !utf8.Valid(body) {
		_, size := utf8.DecodeLastRune(body)
		body = body[:len(body)-size]
	}
	return string(body)
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

func firstMessageID(parsed *ParsedMessage, header gomail.Header, key string, opts ParseOptions) string {
	ids, _ := messageIDList(parsed, header, key, opts)
	if len(ids) == 0 {
		return ""
	}
	return ids[len(ids)-1]
}

func messageIDList(parsed *ParsedMessage, header gomail.Header, key string, opts ParseOptions) ([]string, bool) {
	ids, err := header.MsgIDList(key)
	if err != nil {
		return nil, false
	}
	maxIDs := opts.MaxReferences
	if maxIDs <= 0 {
		maxIDs = 1000
	}
	out := make([]string, 0, min(len(ids), maxIDs))
	truncated := false
	for _, id := range ids {
		if len(out) >= maxIDs {
			truncated = true
			break
		}
		var metadataTruncated bool
		id, metadataTruncated = normalizeMessageIDBounded(id, opts.MaxMetadataBytes)
		parsed.MetadataTruncated = parsed.MetadataTruncated || metadataTruncated
		if id != "" {
			out = append(out, id)
		}
	}
	return out, truncated
}

func normalizeParseOptions(opts ParseOptions) ParseOptions {
	if opts.MaxTextBodyBytes <= 0 {
		opts.MaxTextBodyBytes = 1 << 20
	}
	if opts.MaxHeaderBytes <= 0 {
		opts.MaxHeaderBytes = 1 << 20
	}
	if opts.MaxAttachments <= 0 {
		opts.MaxAttachments = 1000
	}
	if opts.MaxParts <= 0 {
		opts.MaxParts = 10000
	}
	if opts.MaxAddresses <= 0 {
		opts.MaxAddresses = 1000
	}
	if opts.MaxReferences <= 0 {
		opts.MaxReferences = 1000
	}
	if opts.MaxMetadataBytes <= 0 {
		opts.MaxMetadataBytes = defaultMaxMetadataBytes
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
		for len(body) > 0 && !utf8.Valid(body) {
			_, size := utf8.DecodeLastRune(body)
			body = body[:len(body)-size]
		}
	}
	return string(body), truncated, nil
}

func firstAddress(parsed *ParsedMessage, opts ParseOptions, header gomail.Header, key string) (Address, error) {
	addrs, err := header.AddressList(key)
	if err != nil || len(addrs) == 0 {
		return Address{}, err
	}
	return convertAddress(parsed, opts, addrs[0]), nil
}

func addressList(parsed *ParsedMessage, opts ParseOptions, header gomail.Header, key string) []Address {
	addrs, err := header.AddressList(key)
	if err != nil {
		return nil
	}
	limit := opts.MaxAddresses
	if limit <= 0 {
		limit = 1000
	}
	result := make([]Address, 0, min(len(addrs), limit))
	for _, addr := range addrs {
		if len(result) >= limit {
			parsed.AddressesTruncated = true
			break
		}
		result = append(result, convertAddress(parsed, opts, addr))
	}
	return result
}

func convertAddress(parsed *ParsedMessage, opts ParseOptions, addr *gomail.Address) Address {
	if addr == nil {
		return Address{}
	}
	name, nameTruncated := sanitizeHeaderMetadata(addr.Name, opts.MaxMetadataBytes, false)
	address, addressTruncated := sanitizeHeaderMetadata(strings.ToLower(addr.Address), opts.MaxMetadataBytes, false)
	parsed.MetadataTruncated = parsed.MetadataTruncated || nameTruncated || addressTruncated
	return Address{Name: name, Address: address}
}

func isTextPlain(header *gomail.InlineHeader) bool {
	contentType, _, err := header.ContentType()
	if err != nil {
		return true
	}
	return strings.EqualFold(contentType, "text/plain")
}

func normalizeMessageID(messageID string) string {
	normalized, _ := normalizeMessageIDBounded(messageID, defaultMaxMetadataBytes)
	return normalized
}

func normalizeMessageIDBounded(messageID string, maxBytes int) (string, bool) {
	if maxBytes <= 0 {
		maxBytes = defaultMaxMetadataBytes
	}
	messageID, truncated := cleanHeaderMetadataValue(messageID, false)
	messageID = strings.TrimSpace(messageID)
	if messageID == "" {
		return "", truncated
	}
	if strings.HasPrefix(messageID, "<") && strings.HasSuffix(messageID, ">") {
		if len(messageID) > maxBytes {
			return "", true
		}
		return messageID, truncated
	}
	messageID = "<" + messageID + ">"
	if len(messageID) > maxBytes {
		return "", true
	}
	return messageID, truncated
}

func sanitizeHeaderMetadata(value string, maxBytes int, alreadyTruncated bool) (string, bool) {
	if maxBytes <= 0 {
		maxBytes = defaultMaxMetadataBytes
	}
	value, truncated := cleanHeaderMetadataValue(value, alreadyTruncated)
	if len(value) <= maxBytes {
		return value, truncated
	}
	return truncateUTF8(value, maxBytes), true
}

func cleanHeaderMetadataValue(value string, alreadyTruncated bool) (string, bool) {
	value = strings.TrimSpace(value)
	truncated := alreadyTruncated
	if value == "" {
		return "", truncated
	}
	if !utf8.ValidString(value) {
		value = strings.ToValidUTF8(value, "")
		truncated = true
	}
	if containsHeaderMetadataControl(value) {
		value = strings.Map(func(r rune) rune {
			switch r {
			case '\r', '\n', '\t':
				return ' '
			default:
				if r < 0x20 || r == 0x7f {
					return -1
				}
				return r
			}
		}, value)
		value = strings.Join(strings.Fields(value), " ")
		truncated = true
	}
	return value, truncated
}

func containsHeaderMetadataControl(value string) bool {
	for _, r := range value {
		if r < 0x20 || r == 0x7f {
			return true
		}
	}
	return false
}

func truncateUTF8(value string, maxBytes int) string {
	if len(value) <= maxBytes {
		return value
	}
	body := []byte(value[:maxBytes])
	for len(body) > 0 && !utf8.Valid(body) {
		_, size := utf8.DecodeLastRune(body)
		body = body[:len(body)-size]
	}
	return string(body)
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
