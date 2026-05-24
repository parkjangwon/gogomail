package maildb

import "testing"

func TestNormalizeUserMCPAllowedCIDRsPreservesCIDRNotation(t *testing.T) {
	got, err := normalizeUserMCPAllowedCIDRs([]string{" 127.0.0.1/32 ", "192.0.2.1", "2001:db8::1"})
	if err != nil {
		t.Fatalf("normalizeUserMCPAllowedCIDRs returned error: %v", err)
	}
	want := []string{"127.0.0.1/32", "192.0.2.1/32", "2001:db8::1/128"}
	if len(got) != len(want) {
		t.Fatalf("got = %#v, want %#v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got = %#v, want %#v", got, want)
		}
	}
}

func TestNormalizeUserMCPAllowedCIDRsRejectsInvalidValues(t *testing.T) {
	if _, err := normalizeUserMCPAllowedCIDRs([]string{"not-a-cidr"}); err == nil {
		t.Fatal("normalizeUserMCPAllowedCIDRs accepted invalid CIDR")
	}
}
