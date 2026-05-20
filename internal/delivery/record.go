package delivery

import (
	"context"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/gogomail/gogomail/internal/outbound"
)

type AttemptStatus string

const (
	AttemptDelivered AttemptStatus = "delivered"
	AttemptFailed    AttemptStatus = "failed"
	AttemptBounced   AttemptStatus = "bounced"
	AttemptExhausted AttemptStatus = "exhausted"
)

type Attempt struct {
	MessageID         string
	RFCMessageID      string
	CompanyID         string
	DomainID          string
	Farm              string
	Sender            string
	Recipient         string
	RecipientDomain   string
	Status            AttemptStatus
	EnhancedStatus    string
	ErrorMessage      string
	AttemptedAt       time.Time
	DSNReturn         string
	DSNEnvelopeID     string
	DSNNotify         []string
	OriginalRecipient string
	StoragePath       string
}

type Recorder interface {
	RecordAttempt(ctx context.Context, attempt Attempt) error
}

type BulkRecorder interface {
	Recorder
	RecordAttempts(ctx context.Context, attempts []Attempt) error
}

func attemptsFor(job Job, status AttemptStatus, cause error, attemptedAt time.Time) []Attempt {
	return attemptsForRecipients(job, job.Recipients(), status, cause, attemptedAt, dsnRecipientOptionsByAddress(job.DSN.Recipients))
}

func attemptsForRecipients(job Job, recipients []outbound.Address, status AttemptStatus, cause error, attemptedAt time.Time, dsnByAddress map[string]DSNRecipientOptions) []Attempt {
	if attemptedAt.IsZero() {
		attemptedAt = time.Now().UTC()
	}
	message := ""
	if cause != nil {
		message = cause.Error()
	}
	message = truncateUTF8Bytes(message, 2000)

	attempts := make([]Attempt, 0, len(recipients))
	for _, recipient := range recipients {
		attempts = append(attempts, attemptForRecipient(job, recipient, status, cause, message, attemptedAt, dsnByAddress))
	}
	return attempts
}

func attemptForRecipient(job Job, recipient outbound.Address, status AttemptStatus, cause error, message string, attemptedAt time.Time, dsnByAddress map[string]DSNRecipientOptions) Attempt {
	_, domain, _ := strings.Cut(strings.TrimSpace(recipient.Email), "@")
	domain = strings.TrimSuffix(domain, ".")
	dsnRecipient := dsnByAddress[strings.ToLower(strings.TrimSpace(recipient.Email))]
	return Attempt{
		MessageID:         job.MessageID,
		RFCMessageID:      job.RFCMessageID,
		CompanyID:         job.CompanyID,
		DomainID:          job.DomainID,
		Farm:              string(job.Farm),
		Sender:            strings.TrimSpace(job.From.Email),
		Recipient:         recipient.Email,
		RecipientDomain:   strings.ToLower(domain),
		Status:            status,
		EnhancedStatus:    enhancedStatusForAttempt(status, cause),
		ErrorMessage:      message,
		AttemptedAt:       attemptedAt,
		DSNReturn:         job.DSN.Return,
		DSNEnvelopeID:     job.DSN.EnvelopeID,
		DSNNotify:         append([]string(nil), dsnRecipient.Notify...),
		OriginalRecipient: dsnRecipient.OriginalRecipient,
		StoragePath:       job.StoragePath,
	}
}

func dsnRecipientOptionsByAddress(recipients []DSNRecipientOptions) map[string]DSNRecipientOptions {
	if len(recipients) == 0 {
		return nil
	}
	byAddress := make(map[string]DSNRecipientOptions, len(recipients))
	for _, recipient := range recipients {
		normalized := strings.ToLower(strings.TrimSpace(recipient.Address))
		if normalized == "" {
			continue
		}
		if _, ok := byAddress[normalized]; ok {
			continue
		}
		byAddress[normalized] = recipient
	}
	return byAddress
}

func truncateUTF8Bytes(value string, maxBytes int) string {
	if maxBytes <= 0 || len(value) <= maxBytes {
		return value
	}
	value = value[:maxBytes]
	for !utf8.ValidString(value) && len(value) > 0 {
		value = value[:len(value)-1]
	}
	return value
}

type noopRecorder struct{}

func (noopRecorder) RecordAttempt(context.Context, Attempt) error {
	return nil
}

func (noopRecorder) RecordAttempts(context.Context, []Attempt) error {
	return nil
}
