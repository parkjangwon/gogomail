package bimi

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

type fakeResolver struct {
	policy *Policy
	err    error
}

func (r fakeResolver) LookupPolicy(context.Context, string) (*Policy, error) {
	return r.policy, r.err
}

func TestValidateAndFetchDoesNotVerifyVMCByURLPresence(t *testing.T) {
	t.Parallel()

	logo := []byte(`<svg xmlns="http://www.w3.org/2000/svg"></svg>`)
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write(logo)
	}))
	defer server.Close()

	cache := NewLogoCache()
	cache.client = server.Client()
	cache.allowPrivateNetwork = true // test server runs on localhost
	validator := NewValidator(fakeResolver{policy: &Policy{
		Version: "BIMI1",
		LogoURL: server.URL,
		VMCURL:  server.URL + "/vmc.pem",
		Fetched: time.Now(),
		Expires: time.Now().Add(time.Hour),
	}}, cache)

	got, vmcVerified, err := validator.ValidateAndFetch(context.Background(), "example.com")
	if err != nil {
		t.Fatalf("ValidateAndFetch returned error: %v", err)
	}
	if string(got) != string(logo) {
		t.Fatalf("logo = %q, want %q", got, logo)
	}
	if vmcVerified {
		t.Fatal("ValidateAndFetch reported VMC verified from URL presence without certificate validation")
	}
}

func TestFetchLogoCachesSHA256OfBody(t *testing.T) {
	t.Parallel()

	logo := []byte("logo-body")
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write(logo)
	}))
	defer server.Close()

	cache := NewLogoCache()
	cache.client = server.Client()
	cache.allowPrivateNetwork = true // test server runs on localhost
	if _, err := cache.FetchLogo(context.Background(), server.URL); err != nil {
		t.Fatalf("FetchLogo returned error: %v", err)
	}

	sum := sha256.Sum256(logo)
	wantHash := hex.EncodeToString(sum[:])
	cached := cache.cache[server.URL]
	if cached == nil {
		t.Fatal("logo was not cached")
	}
	if cached.hash != wantHash {
		t.Fatalf("hash = %q, want %q", cached.hash, wantHash)
	}
}
