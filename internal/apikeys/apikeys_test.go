package apikeys

import (
	"net"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestGenerateKey(t *testing.T) {
	key, err := GenerateKey()
	if err != nil {
		t.Fatalf("GenerateKey returned error: %v", err)
	}
	if !strings.HasPrefix(key, "gm_") {
		t.Fatalf("key %q does not have gm_ prefix", key)
	}
	if len(key) < 32 {
		t.Fatalf("key %q is too short", key)
	}

	key2, err := GenerateKey()
	if err != nil {
		t.Fatalf("GenerateKey returned error: %v", err)
	}
	if key == key2 {
		t.Fatal("GenerateKey should return unique keys")
	}
}

func TestVerifyKeyFormat(t *testing.T) {
	tests := []struct {
		key   string
		valid bool
	}{
		{"gm_validkey123456789", true},
		{"gm_short", false},
		{"invalid_prefix", false},
		{"", false},
		{"GM_uppercase", false},
	}

	for _, tt := range tests {
		got := VerifyKeyFormat(tt.key)
		if got != tt.valid {
			t.Errorf("VerifyKeyFormat(%q) = %v, want %v", tt.key, got, tt.valid)
		}
	}
}

func TestCheckCIDR(t *testing.T) {
	_, ipNet, err := net.ParseCIDR("192.168.1.0/24")
	if err != nil {
		t.Fatalf("ParseCIDR failed: %v", err)
	}
	allowed := []*net.IPNet{ipNet}

	tests := []struct {
		ip      string
		allowed bool
	}{
		{"192.168.1.100", true},
		{"192.168.1.1", true},
		{"10.0.0.1", false},
		{"192.168.2.1", false},
	}

	for _, tt := range tests {
		ip := net.ParseIP(tt.ip)
		got := CheckCIDR(ip, allowed)
		if got != tt.allowed {
			t.Errorf("CheckCIDR(%q) = %v, want %v", tt.ip, got, tt.allowed)
		}
	}
}

func TestCheckCIDREmptyAllowsAll(t *testing.T) {
	ip := net.ParseIP("10.0.0.1")
	if !CheckCIDR(ip, nil) {
		t.Error("CheckCIDR with empty list should allow all")
	}
}

func TestParseClientIPIgnoresForwardedForFromUntrustedRemote(t *testing.T) {
	req, err := http.NewRequest(http.MethodGet, "http://example.test/", nil)
	if err != nil {
		t.Fatalf("NewRequest returned error: %v", err)
	}
	req.RemoteAddr = "203.0.113.10:4567"
	req.Header.Set("X-Forwarded-For", "192.0.2.50")

	if got := parseClientIP(req, nil); got == nil || got.String() != "203.0.113.10" {
		t.Fatalf("parseClientIP = %v, want untrusted remote address", got)
	}
}

func TestParseClientIPAllowsForwardedForFromLoopbackProxy(t *testing.T) {
	req, err := http.NewRequest(http.MethodGet, "http://example.test/", nil)
	if err != nil {
		t.Fatalf("NewRequest returned error: %v", err)
	}
	req.RemoteAddr = "127.0.0.1:4567"
	req.Header.Set("X-Forwarded-For", "192.0.2.50")

	if got := parseClientIP(req, nil); got == nil || got.String() != "192.0.2.50" {
		t.Fatalf("parseClientIP = %v, want forwarded client address", got)
	}
}

func TestIsKeyExpired(t *testing.T) {
	now := time.Now()

	if IsKeyExpired(now.Add(-time.Hour), now) {
		t.Error("key created 1 hour ago should not be expired")
	}

	if !IsKeyExpired(now.Add(-31*24*time.Hour), now) {
		t.Error("key created 31 days ago should be expired")
	}
}

func TestHashKey(t *testing.T) {
	h1 := HashKey("gm_testkey123")
	h2 := HashKey("gm_testkey123")
	if h1 != h2 {
		t.Error("HashKey should return same hash for same key")
	}

	h3 := HashKey("gm_different")
	if h1 == h3 {
		t.Error("HashKey should return different hash for different key")
	}
}

func TestScopesAllowedByDomainPolicy(t *testing.T) {
	tests := []struct {
		name          string
		keyScopes     []string
		allowedScopes []string
		want          bool
	}{
		{
			name:          "exact scopes allowed",
			keyScopes:     []string{"mail:read", "drive:write"},
			allowedScopes: []string{"mail:read", "drive:write"},
			want:          true,
		},
		{
			name:          "policy narrowing blocks existing key scope",
			keyScopes:     []string{"drive:manage"},
			allowedScopes: []string{"drive:read", "drive:write"},
			want:          false,
		},
		{
			name:          "manage allowed covers family shorthand",
			keyScopes:     []string{"drive"},
			allowedScopes: []string{"drive:manage"},
			want:          true,
		},
		{
			name:          "empty policy allows no scopes",
			keyScopes:     []string{"mail:read"},
			allowedScopes: nil,
			want:          false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := scopesAllowedByDomainPolicy(tt.keyScopes, tt.allowedScopes); got != tt.want {
				t.Fatalf("scopesAllowedByDomainPolicy() = %v, want %v", got, tt.want)
			}
		})
	}
}
