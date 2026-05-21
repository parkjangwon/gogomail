package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gogomail/gogomail/internal/orgchart"
)

type mockOrgChartService struct {
	units   map[string]*orgchart.OrganizationUnit
	members map[string]*orgchart.OrganizationMember
	syncErr error
}

var _ OrgChartService = (*orgchart.Service)(nil)

func newMockOrgChartService() *mockOrgChartService {
	return &mockOrgChartService{
		units:   make(map[string]*orgchart.OrganizationUnit),
		members: make(map[string]*orgchart.OrganizationMember),
	}
}

func (m *mockOrgChartService) CreateUnit(ctx context.Context, unit *orgchart.OrganizationUnit) error {
	if unit.CompanyID == "" || unit.Name == "" {
		return fmt.Errorf("company_id and name are required")
	}
	unit.ID = fmt.Sprintf("unit-%d", len(m.units))
	unit.CreatedAt = time.Now()
	unit.UpdatedAt = time.Now()
	m.units[unit.ID] = unit
	return nil
}

func (m *mockOrgChartService) GetUnit(ctx context.Context, id string) (*orgchart.OrganizationUnit, error) {
	if u, ok := m.units[id]; ok {
		return u, nil
	}
	return nil, fmt.Errorf("not found")
}

func (m *mockOrgChartService) ListUnits(ctx context.Context, companyID string) ([]orgchart.OrganizationUnit, error) {
	var result []orgchart.OrganizationUnit
	for _, u := range m.units {
		if u.CompanyID == companyID {
			result = append(result, *u)
		}
	}
	return result, nil
}

func (m *mockOrgChartService) UpdateUnit(ctx context.Context, unit *orgchart.OrganizationUnit) error {
	if _, ok := m.units[unit.ID]; ok {
		unit.UpdatedAt = time.Now()
		m.units[unit.ID] = unit
		return nil
	}
	return fmt.Errorf("not found")
}

func (m *mockOrgChartService) DeleteUnit(ctx context.Context, id string) error {
	delete(m.units, id)
	return nil
}

func (m *mockOrgChartService) AssignUserToUnit(ctx context.Context, unitID, userID string, role string) error {
	member := &orgchart.OrganizationMember{
		ID:                 fmt.Sprintf("member-%d", len(m.members)),
		OrganizationUnitID: unitID,
		UserID:             userID,
		Role:               role,
		StartedAt:          time.Now(),
		IsPrimary:          true,
		CreatedAt:          time.Now(),
		UpdatedAt:          time.Now(),
	}
	m.members[member.ID] = member
	return nil
}

func (m *mockOrgChartService) RemoveUserFromUnit(ctx context.Context, memberID string) error {
	if mem, ok := m.members[memberID]; ok {
		now := time.Now()
		mem.EndedAt = &now
		return nil
	}
	return fmt.Errorf("not found")
}

func (m *mockOrgChartService) GetHierarchy(ctx context.Context, companyID string) (*orgchart.OrganizationHierarchy, error) {
	for _, u := range m.units {
		if u.CompanyID == companyID && u.ParentID == nil {
			return &orgchart.OrganizationHierarchy{Unit: u}, nil
		}
	}
	return nil, fmt.Errorf("no root unit found")
}

func (m *mockOrgChartService) SyncWithLDAP(ctx context.Context, companyID string) (*orgchart.SyncLog, error) {
	if m.syncErr != nil {
		now := time.Now()
		return &orgchart.SyncLog{
			ID:           "sync-1",
			CompanyID:    companyID,
			Status:       "failed",
			ErrorMessage: m.syncErr.Error(),
			StartedAt:    now,
			CompletedAt:  &now,
		}, m.syncErr
	}
	return &orgchart.SyncLog{
		ID:        "sync-1",
		CompanyID: companyID,
		Status:    "success",
		StartedAt: time.Now(),
	}, nil
}

func TestListUnitsEndpoint(t *testing.T) {
	service := newMockOrgChartService()
	mux := http.NewServeMux()
	RegisterOrgChartRoutes(mux, service, "test-token")

	service.CreateUnit(context.Background(), &orgchart.OrganizationUnit{
		CompanyID: "company-1",
		Name:      "Engineering",
		Type:      "department",
		Status:    "active",
	})

	req := httptest.NewRequest("GET", "/admin/v1/organization/units?company_id=company-1", nil)
	req.Header.Set("X-Admin-Token", "test-token")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestCreateUnitEndpoint(t *testing.T) {
	service := newMockOrgChartService()
	mux := http.NewServeMux()
	RegisterOrgChartRoutes(mux, service, "test-token")

	body := bytes.NewBufferString(`{
		"company_id": "company-1",
		"name": "Engineering",
		"type": "department",
		"status": "active"
	}`)

	req := httptest.NewRequest("POST", "/admin/v1/organization/units", body)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Admin-Token", "test-token")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d", w.Code)
	}
}

func TestGetUnitEndpoint(t *testing.T) {
	service := newMockOrgChartService()
	mux := http.NewServeMux()
	RegisterOrgChartRoutes(mux, service, "test-token")

	unit := &orgchart.OrganizationUnit{
		CompanyID: "company-1",
		Name:      "Engineering",
		Type:      "department",
		Status:    "active",
	}
	service.CreateUnit(context.Background(), unit)

	req := httptest.NewRequest("GET", fmt.Sprintf("/admin/v1/organization/units/%s", unit.ID), nil)
	req.Header.Set("X-Admin-Token", "test-token")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestUpdateUnitEndpoint(t *testing.T) {
	service := newMockOrgChartService()
	mux := http.NewServeMux()
	RegisterOrgChartRoutes(mux, service, "test-token")

	unit := &orgchart.OrganizationUnit{
		CompanyID: "company-1",
		Name:      "Engineering",
		Type:      "department",
		Status:    "active",
	}
	service.CreateUnit(context.Background(), unit)

	body := bytes.NewBufferString(`{
		"company_id": "company-1",
		"name": "Tech",
		"type": "department",
		"status": "active"
	}`)

	req := httptest.NewRequest("PUT", fmt.Sprintf("/admin/v1/organization/units/%s", unit.ID), body)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Admin-Token", "test-token")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestDeleteUnitEndpoint(t *testing.T) {
	service := newMockOrgChartService()
	mux := http.NewServeMux()
	RegisterOrgChartRoutes(mux, service, "test-token")

	unit := &orgchart.OrganizationUnit{
		CompanyID: "company-1",
		Name:      "Engineering",
		Type:      "department",
		Status:    "active",
	}
	service.CreateUnit(context.Background(), unit)

	req := httptest.NewRequest("DELETE", fmt.Sprintf("/admin/v1/organization/units/%s", unit.ID), nil)
	req.Header.Set("X-Admin-Token", "test-token")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", w.Code)
	}
}

func TestGetHierarchyEndpoint(t *testing.T) {
	service := newMockOrgChartService()
	mux := http.NewServeMux()
	RegisterOrgChartRoutes(mux, service, "test-token")

	unit := &orgchart.OrganizationUnit{
		CompanyID: "company-1",
		Name:      "Engineering",
		Type:      "department",
		Status:    "active",
	}
	service.CreateUnit(context.Background(), unit)

	req := httptest.NewRequest("GET", "/admin/v1/organization/hierarchy?company_id=company-1", nil)
	req.Header.Set("X-Admin-Token", "test-token")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestAssignUserEndpoint(t *testing.T) {
	service := newMockOrgChartService()
	mux := http.NewServeMux()
	RegisterOrgChartRoutes(mux, service, "test-token")

	unit := &orgchart.OrganizationUnit{
		CompanyID: "company-1",
		Name:      "Engineering",
		Type:      "department",
		Status:    "active",
	}
	service.CreateUnit(context.Background(), unit)

	body := bytes.NewBufferString(fmt.Sprintf(`{
		"unit_id": "%s",
		"user_id": "user-1",
		"role": "member"
	}`, unit.ID))

	req := httptest.NewRequest("POST", "/admin/v1/organization/members", body)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Admin-Token", "test-token")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d", w.Code)
	}
}

func TestSyncLDAPEndpoint(t *testing.T) {
	service := newMockOrgChartService()
	mux := http.NewServeMux()
	RegisterOrgChartRoutes(mux, service, "test-token")

	req := httptest.NewRequest("POST", "/admin/v1/organization/sync?company_id=company-1", nil)
	req.Header.Set("X-Admin-Token", "test-token")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d", w.Code)
	}

	var result map[string]any
	json.NewDecoder(w.Body).Decode(&result)
	if _, ok := result["sync_log"]; !ok {
		t.Fatal("response should contain sync_log")
	}
}

func TestSyncLDAPEndpointReportsUnconfiguredAdapter(t *testing.T) {
	service := newMockOrgChartService()
	service.syncErr = orgchart.ErrOrgChartSyncNotConfigured
	mux := http.NewServeMux()
	RegisterOrgChartRoutes(mux, service, "test-token")

	req := httptest.NewRequest("POST", "/admin/v1/organization/sync?company_id=company-1", nil)
	req.Header.Set("X-Admin-Token", "test-token")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusNotImplemented {
		t.Fatalf("expected 501, got %d, body = %s", w.Code, w.Body.String())
	}
	if bytes.Contains(w.Body.Bytes(), []byte(orgchart.ErrOrgChartSyncNotConfigured.Error())) {
		t.Fatalf("response body = %q, leaked backend sentinel error", w.Body.String())
	}
	if !bytes.Contains(w.Body.Bytes(), []byte("organization sync is not configured")) {
		t.Fatalf("response body = %q, want public not-configured error", w.Body.String())
	}
}
