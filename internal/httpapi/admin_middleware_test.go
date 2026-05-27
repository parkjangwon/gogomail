package httpapi

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestStripInternalHeadersMiddleware(t *testing.T) {
	headers := []string{
		"X-Gogomail-Resolved-User-ID",
		"X-Gogomail-Tenant-ID",
		"X-Gogomail-Company-ID",
		"X-Gogomail-Domain-ID",
		"X-Gogomail-Principal-ID",
		"X-Gogomail-API-Key-ID",
	}
	var captured http.Header
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		captured = r.Header.Clone()
	})
	handler := StripInternalHeadersMiddleware(next)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	for _, h := range headers {
		req.Header.Set(h, "attacker-value")
	}
	handler.ServeHTTP(httptest.NewRecorder(), req)

	for _, h := range headers {
		if v := captured.Get(h); v != "" {
			t.Errorf("header %q not stripped, got %q", h, v)
		}
	}
}

func TestStripInternalHeadersMiddleware_PreservesOtherHeaders(t *testing.T) {
	var captured http.Header
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		captured = r.Header.Clone()
	})
	handler := StripInternalHeadersMiddleware(next)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer token123")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Gogomail-Resolved-User-ID", "should-be-removed")
	handler.ServeHTTP(httptest.NewRecorder(), req)

	if v := captured.Get("Authorization"); v != "Bearer token123" {
		t.Errorf("Authorization header should be preserved, got %q", v)
	}
	if v := captured.Get("Content-Type"); v != "application/json" {
		t.Errorf("Content-Type header should be preserved, got %q", v)
	}
	if v := captured.Get("X-Gogomail-Resolved-User-ID"); v != "" {
		t.Errorf("X-Gogomail-Resolved-User-ID should be stripped, got %q", v)
	}
}

func TestSecurityHeadersMiddlewareDoesNotLeakProxyHeaders(t *testing.T) {
	handler := SecurityHeadersMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	// Security headers should be present
	if resp.Header.Get("X-Frame-Options") == "" {
		t.Error("X-Frame-Options missing from response")
	}
	if resp.Header.Get("X-Content-Type-Options") == "" {
		t.Error("X-Content-Type-Options missing from response")
	}
}
