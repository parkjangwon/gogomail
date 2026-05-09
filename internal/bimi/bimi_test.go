package bimi_test

import (
	"context"
	"encoding/base64"
	"testing"
	"time"

	"github.com/gogomail/gogomail/internal/bimi"
)

func TestParsePolicy(t *testing.T) {
	tests := []struct {
		name    string
		txt     string
		want    *bimi.Policy
		wantErr bool
	}{
		{
			name: "valid policy with logo",
			txt:  "v=BIMI1; l=https://example.com/logo.svg",
			want: &bimi.Policy{
				Version: "BIMI1",
				LogoURL: "https://example.com/logo.svg",
				VMCURL:  "",
			},
			wantErr: false,
		},
		{
			name: "valid policy with logo and vmc",
			txt:  "v=BIMI1; l=https://example.com/logo.svg; a=https://example.com/vmc.pem",
			want: &bimi.Policy{
				Version: "BIMI1",
				LogoURL: "https://example.com/logo.svg",
				VMCURL:  "https://example.com/vmc.pem",
			},
			wantErr: false,
		},
		{
			name:    "invalid version",
			txt:     "v=BIMI0; l=https://example.com/logo.svg",
			wantErr: true,
		},
		{
			name:    "missing logo url",
			txt:     "v=BIMI1",
			wantErr: true,
		},
		{
			name:    "http logo url",
			txt:     "v=BIMI1; l=http://example.com/logo.svg",
			wantErr: true,
		},
		{
			name:    "empty logo url",
			txt:     "v=BIMI1; l=",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := bimi.ParsePolicy(tt.txt)
			if (err != nil) != tt.wantErr {
				t.Fatalf("ParsePolicy(%s) error = %v, wantErr %v", tt.txt, err, tt.wantErr)
			}
			if err != nil {
				return
			}
			if got.Version != tt.want.Version || got.LogoURL != tt.want.LogoURL || got.VMCURL != tt.want.VMCURL {
				t.Fatalf("ParsePolicy(%s) = %+v, want %+v", tt.txt, got, tt.want)
			}
		})
	}
}

func TestPolicyIsExpired(t *testing.T) {
	tests := []struct {
		name    string
		expires time.Time
		want    bool
	}{
		{
			name:    "not expired",
			expires: time.Now().Add(1 * time.Hour),
			want:    false,
		},
		{
			name:    "expired",
			expires: time.Now().Add(-1 * time.Hour),
			want:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			policy := &bimi.Policy{
				Version: "BIMI1",
				LogoURL: "https://example.com/logo.svg",
				Expires: tt.expires,
			}
			if got := policy.IsExpired(); got != tt.want {
				t.Fatalf("IsExpired() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestLogoCache(t *testing.T) {
	// Create cache (not used in this test, but validates initialization)
	_ = bimi.NewLogoCache()

	// Mock logo data
	logoData := []byte("fake-png-data")

	// Test base64 encoding
	header := bimi.GetLogoHeader(logoData)
	if header == "" {
		t.Fatal("GetLogoHeader returned empty string")
	}

	// Verify it starts with image type and base64 marker
	if !contains(header, "base64,") {
		t.Fatalf("header does not contain base64 marker: %s", header)
	}

	// Verify base64 encoding is valid
	parts := split(header, ",")
	if len(parts) < 2 {
		t.Fatalf("invalid header format: %s", header)
	}

	encoded := parts[1]
	decoded, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		t.Fatalf("failed to decode base64: %v", err)
	}
	if string(decoded) != string(logoData) {
		t.Fatalf("decoded data does not match original")
	}
}

func TestLogoHeaderDetection(t *testing.T) {
	tests := []struct {
		name        string
		data        []byte
		wantContent string
	}{
		{
			name:        "svg logo",
			data:        []byte("<svg>test</svg>"),
			wantContent: "image/svg+xml",
		},
		{
			name:        "png logo",
			data:        []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A},
			wantContent: "image/png",
		},
		{
			name:        "jpeg logo",
			data:        []byte{0xFF, 0xD8, 0xFF, 0xE0},
			wantContent: "image/jpeg",
		},
		{
			name:        "gif logo",
			data:        []byte("GIF89a"),
			wantContent: "image/gif",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			header := bimi.GetLogoHeader(tt.data)
			if !contains(header, tt.wantContent) {
				t.Fatalf("header does not contain %s: %s", tt.wantContent, header)
			}
		})
	}
}

func TestLogoCacheFetchHTTPS(t *testing.T) {
	cache := bimi.NewLogoCache()

	ctx := context.Background()
	logoURL := "http://example.com/logo.svg" // HTTP, not HTTPS

	// Should reject non-HTTPS URLs
	_, err := cache.FetchLogo(ctx, logoURL)
	if err == nil {
		t.Fatal("expected error for non-HTTPS URL")
	}
}

// Helper functions
func contains(s, substr string) bool {
	return len(s) >= len(substr) && s[0:len(substr)] == substr || findIndex(s, substr) >= 0
}

func findIndex(s, substr string) int {
	for i := 0; i < len(s)-len(substr)+1; i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}

func split(s, sep string) []string {
	if len(sep) == 0 {
		panic("empty separator")
	}
	var result []string
	start := 0
	for {
		idx := findIndex(s[start:], sep)
		if idx == -1 {
			result = append(result, s[start:])
			break
		}
		result = append(result, s[start:start+idx])
		start += idx + len(sep)
	}
	return result
}
