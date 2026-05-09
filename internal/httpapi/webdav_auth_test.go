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

	tm, err := auth.NewTokenManager("test-secret-webdav-auth")
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

	tm, err := auth.NewTokenManager("test-secret-webdav-auth")
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

	tm, err := auth.NewTokenManager("test-secret-webdav-auth")
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

	tm, err := auth.NewTokenManager("test-secret-webdav-auth")
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
