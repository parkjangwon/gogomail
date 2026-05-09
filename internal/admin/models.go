package admin

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"time"
)

// Role represents an admin role (builtin or custom).
type Role struct {
	ID          string            `json:"id"`
	CompanyID   string            `json:"company_id"`
	Name        string            `json:"name"`
	Description string            `json:"description,omitempty"`
	IsBuiltin   bool              `json:"is_builtin"`
	CreatedBy   string            `json:"created_by,omitempty"`
	CreatedAt   time.Time         `json:"created_at"`
	UpdatedAt   time.Time         `json:"updated_at,omitempty"`
}

// Permission represents a single permission for a role.
type Permission struct {
	ID              string       `json:"id"`
	RoleID          string       `json:"role_id"`
	Resource        string       `json:"resource"`    // 'users', 'domains', 'logs', 'organization'
	Action          string       `json:"action"`      // 'create', 'read', 'update', 'delete'
	Scope           string       `json:"scope"`       // 'own_company', 'own_domain', 'all'
	Conditions      Conditions   `json:"conditions"`  // extra rules
	CreatedAt       time.Time    `json:"created_at"`
}

// Conditions holds optional permission conditions (e.g., restrictions).
type Conditions struct {
	CanResetPassword *bool  `json:"can_reset_password,omitempty"`
	MaxUsersPerDay   *int   `json:"max_users_per_day,omitempty"`
	RestrictedFields []string `json:"restricted_fields,omitempty"` // fields admin cannot edit
}

func (c Conditions) Value() (driver.Value, error) {
	return json.Marshal(c)
}

func (c *Conditions) Scan(value interface{}) error {
	b, ok := value.([]byte)
	if !ok {
		return fmt.Errorf("type assertion failed")
	}
	return json.Unmarshal(b, &c)
}

// UserRole represents role assignment to a user.
type UserRole struct {
	ID            string    `json:"id"`
	CompanyID     string    `json:"company_id"`
	UserID        string    `json:"user_id"`
	RoleID        string    `json:"role_id"`
	AssignedAt    time.Time `json:"assigned_at"`
	AssignedBy    string    `json:"assigned_by"`
	ExpiresAt     *time.Time `json:"expires_at,omitempty"` // for temporary delegation
}

// AuditPolicyConfig holds audit configuration for a domain.
type AuditPolicyConfig struct {
	ID                       string    `json:"id"`
	CompanyID                string    `json:"company_id"`
	DomainID                 string    `json:"domain_id"`
	AuditLevel               string    `json:"audit_level"` // 'level_1', 'level_2', 'level_3'
	AuditAdminActions        bool      `json:"audit_admin_actions"`
	AuditSecurityEvents      bool      `json:"audit_security_events"`
	AuditUserMailActions     bool      `json:"audit_user_mail_actions"` // level 3
	AuditAPICalls            bool      `json:"audit_api_calls"`
	RetentionDays            int       `json:"retention_days"`
	MaskMailContent          bool      `json:"mask_mail_content"`
	MaskRecipientEmails      bool      `json:"mask_recipient_emails"`
	CreatedAt                time.Time `json:"created_at"`
	UpdatedAt                time.Time `json:"updated_at"`
}

// AuditLog represents a single audit log entry.
type AuditLog struct {
	ID            string          `json:"id"`
	CompanyID     string          `json:"company_id"`
	AdminUserID   string          `json:"admin_user_id"`
	Action        string          `json:"action"` // 'user.create', 'domain.update'
	ResourceType  string          `json:"resource_type"` // 'user', 'domain', 'organization'
	ResourceID    string          `json:"resource_id"`
	Changes       AuditChanges    `json:"changes"`
	IPAddress     string          `json:"ip_address"`
	UserAgent     string          `json:"user_agent"`
	Timestamp     time.Time       `json:"timestamp"`
}

// AuditChanges holds before/after state for auditing.
type AuditChanges struct {
	Before map[string]interface{} `json:"before,omitempty"`
	After  map[string]interface{} `json:"after,omitempty"`
}

func (ac AuditChanges) Value() (driver.Value, error) {
	return json.Marshal(ac)
}

func (ac *AuditChanges) Scan(value interface{}) error {
	b, ok := value.([]byte)
	if !ok {
		return fmt.Errorf("type assertion failed")
	}
	return json.Unmarshal(b, &ac)
}

// LoginAuditLog represents a user login attempt.
type LoginAuditLog struct {
	ID            string    `json:"id"`
	UserID        string    `json:"user_id"`
	CompanyID     string    `json:"company_id"`
	IPAddress     string    `json:"ip_address"`
	UserAgent     string    `json:"user_agent"`
	Success       bool      `json:"success"`
	FailureReason string    `json:"failure_reason,omitempty"`
	Timestamp     time.Time `json:"timestamp"`
}

// Builtin roles (immutable).
const (
	RoleSystemAdmin      = "system_admin"
	RoleDomainAdmin      = "domain_admin"
	RoleSecurityOfficer  = "security_officer"
	RoleHROfficer        = "hr_officer"
	RoleMonitoringOfficer = "monitoring_officer"
	RoleAuditor          = "auditor"
	RoleSupportSpecialist = "support_specialist"
)

// Audit levels.
const (
	AuditLevelOne   = "level_1" // admin actions only
	AuditLevelTwo   = "level_2" // + security events
	AuditLevelThree = "level_3" // + user actions (mail read/delete)
)

// DefaultAuditPolicy returns safe defaults.
func DefaultAuditPolicy() AuditPolicyConfig {
	return AuditPolicyConfig{
		AuditLevel:          AuditLevelTwo,
		AuditAdminActions:   true,
		AuditSecurityEvents: true,
		AuditUserMailActions: false,
		RetentionDays:       90,
		MaskMailContent:     true,
		MaskRecipientEmails: false,
	}
}
