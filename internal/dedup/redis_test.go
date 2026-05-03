package dedup

import (
	"testing"

	smtpd "github.com/gogomail/gogomail/internal/smtp"
)

func TestRedisKeyUsesMessageIDAndRecipient(t *testing.T) {
	t.Parallel()

	got := redisKey(smtpd.DedupKey{
		MessageID: "<abc@example.com>",
		Recipient: "Admin@Example.COM",
	})

	if got != "dedup:<abc@example.com>:admin@example.com" {
		t.Fatalf("redisKey = %q", got)
	}
}
