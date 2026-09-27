package httpapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gogomail/gogomail/internal/auditretention"
	"github.com/gogomail/gogomail/internal/auth"
	"github.com/gogomail/gogomail/internal/maildb"
)

func TestAdminGetAuditRetentionPolicyKeepForever(t *testing.T) {
	t.Parallel()
	service := &fakeAdminService{auditRetentionPolicy: nil}
	mux := http.NewServeMux()
	RegisterAdminRoutes(mux, service, "static-token")

	req := httptest.NewRequest(http.MethodGet, "/admin/v1/companies/company-1/audit-retention", nil)
	req.Header.Set("Authorization", "Bearer static-token")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var body struct {
		Policy auditRetentionPolicyResponse `json:"audit_retention_policy"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if !body.Policy.KeepForever || body.Policy.RetentionDays != 0 {
		t.Fatalf("policy = %+v, want keep_forever", body.Policy)
	}
	if service.lastAuditRetentionCompanyID != "company-1" {
		t.Fatalf("company id = %q", service.lastAuditRetentionCompanyID)
	}
}

func TestAdminGetAuditRetentionPolicyReturnsPolicy(t *testing.T) {
	t.Parallel()
	service := &fakeAdminService{auditRetentionPolicy: &auditretention.Policy{
		CompanyID:     "company-1",
		RetentionDays: 90,
		UpdatedAt:     time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
	}}
	mux := http.NewServeMux()
	RegisterAdminRoutes(mux, service, "static-token")

	req := httptest.NewRequest(http.MethodGet, "/admin/v1/companies/company-1/audit-retention", nil)
	req.Header.Set("Authorization", "Bearer static-token")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var body struct {
		Policy auditRetentionPolicyResponse `json:"audit_retention_policy"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if body.Policy.KeepForever || body.Policy.RetentionDays != 90 {
		t.Fatalf("policy = %+v, want 90d", body.Policy)
	}
}

func TestAdminSetAuditRetentionPolicy(t *testing.T) {
	t.Parallel()
	service := &fakeAdminService{}
	mux := http.NewServeMux()
	RegisterAdminRoutes(mux, service, "static-token")

	req := httptest.NewRequest(http.MethodPut, "/admin/v1/companies/company-1/audit-retention", strings.NewReader(`{"retention_days":180}`))
	req.Header.Set("Authorization", "Bearer static-token")
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if service.lastAuditRetentionSetDays != 180 || service.lastAuditRetentionCompanyID != "company-1" {
		t.Fatalf("set days = %d, company = %q", service.lastAuditRetentionSetDays, service.lastAuditRetentionCompanyID)
	}
}

func TestAdminSetAuditRetentionPolicyRejectsInvalidDays(t *testing.T) {
	t.Parallel()
	service := &fakeAdminService{}
	mux := http.NewServeMux()
	RegisterAdminRoutes(mux, service, "static-token")

	req := httptest.NewRequest(http.MethodPut, "/admin/v1/companies/company-1/audit-retention", strings.NewReader(`{"retention_days":45}`))
	req.Header.Set("Authorization", "Bearer static-token")
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400; body = %s", rec.Code, rec.Body.String())
	}
	if service.lastAuditRetentionSetDays != 0 {
		t.Fatalf("service should not be called with invalid days, got %d", service.lastAuditRetentionSetDays)
	}
}

func TestAdminListAuditRetentionRuns(t *testing.T) {
	t.Parallel()
	service := &fakeAdminService{auditRetentionRuns: []auditretention.RunRecord{{
		ID:            "audit-retention-1",
		CompanyID:     "company-1",
		RetentionDays: 90,
		RowsDeleted:   42,
		Status:        auditretention.RunStatusCompleted,
	}}}
	mux := http.NewServeMux()
	RegisterAdminRoutes(mux, service, "static-token")

	req := httptest.NewRequest(http.MethodGet, "/admin/v1/companies/company-1/audit-retention/runs?limit=10&status=completed", nil)
	req.Header.Set("Authorization", "Bearer static-token")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var body struct {
		Runs []auditretention.RunRecord `json:"audit_retention_runs"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(body.Runs) != 1 || body.Runs[0].RowsDeleted != 42 {
		t.Fatalf("runs = %+v", body.Runs)
	}
	if service.lastAuditRetentionRunList.CompanyID != "company-1" || service.lastAuditRetentionRunList.Status != auditretention.RunStatusCompleted {
		t.Fatalf("run list request = %+v", service.lastAuditRetentionRunList)
	}
}

// TestAdminAuditRetentionTenantIsolation verifies a company_admin cannot read
// or mutate another company's audit retention policy.
func TestAdminAuditRetentionTenantIsolation(t *testing.T) {
	t.Parallel()
	manager, err := auth.NewTokenManager("admin-auth-secret-at-least-32bytes")
	if err != nil {
		t.Fatalf("NewTokenManager: %v", err)
	}
	service := &fakeAdminService{
		domains:         []maildb.DomainView{{ID: "domain-1", CompanyID: "company-1", Name: "example.com"}},
		users:           []maildb.UserView{{ID: "user-1", DomainID: "domain-1", Username: "admin", Role: "company_admin", Status: "active"}},
		sessionVersions: map[string]int64{"user-1": 1},
	}
	manager.SetRevocationChecker(service)
	mux := http.NewServeMux()
	RegisterAdminRoutes(mux, service, "", WithTokenManager(manager))

	// company_admin scoped to company-1.
	token, err := manager.Sign(auth.Claims{UserID: "user-1", DomainID: "domain-1", CompanyID: "company-1", Role: "company_admin", SessionVersion: 1}, time.Minute)
	if err != nil {
		t.Fatalf("Sign: %v", err)
	}

	cases := []struct {
		name   string
		method string
		path   string
		body   string
	}{
		{"get other company", http.MethodGet, "/admin/v1/companies/company-2/audit-retention", ""},
		{"put other company", http.MethodPut, "/admin/v1/companies/company-2/audit-retention", `{"retention_days":90}`},
		{"list other company runs", http.MethodGet, "/admin/v1/companies/company-2/audit-retention/runs", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var reqBody *strings.Reader
			if tc.body != "" {
				reqBody = strings.NewReader(tc.body)
			} else {
				reqBody = strings.NewReader("")
			}
			req := httptest.NewRequest(tc.method, tc.path, reqBody)
			req.Header.Set("Authorization", "Bearer "+token)
			if tc.body != "" {
				req.Header.Set("Content-Type", "application/json")
			}
			rec := httptest.NewRecorder()
			mux.ServeHTTP(rec, req)
			if rec.Code != http.StatusForbidden {
				t.Fatalf("status = %d, want 403 (tenant isolation); body = %s", rec.Code, rec.Body.String())
			}
		})
	}

	// The same admin CAN access its own company.
	req := httptest.NewRequest(http.MethodGet, "/admin/v1/companies/company-1/audit-retention", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("own-company status = %d, want 200; body = %s", rec.Code, rec.Body.String())
	}
}
