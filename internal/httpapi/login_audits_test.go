package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gogomail/gogomail/internal/admin"
)

type mockLoginAuditService struct {
	lastFilter admin.LoginAuditFilter
	logs       []admin.LoginAuditLog
}

func (m *mockLoginAuditService) ListLoginAttempts(ctx context.Context, filter admin.LoginAuditFilter) ([]admin.LoginAuditLog, error) {
	m.lastFilter = filter
	return m.logs, nil
}

func TestCompanyLoginAuditList(t *testing.T) {
	mockSvc := &mockLoginAuditService{
		logs: []admin.LoginAuditLog{
			{
				ID:            "login-1",
				UserID:        "user-1",
				CompanyID:     "company-1",
				IPAddress:     "127.0.0.1",
				UserAgent:     "Mozilla/5.0",
				Success:       false,
				FailureReason: "bad password",
				Timestamp:     time.Date(2026, time.May, 15, 10, 0, 0, 0, time.UTC),
			},
		},
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /admin/v1/companies/{id}/security/login-audits", handleCompanyLoginAudits(mockSvc))

	server := httptest.NewServer(mux)
	defer server.Close()

	resp, err := http.Get(server.URL + "/admin/v1/companies/company-1/security/login-audits?user_id=user-1&success=false&limit=10&offset=5&from_date=2026-05-01T00:00:00Z&to_date=2026-05-31T23:59:59Z")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d", resp.StatusCode)
	}

	var body struct {
		LoginAudits []struct {
			ID            string `json:"id"`
			UserID        string `json:"user_id"`
			CompanyID     string `json:"company_id"`
			IPAddress     string `json:"ip_address"`
			UserAgent     string `json:"user_agent"`
			Success       bool   `json:"success"`
			FailureReason string `json:"failure_reason"`
			Timestamp     string `json:"timestamp"`
		} `json:"login_audits"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if len(body.LoginAudits) != 1 || body.LoginAudits[0].ID != "login-1" {
		t.Fatalf("body = %+v", body)
	}
	if mockSvc.lastFilter.CompanyID != "company-1" || mockSvc.lastFilter.UserID != "user-1" || mockSvc.lastFilter.Success == nil || *mockSvc.lastFilter.Success {
		t.Fatalf("filter = %+v", mockSvc.lastFilter)
	}
	if mockSvc.lastFilter.Limit != 10 || mockSvc.lastFilter.Offset != 5 {
		t.Fatalf("pagination = %+v", mockSvc.lastFilter)
	}
	if mockSvc.lastFilter.StartTime == nil || mockSvc.lastFilter.EndTime == nil {
		t.Fatalf("time filters = %+v", mockSvc.lastFilter)
	}
}
