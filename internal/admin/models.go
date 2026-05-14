package admin

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"time"
)

// Role represents an admin role (builtin or custom).
type Role struct {
	ID          string    `json:"id"`
	CompanyID   string    `json:"company_id"`
	Name        string    `json:"name"`
	Description string    `json:"description,omitempty"`
	IsBuiltin   bool      `json:"is_builtin"`
	CreatedBy   string    `json:"created_by,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at,omitempty"`
}

// RoleSummary is the Admin API read model for roles plus RBAC table counts.
type RoleSummary struct {
	ID               string    `json:"id"`
	CompanyID        string    `json:"company_id"`
	Name             string    `json:"name"`
	Description      string    `json:"description,omitempty"`
	IsBuiltin        bool      `json:"is_builtin"`
	PermissionsCount int       `json:"permissions_count"`
	AssignedUsers    int       `json:"assigned_users"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at,omitempty"`
}

type CreateRoleRequest struct {
	CompanyID   string `json:"company_id"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	CreatedBy   string `json:"created_by,omitempty"`
	IsBuiltin   bool   `json:"is_builtin,omitempty"`
}

// Permission represents a single permission for a role.
type Permission struct {
	ID         string     `json:"id"`
	RoleID     string     `json:"role_id"`
	Resource   string     `json:"resource"`   // 'users', 'domains', 'logs', 'organization'
	Action     string     `json:"action"`     // 'create', 'read', 'update', 'delete'
	Scope      string     `json:"scope"`      // 'own_company', 'own_domain', 'all'
	Conditions Conditions `json:"conditions"` // extra rules
	CreatedAt  time.Time  `json:"created_at"`
}

// Conditions holds optional permission conditions (e.g., restrictions).
type Conditions struct {
	CanResetPassword *bool    `json:"can_reset_password,omitempty"`
	MaxUsersPerDay   *int     `json:"max_users_per_day,omitempty"`
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
	ID         string     `json:"id"`
	CompanyID  string     `json:"company_id"`
	UserID     string     `json:"user_id"`
	RoleID     string     `json:"role_id"`
	AssignedAt time.Time  `json:"assigned_at"`
	AssignedBy string     `json:"assigned_by"`
	ExpiresAt  *time.Time `json:"expires_at,omitempty"` // for temporary delegation
}

// AuditPolicyConfig holds audit configuration for a domain.
type AuditPolicyConfig struct {
	ID                   string    `json:"id"`
	CompanyID            string    `json:"company_id"`
	DomainID             string    `json:"domain_id"`
	AuditLevel           string    `json:"audit_level"` // 'level_1', 'level_2', 'level_3'
	AuditAdminActions    bool      `json:"audit_admin_actions"`
	AuditSecurityEvents  bool      `json:"audit_security_events"`
	AuditUserMailActions bool      `json:"audit_user_mail_actions"` // level 3
	AuditAPICalls        bool      `json:"audit_api_calls"`
	RetentionDays        int       `json:"retention_days"`
	MaskMailContent      bool      `json:"mask_mail_content"`
	MaskRecipientEmails  bool      `json:"mask_recipient_emails"`
	CreatedAt            time.Time `json:"created_at"`
	UpdatedAt            time.Time `json:"updated_at"`
}

// DomainSettings holds domain-level configuration.
type DomainSettings struct {
	DomainID                     string    `json:"domain_id"`
	TLSPolicy                    string    `json:"tls_policy"`     // 'opportunistic', 'require', 'disable'
	QuotaPerUser                 int64     `json:"quota_per_user"` // bytes
	IPWhitelistEnabled           bool      `json:"ip_whitelist_enabled"`
	IPWhitelist                  []string  `json:"ip_whitelist"` // CIDR notation
	Require2FA                   bool      `json:"require_2fa"`
	SessionTimeoutMinutes        int       `json:"session_timeout_minutes"`
	PasswordMinLength            int       `json:"password_min_length"`
	PasswordRequireUppercase     bool      `json:"password_require_uppercase"`
	PasswordRequireNumbers       bool      `json:"password_require_numbers"`
	PasswordRequireSpecialChars  bool      `json:"password_require_special_chars"`
	PasswordExpiryDays           int       `json:"password_expiry_days"`
	UserRegistrationMode         string    `json:"user_registration_mode"` // 'temp_password' or 'email_invite'
	PasswordResetTokenTTLMinutes int       `json:"password_reset_token_ttl_minutes"`
	UpdatedAt                    time.Time `json:"updated_at"`
	UpdatedBy                    string    `json:"updated_by"`
}

// AuditLog represents a single audit log entry.
type AuditLog struct {
	ID           string       `json:"id"`
	CompanyID    string       `json:"company_id"`
	AdminUserID  string       `json:"admin_user_id"`
	Action       string       `json:"action"`        // 'user.create', 'domain.update'
	ResourceType string       `json:"resource_type"` // 'user', 'domain', 'organization'
	ResourceID   string       `json:"resource_id"`
	Changes      AuditChanges `json:"changes"`
	IPAddress    string       `json:"ip_address"`
	UserAgent    string       `json:"user_agent"`
	Timestamp    time.Time    `json:"timestamp"`
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
	RoleSystemAdmin       = "system_admin"
	RoleDomainAdmin       = "domain_admin"
	RoleSecurityOfficer   = "security_officer"
	RoleHROfficer         = "hr_officer"
	RoleMonitoringOfficer = "monitoring_officer"
	RoleAuditor           = "auditor"
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
		AuditLevel:           AuditLevelTwo,
		AuditAdminActions:    true,
		AuditSecurityEvents:  true,
		AuditUserMailActions: false,
		RetentionDays:        90,
		MaskMailContent:      true,
		MaskRecipientEmails:  false,
	}
}

// APISettings holds API-level configuration for a domain.
type APISettings struct {
	DomainID             string    `json:"domain_id"`
	RateLimitRPS         int       `json:"rate_limit_rps"` // requests per second
	RateLimitBPS         int64     `json:"rate_limit_bps"` // bytes per second (0 = unlimited)
	CIDRAllowlistEnabled bool      `json:"cidr_allowlist_enabled"`
	CIDRAllowlist        []string  `json:"cidr_allowlist"` // CIDR or single IP
	RequireAPIKey        bool      `json:"require_api_key"`
	UpdatedAt            time.Time `json:"updated_at"`
	UpdatedBy            string    `json:"updated_by"`
}

// APIKey represents an API key for domain access.
type APIKey struct {
	ID         string     `json:"id"`
	DomainID   string     `json:"domain_id"`
	Name       string     `json:"name"`
	SecretHash string     `json:"-"` // never expose in JSON
	CreatedBy  string     `json:"created_by"`
	CreatedAt  time.Time  `json:"created_at"`
	LastUsedAt *time.Time `json:"last_used_at,omitempty"`
	ExpiresAt  *time.Time `json:"expires_at,omitempty"`
	IsActive   bool       `json:"is_active"`
}
