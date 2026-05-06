package ratelimit

import (
	"strings"
	"testing"
)

func TestFixedWindowRedisKeyHashesRawKey(t *testing.T) {
	t.Parallel()

	key := fixedWindowRedisKey(" Drive Share Public ", "remote=192.0.2.1 token="+strings.Repeat("a", 40))
	if !strings.HasPrefix(key, "ratelimit:drive_share_public:") {
		t.Fatalf("key = %q, want normalized namespace prefix", key)
	}
	if strings.Contains(key, "192.0.2.1") || strings.Contains(key, strings.Repeat("a", 16)) {
		t.Fatalf("key = %q, want raw remote/token material hashed", key)
	}
}

func TestNormalizeLimiterNamespaceBoundsAndSanitizes(t *testing.T) {
	t.Parallel()

	got := normalizeLimiterNamespace(" Drive Share/Public\n" + strings.Repeat("x", 200))
	if strings.ContainsAny(got, " /\n") {
		t.Fatalf("namespace = %q, want sanitized namespace", got)
	}
	if len(got) > 96 {
		t.Fatalf("namespace length = %d, want bounded", len(got))
	}
}
