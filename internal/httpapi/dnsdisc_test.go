package httpapi_test

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gogomail/gogomail/internal/httpapi"
)

// mockDNSResolver satisfies httpapi.DNSResolver for tests.
type mockDNSResolver struct {
	records map[string][]*net.SRV // key: "service:name"
}

func (m *mockDNSResolver) LookupSRV(_ context.Context, service, _, name string) (string, []*net.SRV, error) {
	key := service + ":" + name
	addrs, ok := m.records[key]
	if !ok {
		return "", nil, &net.DNSError{Err: "no such host", Name: name, IsNotFound: true}
	}
	return name, addrs, nil
}

func newMockChecker(records map[string][]*net.SRV) *httpapi.DNSDiscoveryChecker {
	return &httpapi.DNSDiscoveryChecker{Resolver: &mockDNSResolver{records: records}}
}

func TestLookupCalDAVSRVFound(t *testing.T) {
	checker := newMockChecker(map[string][]*net.SRV{
		"caldavs:example.com": {
			{Target: "mail.example.com.", Port: 443, Priority: 10, Weight: 0},
		},
	})
	results, err := checker.LookupCalDAVSRV(context.Background(), "example.com")
	if err != nil {
		t.Fatalf("LookupCalDAVSRV error: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("len = %d, want 1", len(results))
	}
	if results[0].Host != "mail.example.com." {
		t.Errorf("Host = %q, want mail.example.com.", results[0].Host)
	}
	if results[0].Port != 443 {
		t.Errorf("Port = %d, want 443", results[0].Port)
	}
}

func TestLookupCalDAVSRVNotFound(t *testing.T) {
	checker := newMockChecker(map[string][]*net.SRV{})
	_, err := checker.LookupCalDAVSRV(context.Background(), "no-srv.example.com")
	if err == nil {
		t.Fatal("expected error for missing SRV record")
	}
}

func TestLookupCardDAVSRVFound(t *testing.T) {
	checker := newMockChecker(map[string][]*net.SRV{
		"carddavs:example.com": {
			{Target: "contacts.example.com.", Port: 443, Priority: 10, Weight: 0},
		},
	})
	results, err := checker.LookupCardDAVSRV(context.Background(), "example.com")
	if err != nil {
		t.Fatalf("LookupCardDAVSRV error: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("len = %d, want 1", len(results))
	}
	if results[0].Port != 443 {
		t.Errorf("Port = %d, want 443", results[0].Port)
	}
}

func TestCheckAutodiscoveryBothFound(t *testing.T) {
	checker := newMockChecker(map[string][]*net.SRV{
		"caldavs:example.com":  {{Target: "cal.example.com.", Port: 443}},
		"carddavs:example.com": {{Target: "card.example.com.", Port: 443}},
	})
	report := checker.CheckAutodiscovery(context.Background(), "example.com")
	if !report.CalDAVFound {
		t.Error("CalDAVFound = false, want true")
	}
	if !report.CardDAVFound {
		t.Error("CardDAVFound = false, want true")
	}
	if report.Domain != "example.com" {
		t.Errorf("Domain = %q, want example.com", report.Domain)
	}
}

func TestCheckAutodiscoveryNoneFound(t *testing.T) {
	checker := newMockChecker(map[string][]*net.SRV{})
	report := checker.CheckAutodiscovery(context.Background(), "no-dns.example.com")
	if report.CalDAVFound {
		t.Error("CalDAVFound = true, want false")
	}
	if report.CardDAVFound {
		t.Error("CardDAVFound = true, want false")
	}
}

func TestAutodiscoveryHandlerOK(t *testing.T) {
	checker := newMockChecker(map[string][]*net.SRV{
		"caldavs:example.com": {{Target: "cal.example.com.", Port: 443}},
	})
	mux := http.NewServeMux()
	httpapi.RegisterAutodiscoveryRoutes(mux, "test-token", checker)
	srv := httptest.NewServer(mux)
	defer srv.Close()

	req, _ := http.NewRequest(http.MethodGet, srv.URL+"/admin/v1/autodiscovery/check?domain=example.com", nil)
	req.Header.Set("Authorization", "Bearer test-token")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	var report httpapi.AutodiscoveryReport
	if err := json.NewDecoder(resp.Body).Decode(&report); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if !report.CalDAVFound {
		t.Error("caldav_srv_found = false, want true")
	}
	if report.CardDAVFound {
		t.Error("carddav_srv_found = true, want false")
	}
}

func TestAutodiscoveryHandlerMissingDomain(t *testing.T) {
	checker := newMockChecker(map[string][]*net.SRV{})
	mux := http.NewServeMux()
	httpapi.RegisterAutodiscoveryRoutes(mux, "test-token", checker)
	srv := httptest.NewServer(mux)
	defer srv.Close()

	req, _ := http.NewRequest(http.MethodGet, srv.URL+"/admin/v1/autodiscovery/check", nil)
	req.Header.Set("Authorization", "Bearer test-token")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", resp.StatusCode)
	}
}

func TestAutodiscoveryHandlerUnauthorized(t *testing.T) {
	checker := newMockChecker(map[string][]*net.SRV{})
	mux := http.NewServeMux()
	httpapi.RegisterAutodiscoveryRoutes(mux, "test-token", checker)
	srv := httptest.NewServer(mux)
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/admin/v1/autodiscovery/check?domain=x")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", resp.StatusCode)
	}
}
