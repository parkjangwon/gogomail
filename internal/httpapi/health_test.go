package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestLiveHandlerIncludesNoSniffHeader(t *testing.T) {
	t.Parallel()

	mux := http.NewServeMux()
	RegisterHealthRoutes(mux)

	req := httptest.NewRequest(http.MethodGet, "/health/live", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if got := rec.Header().Get("X-Content-Type-Options"); got != "nosniff" {
		t.Fatalf("X-Content-Type-Options = %q, want nosniff", got)
	}
}

func TestInfoHandler(t *testing.T) {
	t.Parallel()

	mux := http.NewServeMux()
	RegisterHealthRoutes(mux)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/info", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if got := rec.Header().Get("X-Content-Type-Options"); got != "nosniff" {
		t.Fatalf("X-Content-Type-Options = %q, want nosniff", got)
	}
	var body InfoResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("json.Unmarshal returned error: %v", err)
	}
	if body.Service != "gogomail" || body.Status != "ok" {
		t.Fatalf("body = %+v", body)
	}
	if body.APIVersion != "v1" || body.BackendContractVersion == "" {
		t.Fatalf("contract metadata = %+v", body)
	}
	if body.APIVersion != APIVersion || body.BackendContractVersion != BackendContractVersion {
		t.Fatalf("metadata constants mismatch: body=%+v constants=%s/%s", body, APIVersion, BackendContractVersion)
	}
}

func TestReadyHandlerIncludesChecks(t *testing.T) {
	t.Parallel()

	mux := http.NewServeMux()
	RegisterHealthRoutes(mux)

	req := httptest.NewRequest(http.MethodGet, "/health/ready", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if got := rec.Header().Get("X-Content-Type-Options"); got != "nosniff" {
		t.Fatalf("X-Content-Type-Options = %q, want nosniff", got)
	}
	var body ReadinessResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("json.Unmarshal returned error: %v", err)
	}
	if body.Status != "ok" || len(body.Checks) < 4 || body.Checks[0].Name != "http" {
		t.Fatalf("body = %+v", body)
	}
	foundContract := false
	for _, check := range body.Checks {
		if check.Name == "api_contract" && check.Detail != "" {
			foundContract = true
		}
	}
	if !foundContract {
		t.Fatalf("api contract readiness check missing: %+v", body.Checks)
	}
}

func TestReadyHandlerReportsRuntimeCheckFailure(t *testing.T) {
	t.Parallel()

	mux := http.NewServeMux()
	RegisterHealthRoutesWithChecks(mux, func(_ context.Context) ReadinessCheck {
		return ReadinessCheck{Name: "database", Status: "error", Detail: "ping failed"}
	})

	req := httptest.NewRequest(http.MethodGet, "/health/ready", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if got := rec.Header().Get("X-Content-Type-Options"); got != "nosniff" {
		t.Fatalf("X-Content-Type-Options = %q, want nosniff", got)
	}
	var body ReadinessResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("json.Unmarshal returned error: %v", err)
	}
	if body.Status != "degraded" {
		t.Fatalf("body = %+v", body)
	}
	foundDatabase := false
	for _, check := range body.Checks {
		if check.Name == "database" && check.Status == "error" && check.Detail != "" {
			foundDatabase = true
		}
	}
	if !foundDatabase {
		t.Fatalf("database check missing: %+v", body.Checks)
	}
}

func TestHealthAndInfoHandlersRejectPayloadMetadata(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		path        string
		body        string
		contentType string
	}{
		{name: "live body", path: "/health/live", body: "{}"},
		{name: "ready content type", path: "/health/ready", contentType: "application/json"},
		{name: "info body", path: "/api/v1/info", body: "{}"},
		{name: "info content type", path: "/api/v1/info", contentType: "application/json"},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			mux := http.NewServeMux()
			RegisterHealthRoutes(mux)

			var bodyReader *strings.Reader
			if tt.body != "" {
				bodyReader = strings.NewReader(tt.body)
			} else {
				bodyReader = strings.NewReader("")
			}
			req := httptest.NewRequest(http.MethodGet, tt.path, bodyReader)
			if tt.body == "" {
				req.ContentLength = 0
			}
			if tt.contentType != "" {
				req.Header.Set("Content-Type", tt.contentType)
			}
			rec := httptest.NewRecorder()
			mux.ServeHTTP(rec, req)

			if rec.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
			}
		})
	}
}

func TestContentDispositionAttachmentSanitizesFilename(t *testing.T) {
	t.Parallel()

	got := contentDispositionAttachment("bad\"\r\nname.pdf")
	want := `attachment; filename="bad___name.pdf"; filename*=UTF-8''bad___name.pdf`
	if got != want {
		t.Fatalf("content disposition = %q, want %q", got, want)
	}
}
