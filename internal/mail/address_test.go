package mail

import "testing"

func TestNormalizeAddressLowercasesDomainAndLocalPart(t *testing.T) {
	t.Parallel()

	got, err := NormalizeAddress("  JangWon@Example.COM  ")
	if err != nil {
		t.Fatalf("NormalizeAddress returned error: %v", err)
	}
	if got != "jangwon@example.com" {
		t.Fatalf("NormalizeAddress = %q, want jangwon@example.com", got)
	}
}

func TestNormalizeAddressRejectsInvalidAddress(t *testing.T) {
	t.Parallel()

	if _, err := NormalizeAddress("not-an-email"); err == nil {
		t.Fatal("NormalizeAddress accepted invalid address")
	}
}
