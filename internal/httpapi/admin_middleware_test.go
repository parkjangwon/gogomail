package httpapi

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestStripInternalProxyHeadersMiddlewareRemovesProxyHeaders(t *testing.T) {
	handler := StripInternalProxyHeadersMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Handler accidentally or intentionally sets proxy headers
		w.Header().Set("X-Forwarded-For", "192.0.2.1")
		w.Header().Set("X-Forwarded-Proto", "https")
		w.Header().Set("X-Forwarded-Host", "example.com")
		w.Header().Set("X-Real-IP", "192.0.2.1")
		w.Header().Set("X-Client-IP", "192.0.2.2")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	}))

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	// All internal proxy headers should be stripped
	if resp.Header.Get("X-Forwarded-For") != "" {
		t.Errorf("X-Forwarded-For present in response (value: %q), should be stripped", resp.Header.Get("X-Forwarded-For"))
	}
	if resp.Header.Get("X-Forwarded-Proto") != "" {
		t.Errorf("X-Forwarded-Proto present in response (value: %q), should be stripped", resp.Header.Get("X-Forwarded-Proto"))
	}
	if resp.Header.Get("X-Forwarded-Host") != "" {
		t.Errorf("X-Forwarded-Host present in response (value: %q), should be stripped", resp.Header.Get("X-Forwarded-Host"))
	}
	if resp.Header.Get("X-Real-IP") != "" {
		t.Errorf("X-Real-IP present in response (value: %q), should be stripped", resp.Header.Get("X-Real-IP"))
	}
	if resp.Header.Get("X-Client-IP") != "" {
		t.Errorf("X-Client-IP present in response (value: %q), should be stripped", resp.Header.Get("X-Client-IP"))
	}
}

func TestSecurityHeadersMiddlewareDoesNotLeakProxyHeaders(t *testing.T) {
	handler := SecurityHeadersMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("X-Forwarded-For", "192.0.2.1")
	req.Header.Set("X-Forwarded-Proto", "https")
	req.Header.Set("X-Forwarded-Host", "example.com")
	req.Header.Set("X-Real-IP", "192.0.2.1")

	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	// Internal proxy headers should NOT be present in response
	if resp.Header.Get("X-Forwarded-For") != "" {
		t.Errorf("X-Forwarded-For present in response (value: %q), should be stripped", resp.Header.Get("X-Forwarded-For"))
	}
	if resp.Header.Get("X-Forwarded-Proto") != "" {
		t.Errorf("X-Forwarded-Proto present in response (value: %q), should be stripped", resp.Header.Get("X-Forwarded-Proto"))
	}
	if resp.Header.Get("X-Forwarded-Host") != "" {
		t.Errorf("X-Forwarded-Host present in response (value: %q), should be stripped", resp.Header.Get("X-Forwarded-Host"))
	}
	if resp.Header.Get("X-Real-IP") != "" {
		t.Errorf("X-Real-IP present in response (value: %q), should be stripped", resp.Header.Get("X-Real-IP"))
	}

	// Security headers should be present
	if resp.Header.Get("X-Frame-Options") == "" {
		t.Error("X-Frame-Options missing from response")
	}
	if resp.Header.Get("X-Content-Type-Options") == "" {
		t.Error("X-Content-Type-Options missing from response")
	}
}

func TestStripProxyHeadersWithWriteWithoutWriteHeader(t *testing.T) {
	// Test that headers are stripped even when WriteHeader is not explicitly called
	handler := StripInternalProxyHeadersMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Forwarded-For", "192.0.2.1")
		// Write without calling WriteHeader (should default to 200)
		w.Write([]byte("OK"))
	}))

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	// X-Forwarded-For should be stripped even without explicit WriteHeader call
	if resp.Header.Get("X-Forwarded-For") != "" {
		t.Errorf("X-Forwarded-For leaked in response (value: %q)", resp.Header.Get("X-Forwarded-For"))
	}
}
