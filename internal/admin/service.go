package admin

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"time"

	"golang.org/x/crypto/bcrypt"
)

var (
	ErrRoleNotFound           = fmt.Errorf("role not found")
	ErrPermissionNotFound     = fmt.Errorf("permission not found")
	ErrUserRoleNotFound       = fmt.Errorf("user role not found")
	ErrAuditPolicyNotFound    = fmt.Errorf("audit policy not found")
	ErrAuditLogNotFound       = fmt.Errorf("audit log not found")
	ErrInvalidRole            = fmt.Errorf("invalid role")
	ErrInvalidPermission      = fmt.Errorf("invalid permission")
	ErrCannotDeleteBuiltin    = fmt.Errorf("cannot delete builtin role")
	ErrMissingRequiredField   = fmt.Errorf("missing required field")
	ErrInvalidResource        = fmt.Errorf("invalid resource")
	ErrInvalidAction          = fmt.Errorf("invalid action")
	ErrInvalidScope           = fmt.Errorf("invalid scope")
	ErrInvalidAuditLevel      = fmt.Errorf("invalid audit level")
	ErrDuplicateRoleAssignment = fmt.Errorf("user already has this role")
)

const (
	ResourceUsers         = "users"
	ResourceDomains       = "domains"
	ResourceLogs          = "logs"
	ResourceOrganization  = "organization"
	ResourceSettings      = "settings"

	ActionCreate = "create"
	ActionRead   = "read"
	ActionUpdate = "update"
	ActionDelete = "delete"
	ActionExport = "export"

	ScopeOwnCompany = "own_company"
	ScopeDomain     = "own_domain"
	ScopeAll        = "all"
)

var validResources = map[string]bool{
	ResourceUsers:        true,
	ResourceDomains:      true,
	ResourceLogs:         true,
	ResourceOrganization: true,
	ResourceSettings:     true,
}

var validActions = map[string]bool{
	ActionCreate: true,
	ActionRead:   true,
	ActionUpdate: true,
	ActionDelete: true,
	ActionExport: true,
}

var validScopes = map[string]bool{
	ScopeOwnCompany: true,
	ScopeDomain:     true,
	ScopeAll:        true,
}

var validAuditLevels = map[string]bool{
	AuditLevelOne:   true,
	AuditLevelTwo:   true,
	AuditLevelThree: true,
}

type Service struct {
	repo RepositoryInterface
}

func NewService(repo RepositoryInterface) *Service {
	return &Service{
		repo: repo,
	}
}

// CreateRole creates a new role (builtin or custom).
func (s *Service) CreateRole(ctx context.Context, role *Role) error {
	if role.CompanyID == "" {
		return fmt.Errorf("%w: company_id", ErrMissingRequiredField)
	}
	if role.Name == "" {
		return fmt.Errorf("%w: name", ErrMissingRequiredField)
	}
	if role.CreatedAt.IsZero() {
		role.CreatedAt = time.Now()
	}
	if role.UpdatedAt.IsZero() {
		role.UpdatedAt = time.Now()
	}
	return s.repo.CreateRole(ctx, role)
}

// GetRole retrieves a role by ID.
func (s *Service) GetRole(ctx context.Context, roleID string) (*Role, error) {
	return s.repo.GetRole(ctx, roleID)
}

// ListRoles lists all roles for a company.
func (s *Service) ListRoles(ctx context.Context, companyID string) ([]Role, error) {
	return s.repo.ListRoles(ctx, companyID)
}

// UpdateRole updates an existing role.
func (s *Service) UpdateRole(ctx context.Context, role *Role) error {
	if role.ID == "" {
		return fmt.Errorf("%w: id", ErrMissingRequiredField)
	}
	role.UpdatedAt = time.Now()
	return s.repo.UpdateRole(ctx, role)
}

// DeleteRole deletes a role (custom roles only).
func (s *Service) DeleteRole(ctx context.Context, roleID string) error {
	role, err := s.repo.GetRole(ctx, roleID)
	if err != nil {
		return err
	}
	if role.IsBuiltin {
		return ErrCannotDeleteBuiltin
	}
	return s.repo.DeleteRole(ctx, roleID)
}

// AddPermissionToRole adds a permission to a role.
func (s *Service) AddPermissionToRole(ctx context.Context, roleID, resource, action, scope string, conditions *Conditions) error {
	if roleID == "" {
		return fmt.Errorf("%w: roleID", ErrMissingRequiredField)
	}
	if resource == "" {
		return fmt.Errorf("%w: resource", ErrMissingRequiredField)
	}
	if action == "" {
		return fmt.Errorf("%w: action", ErrMissingRequiredField)
	}
	if scope == "" {
		return fmt.Errorf("%w: scope", ErrMissingRequiredField)
	}
	if !validResources[resource] {
		return fmt.Errorf("%w: %s", ErrInvalidResource, resource)
	}
	if !validActions[action] {
		return fmt.Errorf("%w: %s", ErrInvalidAction, action)
	}
	if !validScopes[scope] {
		return fmt.Errorf("%w: %s", ErrInvalidScope, scope)
	}

	permission := &Permission{
		RoleID:     roleID,
		Resource:   resource,
		Action:     action,
		Scope:      scope,
		CreatedAt:  time.Now(),
	}
	if conditions != nil {
		permission.Conditions = *conditions
	}
	return s.repo.AddPermission(ctx, permission)
}

// RemovePermissionFromRole removes a permission from a role.
func (s *Service) RemovePermissionFromRole(ctx context.Context, permissionID string) error {
	return s.repo.RemovePermission(ctx, permissionID)
}

// ListPermissionsForRole lists all permissions for a role.
func (s *Service) ListPermissionsForRole(ctx context.Context, roleID string) ([]Permission, error) {
	return s.repo.ListPermissions(ctx, roleID)
}

// AssignUserToRole assigns a user to a role.
func (s *Service) AssignUserToRole(ctx context.Context, companyID, userID, roleID string) error {
	return s.AssignUserToRoleWithExpiry(ctx, companyID, userID, roleID, nil, "")
}

// AssignUserToRoleWithExpiry assigns a user to a role with optional expiration.
func (s *Service) AssignUserToRoleWithExpiry(ctx context.Context, companyID, userID, roleID string, expiresAt *time.Time, assignedBy string) error {
	if companyID == "" {
		return fmt.Errorf("%w: companyID", ErrMissingRequiredField)
	}
	if userID == "" {
		return fmt.Errorf("%w: userID", ErrMissingRequiredField)
	}
	if roleID == "" {
		return fmt.Errorf("%w: roleID", ErrMissingRequiredField)
	}

	userRole := &UserRole{
		CompanyID:  companyID,
		UserID:     userID,
		RoleID:     roleID,
		AssignedAt: time.Now(),
		AssignedBy: assignedBy,
		ExpiresAt:  expiresAt,
	}
	return s.repo.AssignRole(ctx, userRole)
}

// RevokeUserFromRole revokes a role assignment by user-role ID.
func (s *Service) RevokeUserFromRole(ctx context.Context, userRoleID string) error {
	return s.repo.RevokeRole(ctx, userRoleID)
}

// GetUserRoles gets all roles assigned to a user.
func (s *Service) GetUserRoles(ctx context.Context, companyID, userID string) ([]UserRole, error) {
	return s.repo.GetUserRoles(ctx, userID, companyID)
}

// CreateAuditPolicy creates an audit policy for a domain.
func (s *Service) CreateAuditPolicy(ctx context.Context, config *AuditPolicyConfig) error {
	if config.CompanyID == "" {
		return fmt.Errorf("%w: companyID", ErrMissingRequiredField)
	}
	if config.DomainID == "" {
		return fmt.Errorf("%w: domainID", ErrMissingRequiredField)
	}
	if !validAuditLevels[config.AuditLevel] {
		return fmt.Errorf("%w: %s", ErrInvalidAuditLevel, config.AuditLevel)
	}
	if config.RetentionDays <= 0 {
		return fmt.Errorf("retention days must be positive")
	}
	if config.CreatedAt.IsZero() {
		config.CreatedAt = time.Now()
	}
	if config.UpdatedAt.IsZero() {
		config.UpdatedAt = time.Now()
	}
	return s.repo.CreateAuditPolicy(ctx, config)
}

// GetAuditPolicy retrieves audit policy for a domain.
func (s *Service) GetAuditPolicy(ctx context.Context, companyID, domainID string) (*AuditPolicyConfig, error) {
	return s.repo.GetAuditPolicy(ctx, companyID, domainID)
}

// UpdateAuditPolicy updates an audit policy.
func (s *Service) UpdateAuditPolicy(ctx context.Context, config *AuditPolicyConfig) error {
	if config.CompanyID == "" {
		return fmt.Errorf("%w: companyID", ErrMissingRequiredField)
	}
	if config.DomainID == "" {
		return fmt.Errorf("%w: domainID", ErrMissingRequiredField)
	}
	if !validAuditLevels[config.AuditLevel] {
		return fmt.Errorf("%w: %s", ErrInvalidAuditLevel, config.AuditLevel)
	}
	config.UpdatedAt = time.Now()
	return s.repo.UpdateAuditPolicy(ctx, config)
}

// LogAuditEvent logs an audit event.
func (s *Service) LogAuditEvent(ctx context.Context, companyID, adminUserID, action, resourceType, resourceID string, changes *AuditChanges) error {
	if action == "" {
		return fmt.Errorf("%w: action", ErrMissingRequiredField)
	}
	if companyID == "" {
		return fmt.Errorf("%w: companyID", ErrMissingRequiredField)
	}

	auditLog := &AuditLog{
		CompanyID:   companyID,
		AdminUserID: adminUserID,
		Action:      action,
		ResourceType: resourceType,
		ResourceID:  resourceID,
		Timestamp:   time.Now(),
	}
	if changes != nil {
		auditLog.Changes = *changes
	}
	return s.repo.LogAuditEvent(ctx, auditLog)
}

// ListAuditEvents lists audit events.
func (s *Service) ListAuditEvents(ctx context.Context, filter AuditLogFilter) ([]AuditLog, int64, error) {
	return s.repo.ListAuditLogs(ctx, filter)
}

// LogLoginAttempt logs a user login attempt.
func (s *Service) LogLoginAttempt(ctx context.Context, userID, companyID, ipAddress, userAgent string, success bool, failureReason string) error {
	log := &LoginAuditLog{
		UserID:        userID,
		CompanyID:     companyID,
		IPAddress:     ipAddress,
		UserAgent:     userAgent,
		Success:       success,
		FailureReason: failureReason,
		Timestamp:     time.Now(),
	}
	return s.repo.LogLoginAttempt(ctx, log)
}

// ListLoginAttempts lists login attempts.
func (s *Service) ListLoginAttempts(ctx context.Context, filter LoginAuditFilter) ([]LoginAuditLog, error) {
	return s.repo.ListLoginAudits(ctx, filter)
}

// CleanupAuditLogs deletes audit logs older than retention period.
func (s *Service) CleanupAuditLogs(ctx context.Context, companyID string, retentionDays int) (int64, error) {
	cutoff := time.Now().AddDate(0, 0, -retentionDays)
	return s.repo.DeleteAuditLogsBefore(ctx, companyID, cutoff)
}

// GetDomainSettings retrieves domain-level configuration.
func (s *Service) GetDomainSettings(ctx context.Context, domainID string) (*DomainSettings, error) {
	if domainID == "" {
		return nil, fmt.Errorf("%w: domainID", ErrMissingRequiredField)
	}
	return s.repo.GetDomainSettings(ctx, domainID)
}

// UpdateDomainSettings updates domain-level configuration.
func (s *Service) UpdateDomainSettings(ctx context.Context, settings *DomainSettings) error {
	if settings.DomainID == "" {
		return fmt.Errorf("%w: domainID", ErrMissingRequiredField)
	}
	if settings.TLSPolicy == "" || (settings.TLSPolicy != "opportunistic" && settings.TLSPolicy != "require" && settings.TLSPolicy != "disable") {
		return fmt.Errorf("invalid tls_policy: must be opportunistic, require, or disable")
	}
	if settings.QuotaPerUser <= 0 {
		return fmt.Errorf("quota_per_user must be greater than 0")
	}
	if settings.SessionTimeoutMinutes <= 0 {
		return fmt.Errorf("session_timeout_minutes must be greater than 0")
	}
	if settings.PasswordMinLength <= 0 {
		return fmt.Errorf("password_min_length must be greater than 0")
	}
	if settings.PasswordExpiryDays < 0 {
		return fmt.Errorf("password_expiry_days must be >= 0")
	}
	return s.repo.UpdateDomainSettings(ctx, settings)
}

// GetAPISettings retrieves API settings for a domain.
func (s *Service) GetAPISettings(ctx context.Context, domainID string) (*APISettings, error) {
	if domainID == "" {
		return nil, fmt.Errorf("%w: domainID", ErrMissingRequiredField)
	}
	return s.repo.GetAPISettings(ctx, domainID)
}

// UpdateAPISettings updates API settings for a domain.
func (s *Service) UpdateAPISettings(ctx context.Context, settings *APISettings) error {
	if settings.DomainID == "" {
		return fmt.Errorf("%w: domainID", ErrMissingRequiredField)
	}
	if settings.RateLimitRPS <= 0 {
		return fmt.Errorf("rate_limit_rps must be greater than 0")
	}
	if settings.RateLimitBPS < 0 {
		return fmt.Errorf("rate_limit_bps must be >= 0")
	}
	return s.repo.UpdateAPISettings(ctx, settings)
}

// CreateAPIKey creates a new API key and returns the secret.
func (s *Service) CreateAPIKey(ctx context.Context, key *APIKey) (secret string, err error) {
	if key.DomainID == "" {
		return "", fmt.Errorf("%w: domainID", ErrMissingRequiredField)
	}
	if key.Name == "" {
		return "", fmt.Errorf("%w: name", ErrMissingRequiredField)
	}
	if key.CreatedBy == "" {
		return "", fmt.Errorf("%w: createdBy", ErrMissingRequiredField)
	}

	secret = generateAPIKeySecret(32)
	hash, err := bcrypt.GenerateFromPassword([]byte(secret), bcrypt.DefaultCost)
	if err != nil {
		return "", fmt.Errorf("hash api key secret: %w", err)
	}

	key.SecretHash = string(hash)
	key.CreatedAt = time.Now()
	key.IsActive = true

	if err := s.repo.CreateAPIKey(ctx, key); err != nil {
		return "", err
	}
	return secret, nil
}

// ListAPIKeys lists all API keys for a domain.
func (s *Service) ListAPIKeys(ctx context.Context, domainID string) ([]APIKey, error) {
	if domainID == "" {
		return nil, fmt.Errorf("%w: domainID", ErrMissingRequiredField)
	}
	return s.repo.ListAPIKeys(ctx, domainID)
}

// DeleteAPIKey deletes an API key.
func (s *Service) DeleteAPIKey(ctx context.Context, keyID string) error {
	if keyID == "" {
		return fmt.Errorf("%w: keyID", ErrMissingRequiredField)
	}
	return s.repo.DeleteAPIKey(ctx, keyID)
}

// RotateAPIKey rotates an API key by creating a new secret.
func (s *Service) RotateAPIKey(ctx context.Context, keyID string) (newSecret string, err error) {
	if keyID == "" {
		return "", fmt.Errorf("%w: keyID", ErrMissingRequiredField)
	}

	newSecret = generateAPIKeySecret(32)
	hash, err := bcrypt.GenerateFromPassword([]byte(newSecret), bcrypt.DefaultCost)
	if err != nil {
		return "", fmt.Errorf("hash new api key secret: %w", err)
	}

	if err := s.repo.RotateAPIKeySecret(ctx, keyID, string(hash)); err != nil {
		return "", fmt.Errorf("rotate api key: %w", err)
	}
	return newSecret, nil
}

// generateAPIKeySecret generates a random API key secret.
func generateAPIKeySecret(length int) string {
	b := make([]byte, length)
	if _, err := rand.Read(b); err != nil {
		panic(fmt.Sprintf("failed to generate random bytes: %v", err))
	}
	return base64.StdEncoding.EncodeToString(b)
}
