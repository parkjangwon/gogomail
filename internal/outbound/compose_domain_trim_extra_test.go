package outbound

import "testing"

func TestDomainFromAddressTrimsInput(t *testing.T) {
	if got := domainFromAddress(" sender@Example.COM "); got != "Example.COM" {
		t.Fatalf("domainFromAddress = %q, want trimmed domain", got)
	}
}
