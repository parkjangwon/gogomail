package httpapi

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gogomail/gogomail/internal/auth"
	"github.com/gogomail/gogomail/internal/drive"
)

// TestAuthWebDAVBearerTokenRequired verifies that WebDAV requests with a
// TokenManager configured are rejected with 401 when no Bearer token is provided.
func TestAuthWebDAVBearerTokenRequired(t *testing.T) {
	t.Parallel()

	tm, err := auth.NewTokenManager("test-secret-webdav-auth-at-least-32bytes")
	if err != nil {
		t.Fatalf("NewTokenManager: %v", err)
	}

	service := &fakeWebDAVService{}
	mux := http.NewServeMux()
	RegisterWebDAVRoutes(mux, service, WebDAVRouteOptions{TokenManager: tm})

	// No Authorization header — must be rejected.
	req := httptest.NewRequest("PROPFIND", "/dav/", nil)
	req.Header.Set("Depth", "1")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("no token: status = %d, want 401", rec.Code)
	}
}

// TestAuthWebDAVInvalidTokenRejected verifies that WebDAV rejects an invalid Bearer token.
func TestAuthWebDAVInvalidTokenRejected(t *testing.T) {
	t.Parallel()

	tm, err := auth.NewTokenManager("test-secret-webdav-auth-at-least-32bytes")
	if err != nil {
		t.Fatalf("NewTokenManager: %v", err)
	}

	service := &fakeWebDAVService{}
	mux := http.NewServeMux()
	RegisterWebDAVRoutes(mux, service, WebDAVRouteOptions{TokenManager: tm})

	req := httptest.NewRequest("PROPFIND", "/dav/", nil)
	req.Header.Set("Depth", "1")
	req.Header.Set("Authorization", "Bearer not.a.valid.token")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("invalid token: status = %d, want 401", rec.Code)
	}
}

// TestAuthWebDAVValidTokenAccepted verifies that WebDAV accepts a properly signed Bearer token
// and extracts the user ID from claims (enforcing user isolation).
func TestAuthWebDAVValidTokenAccepted(t *testing.T) {
	t.Parallel()

	tm, err := auth.NewTokenManager("test-secret-webdav-auth-at-least-32bytes")
	if err != nil {
		t.Fatalf("NewTokenManager: %v", err)
	}

	service := &fakeWebDAVService{
		nodes: []drive.Node{},
	}
	mux := http.NewServeMux()
	RegisterWebDAVRoutes(mux, service, WebDAVRouteOptions{TokenManager: tm})

	token, err := tm.Sign(auth.Claims{UserID: "user-bearer", DomainID: "dom-1", Role: "user"}, 15*time.Minute)
	if err != nil {
		t.Fatalf("Sign: %v", err)
	}

	req := httptest.NewRequest("PROPFIND", "/dav/", nil)
	req.Header.Set("Depth", "1")
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusMultiStatus {
		t.Fatalf("valid token: status = %d, want 207, body = %s", rec.Code, rec.Body.String())
	}
	// Verify user ID was extracted from token claims, not from query param.
	if service.listReq.UserID != "user-bearer" {
		t.Errorf("listReq.UserID = %q, want user-bearer (from token claims)", service.listReq.UserID)
	}
}

// TestAuthWebDAVPutRequiresBearerToken verifies PUT is also guarded when TokenManager is set.
func TestAuthWebDAVPutRequiresBearerToken(t *testing.T) {
	t.Parallel()

	tm, err := auth.NewTokenManager("test-secret-webdav-auth-at-least-32bytes")
	if err != nil {
		t.Fatalf("NewTokenManager: %v", err)
	}

	service := &fakeWebDAVService{}
	mux := http.NewServeMux()
	RegisterWebDAVRoutes(mux, service, WebDAVRouteOptions{TokenManager: tm})

	// PUT without Authorization should be rejected even if user_id is in query.
	req := httptest.NewRequest(http.MethodPut, "/dav/file.txt?user_id=attacker", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("PUT without bearer: status = %d, want 401", rec.Code)
	}
}

// TestAuthWebDAVBasicAuthHTTPSRequired verifies that Basic auth is rejected over HTTP.
// Note: In httptest, requests are not HTTPS, so this test verifies the HTTP rejection.
func TestAuthWebDAVBasicAuthHTTPSRequired(t *testing.T) {
	t.Parallel()

	tm, err := auth.NewTokenManager("test-secret-webdav-auth-at-least-32bytes")
	if err != nil {
		t.Fatalf("NewTokenManager: %v", err)
	}

	service := &fakeWebDAVService{
		nodes: []drive.Node{},
	}
	mux := http.NewServeMux()
	RegisterWebDAVRoutes(mux, service, WebDAVRouteOptions{TokenManager: tm})

	// Basic auth over HTTP should be rejected with 403 Forbidden.
	token, err := tm.Sign(auth.Claims{UserID: "user-basic", DomainID: "dom-1", Role: "user"}, 15*time.Minute)
	if err != nil {
		t.Fatalf("Sign: %v", err)
	}

	req := httptest.NewRequest("PROPFIND", "/dav/", nil)
	req.Header.Set("Depth", "1")
	req.SetBasicAuth("user-basic", token) // username: user-basic, password: token
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("basic auth over HTTP: status = %d, want 403", rec.Code)
	}
}

// TestAuthWebDAVBasicAuthMissingPassword verifies Basic auth fails if password (token) is invalid.
func TestAuthWebDAVBasicAuthMissingPassword(t *testing.T) {
	t.Parallel()

	tm, err := auth.NewTokenManager("test-secret-webdav-auth-at-least-32bytes")
	if err != nil {
		t.Fatalf("NewTokenManager: %v", err)
	}

	service := &fakeWebDAVService{}
	mux := http.NewServeMux()
	RegisterWebDAVRoutes(mux, service, WebDAVRouteOptions{TokenManager: tm})

	// Basic auth with invalid password (not a valid token).
	req := httptest.NewRequest("PROPFIND", "/dav/", nil)
	req.Header.Set("Depth", "1")
	req.SetBasicAuth("user-test", "not-a-valid-token")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("basic auth over HTTP with invalid token: status = %d, want 403 (HTTP not HTTPS)", rec.Code)
	}
}

// TestAuthWebDAVNoAuthWithoutTokenManager verifies that when TokenManager is nil, requests are rejected.
func TestAuthWebDAVNoAuthWithoutTokenManager(t *testing.T) {
	t.Parallel()

	service := &fakeWebDAVService{}
	mux := http.NewServeMux()
	RegisterWebDAVRoutes(mux, service, WebDAVRouteOptions{TokenManager: nil})

	// Without TokenManager and no Authorization header, should be rejected.
	req := httptest.NewRequest("PROPFIND", "/dav/", nil)
	req.Header.Set("Depth", "1")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("no token manager: status = %d, want 401", rec.Code)
	}
}
