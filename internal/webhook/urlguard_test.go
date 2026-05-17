package webhook

import (
	"context"
	"net"
	"net/http"
	"strings"
	"testing"
)

func TestValidateOutboundHTTPURLRejectsPrivateHosts(t *testing.T) {
	t.Parallel()

	tests := []string{
		"http://localhost/hook",
		"http://127.0.0.1/hook",
		"http://[::1]/hook",
		"http://10.0.0.1/hook",
		"http://172.16.0.1/hook",
		"http://192.168.1.2/hook",
		"http://169.254.169.254/latest/meta-data",
	}
	for _, raw := range tests {
		raw := raw
		t.Run(raw, func(t *testing.T) {
			t.Parallel()
			if _, err := ValidateOutboundHTTPURL(context.Background(), raw, OutboundURLGuardOptions{}); err == nil {
				t.Fatalf("ValidateOutboundHTTPURL(%q) error = nil, want rejection", raw)
			}
		})
	}
}

func TestValidateOutboundHTTPURLRejectsDNSResolvedPrivateHosts(t *testing.T) {
	t.Parallel()

	_, err := ValidateOutboundHTTPURL(context.Background(), "https://metadata.example.test/hook", OutboundURLGuardOptions{
		LookupIPAddr: func(context.Context, string) ([]net.IPAddr, error) {
			return []net.IPAddr{{IP: net.ParseIP("169.254.169.254")}}, nil
		},
	})
	if err == nil || !strings.Contains(err.Error(), "private") {
		t.Fatalf("err = %v, want private rejection", err)
	}
}

func TestValidateOutboundHTTPURLAcceptsPublicResolvedHost(t *testing.T) {
	t.Parallel()

	got, err := ValidateOutboundHTTPURL(context.Background(), "https://hooks.example.test/path", OutboundURLGuardOptions{
		LookupIPAddr: func(context.Context, string) ([]net.IPAddr, error) {
			return []net.IPAddr{{IP: net.ParseIP("93.184.216.34")}}, nil
		},
	})
	if err != nil {
		t.Fatalf("ValidateOutboundHTTPURL returned error: %v", err)
	}
	if got.Scheme != "https" || got.Hostname() != "hooks.example.test" {
		t.Fatalf("url = %s", got.String())
	}
}

func TestGuardedHTTPClientRejectsRedirectToPrivate(t *testing.T) {
	t.Parallel()

	client := GuardedHTTPClient(&http.Client{}, OutboundURLGuardOptions{})
	req, err := http.NewRequest(http.MethodGet, "http://127.0.0.1/private", nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := client.CheckRedirect(req, nil); err == nil {
		t.Fatal("expected private redirect to be rejected")
	}
}
