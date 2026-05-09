package httpapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gogomail/gogomail/internal/maildb"
)

func TestAdminListDeviceTokensHandler(t *testing.T) {
	now := time.Now()
	service := &fakeAdminService{
		pushDevices: []maildb.PushDevice{
			{ID: "dev-1", UserID: "user-1", Platform: "apns", TokenSuffix: "abc123", Status: "active", CreatedAt: now, UpdatedAt: now},
		},
	}
	mux := http.NewServeMux()
	RegisterAdminRoutes(mux, service, "secret")
	req := httptest.NewRequest("GET", "/admin/v1/users/user-1/device-tokens", nil)
	req.Header.Set("X-Admin-Token", "secret")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", w.Code, w.Body.String())
	}
	var resp map[string]any
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	devices, ok := resp["devices"].([]any)
	if !ok || len(devices) != 1 {
		t.Fatalf("devices = %v", resp)
	}
	if service.lastListDevicesUserID != "user-1" {
		t.Fatalf("lastListDevicesUserID = %q", service.lastListDevicesUserID)
	}
}

func TestAdminListDeviceTokensHandlerRejectsUnsafeUserID(t *testing.T) {
	service := &fakeAdminService{}
	mux := http.NewServeMux()
	RegisterAdminRoutes(mux, service, "secret")
	req := httptest.NewRequest("GET", "/admin/v1/users/user%0A1/device-tokens", nil)
	req.Header.Set("X-Admin-Token", "secret")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	if w.Code == http.StatusOK {
		t.Fatalf("expected error status, got 200")
	}
}

func TestAdminListDeviceTokensHandlerRejectsUnknownQueryKeys(t *testing.T) {
	service := &fakeAdminService{}
	mux := http.NewServeMux()
	RegisterAdminRoutes(mux, service, "secret")
	req := httptest.NewRequest("GET", "/admin/v1/users/user-1/device-tokens?unknown=1", nil)
	req.Header.Set("X-Admin-Token", "secret")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	if w.Code == http.StatusOK {
		t.Fatalf("expected error status, got 200")
	}
}

func TestAdminDeleteDeviceTokenHandler(t *testing.T) {
	service := &fakeAdminService{}
	mux := http.NewServeMux()
	RegisterAdminRoutes(mux, service, "secret")
	req := httptest.NewRequest("DELETE", "/admin/v1/users/user-1/device-tokens/dev-1", nil)
	req.Header.Set("X-Admin-Token", "secret")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", w.Code, w.Body.String())
	}
	if service.lastDeleteDeviceUserID != "user-1" || service.lastDeleteDeviceID != "dev-1" {
		t.Fatalf("lastDeleteDevice = %q/%q", service.lastDeleteDeviceUserID, service.lastDeleteDeviceID)
	}
}

func TestAdminDeleteDeviceTokenHandlerRejectsUnsafeDeviceID(t *testing.T) {
	service := &fakeAdminService{}
	mux := http.NewServeMux()
	RegisterAdminRoutes(mux, service, "secret")
	badID := make([]byte, 1025)
	for i := range badID {
		badID[i] = 'x'
	}
	req := httptest.NewRequest("DELETE", "/admin/v1/users/user-1/device-tokens/"+string(badID), nil)
	req.Header.Set("X-Admin-Token", "secret")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	if w.Code == http.StatusOK {
		t.Fatalf("expected error status, got 200")
	}
}

func TestAdminDeleteAllDeviceTokensHandler(t *testing.T) {
	service := &fakeAdminService{deleteAllDevicesCount: 3}
	mux := http.NewServeMux()
	RegisterAdminRoutes(mux, service, "secret")
	req := httptest.NewRequest("DELETE", "/admin/v1/users/user-1/device-tokens", nil)
	req.Header.Set("X-Admin-Token", "secret")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", w.Code, w.Body.String())
	}
	var resp map[string]any
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if service.lastDeleteAllDevicesUserID != "user-1" {
		t.Fatalf("lastDeleteAllDevicesUserID = %q", service.lastDeleteAllDevicesUserID)
	}
	deleted, _ := resp["deleted"].(float64)
	if int(deleted) != 3 {
		t.Fatalf("deleted = %v", resp)
	}
}

func TestAdminDeleteAllDeviceTokensHandlerRejectsUnsafeUserID(t *testing.T) {
	service := &fakeAdminService{}
	mux := http.NewServeMux()
	RegisterAdminRoutes(mux, service, "secret")
	req := httptest.NewRequest("DELETE", "/admin/v1/users/user%0A1/device-tokens", nil)
	req.Header.Set("X-Admin-Token", "secret")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	if w.Code == http.StatusOK {
		t.Fatalf("expected error status, got 200")
	}
}

func TestAdminDeleteAllDeviceTokensHandlerRejectsUnknownQueryKeys(t *testing.T) {
	service := &fakeAdminService{}
	mux := http.NewServeMux()
	RegisterAdminRoutes(mux, service, "secret")
	req := httptest.NewRequest("DELETE", "/admin/v1/users/user-1/device-tokens?unknown=1", nil)
	req.Header.Set("X-Admin-Token", "secret")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	if w.Code == http.StatusOK {
		t.Fatalf("expected error status, got 200")
	}
}
