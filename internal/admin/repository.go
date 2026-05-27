package admin

import (
	"errors"
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/lib/pq"
)

// RepositoryInterface defines admin RBAC data access operations.
type RepositoryInterface interface {
	// Role operations
	CreateRole(ctx context.Context, role *Role) error
	CreateRoleSummary(ctx context.Context, req CreateRoleRequest) (RoleSummary, error)
	GetRole(ctx context.Context, id string) (*Role, error)
	ListRoles(ctx context.Context, companyID string) ([]Role, error)
	ListRoleSummaries(ctx context.Context, companyID string) ([]RoleSummary, error)
	UpdateRole(ctx context.Context, role *Role) error
	DeleteRole(ctx context.Context, id string) error

	// Permission operations
	AddPermission(ctx context.Context, perm *Permission) error
	RemovePermission(ctx context.Context, id string) error
	ListPermissions(ctx context.Context, roleID string) ([]Permission, error)
	GetPermissionsByRole(ctx context.Context, roleID string) ([]Permission, error)

	// User role operations
	AssignRole(ctx context.Context, userRole *UserRole) error
	RevokeRole(ctx context.Context, id string) error
	GetUserRoles(ctx context.Context, userID string, companyID string) ([]UserRole, error)
	ListRolesForUser(ctx context.Context, userID string) ([]Role, error)

	// Audit policy operations
	CreateAuditPolicy(ctx context.Context, policy *AuditPolicyConfig) error
	GetAuditPolicy(ctx context.Context, companyID, domainID string) (*AuditPolicyConfig, error)
	UpdateAuditPolicy(ctx context.Context, policy *AuditPolicyConfig) error

	// Audit log operations
	LogAuditEvent(ctx context.Context, log *AuditLog) error
	ListAuditLogs(ctx context.Context, filter AuditLogFilter) ([]AuditLog, int64, error)
	GetAuditLog(ctx context.Context, id string) (*AuditLog, error)
	DeleteAuditLogsBefore(ctx context.Context, companyID string, before time.Time) (int64, error)

	// Login audit operations
	LogLoginAttempt(ctx context.Context, log *LoginAuditLog) error
	ListLoginAudits(ctx context.Context, filter LoginAuditFilter) ([]LoginAuditLog, error)

	// Domain settings operations
	GetDomainSettings(ctx context.Context, domainID string) (*DomainSettings, error)
	UpdateDomainSettings(ctx context.Context, settings *DomainSettings) error

	// API settings operations
	GetAPISettings(ctx context.Context, domainID string) (*APISettings, error)
	UpdateAPISettings(ctx context.Context, settings *APISettings) error

	// API key operations
	CreateAPIKey(ctx context.Context, key *APIKey) error
	GetAPIKey(ctx context.Context, keyID string) (*APIKey, error)
	ListAPIKeys(ctx context.Context, domainID string) ([]APIKey, error)
	DeleteAPIKey(ctx context.Context, keyID string) error
	UpdateAPIKeyLastUsed(ctx context.Context, keyID string) error
	RotateAPIKeySecret(ctx context.Context, keyID, newSecretHash string) error

	// Alert rule operations
	CreateAlertRule(ctx context.Context, rule *AlertRule) error
	GetAlertRule(ctx context.Context, ruleID string) (*AlertRule, error)
	ListAlertRules(ctx context.Context, companyID string) ([]AlertRule, error)
	UpdateAlertRule(ctx context.Context, rule *AlertRule) error
	DeleteAlertRule(ctx context.Context, ruleID string) error

	// Alert channel operations
	CreateAlertChannel(ctx context.Context, channel *AlertChannel) error
	GetAlertChannel(ctx context.Context, channelID string) (*AlertChannel, error)
	ListAlertChannels(ctx context.Context, companyID string) ([]AlertChannel, error)
	UpdateAlertChannel(ctx context.Context, channel *AlertChannel) error
	DeleteAlertChannel(ctx context.Context, channelID string) error

	// Alert rule channel mapping operations
	CreateAlertRuleChannel(ctx context.Context, mapping *AlertRuleChannel) error
	ListAlertRuleChannels(ctx context.Context, ruleID string) ([]AlertChannel, error)
	DeleteAlertRuleChannel(ctx context.Context, ruleID, channelID string) error

	// Alert event operations
	LogAlertEvent(ctx context.Context, event *AlertEvent) error
	ListAlertEvents(ctx context.Context, filter AlertEventFilter) ([]AlertEvent, bool, error)
	ResolveAlertEvent(ctx context.Context, eventID string) error
}

// Repository implements RepositoryInterface.
type Repository struct {
	db *sql.DB
}

// NewRepository creates a new admin repository.
func NewRepository(db *sql.DB) *Repository {
	return &Repository{db: db}
}

// CreateRole inserts a new admin role.
func (r *Repository) CreateRole(ctx context.Context, role *Role) error {
	err := r.db.QueryRowContext(ctx,
		`INSERT INTO admin_role_definitions (company_id, name, description, is_builtin, created_by, created_at)
		 VALUES ($1, $2, $3, $4, $5, $6)
		 RETURNING id, created_at`,
		role.CompanyID, role.Name, role.Description, role.IsBuiltin, role.CreatedBy, time.Now(),
	).Scan(&role.ID, &role.CreatedAt)
	return err
}

func (r *Repository) CreateRoleSummary(ctx context.Context, req CreateRoleRequest) (RoleSummary, error) {
	var role RoleSummary
	err := r.db.QueryRowContext(ctx,
		`INSERT INTO admin_role_definitions (company_id, name, description, is_builtin, created_by, created_at, updated_at)
		 VALUES ($1, $2, $3, false, NULLIF($4, '')::uuid, NOW(), NOW())
		 RETURNING id, company_id, name, COALESCE(description, ''), is_builtin, created_at, updated_at`,
		req.CompanyID, req.Name, req.Description, req.CreatedBy,
	).Scan(&role.ID, &role.CompanyID, &role.Name, &role.Description, &role.IsBuiltin, &role.CreatedAt, &role.UpdatedAt)
	if err != nil {
		return RoleSummary{}, err
	}
	return role, nil
}

// GetRole retrieves a role by ID.
func (r *Repository) GetRole(ctx context.Context, id string) (*Role, error) {
	role := &Role{}
	err := r.db.QueryRowContext(ctx,
		`SELECT id, company_id, name, description, is_builtin, created_by, created_at, updated_at
		 FROM admin_role_definitions WHERE id = $1`,
		id,
	).Scan(&role.ID, &role.CompanyID, &role.Name, &role.Description, &role.IsBuiltin,
		&role.CreatedBy, &role.CreatedAt, &role.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return role, nil
}

// ListRoles lists all roles in a company.
func (r *Repository) ListRoles(ctx context.Context, companyID string) ([]Role, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, company_id, name, description, is_builtin, created_by, created_at, updated_at
		 FROM admin_role_definitions WHERE company_id = $1 ORDER BY name`,
		companyID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var roles []Role
	for rows.Next() {
		var role Role
		if err := rows.Scan(&role.ID, &role.CompanyID, &role.Name, &role.Description,
			&role.IsBuiltin, &role.CreatedBy, &role.CreatedAt, &role.UpdatedAt); err != nil {
			return nil, err
		}
		roles = append(roles, role)
	}
	return roles, rows.Err()
}

func (r *Repository) ListRoleSummaries(ctx context.Context, companyID string) ([]RoleSummary, error) {
	rows, err := r.db.QueryContext(ctx,
		listRoleSummariesQuery,
		companyID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var roles []RoleSummary
	for rows.Next() {
		var role RoleSummary
		if err := rows.Scan(
			&role.ID,
			&role.CompanyID,
			&role.Name,
			&role.Description,
			&role.IsBuiltin,
			&role.PermissionsCount,
			&role.AssignedUsers,
			&role.CreatedAt,
			&role.UpdatedAt,
		); err != nil {
			return nil, err
		}
		roles = append(roles, role)
	}
	return roles, rows.Err()
}

const listRoleSummariesQuery = `
WITH active_admin_user_roles AS (
  SELECT company_id, user_id, role_id
  FROM admin_user_roles
  WHERE company_id = $1 AND expires_at IS NULL
  UNION ALL
  SELECT company_id, user_id, role_id
  FROM admin_user_roles
  WHERE company_id = $1 AND expires_at > NOW()
)
SELECT
		     ard.id,
		     ard.company_id,
		     ard.name,
		     COALESCE(ard.description, ''),
		     ard.is_builtin,
		     COUNT(DISTINCT arp.id)::int AS permissions_count,
		     COUNT(DISTINCT aur.user_id)::int AS assigned_users,
		     ard.created_at,
		     ard.updated_at
		   FROM admin_role_definitions ard
		   LEFT JOIN admin_role_permissions arp ON arp.role_id = ard.id
		   LEFT JOIN active_admin_user_roles aur
		     ON aur.role_id = ard.id
		    AND aur.company_id = ard.company_id
		   WHERE ard.company_id = $1
		   GROUP BY ard.id, ard.company_id, ard.name, ard.description, ard.is_builtin, ard.created_at, ard.updated_at
		   ORDER BY lower(ard.name), ard.id`

// UpdateRole updates a role.
func (r *Repository) UpdateRole(ctx context.Context, role *Role) error {
	role.UpdatedAt = time.Now()
	_, err := r.db.ExecContext(ctx,
		`UPDATE admin_role_definitions
		 SET name = $1, description = $2, updated_at = $3
		 WHERE id = $4`,
		role.Name, role.Description, role.UpdatedAt, role.ID,
	)
	return err
}

// DeleteRole deletes a role (custom only, builtin cannot be deleted).
func (r *Repository) DeleteRole(ctx context.Context, id string) error {
	result, err := r.db.ExecContext(ctx,
		`DELETE FROM admin_role_definitions WHERE id = $1 AND is_builtin = false`,
		id,
	)
	if err != nil {
		return err
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return fmt.Errorf("role not found or is builtin")
	}
	return nil
}

// AddPermission adds a permission to a role.
func (r *Repository) AddPermission(ctx context.Context, perm *Permission) error {
	err := r.db.QueryRowContext(ctx,
		`INSERT INTO admin_role_permissions (role_id, resource, action, scope, conditions, created_at)
		 VALUES ($1, $2, $3, $4, $5, $6)
		 RETURNING id, created_at`,
		perm.RoleID, perm.Resource, perm.Action, perm.Scope, perm.Conditions, time.Now(),
	).Scan(&perm.ID, &perm.CreatedAt)
	return err
}

// RemovePermission removes a permission.
func (r *Repository) RemovePermission(ctx context.Context, id string) error {
	_, err := r.db.ExecContext(ctx,
		`DELETE FROM admin_role_permissions WHERE id = $1`,
		id,
	)
	return err
}

// ListPermissions lists all permissions for a role.
func (r *Repository) ListPermissions(ctx context.Context, roleID string) ([]Permission, error) {
	return r.GetPermissionsByRole(ctx, roleID)
}

// GetPermissionsByRole retrieves all permissions for a role.
func (r *Repository) GetPermissionsByRole(ctx context.Context, roleID string) ([]Permission, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, role_id, resource, action, scope, conditions, created_at
		 FROM admin_role_permissions WHERE role_id = $1`,
		roleID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var perms []Permission
	for rows.Next() {
		var perm Permission
		if err := rows.Scan(&perm.ID, &perm.RoleID, &perm.Resource, &perm.Action,
			&perm.Scope, &perm.Conditions, &perm.CreatedAt); err != nil {
			return nil, err
		}
		perms = append(perms, perm)
	}
	return perms, rows.Err()
}

// AssignRole assigns a role to a user.
func (r *Repository) AssignRole(ctx context.Context, userRole *UserRole) error {
	err := r.db.QueryRowContext(ctx,
		`INSERT INTO admin_user_roles (company_id, user_id, role_id, assigned_at, assigned_by, expires_at)
		 VALUES ($1, $2, $3, $4, $5, $6)
		 RETURNING id, assigned_at`,
		userRole.CompanyID, userRole.UserID, userRole.RoleID, time.Now(), userRole.AssignedBy, userRole.ExpiresAt,
	).Scan(&userRole.ID, &userRole.AssignedAt)
	return err
}

// RevokeRole revokes a role from a user.
func (r *Repository) RevokeRole(ctx context.Context, id string) error {
	_, err := r.db.ExecContext(ctx,
		`DELETE FROM admin_user_roles WHERE id = $1`,
		id,
	)
	return err
}

// GetUserRoles gets all active roles for a user in a company.
func (r *Repository) GetUserRoles(ctx context.Context, userID, companyID string) ([]UserRole, error) {
	rows, err := r.db.QueryContext(ctx,
		getUserRolesQuery,
		userID, companyID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var userRoles []UserRole
	for rows.Next() {
		var ur UserRole
		if err := rows.Scan(&ur.ID, &ur.CompanyID, &ur.UserID, &ur.RoleID,
			&ur.AssignedAt, &ur.AssignedBy, &ur.ExpiresAt); err != nil {
			return nil, err
		}
		userRoles = append(userRoles, ur)
	}
	return userRoles, rows.Err()
}

const getUserRolesQuery = `
SELECT id, company_id, user_id, role_id, assigned_at, assigned_by, expires_at
FROM (
  SELECT id, company_id, user_id, role_id, assigned_at, assigned_by, expires_at
  FROM admin_user_roles
  WHERE user_id = $1 AND company_id = $2 AND expires_at IS NULL
  UNION ALL
  SELECT id, company_id, user_id, role_id, assigned_at, assigned_by, expires_at
  FROM admin_user_roles
  WHERE user_id = $1 AND company_id = $2 AND expires_at > NOW()
) active_user_roles
ORDER BY assigned_at DESC`

// ListRolesForUser gets all role definitions for a user's active assignments.
func (r *Repository) ListRolesForUser(ctx context.Context, userID string) ([]Role, error) {
	rows, err := r.db.QueryContext(ctx,
		listRolesForUserQuery,
		userID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var roles []Role
	for rows.Next() {
		var role Role
		if err := rows.Scan(&role.ID, &role.CompanyID, &role.Name, &role.Description,
			&role.IsBuiltin, &role.CreatedBy, &role.CreatedAt, &role.UpdatedAt); err != nil {
			return nil, err
		}
		roles = append(roles, role)
	}
	return roles, rows.Err()
}

const listRolesForUserQuery = `
SELECT DISTINCT ard.id, ard.company_id, ard.name, ard.description, ard.is_builtin,
       ard.created_by, ard.created_at, ard.updated_at
FROM admin_role_definitions ard
JOIN (
  SELECT role_id
  FROM admin_user_roles
  WHERE user_id = $1 AND expires_at IS NULL
  UNION ALL
  SELECT role_id
  FROM admin_user_roles
  WHERE user_id = $1 AND expires_at > NOW()
) aur ON ard.id = aur.role_id
ORDER BY ard.name`

// CreateAuditPolicy creates an audit policy for a domain.
func (r *Repository) CreateAuditPolicy(ctx context.Context, policy *AuditPolicyConfig) error {
	err := r.db.QueryRowContext(ctx,
		`INSERT INTO audit_policy_configs (company_id, domain_id, audit_level, audit_admin_actions,
		 audit_security_events, audit_user_mail_actions, audit_api_calls, retention_days,
		 mask_mail_content, mask_recipient_emails, created_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
		 RETURNING id, created_at`,
		policy.CompanyID, policy.DomainID, policy.AuditLevel, policy.AuditAdminActions,
		policy.AuditSecurityEvents, policy.AuditUserMailActions, policy.AuditAPICalls,
		policy.RetentionDays, policy.MaskMailContent, policy.MaskRecipientEmails, time.Now(),
	).Scan(&policy.ID, &policy.CreatedAt)
	return err
}

// GetAuditPolicy retrieves audit policy for a domain.
func (r *Repository) GetAuditPolicy(ctx context.Context, companyID, domainID string) (*AuditPolicyConfig, error) {
	policy := &AuditPolicyConfig{}
	err := r.db.QueryRowContext(ctx,
		`SELECT id, company_id, domain_id, audit_level, audit_admin_actions, audit_security_events,
		        audit_user_mail_actions, audit_api_calls, retention_days, mask_mail_content,
		        mask_recipient_emails, created_at, updated_at
		 FROM audit_policy_configs WHERE company_id = $1 AND domain_id = $2`,
		companyID, domainID,
	).Scan(&policy.ID, &policy.CompanyID, &policy.DomainID, &policy.AuditLevel,
		&policy.AuditAdminActions, &policy.AuditSecurityEvents, &policy.AuditUserMailActions,
		&policy.AuditAPICalls, &policy.RetentionDays, &policy.MaskMailContent,
		&policy.MaskRecipientEmails, &policy.CreatedAt, &policy.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return policy, nil
}

// UpdateAuditPolicy updates an audit policy.
func (r *Repository) UpdateAuditPolicy(ctx context.Context, policy *AuditPolicyConfig) error {
	policy.UpdatedAt = time.Now()
	_, err := r.db.ExecContext(ctx,
		`UPDATE audit_policy_configs
		 SET audit_level = $1, audit_admin_actions = $2, audit_security_events = $3,
		     audit_user_mail_actions = $4, audit_api_calls = $5, retention_days = $6,
		     mask_mail_content = $7, mask_recipient_emails = $8, updated_at = $9
		 WHERE company_id = $10 AND domain_id = $11`,
		policy.AuditLevel, policy.AuditAdminActions, policy.AuditSecurityEvents,
		policy.AuditUserMailActions, policy.AuditAPICalls, policy.RetentionDays,
		policy.MaskMailContent, policy.MaskRecipientEmails, policy.UpdatedAt,
		policy.CompanyID, policy.DomainID,
	)
	return err
}

// LogAuditEvent logs an admin action or security event.
func (r *Repository) LogAuditEvent(ctx context.Context, log *AuditLog) error {
	err := r.db.QueryRowContext(ctx,
		`INSERT INTO audit_logs (company_id, admin_user_id, action, resource_type, resource_id,
		 changes, ip_address, user_agent, timestamp)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		 RETURNING id`,
		log.CompanyID, log.AdminUserID, log.Action, log.ResourceType, log.ResourceID,
		log.Changes, log.IPAddress, log.UserAgent, time.Now(),
	).Scan(&log.ID)
	return err
}

// AuditLogFilter holds query parameters for audit log listing.
type AuditLogFilter struct {
	CompanyID    string
	AdminUserID  string
	Action       string
	ResourceType string
	StartTime    *time.Time
	EndTime      *time.Time
	Limit        int
	Offset       int
}

// ListAuditLogs lists audit logs with filtering.
func (r *Repository) ListAuditLogs(ctx context.Context, filter AuditLogFilter) ([]AuditLog, int64, error) {
	countQuery, countArgs, query, args := buildAuditLogListQueries(filter)
	var total int64
	if err := r.db.QueryRowContext(ctx, countQuery, countArgs...).Scan(&total); err != nil {
		return nil, 0, err
	}

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var logs []AuditLog
	for rows.Next() {
		var log AuditLog
		if err := rows.Scan(&log.ID, &log.CompanyID, &log.AdminUserID, &log.Action,
			&log.ResourceType, &log.ResourceID, &log.Changes, &log.IPAddress,
			&log.UserAgent, &log.Timestamp); err != nil {
			return nil, 0, err
		}
		logs = append(logs, log)
	}
	return logs, total, rows.Err()
}

func buildAuditLogListQueries(filter AuditLogFilter) (string, []interface{}, string, []interface{}) {
	where, filterArgs := buildAuditLogWhereClause(filter)
	countArgs := append([]interface{}(nil), filterArgs...)
	listArgs := append([]interface{}(nil), filterArgs...)
	limitPlaceholder := len(listArgs) + 1
	offsetPlaceholder := len(listArgs) + 2
	listArgs = append(listArgs, filter.Limit, filter.Offset)

	countQuery := "SELECT COUNT(*) FROM audit_logs" + where
	listQuery := `SELECT id, company_id, admin_user_id, action, resource_type, resource_id,
	          changes, ip_address, user_agent, timestamp
	          FROM audit_logs` + where + fmt.Sprintf(` ORDER BY timestamp DESC, id DESC LIMIT $%d OFFSET $%d`, limitPlaceholder, offsetPlaceholder)
	return countQuery, countArgs, listQuery, listArgs
}

func buildAuditLogWhereClause(filter AuditLogFilter) (string, []interface{}) {
	where := " WHERE company_id = $1"
	args := []interface{}{filter.CompanyID}

	if filter.AdminUserID != "" {
		where += fmt.Sprintf(" AND admin_user_id = $%d", len(args)+1)
		args = append(args, filter.AdminUserID)
	}
	if filter.Action != "" {
		where += fmt.Sprintf(" AND action = $%d", len(args)+1)
		args = append(args, filter.Action)
	}
	if filter.ResourceType != "" {
		where += fmt.Sprintf(" AND resource_type = $%d", len(args)+1)
		args = append(args, filter.ResourceType)
	}
	if filter.StartTime != nil {
		where += fmt.Sprintf(" AND timestamp >= $%d", len(args)+1)
		args = append(args, *filter.StartTime)
	}
	if filter.EndTime != nil {
		where += fmt.Sprintf(" AND timestamp <= $%d", len(args)+1)
		args = append(args, *filter.EndTime)
	}
	return where, args
}

// GetAuditLog retrieves a single audit log entry.
func (r *Repository) GetAuditLog(ctx context.Context, id string) (*AuditLog, error) {
	log := &AuditLog{}
	err := r.db.QueryRowContext(ctx,
		`SELECT id, company_id, admin_user_id, action, resource_type, resource_id,
		        changes, ip_address, user_agent, timestamp
		 FROM audit_logs WHERE id = $1`,
		id,
	).Scan(&log.ID, &log.CompanyID, &log.AdminUserID, &log.Action, &log.ResourceType,
		&log.ResourceID, &log.Changes, &log.IPAddress, &log.UserAgent, &log.Timestamp)
	return log, err
}

// DeleteAuditLogsBefore deletes audit logs older than the given time (for retention).
func (r *Repository) DeleteAuditLogsBefore(ctx context.Context, companyID string, before time.Time) (int64, error) {
	result, err := r.db.ExecContext(ctx,
		`DELETE FROM audit_logs WHERE company_id = $1 AND timestamp < $2`,
		companyID, before,
	)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}

// LogLoginAttempt logs a login attempt.
func (r *Repository) LogLoginAttempt(ctx context.Context, log *LoginAuditLog) error {
	err := r.db.QueryRowContext(ctx,
		`INSERT INTO login_audit_logs (user_id, company_id, ip_address, user_agent, success, failure_reason, timestamp)
		 VALUES ($1, $2, $3, $4, $5, $6, $7)
		 RETURNING id`,
		log.UserID, log.CompanyID, log.IPAddress, log.UserAgent, log.Success, log.FailureReason, time.Now(),
	).Scan(&log.ID)
	return err
}

// LoginAuditFilter holds query parameters for login audit listing.
type LoginAuditFilter struct {
	CompanyID string
	UserID    string
	Success   *bool
	StartTime *time.Time
	EndTime   *time.Time
	Limit     int
	Offset    int
}

// ListLoginAudits lists login audit logs.
func (r *Repository) ListLoginAudits(ctx context.Context, filter LoginAuditFilter) ([]LoginAuditLog, error) {
	query := `SELECT id, user_id, company_id, ip_address, user_agent, success, failure_reason, timestamp
	          FROM login_audit_logs WHERE company_id = $1`
	args := []interface{}{filter.CompanyID}

	if filter.UserID != "" {
		query += ` AND user_id = $2`
		args = append(args, filter.UserID)
	}
	if filter.Success != nil {
		query += ` AND success = $` + fmt.Sprintf("%d", len(args)+1)
		args = append(args, *filter.Success)
	}
	if filter.StartTime != nil {
		query += ` AND timestamp >= $` + fmt.Sprintf("%d", len(args)+1)
		args = append(args, *filter.StartTime)
	}
	if filter.EndTime != nil {
		query += ` AND timestamp <= $` + fmt.Sprintf("%d", len(args)+1)
		args = append(args, *filter.EndTime)
	}

	query += ` ORDER BY timestamp DESC LIMIT $` + fmt.Sprintf("%d", len(args)+1) + ` OFFSET $` + fmt.Sprintf("%d", len(args)+2)
	args = append(args, filter.Limit, filter.Offset)

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var logs []LoginAuditLog
	for rows.Next() {
		var log LoginAuditLog
		if err := rows.Scan(&log.ID, &log.UserID, &log.CompanyID, &log.IPAddress,
			&log.UserAgent, &log.Success, &log.FailureReason, &log.Timestamp); err != nil {
			return nil, err
		}
		logs = append(logs, log)
	}
	return logs, rows.Err()
}

// GetDomainSettings retrieves domain-level configuration.
func (r *Repository) GetDomainSettings(ctx context.Context, domainID string) (*DomainSettings, error) {
	if _, err := r.db.ExecContext(ctx,
		`INSERT INTO domain_settings (domain_id, updated_by)
		 VALUES ($1, NULL)
		 ON CONFLICT (domain_id) DO NOTHING`,
		domainID,
	); err != nil {
		return nil, err
	}

	settings := &DomainSettings{}
	var updatedBy sql.NullString
	err := r.db.QueryRowContext(ctx,
		`SELECT domain_id, tls_policy, quota_per_user, ip_whitelist_enabled, ip_whitelist,
		        require_2fa, session_timeout_minutes, password_min_length,
		        password_require_uppercase, password_require_numbers, password_require_special_chars,
		        password_expiry_days, user_registration_mode, password_reset_token_ttl_minutes,
		        updated_at, updated_by
		 FROM domain_settings WHERE domain_id = $1`,
		domainID,
	).Scan(&settings.DomainID, &settings.TLSPolicy, &settings.QuotaPerUser, &settings.IPWhitelistEnabled,
		pq.Array(&settings.IPWhitelist), &settings.Require2FA, &settings.SessionTimeoutMinutes,
		&settings.PasswordMinLength, &settings.PasswordRequireUppercase, &settings.PasswordRequireNumbers,
		&settings.PasswordRequireSpecialChars, &settings.PasswordExpiryDays, &settings.UserRegistrationMode,
		&settings.PasswordResetTokenTTLMinutes,
		&settings.UpdatedAt, &updatedBy)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("domain settings not found")
	}
	if err != nil {
		return nil, err
	}
	if updatedBy.Valid {
		settings.UpdatedBy = updatedBy.String
	}
	return settings, nil
}

// UpdateDomainSettings updates domain-level configuration.
func (r *Repository) UpdateDomainSettings(ctx context.Context, settings *DomainSettings) error {
	settings.UpdatedAt = time.Now()
	_, err := r.db.ExecContext(ctx,
		`UPDATE domain_settings
		 SET tls_policy = $1, quota_per_user = $2, ip_whitelist_enabled = $3, ip_whitelist = $4,
		     require_2fa = $5, session_timeout_minutes = $6, password_min_length = $7,
		     password_require_uppercase = $8, password_require_numbers = $9,
		     password_require_special_chars = $10, password_expiry_days = $11,
		     user_registration_mode = $12, password_reset_token_ttl_minutes = $13,
		     updated_at = $14, updated_by = $15
		 WHERE domain_id = $16`,
		settings.TLSPolicy, settings.QuotaPerUser, settings.IPWhitelistEnabled, pq.Array(settings.IPWhitelist),
		settings.Require2FA, settings.SessionTimeoutMinutes, settings.PasswordMinLength,
		settings.PasswordRequireUppercase, settings.PasswordRequireNumbers,
		settings.PasswordRequireSpecialChars, settings.PasswordExpiryDays,
		settings.UserRegistrationMode, settings.PasswordResetTokenTTLMinutes,
		settings.UpdatedAt, settings.UpdatedBy,
		settings.DomainID,
	)
	return err
}

// GetAPISettings retrieves API settings for a domain.
func (r *Repository) GetAPISettings(ctx context.Context, domainID string) (*APISettings, error) {
	var settings APISettings
	var cidrList pq.StringArray
	err := r.db.QueryRowContext(ctx,
		`SELECT domain_id, rate_limit_rps, rate_limit_bps, cidr_allowlist_enabled, cidr_allowlist,
		        require_api_key, updated_at, updated_by
		 FROM api_settings WHERE domain_id = $1`,
		domainID,
	).Scan(&settings.DomainID, &settings.RateLimitRPS, &settings.RateLimitBPS,
		&settings.CIDRAllowlistEnabled, &cidrList, &settings.RequireAPIKey,
		&settings.UpdatedAt, &settings.UpdatedBy)
	if err != nil {
		return nil, err
	}
	settings.CIDRAllowlist = cidrList
	return &settings, nil
}

// UpdateAPISettings updates API settings for a domain.
func (r *Repository) UpdateAPISettings(ctx context.Context, settings *APISettings) error {
	settings.UpdatedAt = time.Now()
	_, err := r.db.ExecContext(ctx,
		`UPDATE api_settings
		 SET rate_limit_rps = $1, rate_limit_bps = $2, cidr_allowlist_enabled = $3,
		     cidr_allowlist = $4, require_api_key = $5, updated_at = $6, updated_by = $7
		 WHERE domain_id = $8`,
		settings.RateLimitRPS, settings.RateLimitBPS, settings.CIDRAllowlistEnabled,
		pq.Array(settings.CIDRAllowlist), settings.RequireAPIKey,
		settings.UpdatedAt, settings.UpdatedBy, settings.DomainID,
	)
	return err
}

// CreateAPIKey creates a new API key.
func (r *Repository) CreateAPIKey(ctx context.Context, key *APIKey) error {
	return r.db.QueryRowContext(ctx,
		`INSERT INTO domain_api_keys (domain_id, name, key_hash, scopes, allowed_cidrs, expires_at, revoked, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, $5, $6, false, $7, $7) RETURNING id`,
		key.DomainID, key.Name, key.SecretHash, pq.Array(key.Scopes), pq.Array(key.AllowedCIDRs), key.ExpiresAt, key.CreatedAt,
	).Scan(&key.ID)
}

// GetAPIKey retrieves an API key by ID.
func (r *Repository) GetAPIKey(ctx context.Context, keyID string) (*APIKey, error) {
	var key APIKey
	var revoked bool
	err := r.db.QueryRowContext(ctx,
		`SELECT id::text, domain_id::text, name, key_hash, scopes, allowed_cidrs, created_at, expires_at, revoked
		 FROM domain_api_keys WHERE id = $1`,
		keyID,
	).Scan(&key.ID, &key.DomainID, &key.Name, &key.SecretHash, pq.Array(&key.Scopes), pq.Array(&key.AllowedCIDRs),
		&key.CreatedAt, &key.ExpiresAt, &revoked)
	key.CreatedBy = "domain-api-key"
	key.IsActive = !revoked
	return &key, err
}

// ListAPIKeys lists all API keys for a domain.
func (r *Repository) ListAPIKeys(ctx context.Context, domainID string) ([]APIKey, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id::text, domain_id::text, name, key_hash, scopes, allowed_cidrs, created_at, expires_at, revoked
		 FROM domain_api_keys WHERE domain_id = $1 ORDER BY created_at DESC`,
		domainID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var keys []APIKey
	for rows.Next() {
		var key APIKey
		var revoked bool
		if err := rows.Scan(&key.ID, &key.DomainID, &key.Name, &key.SecretHash, pq.Array(&key.Scopes), pq.Array(&key.AllowedCIDRs),
			&key.CreatedAt, &key.ExpiresAt, &revoked); err != nil {
			return nil, err
		}
		key.CreatedBy = "domain-api-key"
		key.IsActive = !revoked
		keys = append(keys, key)
	}
	return keys, rows.Err()
}

// DeleteAPIKey deletes an API key.
func (r *Repository) DeleteAPIKey(ctx context.Context, keyID string) error {
	_, err := r.db.ExecContext(ctx, `UPDATE domain_api_keys SET revoked = true, updated_at = now() WHERE id = $1`, keyID)
	return err
}

// UpdateAPIKeyLastUsed updates the last_used_at timestamp for an API key.
func (r *Repository) UpdateAPIKeyLastUsed(ctx context.Context, keyID string) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE domain_api_keys SET updated_at = NOW() WHERE id = $1`,
		keyID,
	)
	return err
}

// RotateAPIKeySecret updates the secret hash for an API key.
func (r *Repository) RotateAPIKeySecret(ctx context.Context, keyID, newSecretHash string) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE domain_api_keys SET key_hash = $1, updated_at = now() WHERE id = $2`,
		newSecretHash, keyID,
	)
	return err
}
