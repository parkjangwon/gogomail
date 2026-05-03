package outbound

import "testing"

func TestDomainFromAddressFallsBackToLocalhost(t *testing.T) {
	if got := domainFromAddress("not-an-address"); got != "localhost" {
		t.Fatalf("domainFromAddress = %q, want localhost", got)
	}
}
