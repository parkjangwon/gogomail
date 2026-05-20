package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gogomail/gogomail/internal/audit"
)

type mockAuditRepository struct {
	logs    map[string]audit.Log
	getErr  error
	listErr error
}

func (m *mockAuditRepository) GetByID(ctx context.Context, id string) (*audit.Log, error) {
	if m.getErr != nil {
		return nil, m.getErr
	}
	if log, ok := m.logs[id]; ok {
		return &log, nil
	}
	return nil, nil
}

func (m *mockAuditRepository) ListWithFilters(ctx context.Context, filters audit.ListFilters) ([]audit.Log, error) {
	if m.listErr != nil {
		return nil, m.listErr
	}
	var result []audit.Log
	for _, log := range m.logs {
		if filters.CompanyID != "" && log.CompanyID != filters.CompanyID {
			continue
		}
		if filters.DomainID != "" && log.DomainID != filters.DomainID {
			continue
		}
		if filters.Category != "" && log.Category != filters.Category {
			continue
		}
		result = append(result, log)
	}
	return result, nil
}

func TestAuditLogGet(t *testing.T) {
	mockRepo := &mockAuditRepository{
		logs: map[string]audit.Log{
			"test-id-1": {
				CompanyID:  "company-1",
				DomainID:   "domain-1",
				Category:   "mail",
				Action:     "mail.received",
				TargetType: "message",
				TargetID:   "msg-1",
				Result:     "success",
				Detail:     json.RawMessage(`{"recipient":"test@example.com"}`),
				CreatedAt:  time.Now().UTC(),
			},
		},
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /admin/v1/audit-logs/{id}", handleAuditLogGet(mockRepo))

	server := httptest.NewServer(mux)
	defer server.Close()

	// Test successful get
	resp, err := http.Get(server.URL + "/admin/v1/audit-logs/test-id-1")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}

	var respLog auditLogResponse
	if err := json.NewDecoder(resp.Body).Decode(&respLog); err != nil {
		t.Fatal(err)
	}

	if respLog.Category != "mail" {
		t.Errorf("expected category 'mail', got %q", respLog.Category)
	}

	// Test 404
	resp, err = http.Get(server.URL + "/admin/v1/audit-logs/nonexistent")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("expected 404, got %d", resp.StatusCode)
	}
}

func TestAuditLogList(t *testing.T) {
	mockRepo := &mockAuditRepository{
		logs: map[string]audit.Log{
			"log-1": {
				CompanyID:  "company-1",
				DomainID:   "domain-1",
				Category:   "mail",
				Action:     "mail.received",
				TargetType: "message",
				Result:     "success",
				CreatedAt:  time.Now().UTC(),
			},
			"log-2": {
				CompanyID:  "company-1",
				DomainID:   "domain-1",
				Category:   "dav",
				Action:     "calendar.changed",
				TargetType: "calendar",
				Result:     "success",
				CreatedAt:  time.Now().UTC().Add(-1 * time.Hour),
			},
		},
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /admin/v1/audit-logs", handleAuditLogList(mockRepo))

	server := httptest.NewServer(mux)
	defer server.Close()

	resp, err := http.Get(server.URL + "/admin/v1/audit-logs")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}

	var respList auditLogListResponse
	if err := json.NewDecoder(resp.Body).Decode(&respList); err != nil {
		t.Fatal(err)
	}

	if len(respList.AuditLogs) == 0 {
		t.Error("expected audit logs, got none")
	}

	// Test with filters
	resp, err = http.Get(server.URL + "/admin/v1/audit-logs?category=mail")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	var filteredList auditLogListResponse
	if err := json.NewDecoder(resp.Body).Decode(&filteredList); err != nil {
		t.Fatal(err)
	}

	if len(filteredList.AuditLogs) != 1 {
		t.Errorf("expected 1 filtered log, got %d", len(filteredList.AuditLogs))
	}
}

func TestAuditLogHandlersHideInternalErrors(t *testing.T) {
	t.Parallel()

	secretErr := errors.New("postgres password=secret query failed")
	tests := []struct {
		name    string
		handler http.HandlerFunc
		path    string
	}{
		{
			name:    "get",
			handler: handleAuditLogGet(&mockAuditRepository{getErr: secretErr}),
			path:    "/admin/v1/audit-logs/log-1",
		},
		{
			name:    "list",
			handler: handleAuditLogList(&mockAuditRepository{listErr: secretErr}),
			path:    "/admin/v1/audit-logs",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tc.path, nil)
			if tc.name == "get" {
				req.SetPathValue("id", "log-1")
			}
			rec := httptest.NewRecorder()

			tc.handler(rec, req)

			if rec.Code != http.StatusInternalServerError {
				t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
			}
			body, err := io.ReadAll(rec.Result().Body)
			if err != nil {
				t.Fatalf("read body: %v", err)
			}
			if strings.Contains(string(body), "password=secret") || strings.Contains(string(body), "query failed") {
				t.Fatalf("internal error leaked to response: %q", string(body))
			}
			if !strings.Contains(string(body), "internal server error") {
				t.Fatalf("body = %q, want generic internal server error", string(body))
			}
		})
	}
}

func TestRegisterAuditLogRoutesRejectsMissingAuthConfig(t *testing.T) {
	t.Parallel()

	mux := http.NewServeMux()
	RegisterAuditLogRoutes(mux, &mockAuditRepository{}, "")

	req := httptest.NewRequest(http.MethodGet, "/admin/v1/audit-logs", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
}
