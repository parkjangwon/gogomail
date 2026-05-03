package ratelimit

import (
	"testing"

	smtpd "github.com/gogomail/gogomail/internal/smtp"
)

func TestRedisKeyUsesUnknownForBlankRemoteAddress(t *testing.T) {
	key := redisKey(smtpd.RateLimitKey{Stage: smtpd.StageRcpt, RemoteAddr: " \t "})
	if key != "ratelimit:rcpt:unknown" {
		t.Fatalf("redisKey = %q, want unknown remote fallback", key)
	}
}
