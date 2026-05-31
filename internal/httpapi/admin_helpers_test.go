package httpapi

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestParseBoundedAdminQuery(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		target     string
		wantValue  string
		wantOK     bool
		wantStatus int
		wantBody   string
	}{
		{
			name:      "missing optional query is ok",
			target:    "/admin/v1/example",
			wantValue: "",
			wantOK:    true,
		},
		{
			name:      "trims query value",
			target:    "/admin/v1/example?company_id=%20company-1%20",
			wantValue: "company-1",
			wantOK:    true,
		},
		{
			name:       "rejects repeated query value",
			target:     "/admin/v1/example?company_id=a&company_id=b",
			wantOK:     false,
			wantStatus: http.StatusBadRequest,
			wantBody:   "company_id must not be repeated",
		},
		{
			name:       "rejects CRLF query value",
			target:     "/admin/v1/example?company_id=company%0A1",
			wantOK:     false,
			wantStatus: http.StatusBadRequest,
			wantBody:   "company_id must not contain CR or LF",
		},
		{
			name:       "rejects oversized query value",
			target:     "/admin/v1/example?company_id=" + strings.Repeat("x", maxAdminQueryFilterBytes+1),
			wantOK:     false,
			wantStatus: http.StatusBadRequest,
			wantBody:   "company_id is too long",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			rec := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, tt.target, nil)
			got, ok := parseBoundedAdminQuery(rec, req, "company_id")
			if ok != tt.wantOK {
				t.Fatalf("ok = %v, want %v", ok, tt.wantOK)
			}
			if got != tt.wantValue {
				t.Fatalf("value = %q, want %q", got, tt.wantValue)
			}
			if tt.wantStatus != 0 && rec.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d, body = %s", rec.Code, tt.wantStatus, rec.Body.String())
			}
			if tt.wantBody != "" && !strings.Contains(rec.Body.String(), tt.wantBody) {
				t.Fatalf("body = %q, want substring %q", rec.Body.String(), tt.wantBody)
			}
		})
	}
}

func TestParseBoundedAdminPathValue(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		value      string
		wantValue  string
		wantOK     bool
		wantStatus int
		wantBody   string
	}{
		{
			name:      "trims path value",
			value:     " company-1 ",
			wantValue: "company-1",
			wantOK:    true,
		},
		{
			name:       "requires path value",
			value:      "   ",
			wantOK:     false,
			wantStatus: http.StatusBadRequest,
			wantBody:   "id is required",
		},
		{
			name:       "rejects CRLF path value",
			value:      "company\r1",
			wantOK:     false,
			wantStatus: http.StatusBadRequest,
			wantBody:   "id must not contain CR or LF",
		},
		{
			name:       "rejects oversized path value",
			value:      strings.Repeat("x", maxAdminQueryFilterBytes+1),
			wantOK:     false,
			wantStatus: http.StatusBadRequest,
			wantBody:   "id is too long",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			rec := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, "/admin/v1/example/value", nil)
			req.SetPathValue("id", tt.value)
			got, ok := parseBoundedAdminPathValue(rec, req, "id")
			if ok != tt.wantOK {
				t.Fatalf("ok = %v, want %v", ok, tt.wantOK)
			}
			if got != tt.wantValue {
				t.Fatalf("value = %q, want %q", got, tt.wantValue)
			}
			if tt.wantStatus != 0 && rec.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d, body = %s", rec.Code, tt.wantStatus, rec.Body.String())
			}
			if tt.wantBody != "" && !strings.Contains(rec.Body.String(), tt.wantBody) {
				t.Fatalf("body = %q, want substring %q", rec.Body.String(), tt.wantBody)
			}
		})
	}
}
