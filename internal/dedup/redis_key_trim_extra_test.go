package dedup

import (
	"testing"

	smtpd "github.com/gogomail/gogomail/internal/smtp"
)

func TestRedisKeyTrimsMessageIDAndRecipient(t *testing.T) {
	key := redisKey(smtpd.DedupKey{MessageID: " <id@example.com> ", Recipient: " RCPT@Example.COM "})
	if key != "dedup:<id@example.com>:rcpt@example.com" {
		t.Fatalf("redisKey = %q, want trimmed lower-cased key", key)
	}
}
