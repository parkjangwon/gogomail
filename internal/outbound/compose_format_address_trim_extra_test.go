package outbound

import "testing"

func TestFormatAddressTrimsEmailWhitespace(t *testing.T) {
	if got := formatAddress(Address{Name: "Ops", Email: " ops@example.com "}); got != `"Ops" <ops@example.com>` {
		t.Fatalf("formatAddress = %q, want trimmed mailbox", got)
	}
}
