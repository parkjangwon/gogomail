package httpapi_test

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/gogomail/gogomail/internal/auth"
	"github.com/gogomail/gogomail/internal/httpapi"
	"github.com/gogomail/gogomail/internal/maildb"
	"github.com/gogomail/gogomail/internal/sso"
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

// fakeSSOFlowService implements SSOFlowService for flow route tests.
type fakeSSOFlowService struct {
	*fakeSSOAdminService
	users map[string]maildb.SSOUserInfo // keyed by email
}

func newFakeSSOFlowService() *fakeSSOFlowService {
	return &fakeSSOFlowService{
		fakeSSOAdminService: newFakeSSOAdminService(),
		users:               make(map[string]maildb.SSOUserInfo),
	}
}

func (f *fakeSSOFlowService) GetUserByEmail(_ context.Context, email string) (maildb.SSOUserInfo, error) {
	info, ok := f.users[email]
	if !ok {
		return maildb.SSOUserInfo{}, fmt.Errorf("user not found")
	}
	return info, nil
}

func (f *fakeSSOFlowService) JITCreateSSOUser(_ context.Context, email, domainID, _ string) (maildb.SSOUserInfo, error) {
	info := maildb.SSOUserInfo{
		UserID:   "jit-user-id",
		DomainID: domainID,
		Email:    email,
	}
	f.users[email] = info
	return info, nil
}

func newFakeTM(t *testing.T) *auth.TokenManager {
	t.Helper()
	tm, err := auth.NewTokenManager("test-secret-for-sso-unit-tests")
	if err != nil {
		t.Fatalf("NewTokenManager: %v", err)
	}
	return tm
}

func newSSOFlowServer(svc httpapi.SSOFlowService, tm *auth.TokenManager) *httptest.Server {
	mux := http.NewServeMux()
	httpapi.RegisterSSORoutes(mux, svc, tm)
	return httptest.NewServer(mux)
}

func TestSSOInitiateSAMLRedirect(t *testing.T) {
	svc := newFakeSSOFlowService()
	svc.UpsertSSOConfig(context.Background(), maildb.SSOConfig{ //nolint:errcheck
		DomainID: "dom-saml",
		Provider: "saml",
		SSOURL:   "https://idp.example.com/sso",
		EntityID: "https://app.example.com",
	})

	srv := newSSOFlowServer(svc, newFakeTM(t))
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
	svc := newFakeSSOFlowService()
	svc.UpsertSSOConfig(context.Background(), maildb.SSOConfig{ //nolint:errcheck
		DomainID: "dom-oidc",
		Provider: "oidc",
		ClientID: "client123",
		SSOURL:   "https://idp.example.com/auth",
	})

	srv := newSSOFlowServer(svc, newFakeTM(t))
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
	loc := resp.Header.Get("Location")
	if !strings.Contains(loc, "code_challenge_method=S256") {
		t.Errorf("Location %q missing code_challenge_method=S256 (PKCE not applied)", loc)
	}
	if !strings.Contains(loc, "code_challenge=") {
		t.Errorf("Location %q missing code_challenge", loc)
	}
}

func TestSSOInitiateDomainNotConfigured(t *testing.T) {
	srv := newSSOFlowServer(newFakeSSOFlowService(), newFakeTM(t))
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

// buildMinimalSAMLResponse constructs a base64-encoded SAML Response XML with the given email as NameID.
func buildMinimalSAMLResponse(email string) string {
	const samlNS = "urn:oasis:names:tc:SAML:2.0:assertion"
	xml := fmt.Sprintf(`<samlp:Response xmlns:samlp="urn:oasis:names:tc:SAML:2.0:protocol" xmlns:saml="%s">`+
		`<saml:Assertion>`+
		`<saml:Subject><saml:NameID Format="urn:oasis:names:tc:SAML:1.1:nameid-format:emailAddress">%s</saml:NameID></saml:Subject>`+
		`</saml:Assertion>`+
		`</samlp:Response>`, samlNS, email)
	return base64.StdEncoding.EncodeToString([]byte(xml))
}

// buildMinimalIDToken builds a JWT with a valid exp claim (year 2099).
// ClientSecret is empty in tests so VerifyAndParseIDToken skips signature verification
// while still enforcing exp/iat/aud claims.
func buildMinimalIDToken(email string) string {
	// 4070908800 = 2099-01-01T00:00:00Z — valid for the foreseeable test future.
	const exp2099 = 4070908800
	header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"HS256","typ":"JWT"}`))
	payload := base64.RawURLEncoding.EncodeToString([]byte(
		fmt.Sprintf(`{"sub":"sub123","email":%q,"iss":"https://idp.example.com","aud":"client123","exp":%d}`, email, exp2099),
	))
	sig := base64.RawURLEncoding.EncodeToString([]byte("fakesig"))
	return header + "." + payload + "." + sig
}

// buildExpiredIDToken builds a JWT with exp in the past to test rejection.
func buildExpiredIDToken(email string) string {
	header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"HS256","typ":"JWT"}`))
	payload := base64.RawURLEncoding.EncodeToString([]byte(
		fmt.Sprintf(`{"sub":"sub123","email":%q,"iss":"https://idp.example.com","exp":1000}`, email),
	))
	sig := base64.RawURLEncoding.EncodeToString([]byte("fakesig"))
	return header + "." + payload + "." + sig
}

func TestSSOSAMLACSKnownUser(t *testing.T) {
	svc := newFakeSSOFlowService()
	svc.UpsertSSOConfig(context.Background(), maildb.SSOConfig{ //nolint:errcheck
		DomainID: "dom-saml",
		Provider: "saml",
		SSOURL:   "https://idp.example.com/sso",
		EntityID: "https://app.example.com",
	})
	svc.users["alice@example.com"] = maildb.SSOUserInfo{
		UserID:   "user-alice",
		DomainID: "dom-saml",
		Email:    "alice@example.com",
	}

	tm := newFakeTM(t)
	srv := newSSOFlowServer(svc, tm)
	defer srv.Close()

	form := url.Values{
		"SAMLResponse": {buildMinimalSAMLResponse("alice@example.com")},
		"RelayState":   {"dom-saml"},
	}
	resp, err := http.Post(srv.URL+"/auth/sso/saml/acs", "application/x-www-form-urlencoded", strings.NewReader(form.Encode()))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	var tr struct {
		Token string `json:"token"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&tr); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if tr.Token == "" {
		t.Error("expected non-empty token")
	}
}

func TestSSOSAMLACSMissingSAMLResponse(t *testing.T) {
	srv := newSSOFlowServer(newFakeSSOFlowService(), newFakeTM(t))
	defer srv.Close()

	resp, err := http.Post(srv.URL+"/auth/sso/saml/acs", "application/x-www-form-urlencoded", strings.NewReader("RelayState=dom"))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", resp.StatusCode)
	}
}

func TestSSOSAMLACSJITProvisioning(t *testing.T) {
	svc := newFakeSSOFlowService()
	svc.UpsertSSOConfig(context.Background(), maildb.SSOConfig{ //nolint:errcheck
		DomainID:        "dom-jit",
		Provider:        "saml",
		SSOURL:          "https://idp.example.com/sso",
		EntityID:        "https://app.example.com",
		JITProvisioning: true,
	})

	srv := newSSOFlowServer(svc, newFakeTM(t))
	defer srv.Close()

	form := url.Values{
		"SAMLResponse": {buildMinimalSAMLResponse("newuser@example.com")},
		"RelayState":   {"dom-jit"},
	}
	resp, err := http.Post(srv.URL+"/auth/sso/saml/acs", "application/x-www-form-urlencoded", strings.NewReader(form.Encode()))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200 (JIT provision should create user)", resp.StatusCode)
	}
}

func TestSSOOIDCCallbackKnownUser(t *testing.T) {
	email := "bob@example.com"
	idToken := buildMinimalIDToken(email)

	var receivedCodeVerifier string
	// Start a mock token endpoint server that captures code_verifier.
	tokenSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err == nil {
			receivedCodeVerifier = r.FormValue("code_verifier")
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"id_token": idToken}) //nolint:errcheck
	}))
	defer tokenSrv.Close()

	svc := newFakeSSOFlowService()
	svc.UpsertSSOConfig(context.Background(), maildb.SSOConfig{ //nolint:errcheck
		DomainID:     "dom-oidc",
		Provider:     "oidc",
		ClientID:     "client123",
		SSOURL:       "https://idp.example.com/auth",
		DiscoveryURL: tokenSrv.URL, // used as token endpoint in tests
	})
	svc.users[email] = maildb.SSOUserInfo{
		UserID:   "user-bob",
		DomainID: "dom-oidc",
		Email:    email,
	}

	tm := newFakeTM(t)
	srv := newSSOFlowServer(svc, tm)
	defer srv.Close()

	// Use PKCE state so code_verifier is forwarded to the token endpoint.
	state, _, err := sso.GenerateOIDCStateWithPKCE("dom-oidc")
	if err != nil {
		t.Fatal(err)
	}
	resp, err := http.Get(srv.URL + "/auth/sso/oidc/callback?code=test-code&state=" + state)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	var tr struct {
		Token string `json:"token"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&tr); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if tr.Token == "" {
		t.Error("expected non-empty token")
	}
	if receivedCodeVerifier == "" {
		t.Error("token endpoint did not receive code_verifier (PKCE not forwarded)")
	}
}

func TestSSOOIDCCallbackMissingCode(t *testing.T) {
	svc := newFakeSSOFlowService()
	srv := newSSOFlowServer(svc, newFakeTM(t))
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/auth/sso/oidc/callback?state=somestate")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", resp.StatusCode)
	}
}

func TestSSOOIDCCallbackInvalidState(t *testing.T) {
	svc := newFakeSSOFlowService()
	srv := newSSOFlowServer(svc, newFakeTM(t))
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/auth/sso/oidc/callback?code=abc&state=!!!invalid!!!")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", resp.StatusCode)
	}
}

func TestSSOSAMLACSCustomTTL(t *testing.T) {
	svc := newFakeSSOFlowService()
	svc.UpsertSSOConfig(context.Background(), maildb.SSOConfig{ //nolint:errcheck
		DomainID:          "dom-ttl",
		Provider:          "saml",
		SSOURL:            "https://idp.example.com/sso",
		EntityID:          "https://app.example.com",
		SessionTTLSeconds: 3600,
	})
	svc.users["ttl@example.com"] = maildb.SSOUserInfo{
		UserID:   "user-ttl",
		DomainID: "dom-ttl",
		Email:    "ttl@example.com",
	}

	srv := newSSOFlowServer(svc, newFakeTM(t))
	defer srv.Close()

	form := url.Values{
		"SAMLResponse": {buildMinimalSAMLResponse("ttl@example.com")},
		"RelayState":   {"dom-ttl"},
	}
	resp, err := http.Post(srv.URL+"/auth/sso/saml/acs", "application/x-www-form-urlencoded", strings.NewReader(form.Encode()))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	var tr struct {
		Token     string `json:"token"`
		ExpiresIn int    `json:"expires_in"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&tr); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if tr.ExpiresIn != 3600 {
		t.Errorf("expires_in = %d, want 3600 (domain TTL)", tr.ExpiresIn)
	}
}

func TestSSOSAMLACSDefaultTTL(t *testing.T) {
	svc := newFakeSSOFlowService()
	svc.UpsertSSOConfig(context.Background(), maildb.SSOConfig{ //nolint:errcheck
		DomainID: "dom-default-ttl",
		Provider: "saml",
		SSOURL:   "https://idp.example.com/sso",
		EntityID: "https://app.example.com",
		// SessionTTLSeconds zero → default 900s
	})
	svc.users["default@example.com"] = maildb.SSOUserInfo{
		UserID:   "user-default",
		DomainID: "dom-default-ttl",
		Email:    "default@example.com",
	}

	srv := newSSOFlowServer(svc, newFakeTM(t))
	defer srv.Close()

	form := url.Values{
		"SAMLResponse": {buildMinimalSAMLResponse("default@example.com")},
		"RelayState":   {"dom-default-ttl"},
	}
	resp, err := http.Post(srv.URL+"/auth/sso/saml/acs", "application/x-www-form-urlencoded", strings.NewReader(form.Encode()))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	var tr struct {
		ExpiresIn int `json:"expires_in"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&tr); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if tr.ExpiresIn != 900 {
		t.Errorf("expires_in = %d, want 900 (default TTL)", tr.ExpiresIn)
	}
}

func TestSSOOIDCCallbackCustomTTL(t *testing.T) {
	email := "oidc-ttl@example.com"
	idToken := buildMinimalIDToken(email)

	tokenSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"id_token": idToken}) //nolint:errcheck
	}))
	defer tokenSrv.Close()

	svc := newFakeSSOFlowService()
	svc.UpsertSSOConfig(context.Background(), maildb.SSOConfig{ //nolint:errcheck
		DomainID:          "dom-oidc-ttl",
		Provider:          "oidc",
		ClientID:          "client123",
		SSOURL:            "https://idp.example.com/auth",
		DiscoveryURL:      tokenSrv.URL,
		SessionTTLSeconds: 7200,
	})
	svc.users[email] = maildb.SSOUserInfo{
		UserID:   "user-oidc-ttl",
		DomainID: "dom-oidc-ttl",
		Email:    email,
	}

	srv := newSSOFlowServer(svc, newFakeTM(t))
	defer srv.Close()

	state, _, err := sso.GenerateOIDCStateWithPKCE("dom-oidc-ttl")
	if err != nil {
		t.Fatal(err)
	}
	resp, err := http.Get(srv.URL + "/auth/sso/oidc/callback?code=test-code&state=" + state)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	var tr struct {
		ExpiresIn int `json:"expires_in"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&tr); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if tr.ExpiresIn != 7200 {
		t.Errorf("expires_in = %d, want 7200 (domain TTL)", tr.ExpiresIn)
	}
}

// TestJWTExpiredIDTokenRejected verifies that an OIDC callback with an expired
// ID token is rejected with 401.
func TestJWTExpiredIDTokenRejected(t *testing.T) {
	expiredToken := buildExpiredIDToken("victim@example.com")

	tokenSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"id_token": expiredToken}) //nolint:errcheck
	}))
	defer tokenSrv.Close()

	svc := newFakeSSOFlowService()
	svc.UpsertSSOConfig(context.Background(), maildb.SSOConfig{ //nolint:errcheck
		DomainID:     "dom-expired",
		Provider:     "oidc",
		ClientID:     "client123",
		SSOURL:       "https://idp.example.com/auth",
		DiscoveryURL: tokenSrv.URL,
	})
	svc.users["victim@example.com"] = maildb.SSOUserInfo{
		UserID:   "user-victim",
		DomainID: "dom-expired",
		Email:    "victim@example.com",
	}

	srv := newSSOFlowServer(svc, newFakeTM(t))
	defer srv.Close()

	state, _, err := sso.GenerateOIDCStateWithPKCE("dom-expired")
	if err != nil {
		t.Fatal(err)
	}
	resp, err := http.Get(srv.URL + "/auth/sso/oidc/callback?code=test-code&state=" + state)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("expired token: status = %d, want 401", resp.StatusCode)
	}
}

