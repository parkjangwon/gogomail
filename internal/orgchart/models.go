package orgchart

import "time"

// OrganizationUnit represents a department/team/division in the organization.
type OrganizationUnit struct {
	ID              string    `json:"id"`
	CompanyID       string    `json:"company_id"`
	ParentID        *string   `json:"parent_id,omitempty"`
	Name            string    `json:"name"`
	Type            string    `json:"type"` // department, team, division, etc
	Description     string    `json:"description,omitempty"`
	DisplayName     string    `json:"display_name,omitempty"`
	ManagerUserID   *string   `json:"manager_user_id,omitempty"`
	Status          string    `json:"status"` // active, archived, inactive
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

// OrganizationMember represents a user assignment to an organization unit.
type OrganizationMember struct {
	ID                   string     `json:"id"`
	OrganizationUnitID   string     `json:"organization_unit_id"`
	UserID               string     `json:"user_id"`
	Role                 string     `json:"role"` // member, manager, admin
	Title                string     `json:"title,omitempty"`
	StartedAt            time.Time  `json:"started_at"`
	EndedAt              *time.Time `json:"ended_at,omitempty"`
	IsPrimary            bool       `json:"is_primary"`
	CreatedAt            time.Time  `json:"created_at"`
	UpdatedAt            time.Time  `json:"updated_at"`
}

// SyncLog represents an organization sync operation.
type SyncLog struct {
	ID             string     `json:"id"`
	CompanyID      string     `json:"company_id"`
	SyncSource     string     `json:"sync_source"` // ldap, azure_ad, okta, etc
	StartedAt      time.Time  `json:"started_at"`
	CompletedAt    *time.Time `json:"completed_at,omitempty"`
	Status         string     `json:"status"` // in_progress, success, failed
	UnitsCreated   int        `json:"units_created"`
	UnitsUpdated   int        `json:"units_updated"`
	UsersSynced    int        `json:"users_synced"`
	ErrorMessage   string     `json:"error_message,omitempty"`
	CreatedAt      time.Time  `json:"created_at"`
}

// OrganizationHierarchy represents a full org tree node.
type OrganizationHierarchy struct {
	Unit        *OrganizationUnit        `json:"unit"`
	Manager     *string                  `json:"manager_user_id,omitempty"`
	Members     []OrganizationMember     `json:"members,omitempty"`
	Children    []OrganizationHierarchy  `json:"children,omitempty"`
}
