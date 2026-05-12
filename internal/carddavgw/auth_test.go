package carddavgw

import (
	"context"
	"crypto/tls"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	smtpd "github.com/gogomail/gogomail/internal/smtp"
)

func TestBasicAuthResolverAuthenticatesSubmissionUser(t *testing.T) {
	t.Parallel()

	resolver := NewBasicAuthResolver(fakeCardDAVAuthenticator{username: "user@example.com", password: "secret", userID: "user-1"}, false)
	req := httptest.NewRequest("PROPFIND", "/carddav/principals/user-1/", nil)
	req.TLS = tlsStateForTest()
	req.SetBasicAuth("user@example.com", "secret")
	userID, err := resolver.Resolve(req)
	if err != nil {
		t.Fatalf("Resolve returned error: %v", err)
	}
	if userID != "user-1" {
		t.Fatalf("userID = %q", userID)
	}
}

func TestBasicAuthResolverRejectsForwardedProtoWithoutTrustedProxyConfig(t *testing.T) {
	t.Parallel()

	resolver := NewBasicAuthResolver(fakeCardDAVAuthenticator{username: "user@example.com", password: "secret", userID: "user-1"}, false)
	req := httptest.NewRequest("PROPFIND", "/carddav/principals/user-1/", nil)
	req.Header.Set("X-Forwarded-Proto", "https")
	req.SetBasicAuth("user@example.com", "secret")
	if _, err := resolver.Resolve(req); err == nil || !strings.Contains(err.Error(), "requires TLS") {
		t.Fatalf("Resolve error = %v, want requires TLS", err)
	}
}

func TestBasicAuthResolverRejectsUntrustedForwardedProtoFromRemoteProxy(t *testing.T) {
	t.Parallel()

	resolver := NewBasicAuthResolver(fakeCardDAVAuthenticator{username: "user@example.com", password: "secret", userID: "user-1"}, false)
	trusted, err := resolver.WithTrustedProxies([]string{"127.0.0.1/8"})
	if err != nil {
		t.Fatalf("WithTrustedProxies returned error: %v", err)
	}
	resolver = trusted
	resolver.TrustForwardedProto = true
	req := httptest.NewRequest("PROPFIND", "/carddav/principals/user-1/", nil)
	req.RemoteAddr = "203.0.113.1:1234"
	req.Header.Set("X-Forwarded-Proto", "https")
	req.SetBasicAuth("user@example.com", "secret")
	if _, err := resolver.Resolve(req); err == nil || !strings.Contains(err.Error(), "requires TLS") {
		t.Fatalf("Resolve error = %v, want requires TLS", err)
	}
}

func TestBasicAuthResolverAllowsTrustedProxyForwardedProto(t *testing.T) {
	t.Parallel()

	resolver := NewBasicAuthResolver(fakeCardDAVAuthenticator{username: "user@example.com", password: "secret", userID: "user-1"}, false)
	trusted, err := resolver.WithTrustedProxies([]string{"127.0.0.1/8"})
	if err != nil {
		t.Fatalf("WithTrustedProxies returned error: %v", err)
	}
	resolver = trusted
	resolver.TrustForwardedProto = true
	req := httptest.NewRequest("PROPFIND", "/carddav/principals/user-1/", nil)
	req.RemoteAddr = "127.0.0.1:1234"
	req.Header.Set("X-Forwarded-Proto", "https")
	req.SetBasicAuth("user@example.com", "secret")
	if _, err := resolver.Resolve(req); err != nil {
		t.Fatalf("Resolve returned error behind trusted HTTPS proxy: %v", err)
	}
}

func TestBasicAuthResolverRejectsInvalidTrustedProxies(t *testing.T) {
	t.Parallel()

	resolver := NewBasicAuthResolver(fakeCardDAVAuthenticator{username: "user@example.com", password: "secret", userID: "user-1"}, false)
	if _, err := resolver.WithTrustedProxies([]string{"bad-proxy"}); err == nil {
		t.Fatal("WithTrustedProxies error = nil, want invalid proxy rejection")
	}
}

func TestBasicAuthResolverRejectsMalformedForwardedProto(t *testing.T) {
	t.Parallel()

	resolver := NewBasicAuthResolver(fakeCardDAVAuthenticator{username: "user@example.com", password: "secret", userID: "user-1"}, false)
	trusted, err := resolver.WithTrustedProxies([]string{"127.0.0.1/8"})
	if err != nil {
		t.Fatalf("WithTrustedProxies returned error: %v", err)
	}
	resolver = trusted
	resolver.TrustForwardedProto = true
	req := httptest.NewRequest("PROPFIND", "/carddav/principals/user-1/", nil)
	req.RemoteAddr = "127.0.0.1:1234"
	req.Header.Set("X-Forwarded-Proto", "https, http")
	req.SetBasicAuth("user@example.com", "secret")
	if _, err := resolver.Resolve(req); err == nil || !strings.Contains(err.Error(), "requires TLS") {
		t.Fatalf("Resolve error = %v, want requires TLS", err)
	}
}

func TestBasicAuthResolverAllowsUppercaseForwardedProtoWithWhitespace(t *testing.T) {
	t.Parallel()

	resolver := NewBasicAuthResolver(fakeCardDAVAuthenticator{username: "user@example.com", password: "secret", userID: "user-1"}, false)
	trusted, err := resolver.WithTrustedProxies([]string{"127.0.0.1/8"})
	if err != nil {
		t.Fatalf("WithTrustedProxies returned error: %v", err)
	}
	resolver = trusted
	resolver.TrustForwardedProto = true
	req := httptest.NewRequest("PROPFIND", "/carddav/principals/user-1/", nil)
	req.RemoteAddr = "127.0.0.1:1234"
	req.Header.Set("X-Forwarded-Proto", " HTTPS ")
	req.SetBasicAuth("user@example.com", "secret")
	if _, err := resolver.Resolve(req); err != nil {
		t.Fatalf("Resolve returned error behind uppercase HTTPS proxy: %v", err)
	}
}

func TestBasicAuthResolverRejectsUntrustedForwardedProto(t *testing.T) {
	t.Parallel()

	resolver := NewBasicAuthResolver(fakeCardDAVAuthenticator{username: "user@example.com", password: "secret", userID: "user-1"}, false)
	resolver.TrustForwardedProto = false
	req := httptest.NewRequest("PROPFIND", "/carddav/principals/user-1/", nil)
	req.Header.Set("X-Forwarded-Proto", "https")
	req.SetBasicAuth("user@example.com", "secret")
	if _, err := resolver.Resolve(req); err == nil || !strings.Contains(err.Error(), "requires TLS") {
		t.Fatalf("Resolve error = %v, want requires TLS", err)
	}
}

func TestBasicAuthResolverReturnsUnauthorizedChallenge(t *testing.T) {
	t.Parallel()

	resolver := NewBasicAuthResolver(fakeCardDAVAuthenticator{username: "user@example.com", password: "secret", userID: "user-1"}, false)
	req := newNoAuthTLSRequest()
	_, err := resolver.Resolve(req)
	if err == nil {
		t.Fatal("Resolve should fail when auth is missing")
	}
	type unauthorizedChallenge interface {
		WWWAuthenticate() string
	}
	challenge, ok := err.(unauthorizedChallenge)
	if !ok {
		t.Fatalf("Resolve error missing challenge interface: %T", err)
	}
	if challenge.WWWAuthenticate() != cardDAVWWWAuthenticate {
		t.Fatalf("challenge = %q, want %q", challenge.WWWAuthenticate(), cardDAVWWWAuthenticate)
	}
}

func TestBasicAuthResolverRejectsUnsafeRequests(t *testing.T) {
	t.Parallel()

	resolver := NewBasicAuthResolver(fakeCardDAVAuthenticator{username: "user@example.com", password: "secret", userID: "user-1"}, false)
	cases := []struct {
		name string
		req  *http.Request
		want string
	}{
		{name: "missing tls", req: newBasicAuthRequest("user@example.com", "secret", false), want: "requires TLS"},
		{name: "missing auth", req: newNoAuthTLSRequest(), want: "basic authentication is required"},
		{name: "bad username", req: newBasicAuthRequest("user\n@example.com", "secret", true), want: "username must not contain line breaks"},
		{name: "long password", req: newBasicAuthRequest("user@example.com", strings.Repeat("p", maxCardDAVBasicPasswordBytes+1), true), want: "password is too long"},
		{name: "bad password", req: newBasicAuthRequest("user@example.com", "wrong", true), want: "invalid carddav credentials"},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			if _, err := resolver.Resolve(tc.req); err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("Resolve error = %v, want %q", err, tc.want)
			}
		})
	}
}

func TestBasicAuthResolverRejectsInvalidAuthenticatedUserID(t *testing.T) {
	t.Parallel()

	resolver := NewBasicAuthResolver(fakeCardDAVAuthenticator{username: "user@example.com", password: "secret", userID: "bad\nuser"}, true)
	req := httptest.NewRequest("PROPFIND", "/carddav/principals/user-1/", nil)
	req.SetBasicAuth("user@example.com", "secret")
	if _, err := resolver.Resolve(req); err == nil || !strings.Contains(err.Error(), "user id is invalid") {
		t.Fatalf("Resolve error = %v, want invalid user id", err)
	}
}

func newBasicAuthRequest(username string, password string, tls bool) *http.Request {
	req := httptest.NewRequest("PROPFIND", "/carddav/principals/user-1/", nil)
	if tls {
		req.TLS = tlsStateForTest()
	}
	req.SetBasicAuth(username, password)
	return req
}

func newNoAuthTLSRequest() *http.Request {
	req := httptest.NewRequest("PROPFIND", "/carddav/principals/user-1/", nil)
	req.TLS = tlsStateForTest()
	return req
}

type fakeCardDAVAuthenticator struct {
	username string
	password string
	userID   string
}

func (a fakeCardDAVAuthenticator) AuthenticatePlain(_ context.Context, _ string, username string, password string) (smtpd.SubmissionUser, error) {
	if username != a.username || password != a.password {
		return smtpd.SubmissionUser{}, errFakeCardDAVNotFound
	}
	return smtpd.SubmissionUser{UserID: a.userID}, nil
}

func tlsStateForTest() *tls.ConnectionState {
	return &tls.ConnectionState{Version: tls.VersionTLS12}
}
