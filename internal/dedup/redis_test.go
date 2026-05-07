package dedup

import (
	"context"
	"strings"
	"testing"
	"time"

	smtpd "github.com/gogomail/gogomail/internal/smtp"
)

func TestRedisDeduplicatorNilClientAllows(t *testing.T) {
	t.Parallel()

	d := NewRedisDeduplicator(nil, 24*24*time.Hour)
	ok, err := d.CheckAndSet(context.Background(), smtpd.DedupKey{
		MessageID: "<test@example.com>",
		Recipient: "rcpt@example.com",
	})
	if err != nil {
		t.Fatalf("CheckAndSet error = %v, want nil", err)
	}
	if !ok {
		t.Fatalf("ok = false, want true for nil client")
	}
}

func TestRedisKeyUsesMessageIDAndRecipient(t *testing.T) {
	t.Parallel()

	got := redisKey(smtpd.DedupKey{
		MessageID: "<abc@example.com>",
		Recipient: "Admin@Example.COM",
	})

	if !strings.HasPrefix(got, "dedup:v2:") {
		t.Fatalf("redisKey = %q, want v2 hashed key", got)
	}
	if strings.Contains(got, "<abc@example.com>") || strings.Contains(got, "admin@example.com") {
		t.Fatalf("redisKey = %q", got)
	}
}

func TestRedisKeyIsFixedLengthForOversizedInputs(t *testing.T) {
	t.Parallel()

	got := redisKey(smtpd.DedupKey{
		MessageID: strings.Repeat("m", 10_000),
		Recipient: strings.Repeat("r", 10_000) + "@example.com",
	})
	if len(got) != len("dedup:v2:")+64+1+64 {
		t.Fatalf("redisKey length = %d, want fixed hashed key length", len(got))
	}
	if strings.Contains(got, "\r") || strings.Contains(got, "\n") {
		t.Fatalf("redisKey contains line break: %q", got)
	}
}
