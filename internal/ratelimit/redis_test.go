package ratelimit

import (
	"testing"

	smtpd "github.com/gogomail/gogomail/internal/smtp"
)

func TestRedisKeyIncludesStageAndRemoteAddr(t *testing.T) {
	t.Parallel()

	got := redisKey(smtpd.RateLimitKey{
		Stage:      smtpd.StageRcpt,
		RemoteAddr: "127.0.0.1",
		Recipient:  "admin@example.com",
	})

	if got != "ratelimit:rcpt:127.0.0.1" {
		t.Fatalf("redisKey = %q", got)
	}
}
