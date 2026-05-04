package outbound

import (
	"strings"
	"testing"
)

func TestBuildAndParseVERPReturnPath(t *testing.T) {
	returnPath, err := BuildVERPReturnPath("bounce@example.com", "User@Example.NET", "Msg-123")
	if err != nil {
		t.Fatalf("BuildVERPReturnPath() error = %v", err)
	}
	parsed, ok := ParseVERPReturnPath(returnPath)
	if !ok {
		t.Fatalf("ParseVERPReturnPath(%q) failed", returnPath)
	}
	if parsed.BaseLocal != "bounce" || parsed.Domain != "example.com" {
		t.Fatalf("parsed base = %+v", parsed)
	}
	if parsed.Recipient != "user@example.net" {
		t.Fatalf("recipient = %q, want normalized user@example.net", parsed.Recipient)
	}
	if parsed.Token != "msg-123" {
		t.Fatalf("token = %q, want msg-123", parsed.Token)
	}
}

func TestBuildVERPRejectsInvalidRecipient(t *testing.T) {
	if _, err := BuildVERPReturnPath("bounce@example.com", "not an address", "id"); err == nil {
		t.Fatal("BuildVERPReturnPath() error = nil, want invalid recipient error")
	}
}

func TestParseVERPReturnPathRejectsPlainAddress(t *testing.T) {
	if _, ok := ParseVERPReturnPath("bounce@example.com"); ok {
		t.Fatal("ParseVERPReturnPath() succeeded for non-VERP address")
	}
}

func TestParseVERPReturnPathRejectsOversizedInputsBeforeDecode(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name    string
		address string
	}{
		{
			name:    "address",
			address: strings.Repeat("a", maxVERPAddressBytes+1),
		},
		{
			name:    "local",
			address: strings.Repeat("l", maxVERPLocalPartBytes+1) + "@example.com",
		},
		{
			name:    "encoded_recipient",
			address: "bounce+" + strings.Repeat("e", maxVERPEncodedRecipientBytes+1) + "@example.com",
		},
		{
			name:    "token",
			address: "bounce+" + strings.Repeat("t", maxVERPTokenBytes+1) + "--dXNlckBleGFtcGxlLmNvbQ@example.com",
		},
	} {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			if _, ok := ParseVERPReturnPath(tc.address); ok {
				t.Fatalf("ParseVERPReturnPath accepted oversized %s", tc.name)
			}
		})
	}
}

func TestBuildVERPReturnPathCapsToken(t *testing.T) {
	t.Parallel()

	returnPath, err := BuildVERPReturnPath("bounce@example.com", "user@example.net", strings.Repeat("t", maxVERPTokenBytes+20))
	if err != nil {
		t.Fatalf("BuildVERPReturnPath() error = %v", err)
	}
	parsed, ok := ParseVERPReturnPath(returnPath)
	if !ok {
		t.Fatalf("ParseVERPReturnPath(%q) failed", returnPath)
	}
	if len(parsed.Token) != maxVERPTokenBytes {
		t.Fatalf("token length = %d, want %d", len(parsed.Token), maxVERPTokenBytes)
	}
}
