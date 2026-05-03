package outbound

import "testing"

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
