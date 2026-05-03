package delivery

import (
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
