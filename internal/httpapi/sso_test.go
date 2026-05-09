package httpapi_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gogomail/gogomail/internal/httpapi"
	"github.com/gogomail/gogomail/internal/maildb"
)

// fakeSSOAdminService is an in-memory SSOAdminService for tests.
type fakeSSOAdminService struct {
	configs map[string]maildb.SSOConfig
}

func newFakeSSOAdminService() *fakeSSOAdminService {
	return &fakeSSOAdminService{configs: make(map[string]maildb.SSOConfig)}
}

func (f *fakeSSOAdminService) GetSSOConfig(_ context.Context, domainID string) (maildb.SSOConfig, error) {
	cfg, ok := f.configs[domainID]
	if !ok {
		return maildb.SSOConfig{}, fmt.Errorf("not found")
	}
	return cfg, nil
}

func (f *fakeSSOAdminService) UpsertSSOConfig(_ context.Context, cfg maildb.SSOConfig) error {
	if err := maildb.ValidateSSOConfig(cfg); err != nil {
		return err
	}
	f.configs[cfg.DomainID] = cfg
	return nil
}

func (f *fakeSSOAdminService) DeleteSSOConfig(_ context.Context, domainID string) error {
	if _, ok := f.configs[domainID]; !ok {
		return fmt.Errorf("not found")
	}
	delete(f.configs, domainID)
	return nil
}

const ssoAdminToken = "test-admin-token"

func newSSOAdminServer(svc httpapi.SSOAdminService) *httptest.Server {
	mux := http.NewServeMux()
	httpapi.RegisterSSOAdminRoutes(mux, svc, ssoAdminToken)
	return httptest.NewServer(mux)
}

func ssoAdminRequest(t *testing.T, srv *httptest.Server, method, path string, body any) *http.Response {
	t.Helper()
	var buf bytes.Buffer
	if body != nil {
		if err := json.NewEncoder(&buf).Encode(body); err != nil {
			t.Fatalf("encode: %v", err)
		}
	}
	req, err := http.NewRequest(method, srv.URL+path, &buf)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Authorization", "Bearer "+ssoAdminToken)
	if method == http.MethodPost || method == http.MethodPut || method == http.MethodPatch {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	return resp
}

func TestSSOAdminPutAndGet(t *testing.T) {
	svc := newFakeSSOAdminService()
	srv := newSSOAdminServer(svc)
	defer srv.Close()

	putResp := ssoAdminRequest(t, srv, http.MethodPut, "/admin/v1/sso-configurations/domain-1", maildb.SSOConfig{
		Provider: "saml",
		SSOURL:   "https://idp.example.com/sso",
		EntityID: "https://app.example.com",
	})
	defer putResp.Body.Close()
	if putResp.StatusCode != http.StatusNoContent {
		t.Fatalf("put status = %d, want 204", putResp.StatusCode)
	}

	getResp := ssoAdminRequest(t, srv, http.MethodGet, "/admin/v1/sso-configurations/domain-1", nil)
	defer getResp.Body.Close()
	if getResp.StatusCode != http.StatusOK {
		t.Fatalf("get status = %d, want 200", getResp.StatusCode)
	}
	var cfg maildb.SSOConfig
	if err := json.NewDecoder(getResp.Body).Decode(&cfg); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if cfg.Provider != "saml" {
		t.Errorf("provider = %q, want saml", cfg.Provider)
	}
}

func TestSSOAdminGetNotFound(t *testing.T) {
	svc := newFakeSSOAdminService()
	srv := newSSOAdminServer(svc)
	defer srv.Close()

	resp := ssoAdminRequest(t, srv, http.MethodGet, "/admin/v1/sso-configurations/no-such-domain", nil)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("status = %d, want 404", resp.StatusCode)
	}
}

func TestSSOAdminDeleteExisting(t *testing.T) {
	svc := newFakeSSOAdminService()
	srv := newSSOAdminServer(svc)
	defer srv.Close()

	ssoAdminRequest(t, srv, http.MethodPut, "/admin/v1/sso-configurations/domain-2", maildb.SSOConfig{
		Provider: "oidc",
		ClientID: "client123",
	}).Body.Close()

	delResp := ssoAdminRequest(t, srv, http.MethodDelete, "/admin/v1/sso-configurations/domain-2", nil)
	defer delResp.Body.Close()
	if delResp.StatusCode != http.StatusNoContent {
		t.Fatalf("delete status = %d, want 204", delResp.StatusCode)
	}

	getResp := ssoAdminRequest(t, srv, http.MethodGet, "/admin/v1/sso-configurations/domain-2", nil)
	defer getResp.Body.Close()
	if getResp.StatusCode != http.StatusNotFound {
		t.Errorf("after delete, get status = %d, want 404", getResp.StatusCode)
	}
}

func TestSSOAdminPutInvalidProvider(t *testing.T) {
	svc := newFakeSSOAdminService()
	srv := newSSOAdminServer(svc)
	defer srv.Close()

	resp := ssoAdminRequest(t, srv, http.MethodPut, "/admin/v1/sso-configurations/domain-3", maildb.SSOConfig{
		Provider: "unknown",
	})
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", resp.StatusCode)
	}
}

// fakeSSOFlowService wraps fakeSSOAdminService for flow route tests.
type fakeSSOFlowService struct {
	*fakeSSOAdminService
}

func TestSSOInitiateSAMLRedirect(t *testing.T) {
	svc := &fakeSSOFlowService{newFakeSSOAdminService()}
	svc.UpsertSSOConfig(context.Background(), maildb.SSOConfig{ //nolint:errcheck
		DomainID: "dom-saml",
		Provider: "saml",
		SSOURL:   "https://idp.example.com/sso",
		EntityID: "https://app.example.com",
	})

	mux := http.NewServeMux()
	httpapi.RegisterSSORoutes(mux, svc)
	srv := httptest.NewServer(mux)
	defer srv.Close()

	client := &http.Client{CheckRedirect: func(req *http.Request, via []*http.Request) error {
		return http.ErrUseLastResponse
	}}
	resp, err := client.Get(srv.URL + "/auth/sso/initiate?domain=dom-saml")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusFound {
		t.Errorf("status = %d, want 302", resp.StatusCode)
	}
	if loc := resp.Header.Get("Location"); loc == "" {
		t.Error("expected Location header")
	}
}

func TestSSOInitiateOIDCRedirect(t *testing.T) {
	svc := &fakeSSOFlowService{newFakeSSOAdminService()}
	svc.UpsertSSOConfig(context.Background(), maildb.SSOConfig{ //nolint:errcheck
		DomainID:  "dom-oidc",
		Provider:  "oidc",
		ClientID:  "client123",
		SSOURL:    "https://idp.example.com/auth",
	})

	mux := http.NewServeMux()
	httpapi.RegisterSSORoutes(mux, svc)
	srv := httptest.NewServer(mux)
	defer srv.Close()

	client := &http.Client{CheckRedirect: func(req *http.Request, via []*http.Request) error {
		return http.ErrUseLastResponse
	}}
	resp, err := client.Get(srv.URL + "/auth/sso/initiate?domain=dom-oidc")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusFound {
		t.Errorf("status = %d, want 302", resp.StatusCode)
	}
}

func TestSSOInitiateDomainNotConfigured(t *testing.T) {
	mux := http.NewServeMux()
	httpapi.RegisterSSORoutes(mux, &fakeSSOFlowService{newFakeSSOAdminService()})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/auth/sso/initiate?domain=no-config")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("status = %d, want 404", resp.StatusCode)
	}
}
