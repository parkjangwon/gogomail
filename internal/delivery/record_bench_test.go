package delivery

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"
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
