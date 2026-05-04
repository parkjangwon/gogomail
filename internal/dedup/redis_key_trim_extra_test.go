package dedup

import (
	"strings"
	"testing"

	smtpd "github.com/gogomail/gogomail/internal/smtp"
)

func TestRedisKeyTrimsMessageIDAndRecipient(t *testing.T) {
	key := redisKey(smtpd.DedupKey{MessageID: " <id@example.com> ", Recipient: " RCPT@Example.COM "})
	normalized := redisKey(smtpd.DedupKey{MessageID: "<id@example.com>", Recipient: "rcpt@example.com"})
	if key != normalized {
		t.Fatalf("redisKey = %q, want trimmed lower-cased key %q", key, normalized)
	}
	if strings.Contains(key, "<id@example.com>") || strings.Contains(key, "rcpt@example.com") {
		t.Fatalf("redisKey leaked raw dedupe components: %q", key)
	}
}
