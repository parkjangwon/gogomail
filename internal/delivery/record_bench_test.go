package delivery

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/gogomail/gogomail/internal/outbound"
)

func BenchmarkRecordAttemptBatchBulkVsIndividual(b *testing.B) {
	attempts := benchmarkAttempts(100)

	b.Run("bulk_recorder", func(b *testing.B) {
		recorder := &benchmarkBulkRecorder{}
		handler := NewHandler(nil, nil, recorder, nil)
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			if err := handler.recordAttemptBatch(context.Background(), attempts); err != nil {
				b.Fatalf("recordAttemptBatch returned error: %v", err)
			}
		}
		b.ReportMetric(float64(recorder.batchCalls)/float64(b.N), "record_calls/op")
	})

	b.Run("individual_recorder", func(b *testing.B) {
		recorder := &benchmarkIndividualRecorder{}
		handler := NewHandler(nil, nil, recorder, nil)
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			if err := handler.recordAttemptBatch(context.Background(), attempts); err != nil {
				b.Fatalf("recordAttemptBatch returned error: %v", err)
			}
		}
		b.ReportMetric(float64(recorder.calls)/float64(b.N), "record_calls/op")
	})
}

func BenchmarkRecordPartialAttempts1K(b *testing.B) {
	recipients := benchmarkRecipients(1000, 10)
	partial := &PartialDeliveryError{
		Delivered: recipients[:500],
		Failed:    make([]RecipientDeliveryError, 0, 500),
	}
	dsnRecipients := make([]DSNRecipientOptions, 0, len(recipients))
	for i, recipient := range recipients {
		dsnRecipients = append(dsnRecipients, DSNRecipientOptions{
			Address:           recipient.Email,
			Notify:            []string{"FAILURE", "DELAY"},
			OriginalRecipient: fmt.Sprintf("rfc822;%s", recipient.Email),
		})
		if i >= 500 {
			partial.Failed = append(partial.Failed, RecipientDeliveryError{
				Recipient: recipient,
				Err:       errors.New("temporary smtp failure"),
			})
		}
	}
	job := Job{QueuedMessage: QueuedMessage{
		MessageID:    "018f0000-0000-7000-8000-000000000001",
		RFCMessageID: "<bench@example.com>",
		Farm:         "general",
		From:         outboundAddress("sender@example.com"),
		To:           recipients,
		DSN: DSNOptions{
			Return:     "FULL",
			EnvelopeID: "bench-envelope",
			Recipients: dsnRecipients,
		},
		StoragePath: "messages/bench.eml",
	}}
	handler := NewHandler(nil, nil, &benchmarkBulkRecorder{}, nil)

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if err := handler.recordPartialAttempts(context.Background(), job, partial); err != nil {
			b.Fatalf("recordPartialAttempts returned error: %v", err)
		}
	}
}

func outboundAddress(email string) outbound.Address {
	return outbound.Address{Email: email}
}

func benchmarkAttempts(count int) []Attempt {
	attempts := make([]Attempt, 0, count)
	for _, recipient := range benchmarkRecipients(count, 10) {
		_, domain, _ := strings.Cut(recipient.Email, "@")
		attempts = append(attempts, Attempt{
			MessageID:       "018f0000-0000-7000-8000-000000000001",
			RFCMessageID:    "<bench@example.com>",
			Farm:            "general",
			Sender:          "sender@example.com",
			Recipient:       recipient.Email,
			RecipientDomain: domain,
			Status:          AttemptDelivered,
			EnhancedStatus:  "2.0.0",
			AttemptedAt:     time.Date(2026, 5, 21, 0, 0, 0, 0, time.UTC),
		})
	}
	return attempts
}

type benchmarkBulkRecorder struct {
	batchCalls int
	attempts   int
}

func (r *benchmarkBulkRecorder) RecordAttempt(context.Context, Attempt) error {
	return fmt.Errorf("RecordAttempt should not be called")
}

func (r *benchmarkBulkRecorder) RecordAttempts(_ context.Context, attempts []Attempt) error {
	r.batchCalls++
	r.attempts += len(attempts)
	return nil
}

type benchmarkIndividualRecorder struct {
	calls int
}

func (r *benchmarkIndividualRecorder) RecordAttempt(context.Context, Attempt) error {
	r.calls++
	return nil
}
