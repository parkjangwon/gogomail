package smtpd

import "testing"

func TestRemoteHostExtractsBracketedIPv6Host(t *testing.T) {
	if got := remoteHost("[2001:db8::1]:2525"); got != "2001:db8::1" {
		t.Fatalf("remoteHost = %q, want IPv6 host", got)
	}
}
