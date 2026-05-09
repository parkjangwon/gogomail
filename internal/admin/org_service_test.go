package admin

import (
	"context"
	"fmt"
	"testing"
	"time"
)

// OrganizationUnit represents a department, team, or division
type OrganizationUnit struct {
	ID           string `json:"id"`
	CompanyID    string `json:"company_id"`
	ParentID     string `json:"parent_id,omitempty"`
	Name         string `json:"name"`
	Type         string `json:"type"` // department, team, division
	Description  string `json:"description,omitempty"`
	ManagerID    string `json:"manager_id,omitempty"`
	Status       string `json:"status"` // active, archived
}

// OrgUnitFilter holds query parameters for org unit listing
type OrgUnitFilter struct {
	CompanyID string
	ParentID  string
	Type      string
	Status    string
	Limit     int
	Offset    int
}

// OrgRepository defines org data access operations
type OrgRepository interface {
	CreateUnit(ctx context.Context, unit *OrganizationUnit) error
	GetUnit(ctx context.Context, unitID string) (*OrganizationUnit, error)
	ListUnits(ctx context.Context, filter *OrgUnitFilter) ([]*OrganizationUnit, int64, error)
	UpdateUnit(ctx context.Context, unit *OrganizationUnit) error
	DeleteUnit(ctx context.Context, unitID string) error
	GetUnitsByParent(ctx context.Context, companyID, parentID string) ([]*OrganizationUnit, error)
}

// Mock repository for testing
type mockOrgRepository struct {
	units map[string]*OrganizationUnit
}

func newMockOrgRepository() *mockOrgRepository {
	return &mockOrgRepository{
		units: make(map[string]*OrganizationUnit),
	}
}

func (m *mockOrgRepository) CreateUnit(ctx context.Context, unit *OrganizationUnit) error {
	if unit.ID == "" {
		unit.ID = "org-" + time.Now().Format("20060102150405")
	}
	m.units[unit.ID] = unit
	return nil
}

func (m *mockOrgRepository) GetUnit(ctx context.Context, unitID string) (*OrganizationUnit, error) {
	if unit, ok := m.units[unitID]; ok {
		return unit, nil
	}
	return nil, ErrOrgUnitNotFound
}

func (m *mockOrgRepository) ListUnits(ctx context.Context, filter *OrgUnitFilter) ([]*OrganizationUnit, int64, error) {
	var filtered []*OrganizationUnit
	for _, unit := range m.units {
		if unit.CompanyID == filter.CompanyID {
			if filter.ParentID != "" && unit.ParentID != filter.ParentID {
				continue
			}
			if filter.Status != "" && unit.Status != filter.Status {
				continue
			}
			filtered = append(filtered, unit)
		}
	}
	return filtered, int64(len(filtered)), nil
}

func (m *mockOrgRepository) UpdateUnit(ctx context.Context, unit *OrganizationUnit) error {
	if _, ok := m.units[unit.ID]; !ok {
		return ErrOrgUnitNotFound
	}
	m.units[unit.ID] = unit
	return nil
}

func (m *mockOrgRepository) DeleteUnit(ctx context.Context, unitID string) error {
	delete(m.units, unitID)
	return nil
}

func (m *mockOrgRepository) GetUnitsByParent(ctx context.Context, companyID, parentID string) ([]*OrganizationUnit, error) {
	var children []*OrganizationUnit
	for _, unit := range m.units {
		if unit.CompanyID == companyID && unit.ParentID == parentID {
			children = append(children, unit)
		}
	}
	return children, nil
}

var (
	ErrOrgUnitNotFound = fmt.Errorf("organization unit not found")
	ErrInvalidUnitType = fmt.Errorf("invalid unit type")
)

func TestCreateUnit(t *testing.T) {
	repo := newMockOrgRepository()
	svc := NewOrgService(repo)
	ctx := context.Background()

	tests := []struct {
		name      string
		unit      *OrganizationUnit
		shouldErr bool
	}{
		{
			name: "valid unit creation",
			unit: &OrganizationUnit{
				CompanyID:   "company-1",
				Name:        "Engineering",
				Type:        "department",
				Description: "Engineering department",
				Status:      "active",
			},
			shouldErr: false,
		},
		{
			name: "missing companyID",
			unit: &OrganizationUnit{
				Name:   "Engineering",
				Type:   "department",
				Status: "active",
			},
			shouldErr: true,
		},
		{
			name: "missing name",
			unit: &OrganizationUnit{
				CompanyID: "company-1",
				Type:      "department",
				Status:    "active",
			},
			shouldErr: true,
		},
		{
			name: "invalid type",
			unit: &OrganizationUnit{
				CompanyID: "company-1",
				Name:      "Engineering",
				Type:      "invalid",
				Status:    "active",
			},
			shouldErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := svc.CreateUnit(ctx, tt.unit)
			if (err != nil) != tt.shouldErr {
				t.Errorf("CreateUnit() error = %v, shouldErr %v", err, tt.shouldErr)
			}
		})
	}
}

func TestHierarchy(t *testing.T) {
	repo := newMockOrgRepository()
	svc := NewOrgService(repo)
	ctx := context.Background()

	// Create parent unit
	parent := &OrganizationUnit{
		CompanyID: "company-1",
		Name:      "Engineering",
		Type:      "department",
		Status:    "active",
	}
	svc.CreateUnit(ctx, parent)
	parentID := parent.ID

	// Create child unit
	child := &OrganizationUnit{
		CompanyID: "company-1",
		ParentID:  parentID,
		Name:      "Backend",
		Type:      "team",
		Status:    "active",
	}
	svc.CreateUnit(ctx, child)

	// Verify parent-child relationship
	units, err := repo.GetUnitsByParent(ctx, "company-1", parentID)
	if err != nil {
		t.Errorf("GetUnitsByParent() error = %v", err)
	}
	if len(units) != 1 {
		t.Errorf("GetUnitsByParent() returned %d units, expected 1", len(units))
	}
}

func TestDeleteUnit(t *testing.T) {
	repo := newMockOrgRepository()
	svc := NewOrgService(repo)
	ctx := context.Background()

	// Create unit
	unit := &OrganizationUnit{
		CompanyID: "company-1",
		Name:      "Engineering",
		Type:      "department",
		Status:    "active",
	}
	svc.CreateUnit(ctx, unit)
	unitID := unit.ID

	// Delete unit
	err := svc.DeleteUnit(ctx, unitID)
	if err != nil {
		t.Errorf("DeleteUnit() error = %v", err)
	}

	// Verify unit is gone
	_, err = repo.GetUnit(ctx, unitID)
	if err == nil {
		t.Error("DeleteUnit() failed - unit still exists")
	}
}
