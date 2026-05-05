package ratelimit

import (
	"testing"

	smtpd "github.com/gogomail/gogomail/internal/smtp"
)

func TestRedisKeyIncludesStageAndRemoteHost(t *testing.T) {
	t.Parallel()

	got := redisKey(smtpd.RateLimitKey{
		Stage:      smtpd.StageRcpt,
		RemoteAddr: "127.0.0.1:54321",
		Recipient:  "admin@example.com",
	})

	if got != "ratelimit:rcpt:127.0.0.1" {
		t.Fatalf("redisKey = %q", got)
	}
}

func TestRedisKeyNormalizesRemoteHostBuckets(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		remoteAddr string
		want       string
	}{
		{name: "ipv4 address", remoteAddr: "192.0.2.10", want: "ratelimit:rcpt:192.0.2.10"},
		{name: "ipv4 address port", remoteAddr: "192.0.2.10:2525", want: "ratelimit:rcpt:192.0.2.10"},
		{name: "ipv6 address", remoteAddr: "2001:db8::1", want: "ratelimit:rcpt:2001:db8::1"},
		{name: "ipv6 address port", remoteAddr: "[2001:db8::1]:2525", want: "ratelimit:rcpt:2001:db8::1"},
		{name: "ipv4 mapped ipv6", remoteAddr: "[::ffff:192.0.2.10]:2525", want: "ratelimit:rcpt:192.0.2.10"},
		{name: "malformed", remoteAddr: "127.0.0.1:notaport", want: "ratelimit:rcpt:unknown"},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := redisKey(smtpd.RateLimitKey{
				Stage:      smtpd.StageRcpt,
				RemoteAddr: tt.remoteAddr,
				Recipient:  "admin@example.com",
			})
			if got != tt.want {
				t.Fatalf("redisKey = %q, want %q", got, tt.want)
			}
		})
	}
}
