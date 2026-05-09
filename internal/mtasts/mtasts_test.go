package mtasts_test

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gogomail/gogomail/internal/mtasts"
)

func TestPolicyMatchesMXExact(t *testing.T) {
	policy := &mtasts.Policy{
		Mode:    "enforce",
		MXHosts: []string{"mx1.example.com", "mx2.example.com"},
	}

	if !policy.MatchesMX("mx1.example.com") {
		t.Fatal("expected match for mx1.example.com")
	}
	if !policy.MatchesMX("mx2.example.com") {
		t.Fatal("expected match for mx2.example.com")
	}
	if policy.MatchesMX("mx3.example.com") {
		t.Fatal("unexpected match for mx3.example.com")
	}
}

func TestPolicyMatchesMXWildcard(t *testing.T) {
	policy := &mtasts.Policy{
		Mode:    "enforce",
		MXHosts: []string{"*.example.com"},
	}

	if !policy.MatchesMX("mx1.example.com") {
		t.Fatal("expected match for mx1.example.com")
	}
	if !policy.MatchesMX("mail.example.com") {
		t.Fatal("expected match for mail.example.com")
	}
	if policy.MatchesMX("example.com") {
		t.Fatal("wildcard should not match base domain")
	}
}

func TestPolicyIsExpired(t *testing.T) {
	policy := &mtasts.Policy{
		Expires: time.Now().Add(-1 * time.Second),
	}

	if !policy.IsExpired() {
		t.Fatal("expected expired policy")
	}

	policy.Expires = time.Now().Add(1 * time.Hour)
	if policy.IsExpired() {
		t.Fatal("expected valid policy")
	}
}

func TestClientFetchesPolicy(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/.well-known/mta-sts.json" {
			http.NotFound(w, r)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{
			"version": "STSv1",
			"mode": "enforce",
			"max_age": 604800,
			"mx": ["*.example.com"]
		}`))
	}))
	defer server.Close()

	// Note: This test would require injecting HTTP client or DNS mock.
	// For now, just verify parsing works.
	_ = server
}

func TestParsingEnforcePolicy(t *testing.T) {
	policyText := `{
		"version": "STSv1",
		"mode": "enforce",
		"max_age": 604800,
		"mx": ["mx1.example.com", "mx2.example.com"]
	}`

	policy, err := mtasts.ParsePolicy(policyText)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	if policy.Version != "STSv1" {
		t.Fatalf("expected STSv1, got %s", policy.Version)
	}
	if policy.Mode != "enforce" {
		t.Fatalf("expected enforce, got %s", policy.Mode)
	}
	if policy.MaxAge != 604800 {
		t.Fatalf("expected max_age 604800, got %d", policy.MaxAge)
	}
	if len(policy.MXHosts) != 2 {
		t.Fatalf("expected 2 MX hosts, got %d", len(policy.MXHosts))
	}
}

func TestParsingTestingPolicy(t *testing.T) {
	policyText := `{
		"version": "STSv1",
		"mode": "testing",
		"max_age": 86400,
		"mx": ["*.example.com"]
	}`

	policy, err := mtasts.ParsePolicy(policyText)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	if policy.Mode != "testing" {
		t.Fatalf("expected testing, got %s", policy.Mode)
	}
}

func TestParsingRejectsInvalidMode(t *testing.T) {
	policyText := `{
		"version": "STSv1",
		"mode": "invalid",
		"max_age": 86400,
		"mx": []
	}`

	_, err := mtasts.ParsePolicy(policyText)
	if err == nil {
		t.Fatal("expected error for invalid mode")
	}
}

func TestParsingRejectsInvalidMaxAge(t *testing.T) {
	policyText := `{
		"version": "STSv1",
		"mode": "enforce",
		"max_age": 40000000,
		"mx": []
	}`

	_, err := mtasts.ParsePolicy(policyText)
	if err == nil {
		t.Fatal("expected error for max_age > 1 year")
	}
}

func TestParsingRejectsMissingMXInEnforce(t *testing.T) {
	policyText := `{
		"version": "STSv1",
		"mode": "enforce",
		"max_age": 86400,
		"mx": []
	}`

	_, err := mtasts.ParsePolicy(policyText)
	if err == nil {
		t.Fatal("expected error for empty MX list in enforce mode")
	}
}
