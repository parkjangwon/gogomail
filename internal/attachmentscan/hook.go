package attachmentscan

import (
	"context"
	"fmt"
	"os"
	"strings"
	"unicode/utf8"

	smtpd "github.com/gogomail/gogomail/internal/smtp"
)

type Verdict string

const (
	VerdictAccept   Verdict = "accept"
	VerdictReject   Verdict = "reject"
	VerdictTempfail Verdict = "tempfail"

	maxScannerReasonBytes = 500
)

type Request struct {
	RemoteAddr     string
	EnvelopeFrom   string
	Recipients     []string
	CompanyID      string
	DomainID       string
	UserID         string
	SubmissionUser string
	MessageID      string
	Subject        string
	Size           int64
	Attachments    []Attachment
}

type Attachment struct {
	Filename string
}

type Result struct {
	Verdict Verdict
	Reason  string
}

type Scanner interface {
	ScanAttachments(ctx context.Context, req Request) (Result, error)
}

type StreamScanner interface {
	ScanStream(ctx context.Context, name string, file *os.File) (Result, error)
}

type HookOptions struct {
	Scanner Scanner
}

func Hook(opts HookOptions) smtpd.Hook {
	return func(ctx context.Context, event smtpd.Event) error {
		if event.Stage != smtpd.StageParsed || opts.Scanner == nil || !event.Parsed.HasAttachment {
			return nil
		}
		result, err := opts.Scanner.ScanAttachments(ctx, requestFromSMTPEvent(event))
		if err != nil {
			return fmt.Errorf("attachment scanner failed: %w", err)
		}
		return enforceResult(result)
	}
}

type StreamHookOptions struct {
	Scanner StreamScanner
}

func StreamHook(opts StreamHookOptions) smtpd.Hook {
	return func(ctx context.Context, event smtpd.Event) error {
		if event.Stage != smtpd.StageSpooled || opts.Scanner == nil || strings.TrimSpace(event.SpoolPath) == "" {
			return nil
		}
		file, err := os.Open(event.SpoolPath)
		if err != nil {
			return fmt.Errorf("open spooled message for attachment scan: %w", err)
		}
		defer file.Close()
		result, err := opts.Scanner.ScanStream(ctx, "smtp-message.eml", file)
		if err != nil {
			return fmt.Errorf("attachment stream scanner failed: %w", err)
		}
		return enforceResult(result)
	}
}

func requestFromSMTPEvent(event smtpd.Event) Request {
	attachments := make([]Attachment, 0, len(event.Parsed.Attachments))
	for _, attachment := range event.Parsed.Attachments {
		attachments = append(attachments, Attachment{Filename: attachment.Filename})
	}
	return Request{
		RemoteAddr:     event.RemoteAddr,
		EnvelopeFrom:   event.EnvelopeFrom,
		Recipients:     append([]string(nil), event.Recipients...),
		CompanyID:      firstNonEmpty(event.Mailbox.CompanyID, event.SubmissionUser.CompanyID),
		DomainID:       firstNonEmpty(event.Mailbox.DomainID, event.SubmissionUser.DomainID),
		UserID:         firstNonEmpty(event.Mailbox.UserID, event.SubmissionUser.UserID),
		SubmissionUser: event.SubmissionUser.Address,
		MessageID:      event.Parsed.MessageID,
		Subject:        event.Parsed.Subject,
		Size:           event.Size,
		Attachments:    attachments,
	}
}

func enforceResult(result Result) error {
	switch result.Verdict {
	case "", VerdictAccept:
		return nil
	case VerdictReject:
		return fmt.Errorf("attachment scanner rejected message: %s", cleanReason(result.Reason))
	case VerdictTempfail:
		return fmt.Errorf("attachment scanner temporarily failed message: %s", cleanReason(result.Reason))
	default:
		return fmt.Errorf("attachment scanner returned unsupported verdict %q", result.Verdict)
	}
}

func cleanReason(reason string) string {
	reason = strings.TrimSpace(reason)
	if reason == "" {
		return "no reason supplied"
	}
	reason = strings.Map(func(r rune) rune {
		switch r {
		case '\r', '\n':
			return -1
		default:
			return r
		}
	}, reason)
	return truncateUTF8Bytes(reason, maxScannerReasonBytes)
}

func truncateUTF8Bytes(value string, maxBytes int) string {
	if maxBytes <= 0 || len(value) <= maxBytes {
		return value
	}
	value = value[:maxBytes]
	for len(value) > 0 && !utf8.ValidString(value) {
		value = value[:len(value)-1]
	}
	return value
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			return value
		}
	}
	return ""
}
