package smtpd

import (
	"strings"
	"testing"
	"time"
)

func TestBuildReceivedHeaderUsesSafeFallbackTokens(t *testing.T) {
	header := BuildReceivedHeaderWithProtocol("", "", "", "", time.Unix(0, 0))
	for _, want := range []string{"from unknown", "by localhost", "with ESMTP"} {
		if !strings.Contains(header, want) {
			t.Fatalf("header %q missing %q", header, want)
		}
	}
}
