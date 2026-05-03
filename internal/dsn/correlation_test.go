package dsn

import (
	"testing"

	"github.com/gogomail/gogomail/internal/outbound"
)

func TestCorrelateVERPReturnPath(t *testing.T) {
	t.Parallel()

	returnPath, err := outbound.BuildVERPReturnPath("Bounce@Example.COM", "User@Example.NET", "Msg-123")
	if err != nil {
		t.Fatalf("BuildVERPReturnPath returned error: %v", err)
	}
	correlation, err := CorrelateVERPReturnPath("<" + returnPath + ">")
	if err != nil {
		t.Fatalf("CorrelateVERPReturnPath returned error: %v", err)
	}
	if correlation.BaseAddress != "bounce@example.com" {
		t.Fatalf("BaseAddress = %q", correlation.BaseAddress)
	}
	if correlation.OriginalRecipient != "user@example.net" {
		t.Fatalf("OriginalRecipient = %q", correlation.OriginalRecipient)
	}
	if correlation.Token != "msg-123" {
		t.Fatalf("Token = %q", correlation.Token)
	}
}

func TestCorrelateVERPReturnPathRejectsPlainAddress(t *testing.T) {
	t.Parallel()

	if _, err := CorrelateVERPReturnPath("bounce@example.com"); err == nil {
		t.Fatal("CorrelateVERPReturnPath accepted plain address")
	}
}
