package delivery

import (
	"encoding/json"
	"errors"
	"math"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/gogomail/gogomail/internal/outbound"
)

func TestRetryPolicyNextDelay(t *testing.T) {
	t.Parallel()

	policy := RetryPolicy{Delays: []time.Duration{time.Minute, 2 * time.Minute}}
	delay, ok := policy.NextDelay(0)
	if !ok || delay != time.Minute {
		t.Fatalf("NextDelay(0) = %s, %v; want 1m, true", delay, ok)
	}
	delay, ok = policy.NextDelay(1)
	if !ok || delay != 2*time.Minute {
		t.Fatalf("NextDelay(1) = %s, %v; want 2m, true", delay, ok)
	}
	_, ok = policy.NextDelay(2)
	if ok {
		t.Fatal("NextDelay(2) returned ok after attempts exhausted")
	}
}

func TestRetryPolicyNextScheduledDelayAppliesDeterministicJitter(t *testing.T) {
	t.Parallel()

	policy := RetryPolicy{Delays: []time.Duration{10 * time.Minute}, JitterRatio: 0.20}
	first, ok := policy.NextScheduledDelay("msg-1", 0)
	if !ok {
		t.Fatal("NextScheduledDelay returned ok=false")
	}
	second, ok := policy.NextScheduledDelay("msg-1", 0)
	if !ok {
		t.Fatal("NextScheduledDelay returned ok=false on second call")
	}
	if first != second {
		t.Fatalf("jitter = %s then %s, want deterministic value", first, second)
	}
	if first < 8*time.Minute || first > 12*time.Minute {
		t.Fatalf("jittered delay = %s, want within ±20%% of 10m", first)
	}
}

func TestRetryPolicyNextScheduledDelayCapsMaxDelay(t *testing.T) {
	t.Parallel()

	policy := RetryPolicy{
		Delays:      []time.Duration{24 * time.Hour},
		JitterRatio: 0,
		MaxDelay:    12 * time.Hour,
	}
	delay, ok := policy.NextScheduledDelay("msg-1", 0)
	if !ok {
		t.Fatal("NextScheduledDelay returned ok=false")
	}
	if delay != 12*time.Hour {
		t.Fatalf("delay = %s, want max cap 12h", delay)
	}
}

func TestRetryPolicyNextScheduledDelayCanDisableJitter(t *testing.T) {
	t.Parallel()

	policy := RetryPolicy{Delays: []time.Duration{time.Minute}}
	delay, ok := policy.NextScheduledDelay("msg-1", 0)
	if !ok {
		t.Fatal("NextScheduledDelay returned ok=false")
	}
	if delay != time.Minute {
		t.Fatalf("delay = %s, want base delay with jitter disabled", delay)
	}
}

func TestRetryPolicyNextScheduledDelayClampsHugeJitter(t *testing.T) {
	t.Parallel()

	policy := RetryPolicy{Delays: []time.Duration{10 * time.Minute}, JitterRatio: 99}
	delay, ok := policy.NextScheduledDelay("msg-1", 0)
	if !ok {
		t.Fatal("NextScheduledDelay returned ok=false")
	}
	if delay < time.Millisecond || delay > 20*time.Minute {
		t.Fatalf("delay = %s, want clamped jitter range", delay)
	}
}

func TestRetryPolicyNextScheduledDelayIgnoresNaNJitter(t *testing.T) {
	t.Parallel()

	policy := RetryPolicy{Delays: []time.Duration{time.Minute}, JitterRatio: math.NaN()}
	delay, ok := policy.NextScheduledDelay("msg-1", 0)
	if !ok {
		t.Fatal("NextScheduledDelay returned ok=false")
	}
	if delay != time.Minute {
		t.Fatalf("delay = %s, want base delay for NaN jitter", delay)
	}
}

func TestRetrySchedulerReportsExhaustion(t *testing.T) {
	t.Parallel()

	policy := RetryPolicy{Delays: []time.Duration{time.Minute}}
	if _, ok := policy.NextDelay(1); ok {
		t.Fatal("NextDelay returned ok for exhausted retry")
	}
}

func TestAggressiveBulkRetryPolicy(t *testing.T) {
	t.Parallel()

	policy := AggressiveBulkRetryPolicy()

	tests := []struct {
		attempt  int
		minDelay time.Duration
		maxDelay time.Duration
	}{
		{0, time.Minute, 3 * time.Minute},       // ~2 min with ±15% jitter
		{1, 8 * time.Minute, 12 * time.Minute},  // ~10 min with ±15% jitter
		{2, 50 * time.Minute, 70 * time.Minute}, // ~1 hour with ±15% jitter
		{3, 5 * time.Hour, 7 * time.Hour},       // ~6 hours with ±15% jitter
		{4, 10 * time.Hour, 12 * time.Hour},     // ~12 hours with ±15% jitter
	}

	for _, tt := range tests {
		delay, ok := policy.NextScheduledDelay("test-msg", tt.attempt)
		if !ok {
			t.Fatalf("attempt %d: expected ok=true", tt.attempt)
		}
		if delay < tt.minDelay || delay > tt.maxDelay {
			t.Fatalf("attempt %d: delay %s outside range [%s, %s]", tt.attempt, delay, tt.minDelay, tt.maxDelay)
		}
	}

	// Verify exhaustion after 5 attempts
	_, ok := policy.NextDelay(5)
	if ok {
		t.Fatal("expected ok=false after exhausting all delays")
	}
}

func TestAggressiveBulkRetryPolicyCapsMaxDelay(t *testing.T) {
	t.Parallel()

	policy := AggressiveBulkRetryPolicy()

	// Max delay should be 12 hours
	delay, _ := policy.NextScheduledDelay("test-msg", 10)
	if delay > 12*time.Hour {
		t.Fatalf("max delay exceeded: %s > 12h", delay)
	}
}

func TestRetryErrorMessageTruncatesAtUTF8Boundary(t *testing.T) {
	t.Parallel()

	message := retryErrorMessage(errors.New(strings.Repeat("a", 1999) + "한"))
	if len(message) > 2000 {
		t.Fatalf("retry error length = %d, want <= 2000 bytes", len(message))
	}
	if !utf8.ValidString(message) {
		t.Fatalf("retry error is invalid UTF-8: %q", message)
	}
}

func TestNormalizeRetryFarmDefaultsMalformedValues(t *testing.T) {
	t.Parallel()

	if got := normalizeRetryFarm(outbound.Farm(" weird ")); got != outbound.FarmGeneral {
		t.Fatalf("normalizeRetryFarm = %q, want general", got)
	}
	if got := normalizeRetryFarm(outbound.Farm("BULK")); got != outbound.FarmBulk {
		t.Fatalf("normalizeRetryFarm = %q, want bulk", got)
	}
}

func TestRetryDedupeKeyIncludesAttemptAndRecipients(t *testing.T) {
	t.Parallel()

	key := retryDedupeKey(Job{QueuedMessage: QueuedMessage{
		MessageID:    " msg-1 ",
		RetryAttempt: 2,
		To:           []outbound.Address{{Email: "User@Example.NET"}, {Email: "user@example.net"}},
		Cc:           []outbound.Address{{Email: "other@example.net"}},
	}})
	if key != "retry:msg-1:3:other@example.net,user@example.net" {
		t.Fatalf("retryDedupeKey = %q, want message/next-attempt/unique recipients", key)
	}
}

func TestRetryDedupeKeyIsRecipientOrderStable(t *testing.T) {
	t.Parallel()

	left := retryDedupeKey(Job{QueuedMessage: QueuedMessage{
		MessageID:    "msg-1",
		RetryAttempt: 1,
		To:           []outbound.Address{{Email: "a@example.net"}, {Email: "b@example.net"}},
	}})
	right := retryDedupeKey(Job{QueuedMessage: QueuedMessage{
		MessageID:    "msg-1",
		RetryAttempt: 1,
		To:           []outbound.Address{{Email: "b@example.net"}, {Email: "a@example.net"}},
	}})
	if left != right {
		t.Fatalf("retryDedupeKey order mismatch: %q != %q", left, right)
	}
}

func TestRetryPayloadPatchesRawPayloadAttempt(t *testing.T) {
	t.Parallel()

	job := Job{
		QueuedMessage: QueuedMessage{MessageID: "msg-1", RetryAttempt: 1},
		rawPayload: json.RawMessage(`{
			"event":"mail.queued",
			"message_id":"msg-1",
			"retry_attempt":1,
			"metadata":{"retry_attempt":99},
			"unknown":{"kept":true}
		}`),
	}
	next := job.QueuedMessage
	next.RetryAttempt = 2

	payload, err := retryPayload(job, next)
	if err != nil {
		t.Fatalf("retryPayload returned error: %v", err)
	}
	var got map[string]json.RawMessage
	if err := json.Unmarshal(payload, &got); err != nil {
		t.Fatalf("retry payload is invalid JSON: %v", err)
	}
	if string(got["retry_attempt"]) != "2" {
		t.Fatalf("retry_attempt = %s, want 2 in %s", got["retry_attempt"], payload)
	}
	if string(got["metadata"]) != `{"retry_attempt":99}` {
		t.Fatalf("metadata = %s, want nested retry_attempt unchanged", got["metadata"])
	}
	if string(got["unknown"]) != `{"kept":true}` {
		t.Fatalf("unknown = %s, want preserved", got["unknown"])
	}
}

func TestRetryPayloadInsertsMissingAttemptInRawPayload(t *testing.T) {
	t.Parallel()

	job := Job{
		QueuedMessage: QueuedMessage{MessageID: "msg-1"},
		rawPayload:    json.RawMessage(`{"event":"mail.queued","message_id":"msg-1"}`),
	}
	next := job.QueuedMessage
	next.RetryAttempt = 1

	payload, err := retryPayload(job, next)
	if err != nil {
		t.Fatalf("retryPayload returned error: %v", err)
	}
	if !strings.Contains(string(payload), `"retry_attempt":1`) {
		t.Fatalf("payload = %s, want inserted retry_attempt", payload)
	}
}

func TestRetryPayloadFallsBackForMalformedRawPayload(t *testing.T) {
	t.Parallel()

	job := Job{
		QueuedMessage: QueuedMessage{
			Event:        "mail.queued",
			MessageID:    "msg-1",
			RetryAttempt: 4,
			To:           []outbound.Address{{Email: "recipient@example.net"}},
		},
		rawPayload: json.RawMessage(`{"event":"mail.queued","message_id"`),
	}
	next := job.QueuedMessage
	next.RetryAttempt = 5

	payload, err := retryPayload(job, next)
	if err != nil {
		t.Fatalf("retryPayload returned error: %v", err)
	}
	var got QueuedMessage
	if err := json.Unmarshal(payload, &got); err != nil {
		t.Fatalf("fallback payload is invalid JSON: %v", err)
	}
	if got.RetryAttempt != 5 || len(got.To) != 1 || got.To[0].Email != "recipient@example.net" {
		t.Fatalf("fallback payload = %+v, want marshaled next queued message", got)
	}
}

func TestRetryPayloadFallsBackForDuplicateAttemptKey(t *testing.T) {
	t.Parallel()

	job := Job{
		QueuedMessage: QueuedMessage{
			Event:        "mail.queued",
			MessageID:    "msg-1",
			RetryAttempt: 2,
		},
		rawPayload: json.RawMessage(`{"event":"mail.queued","retry_attempt":2,"retry_attempt":99}`),
	}
	next := job.QueuedMessage
	next.RetryAttempt = 3

	payload, err := retryPayload(job, next)
	if err != nil {
		t.Fatalf("retryPayload returned error: %v", err)
	}
	var got QueuedMessage
	if err := json.Unmarshal(payload, &got); err != nil {
		t.Fatalf("fallback payload is invalid JSON: %v", err)
	}
	if got.RetryAttempt != 3 {
		t.Fatalf("RetryAttempt = %d, want fallback marshaled attempt 3", got.RetryAttempt)
	}
	if strings.Count(string(payload), "retry_attempt") != 1 {
		t.Fatalf("payload = %s, want duplicate retry_attempt removed by fallback", payload)
	}
}
