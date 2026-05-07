package ratelimit

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestRedisLimiterConstructorDefaults(t *testing.T) {
	t.Parallel()

	l := NewRedisLimiter(nil, 0, 0)
	if l.limit != 60 {
		t.Fatalf("limit = %d, want 60 default", l.limit)
	}
	if l.window != time.Minute {
		t.Fatalf("window = %v, want 1m default", l.window)
	}
}

func TestRedisLimiterConstructorNegativeLimit(t *testing.T) {
	t.Parallel()

	l := NewRedisLimiter(nil, -5, 0)
	if l.limit != 60 {
		t.Fatalf("limit = %d, want 60 default", l.limit)
	}
}

func TestRedisLimiterConstructorNegativeWindow(t *testing.T) {
	t.Parallel()

	l := NewRedisLimiter(nil, 0, -time.Hour)
	if l.window != time.Minute {
		t.Fatalf("window = %v, want 1m default", l.window)
	}
}

func TestRedisFixedWindowLimiterConstructorDefaults(t *testing.T) {
	t.Parallel()

	l := NewRedisFixedWindowLimiter(nil, "", 0, 0)
	if l.limit != 60 {
		t.Fatalf("limit = %d, want 60 default", l.limit)
	}
	if l.window != time.Minute {
		t.Fatalf("window = %v, want 1m default", l.window)
	}
}

func TestRedisFixedWindowLimiterConstructorNegativeLimit(t *testing.T) {
	t.Parallel()

	l := NewRedisFixedWindowLimiter(nil, "", -10, 0)
	if l.limit != 60 {
		t.Fatalf("limit = %d, want 60 default", l.limit)
	}
}

func TestRedisFixedWindowLimiterConstructorNegativeWindow(t *testing.T) {
	t.Parallel()

	l := NewRedisFixedWindowLimiter(nil, "", 0, -time.Hour)
	if l.window != time.Minute {
		t.Fatalf("window = %v, want 1m default", l.window)
	}
}

func TestRedisFixedWindowLimiterNilClientAllows(t *testing.T) {
	t.Parallel()

	l := NewRedisFixedWindowLimiter(nil, "test", 10, time.Minute)
	dec, err := l.Allow(context.Background(), "any-key")
	if err != nil {
		t.Fatalf("Allow error = %v, want nil", err)
	}
	if !dec.Allowed {
		t.Fatalf("dec.Allowed = false, want true for nil client")
	}
}

func TestRedisFixedWindowLimiterNamespaceNormalization(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		namespace string
		want      string
	}{
		{name: "empty", namespace: "", want: "generic"},
		{name: "spaces", namespace: "  test  ", want: "test"},
		{name: "mixed case", namespace: "TeSt", want: "test"},
		{name: "underscores preserved", namespace: "test_namespace", want: "test_namespace"},
		{name: "colons preserved", namespace: "a:b:c", want: "a:b:c"},
		{name: "hyphens preserved", namespace: "test-namespace", want: "test-namespace"},
		{name: "spaces replaced", namespace: "test namespace", want: "test_namespace"},
		{name: "slashes replaced", namespace: "test/namespace", want: "test_namespace"},
		{name: "dots replaced", namespace: "test.namespace", want: "test_namespace"},
		{name: "many spaces", namespace: "  a  b  c  ", want: "a__b__c"},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := normalizeLimiterNamespace(tt.namespace)
			if got != tt.want {
				t.Fatalf("normalizeLimiterNamespace(%q) = %q, want %q", tt.namespace, got, tt.want)
			}
		})
	}
}

func TestRedisFixedWindowLimiterNamespaceLengthBound(t *testing.T) {
	t.Parallel()

	long := normalizeLimiterNamespace("a" + strings.Repeat("b", 200))
	if len(long) > 96 {
		t.Fatalf("namespace length = %d, want <= 96", len(long))
	}
}

func TestRemoteBucketEdgeCases(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		remoteAddr string
		want      string
	}{
		{name: "empty", remoteAddr: "", want: "unknown"},
		{name: "spaces only", remoteAddr: "   ", want: "unknown"},
		{name: "plain ipv4", remoteAddr: "192.0.2.1", want: "192.0.2.1"},
		{name: "ipv4 with port", remoteAddr: "192.0.2.1:2525", want: "192.0.2.1"},
		{name: "plain ipv6", remoteAddr: "2001:db8::1", want: "2001:db8::1"},
		{name: "ipv6 with port", remoteAddr: "[2001:db8::1]:2525", want: "2001:db8::1"},
		{name: "ipv4 mapped ipv6", remoteAddr: "::ffff:192.0.2.1", want: "192.0.2.1"},
		{name: "ipv4 mapped ipv6 with port", remoteAddr: "[::ffff:192.0.2.1]:2525", want: "192.0.2.1"},
		{name: "localhost", remoteAddr: "127.0.0.1", want: "127.0.0.1"},
		{name: "localhost with port", remoteAddr: "127.0.0.1:54321", want: "127.0.0.1"},
		{name: "unparseable", remoteAddr: "not-an-ip", want: "unknown"},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := RemoteBucket(tt.remoteAddr)
			if got != tt.want {
				t.Fatalf("RemoteBucket(%q) = %q, want %q", tt.remoteAddr, got, tt.want)
			}
		})
	}
}