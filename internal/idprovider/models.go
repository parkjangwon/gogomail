package idprovider

import "time"

// User represents a user in the identity system.
type User struct {
	ID           string                 `json:"id"`
	DomainID     string                 `json:"domain_id"`
	OrgID        *string                `json:"org_id,omitempty"`
	Username     string                 `json:"username"`
	DisplayName  string                 `json:"display_name"`
	RecoveryEmail string                `json:"recovery_email"`
	AuthSource   string                 `json:"auth_source"`
	Role         string                 `json:"role"`
	Status       string                 `json:"status"`
	Settings     map[string]interface{} `json:"settings"`
	CreatedAt    time.Time              `json:"created_at"`
	UpdatedAt    time.Time              `json:"updated_at"`
}

// Group represents a group in the identity system.
type Group struct {
	ID          string                 `json:"id"`
	DomainID    string                 `json:"domain_id"`
	OrgID       *string                `json:"org_id,omitempty"`
	Name        string                 `json:"name"`
	Slug        string                 `json:"slug"`
	Description string                 `json:"description"`
	Status      string                 `json:"status"`
	Settings    map[string]interface{} `json:"settings"`
	CreatedAt   time.Time              `json:"created_at"`
	UpdatedAt   time.Time              `json:"updated_at"`
}

// Member represents a member of a group.
type Member struct {
	ID         string                 `json:"id"`
	Kind       string                 `json:"kind"` // user, organization, group, resource
	MemberID   string                 `json:"member_id"`
	Role       string                 `json:"role"` // member, manager, owner
	Status     string                 `json:"status"`
	CreatedAt  time.Time              `json:"created_at"`
	UpdatedAt  time.Time              `json:"updated_at"`
}
