package orgchart

import (
	"context"
	"fmt"
	"time"
)

const (
	// CapabilityStatusPlaceholder marks organization sync as a product-visible
	// placeholder until a real external directory adapter is configured.
	CapabilityStatusPlaceholder = "placeholder"
)

// RepositoryInterface defines organization data access operations.
type RepositoryInterface interface {
	CreateUnit(ctx context.Context, unit *OrganizationUnit) error
	GetUnit(ctx context.Context, id string) (*OrganizationUnit, error)
	ListUnits(ctx context.Context, companyID string) ([]OrganizationUnit, error)
	UpdateUnit(ctx context.Context, unit *OrganizationUnit) error
	DeleteUnit(ctx context.Context, id string) error
	AssignUser(ctx context.Context, member *OrganizationMember) error
	GetMembersInUnit(ctx context.Context, unitID string) ([]OrganizationMember, error)
	RemoveUser(ctx context.Context, memberID string) error
	LogSync(ctx context.Context, log *SyncLog) error
	UpdateSyncLog(ctx context.Context, log *SyncLog) error
}

// Service manages organization structure operations.
type Service struct {
	repo        RepositoryInterface
	syncAdapter OrgChartSyncAdapter
}

// NewService creates a new organization service.
func NewService(repo RepositoryInterface, adapter OrgChartSyncAdapter) *Service {
	if adapter == nil {
		adapter = &NoopOrgChartAdapter{}
	}
	return &Service{
		repo:        repo,
		syncAdapter: adapter,
	}
}

// CreateUnit creates a new organization unit.
func (s *Service) CreateUnit(ctx context.Context, unit *OrganizationUnit) error {
	if unit.CompanyID == "" {
		return fmt.Errorf("company_id is required")
	}
	if unit.Name == "" {
		return fmt.Errorf("unit name is required")
	}
	if unit.Type == "" {
		unit.Type = "department"
	}
	if unit.Status == "" {
		unit.Status = "active"
	}
	return s.repo.CreateUnit(ctx, unit)
}

// GetUnit retrieves an organization unit.
func (s *Service) GetUnit(ctx context.Context, id string) (*OrganizationUnit, error) {
	return s.repo.GetUnit(ctx, id)
}

// ListUnits lists all units in a company.
func (s *Service) ListUnits(ctx context.Context, companyID string) ([]OrganizationUnit, error) {
	return s.repo.ListUnits(ctx, companyID)
}

// GetHierarchy retrieves the full organization hierarchy.
func (s *Service) GetHierarchy(ctx context.Context, companyID string) (*OrganizationHierarchy, error) {
	units, err := s.repo.ListUnits(ctx, companyID)
	if err != nil {
		return nil, err
	}

	// Find root units (no parent)
	var roots []OrganizationUnit
	unitByID := make(map[string]*OrganizationUnit)
	childrenByParent := make(map[string][]OrganizationUnit)

	for i := range units {
		unitByID[units[i].ID] = &units[i]
		if units[i].ParentID == nil {
			roots = append(roots, units[i])
		} else {
			childrenByParent[*units[i].ParentID] = append(childrenByParent[*units[i].ParentID], units[i])
		}
	}

	// Build hierarchy recursively
	var buildHierarchy func(unit *OrganizationUnit) *OrganizationHierarchy
	buildHierarchy = func(unit *OrganizationUnit) *OrganizationHierarchy {
		hierarchy := &OrganizationHierarchy{Unit: unit}

		// Get members
		members, err := s.repo.GetMembersInUnit(ctx, unit.ID)
		if err == nil {
			hierarchy.Members = members
		}

		// Add children
		for _, child := range childrenByParent[unit.ID] {
			hierarchy.Children = append(hierarchy.Children, *buildHierarchy(&child))
		}

		return hierarchy
	}

	if len(roots) == 0 {
		return nil, fmt.Errorf("no root organization units found")
	}

	// Return the first root (companies typically have one org root)
	return buildHierarchy(&roots[0]), nil
}

// SyncWithLDAP synchronizes organization structure with LDAP.
func (s *Service) SyncWithLDAP(ctx context.Context, companyID string) (*SyncLog, error) {
	log := &SyncLog{
		CompanyID:  companyID,
		SyncSource: "ldap",
		StartedAt:  time.Now(),
		Status:     "in_progress",
	}

	if err := s.repo.LogSync(ctx, log); err != nil {
		return nil, fmt.Errorf("create sync log: %w", err)
	}

	// Call external LDAP sync adapter
	if err := s.syncAdapter.SyncOrgChart(ctx); err != nil {
		log.Status = "failed"
		log.ErrorMessage = err.Error()
		log.CompletedAt = timePtr(time.Now())
		if updateErr := s.repo.UpdateSyncLog(ctx, log); updateErr != nil {
			return nil, fmt.Errorf("sync failed and couldn't update log: %w", err)
		}
		return log, fmt.Errorf("ldap sync: %w", err)
	}

	// Mark sync as successful
	log.Status = "success"
	log.CompletedAt = timePtr(time.Now())
	if err := s.repo.UpdateSyncLog(ctx, log); err != nil {
		return nil, fmt.Errorf("update sync log: %w", err)
	}

	return log, nil
}

// AssignUserToUnit assigns a user to an organization unit.
func (s *Service) AssignUserToUnit(ctx context.Context, unitID, userID string, role string) error {
	if unitID == "" || userID == "" {
		return fmt.Errorf("unit_id and user_id are required")
	}
	if role == "" {
		role = "member"
	}

	member := &OrganizationMember{
		OrganizationUnitID: unitID,
		UserID:             userID,
		Role:               role,
		StartedAt:          time.Now(),
		IsPrimary:          true,
	}

	return s.repo.AssignUser(ctx, member)
}

// RemoveUserFromUnit removes a user from an organization unit.
func (s *Service) RemoveUserFromUnit(ctx context.Context, memberID string) error {
	return s.repo.RemoveUser(ctx, memberID)
}

// GetUserUnits gets all units a user is assigned to.
func (s *Service) GetUserUnits(ctx context.Context, userID string) ([]OrganizationUnit, error) {
	return nil, fmt.Errorf("organization chart placeholder: user-unit lookup is not available yet")
}

// UpdateUnit updates an organization unit.
func (s *Service) UpdateUnit(ctx context.Context, unit *OrganizationUnit) error {
	if unit.ID == "" {
		return fmt.Errorf("unit id is required")
	}
	return s.repo.UpdateUnit(ctx, unit)
}

// DeleteUnit deletes an organization unit (and all members).
func (s *Service) DeleteUnit(ctx context.Context, id string) error {
	return s.repo.DeleteUnit(ctx, id)
}

func timePtr(t time.Time) *time.Time {
	return &t
}
