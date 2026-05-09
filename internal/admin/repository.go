package admin

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

// RepositoryInterface defines admin RBAC data access operations.
type RepositoryInterface interface {
	// Role operations
	CreateRole(ctx context.Context, role *Role) error
	GetRole(ctx context.Context, id string) (*Role, error)
	ListRoles(ctx context.Context, companyID string) ([]Role, error)
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
		`SELECT id, company_id, user_id, role_id, assigned_at, assigned_by, expires_at
		 FROM admin_user_roles
		 WHERE user_id = $1 AND company_id = $2
		 AND (expires_at IS NULL OR expires_at > NOW())
		 ORDER BY assigned_at DESC`,
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

// ListRolesForUser gets all role definitions for a user's active assignments.
func (r *Repository) ListRolesForUser(ctx context.Context, userID string) ([]Role, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT DISTINCT ard.id, ard.company_id, ard.name, ard.description, ard.is_builtin,
		        ard.created_by, ard.created_at, ard.updated_at
		 FROM admin_role_definitions ard
		 JOIN admin_user_roles aur ON ard.id = aur.role_id
		 WHERE aur.user_id = $1 AND (aur.expires_at IS NULL OR aur.expires_at > NOW())
		 ORDER BY ard.name`,
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
	query := `SELECT id, company_id, admin_user_id, action, resource_type, resource_id,
	          changes, ip_address, user_agent, timestamp
	          FROM audit_logs WHERE company_id = $1`
	args := []interface{}{filter.CompanyID}
	argNum := 2

	if filter.AdminUserID != "" {
		query += ` AND admin_user_id = $` + fmt.Sprintf("%d", argNum)
		args = append(args, filter.AdminUserID)
		argNum++
	}
	if filter.Action != "" {
		query += ` AND action = $` + fmt.Sprintf("%d", argNum)
		args = append(args, filter.Action)
		argNum++
	}
	if filter.ResourceType != "" {
		query += ` AND resource_type = $` + fmt.Sprintf("%d", argNum)
		args = append(args, filter.ResourceType)
		argNum++
	}
	if filter.StartTime != nil {
		query += ` AND timestamp >= $` + fmt.Sprintf("%d", argNum)
		args = append(args, *filter.StartTime)
		argNum++
	}
	if filter.EndTime != nil {
		query += ` AND timestamp <= $` + fmt.Sprintf("%d", argNum)
		args = append(args, *filter.EndTime)
		argNum++
	}

	// Count total
	countQuery := "SELECT COUNT(*) FROM audit_logs WHERE company_id = $1"
	countArgs := []interface{}{filter.CompanyID}
	if filter.AdminUserID != "" {
		countQuery += " AND admin_user_id = $2"
		countArgs = append(countArgs, filter.AdminUserID)
	}
	var total int64
	r.db.QueryRowContext(ctx, countQuery, countArgs...).Scan(&total)

	query += ` ORDER BY timestamp DESC LIMIT $` + fmt.Sprintf("%d", argNum) + ` OFFSET $` + fmt.Sprintf("%d", argNum+1)
	args = append(args, filter.Limit, filter.Offset)

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
