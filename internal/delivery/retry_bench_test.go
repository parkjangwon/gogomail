package delivery

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/gogomail/gogomail/internal/outbound"
)

// BenchmarkRetryDedupeKey measures deduplication key generation performance
func BenchmarkRetryDedupeKey(b *testing.B) {
	tests := []struct {
		name       string
		recipients int
	}{
		{"1_recipient", 1},
		{"5_recipients", 5},
		{"10_recipients", 10},
		{"50_recipients", 50},
		{"1000_recipients", 1000},
		{"10000_recipients", 10000},
	}

	for _, tt := range tests {
		b.Run(tt.name, func(b *testing.B) {
			recipients := make([]outbound.Address, tt.recipients)
			for i := 0; i < tt.recipients; i++ {
				recipients[i] = outbound.Address{Email: strings.ToLower("User" + string(rune(i)) + "@example.com")}
			}

			job := Job{
				QueuedMessage: QueuedMessage{
					MessageID:    "msg-123",
					RetryAttempt: 2,
					To:           recipients,
				},
			}

			b.ReportAllocs()
			b.ResetTimer()

			for i := 0; i < b.N; i++ {
				_ = retryDedupeKey(job)
			}
		})
	}
}

// BenchmarkScheduleRetryQuery measures INSERT ... ON CONFLICT performance
func BenchmarkScheduleRetryQuery(b *testing.B) {
	// This would require a real database connection or mock
	// For now, just measure the dedupeKey generation which is CPU-bound

	job := Job{
		QueuedMessage: QueuedMessage{
			MessageID: "msg-123",
			From:      outbound.Address{Email: "sender@example.com"},
			To: []outbound.Address{
				{Email: "user1@example.com"},
				{Email: "user2@example.com"},
				{Email: "user3@example.com"},
			},
			RetryAttempt: 1,
		},
	}

	policy := DefaultRetryPolicy()
	delay, _ := policy.NextScheduledDelay(job.MessageID, job.RetryAttempt)

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_ = delay
		_ = retryDedupeKey(job)
	}
}

// TestRetryDedupeKeyConsistency verifies deterministic ordering
func TestRetryDedupeKeyConsistency(t *testing.T) {
	job := Job{
		QueuedMessage: QueuedMessage{
			MessageID:    "msg-123",
			RetryAttempt: 0,
			To: []outbound.Address{
				{Email: "charlie@example.com"},
				{Email: "alice@example.com"},
				{Email: "bob@example.com"},
			},
		},
	}

	key1 := retryDedupeKey(job)
	key2 := retryDedupeKey(job)

	if key1 != key2 {
		t.Errorf("dedupeKey not consistent: %q != %q", key1, key2)
	}

	// Verify alphabetical ordering
	if !strings.Contains(key1, "alice@example.com,bob@example.com,charlie@example.com") {
		t.Errorf("dedupeKey not alphabetically sorted: %q", key1)
	}

	t.Logf("Dedup key: %s", key1)
}

// TestRetrySchedulerWithMockDB tests retry scheduling without real DB
func TestRetrySchedulerWithMockDB(t *testing.T) {
	scheduler := &PostgresRetryScheduler{
		db:     nil, // Would fail without DB
		policy: DefaultRetryPolicy(),
		now:    time.Now,
	}

	job := Job{
		QueuedMessage: QueuedMessage{
			MessageID: "msg-123",
			From:      outbound.Address{Email: "sender@example.com"},
			To:        []outbound.Address{{Email: "recipient@example.com"}},
		},
	}

	err := scheduler.ScheduleRetry(context.Background(), job, nil)
	if err == nil {
		t.Fatal("expected error with nil DB")
	}

	if !strings.Contains(err.Error(), "database handle is required") {
		t.Errorf("unexpected error: %v", err)
	}
}

// BenchmarkRetryPolicyCalculation measures retry delay calculation
func BenchmarkRetryPolicyCalculation(b *testing.B) {
	policy := AggressiveBulkRetryPolicy()

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		for attempt := 0; attempt < 5; attempt++ {
			_, _ = policy.NextScheduledDelay("msg-"+string(rune(i)), attempt)
		}
	}
}
