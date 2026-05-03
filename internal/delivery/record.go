package delivery

import (
	"context"
	"strings"
	"time"
	"unicode/utf8"
)

type AttemptStatus string

const (
	AttemptDelivered AttemptStatus = "delivered"
	AttemptFailed    AttemptStatus = "failed"
	AttemptBounced   AttemptStatus = "bounced"
)

type Attempt struct {
	MessageID       string
	RFCMessageID    string
	CompanyID       string
	DomainID        string
	Farm            string
	Recipient       string
	RecipientDomain string
	Status          AttemptStatus
	ErrorMessage    string
	AttemptedAt     time.Time
}

type Recorder interface {
	RecordAttempt(ctx context.Context, attempt Attempt) error
}

func attemptsFor(job Job, status AttemptStatus, cause error, attemptedAt time.Time) []Attempt {
	if attemptedAt.IsZero() {
		attemptedAt = time.Now().UTC()
	}
	message := ""
	if cause != nil {
		message = cause.Error()
	}
	message = truncateUTF8Bytes(message, 2000)

	recipients := job.Recipients()
	attempts := make([]Attempt, 0, len(recipients))
	for _, recipient := range recipients {
		_, domain, _ := strings.Cut(strings.TrimSpace(recipient.Email), "@")
		domain = strings.TrimSuffix(domain, ".")
		attempts = append(attempts, Attempt{
			MessageID:       job.MessageID,
			RFCMessageID:    job.RFCMessageID,
			CompanyID:       job.CompanyID,
			DomainID:        job.DomainID,
			Farm:            string(job.Farm),
			Recipient:       recipient.Email,
			RecipientDomain: strings.ToLower(domain),
			Status:          status,
			ErrorMessage:    message,
			AttemptedAt:     attemptedAt,
		})
	}
	return attempts
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
