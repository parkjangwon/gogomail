package mail

import "testing"

func TestNormalizeAddressTrimsOuterWhitespace(t *testing.T) {
	got, err := NormalizeAddress("\tAdmin@Example.COM \n")
	if err != nil {
		t.Fatalf("NormalizeAddress returned error: %v", err)
	}
	if got != "admin@example.com" {
		t.Fatalf("NormalizeAddress = %q, want admin@example.com", got)
	}
}
