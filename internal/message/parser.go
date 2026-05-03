package message

import (
	"errors"
	"fmt"
	"io"
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
}

type ParsedMessage struct {
	MessageID         string
	Subject           string
	From              Address
	To                []Address
	Cc                []Address
	Bcc               []Address
	Date              time.Time
	TextBody          string
	TextBodyTruncated bool
	HasAttachment     bool
	Attachments       []Attachment
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

	for {
		part, err := reader.NextPart()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return ParsedMessage{}, fmt.Errorf("read mail part: %w", err)
		}

		switch header := part.Header.(type) {
		case *gomail.InlineHeader:
			if parsed.TextBody == "" && isTextPlain(header) {
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
			parsed.HasAttachment = true
			parsed.Attachments = append(parsed.Attachments, Attachment{Filename: filename})
		}
	}

	return parsed, nil
}

func normalizeParseOptions(opts ParseOptions) ParseOptions {
	if opts.MaxTextBodyBytes <= 0 {
		opts.MaxTextBodyBytes = 1 << 20
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
