package orgchart

import (
	"context"
	"fmt"
	"testing"
	"time"
)

type mockRepository struct {
	units   map[string]*OrganizationUnit
	members map[string]*OrganizationMember
}

func newMockRepository() *mockRepository {
	return &mockRepository{
		units:   make(map[string]*OrganizationUnit),
		members: make(map[string]*OrganizationMember),
	}
}

func (m *mockRepository) CreateUnit(ctx context.Context, unit *OrganizationUnit) error {
	if unit.CompanyID == "" || unit.Name == "" {
		return fmt.Errorf("company_id and name are required")
	}
	unit.ID = fmt.Sprintf("unit-%d", len(m.units))
	unit.CreatedAt = time.Now()
	unit.UpdatedAt = time.Now()
	m.units[unit.ID] = unit
	return nil
}

func (m *mockRepository) GetUnit(ctx context.Context, id string) (*OrganizationUnit, error) {
	if u, ok := m.units[id]; ok {
		return u, nil
	}
	return nil, fmt.Errorf("not found")
}

func (m *mockRepository) ListUnits(ctx context.Context, companyID string) ([]OrganizationUnit, error) {
	var result []OrganizationUnit
	for _, u := range m.units {
		if u.CompanyID == companyID {
			result = append(result, *u)
		}
	}
	return result, nil
}

func (m *mockRepository) UpdateUnit(ctx context.Context, unit *OrganizationUnit) error {
	if _, ok := m.units[unit.ID]; ok {
		unit.UpdatedAt = time.Now()
		m.units[unit.ID] = unit
		return nil
	}
	return fmt.Errorf("not found")
}

func (m *mockRepository) DeleteUnit(ctx context.Context, id string) error {
	delete(m.units, id)
	return nil
}

func (m *mockRepository) AssignUser(ctx context.Context, member *OrganizationMember) error {
	member.ID = fmt.Sprintf("member-%d", len(m.members))
	member.CreatedAt = time.Now()
	member.UpdatedAt = time.Now()
	m.members[member.ID] = member
	return nil
}

func (m *mockRepository) GetMembersInUnit(ctx context.Context, unitID string) ([]OrganizationMember, error) {
	var result []OrganizationMember
	for _, mem := range m.members {
		if mem.OrganizationUnitID == unitID && mem.EndedAt == nil {
			result = append(result, *mem)
		}
	}
	return result, nil
}

func (m *mockRepository) RemoveUser(ctx context.Context, memberID string) error {
	if mem, ok := m.members[memberID]; ok {
		now := time.Now()
		mem.EndedAt = &now
		return nil
	}
	return fmt.Errorf("not found")
}

func (m *mockRepository) LogSync(ctx context.Context, log *SyncLog) error {
	log.ID = fmt.Sprintf("sync-%d", time.Now().Unix())
	log.CreatedAt = time.Now()
	return nil
}

func (m *mockRepository) UpdateSyncLog(ctx context.Context, log *SyncLog) error {
	return nil
}

type mockAdapter struct {
	shouldFail bool
}

func (ma *mockAdapter) SyncOrgChart(ctx context.Context) error {
	if ma.shouldFail {
		return fmt.Errorf("sync failed")
	}
	return nil
}

func TestServiceCreateUnitValidation(t *testing.T) {
	repo := newMockRepository()
	svc := NewService(repo, nil)
	ctx := context.Background()

	// Missing company_id
	unit := &OrganizationUnit{Name: "Test"}
	err := svc.CreateUnit(ctx, unit)
	if err == nil {
		t.Fatal("should fail with missing company_id")
	}

	// Missing name
	unit = &OrganizationUnit{CompanyID: "company-1"}
	err = svc.CreateUnit(ctx, unit)
	if err == nil {
		t.Fatal("should fail with missing name")
	}
}

func TestServiceCreateUnitDefaults(t *testing.T) {
	repo := newMockRepository()
	svc := NewService(repo, nil)
	ctx := context.Background()

	unit := &OrganizationUnit{
		CompanyID: "company-1",
		Name:      "Engineering",
	}
	err := svc.CreateUnit(ctx, unit)
	if err != nil {
		t.Fatalf("CreateUnit failed: %v", err)
	}

	if unit.Type != "department" {
		t.Fatalf("expected default type 'department', got %s", unit.Type)
	}
	if unit.Status != "active" {
		t.Fatalf("expected default status 'active', got %s", unit.Status)
	}
}

func TestServiceGetHierarchy(t *testing.T) {
	repo := newMockRepository()
	svc := NewService(repo, nil)
	ctx := context.Background()

	companyID := "company-1"

	// Create root unit
	root := &OrganizationUnit{
		CompanyID: companyID,
		Name:      "Root",
		Type:      "department",
		Status:    "active",
	}
	svc.CreateUnit(ctx, root)

	// Create child unit
	child := &OrganizationUnit{
		CompanyID: companyID,
		Name:      "Child",
		Type:      "team",
		Status:    "active",
		ParentID:  &root.ID,
	}
	svc.CreateUnit(ctx, child)

	hierarchy, err := svc.GetHierarchy(ctx, companyID)
	if err != nil {
		t.Fatalf("GetHierarchy failed: %v", err)
	}

	if hierarchy.Unit.ID != root.ID {
		t.Fatalf("root unit ID mismatch")
	}

	if len(hierarchy.Children) != 1 {
		t.Fatalf("expected 1 child, got %d", len(hierarchy.Children))
	}

	if hierarchy.Children[0].Unit.ID != child.ID {
		t.Fatalf("child unit ID mismatch")
	}
}

func TestServiceGetHierarchyNoRoot(t *testing.T) {
	repo := newMockRepository()
	svc := NewService(repo, nil)
	ctx := context.Background()

	_, err := svc.GetHierarchy(ctx, "nonexistent")
	if err == nil {
		t.Fatal("should fail with no root units")
	}
}

func TestServiceSyncWithLDAPSuccess(t *testing.T) {
	repo := newMockRepository()
	adapter := &mockAdapter{shouldFail: false}
	svc := NewService(repo, adapter)
	ctx := context.Background()

	log, err := svc.SyncWithLDAP(ctx, "company-1")
	if err != nil {
		t.Fatalf("SyncWithLDAP failed: %v", err)
	}

	if log.Status != "success" {
		t.Fatalf("expected status 'success', got %s", log.Status)
	}
	if log.CompletedAt == nil {
		t.Fatal("CompletedAt should be set")
	}
}

func TestServiceSyncWithLDAPFailure(t *testing.T) {
	repo := newMockRepository()
	adapter := &mockAdapter{shouldFail: true}
	svc := NewService(repo, adapter)
	ctx := context.Background()

	log, err := svc.SyncWithLDAP(ctx, "company-1")
	if err == nil {
		t.Fatal("should fail when adapter fails")
	}

	if log.Status != "failed" {
		t.Fatalf("expected status 'failed', got %s", log.Status)
	}
	if log.ErrorMessage == "" {
		t.Fatal("ErrorMessage should be set")
	}
}

func TestServiceAssignUserToUnit(t *testing.T) {
	repo := newMockRepository()
	svc := NewService(repo, nil)
	ctx := context.Background()

	unit := &OrganizationUnit{
		CompanyID: "company-1",
		Name:      "Engineering",
		Type:      "department",
		Status:    "active",
	}
	svc.CreateUnit(ctx, unit)

	err := svc.AssignUserToUnit(ctx, unit.ID, "user-1", "manager")
	if err != nil {
		t.Fatalf("AssignUserToUnit failed: %v", err)
	}

	members, _ := repo.GetMembersInUnit(ctx, unit.ID)
	if len(members) != 1 {
		t.Fatalf("expected 1 member, got %d", len(members))
	}
	if members[0].Role != "manager" {
		t.Fatalf("expected role 'manager', got %s", members[0].Role)
	}
}

func TestServiceAssignUserToUnitDefaults(t *testing.T) {
	repo := newMockRepository()
	svc := NewService(repo, nil)
	ctx := context.Background()

	unit := &OrganizationUnit{
		CompanyID: "company-1",
		Name:      "Engineering",
		Type:      "department",
		Status:    "active",
	}
	svc.CreateUnit(ctx, unit)

	err := svc.AssignUserToUnit(ctx, unit.ID, "user-1", "")
	if err != nil {
		t.Fatalf("AssignUserToUnit failed: %v", err)
	}

	members, _ := repo.GetMembersInUnit(ctx, unit.ID)
	if members[0].Role != "member" {
		t.Fatalf("expected default role 'member', got %s", members[0].Role)
	}
}

func TestServiceRemoveUserFromUnit(t *testing.T) {
	repo := newMockRepository()
	svc := NewService(repo, nil)
	ctx := context.Background()

	unit := &OrganizationUnit{
		CompanyID: "company-1",
		Name:      "Engineering",
		Type:      "department",
		Status:    "active",
	}
	svc.CreateUnit(ctx, unit)
	svc.AssignUserToUnit(ctx, unit.ID, "user-1", "member")

	members, _ := repo.GetMembersInUnit(ctx, unit.ID)
	memberID := members[0].ID

	err := svc.RemoveUserFromUnit(ctx, memberID)
	if err != nil {
		t.Fatalf("RemoveUserFromUnit failed: %v", err)
	}

	members, _ = repo.GetMembersInUnit(ctx, unit.ID)
	if len(members) != 0 {
		t.Fatalf("expected 0 members after removal, got %d", len(members))
	}
}

func TestServiceUpdateUnit(t *testing.T) {
	repo := newMockRepository()
	svc := NewService(repo, nil)
	ctx := context.Background()

	unit := &OrganizationUnit{
		CompanyID: "company-1",
		Name:      "Engineering",
		Type:      "department",
		Status:    "active",
	}
	svc.CreateUnit(ctx, unit)

	unit.Name = "Tech"
	err := svc.UpdateUnit(ctx, unit)
	if err != nil {
		t.Fatalf("UpdateUnit failed: %v", err)
	}

	updated, _ := repo.GetUnit(ctx, unit.ID)
	if updated.Name != "Tech" {
		t.Fatalf("name not updated: %s", updated.Name)
	}
}
