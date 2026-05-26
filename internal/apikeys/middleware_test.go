package apikeys

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
)

type fakeVerifier struct {
	info *KeyInfo
	err  error
}

func (f *fakeVerifier) Verify(ctx context.Context, keyHash string, ip net.IP) (*KeyInfo, error) {
	return f.info, f.err
}

func TestMiddlewareIgnoresNonAPIKey(t *testing.T) {
	middleware := Middleware(&fakeVerifier{}, nil)
	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
}

func TestMiddlewareAcceptsValidAPIKey(t *testing.T) {
	middleware := Middleware(&fakeVerifier{
		info: &KeyInfo{DomainID: "domain-1", Scopes: []string{"mail"}},
	}, nil)
	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		info, ok := KeyInfoFromContext(r.Context())
		if !ok {
			t.Fatal("KeyInfo not in context")
		}
		if info.DomainID != "domain-1" {
			t.Fatalf("DomainID = %s, want domain-1", info.DomainID)
		}
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer gm_testkey123456789")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
}

func TestMiddlewareRejectsInvalidAPIKey(t *testing.T) {
	middleware := Middleware(&fakeVerifier{err: errors.New("not found")}, nil)
	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer gm_testkey123456789")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rec.Code)
	}
}
