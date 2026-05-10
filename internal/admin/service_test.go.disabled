package admin

import (
	"context"
	"testing"
	"time"
)

type mockRepository struct {
	roles          map[string]*Role
	permissions    map[string][]*Permission
	userRoles      map[string][]*UserRole
	auditPolicies  map[string]*AuditPolicyConfig
	auditLogs      []*AuditLog
	loginAuditLogs []*LoginAuditLog
}

func newMockRepository() *mockRepository {
	return &mockRepository{
		roles:          make(map[string]*Role),
		permissions:    make(map[string][]*Permission),
		userRoles:      make(map[string][]*UserRole),
		auditPolicies:  make(map[string]*AuditPolicyConfig),
		auditLogs:      make([]*AuditLog, 0),
		loginAuditLogs: make([]*LoginAuditLog, 0),
	}
}

func (m *mockRepository) CreateRole(ctx context.Context, role *Role) error {
	if role.ID == "" {
		role.ID = "role-" + time.Now().Format("20060102150405")
	}
	m.roles[role.ID] = role
	return nil
}

func (m *mockRepository) GetRole(ctx context.Context, roleID string) (*Role, error) {
	if role, ok := m.roles[roleID]; ok {
		return role, nil
	}
	return nil, ErrRoleNotFound
}

func (m *mockRepository) ListRoles(ctx context.Context, companyID string) ([]Role, error) {
	var roles []Role
	for _, role := range m.roles {
		if role.CompanyID == companyID {
			roles = append(roles, *role)
		}
	}
	return roles, nil
}

func (m *mockRepository) UpdateRole(ctx context.Context, role *Role) error {
	if _, ok := m.roles[role.ID]; !ok {
		return ErrRoleNotFound
	}
	m.roles[role.ID] = role
	return nil
}

func (m *mockRepository) DeleteRole(ctx context.Context, roleID string) error {
	delete(m.roles, roleID)
	return nil
}

func (m *mockRepository) AddPermission(ctx context.Context, permission *Permission) error {
	if permission.ID == "" {
		permission.ID = "perm-" + time.Now().Format("20060102150405")
	}
	m.permissions[permission.RoleID] = append(m.permissions[permission.RoleID], permission)
	return nil
}

func (m *mockRepository) RemovePermission(ctx context.Context, permissionID string) error {
	for roleID, perms := range m.permissions {
		for i, p := range perms {
			if p.ID == permissionID {
				m.permissions[roleID] = append(perms[:i], perms[i+1:]...)
				return nil
			}
		}
	}
	return ErrPermissionNotFound
}

func (m *mockRepository) ListPermissions(ctx context.Context, roleID string) ([]Permission, error) {
	var perms []Permission
	for _, p := range m.permissions[roleID] {
		perms = append(perms, *p)
	}
	return perms, nil
}

func (m *mockRepository) GetPermissionsByRole(ctx context.Context, roleID string) ([]Permission, error) {
	var perms []Permission
	for _, p := range m.permissions[roleID] {
		perms = append(perms, *p)
	}
	return perms, nil
}

func (m *mockRepository) AssignRole(ctx context.Context, userRole *UserRole) error {
	if userRole.ID == "" {
		userRole.ID = "ur-" + time.Now().Format("20060102150405")
	}
	m.userRoles[userRole.UserID] = append(m.userRoles[userRole.UserID], userRole)
	return nil
}

func (m *mockRepository) RevokeRole(ctx context.Context, userRoleID string) error {
	for userID, roles := range m.userRoles {
		for i, ur := range roles {
			if ur.ID == userRoleID {
				m.userRoles[userID] = append(roles[:i], roles[i+1:]...)
				return nil
			}
		}
	}
	return ErrUserRoleNotFound
}

func (m *mockRepository) GetUserRoles(ctx context.Context, userID, companyID string) ([]UserRole, error) {
	var roles []UserRole
	for _, ur := range m.userRoles[userID] {
		if ur.CompanyID == companyID {
			roles = append(roles, *ur)
		}
	}
	return roles, nil
}

func (m *mockRepository) ListRolesForUser(ctx context.Context, userID string) ([]Role, error) {
	// This would require fetching roles, returning stub for mock
	return []Role{}, nil
}

func (m *mockRepository) CreateAuditPolicy(ctx context.Context, config *AuditPolicyConfig) error {
	if config.ID == "" {
		config.ID = "ap-" + time.Now().Format("20060102150405")
	}
	key := config.CompanyID + ":" + config.DomainID
	m.auditPolicies[key] = config
	return nil
}

func (m *mockRepository) GetAuditPolicy(ctx context.Context, companyID, domainID string) (*AuditPolicyConfig, error) {
	key := companyID + ":" + domainID
	if policy, ok := m.auditPolicies[key]; ok {
		return policy, nil
	}
	return nil, ErrAuditPolicyNotFound
}

func (m *mockRepository) UpdateAuditPolicy(ctx context.Context, config *AuditPolicyConfig) error {
	key := config.CompanyID + ":" + config.DomainID
	if _, ok := m.auditPolicies[key]; !ok {
		return ErrAuditPolicyNotFound
	}
	m.auditPolicies[key] = config
	return nil
}

func (m *mockRepository) LogAuditEvent(ctx context.Context, log *AuditLog) error {
	if log.ID == "" {
		log.ID = "al-" + time.Now().Format("20060102150405")
	}
	m.auditLogs = append(m.auditLogs, log)
	return nil
}

func (m *mockRepository) ListAuditLogs(ctx context.Context, filter AuditLogFilter) ([]AuditLog, int64, error) {
	var logs []AuditLog
	for _, log := range m.auditLogs {
		logs = append(logs, *log)
	}
	return logs, int64(len(logs)), nil
}

func (m *mockRepository) GetAuditLog(ctx context.Context, logID string) (*AuditLog, error) {
	for _, log := range m.auditLogs {
		if log.ID == logID {
			return log, nil
		}
	}
	return nil, ErrAuditLogNotFound
}

func (m *mockRepository) DeleteAuditLogsBefore(ctx context.Context, companyID string, before time.Time) (int64, error) {
	oldLen := len(m.auditLogs)
	filtered := make([]*AuditLog, 0)
	for _, log := range m.auditLogs {
		if log.CompanyID != companyID || log.Timestamp.After(before) {
			filtered = append(filtered, log)
		}
	}
	m.auditLogs = filtered
	return int64(oldLen - len(filtered)), nil
}

func (m *mockRepository) LogLoginAttempt(ctx context.Context, log *LoginAuditLog) error {
	if log.ID == "" {
		log.ID = "la-" + time.Now().Format("20060102150405")
	}
	m.loginAuditLogs = append(m.loginAuditLogs, log)
	return nil
}

func (m *mockRepository) ListLoginAudits(ctx context.Context, filter LoginAuditFilter) ([]LoginAuditLog, error) {
	var logs []LoginAuditLog
	for _, log := range m.loginAuditLogs {
		logs = append(logs, *log)
	}
	return logs, nil
}

func TestCreateRole(t *testing.T) {
	repo := newMockRepository()
	svc := NewService(repo)
	ctx := context.Background()

	tests := []struct {
		name      string
		role      *Role
		shouldErr bool
	}{
		{
			name: "valid custom role",
			role: &Role{
				CompanyID:   "company-1",
				Name:        "Custom Admin",
				Description: "Custom admin role",
				IsBuiltin:   false,
			},
			shouldErr: false,
		},
		{
			name: "missing company_id",
			role: &Role{
				Name:      "Invalid Role",
				IsBuiltin: false,
			},
			shouldErr: true,
		},
		{
			name: "missing name",
			role: &Role{
				CompanyID: "company-1",
				IsBuiltin: false,
			},
			shouldErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := svc.CreateRole(ctx, tt.role)
			if (err != nil) != tt.shouldErr {
				t.Errorf("CreateRole() error = %v, shouldErr %v", err, tt.shouldErr)
			}
		})
	}
}

func TestDeleteRole(t *testing.T) {
	repo := newMockRepository()
	svc := NewService(repo)
	ctx := context.Background()

	builtin := &Role{
		ID:        "builtin-role",
		CompanyID: "company-1",
		Name:      "System Admin",
		IsBuiltin: true,
	}
	custom := &Role{
		ID:        "custom-role",
		CompanyID: "company-1",
		Name:      "Custom Admin",
		IsBuiltin: false,
	}

	repo.CreateRole(ctx, builtin)
	repo.CreateRole(ctx, custom)

	tests := []struct {
		name      string
		roleID    string
		shouldErr bool
	}{
		{
			name:      "cannot delete builtin role",
			roleID:    "builtin-role",
			shouldErr: true,
		},
		{
			name:      "can delete custom role",
			roleID:    "custom-role",
			shouldErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := svc.DeleteRole(ctx, tt.roleID)
			if (err != nil) != tt.shouldErr {
				t.Errorf("DeleteRole() error = %v, shouldErr %v", err, tt.shouldErr)
			}
		})
	}
}

func TestAssignUserToRole(t *testing.T) {
	repo := newMockRepository()
	svc := NewService(repo)
	ctx := context.Background()

	role := &Role{
		ID:        "role-1",
		CompanyID: "company-1",
		Name:      "Admin",
		IsBuiltin: false,
	}
	repo.CreateRole(ctx, role)

	tests := []struct {
		name      string
		companyID string
		userID    string
		roleID    string
		shouldErr bool
	}{
		{
			name:      "valid assignment",
			companyID: "company-1",
			userID:    "user-1",
			roleID:    "role-1",
			shouldErr: false,
		},
		{
			name:      "missing companyID",
			companyID: "",
			userID:    "user-1",
			roleID:    "role-1",
			shouldErr: true,
		},
		{
			name:      "missing userID",
			companyID: "company-1",
			userID:    "",
			roleID:    "role-1",
			shouldErr: true,
		},
		{
			name:      "missing roleID",
			companyID: "company-1",
			userID:    "user-1",
			roleID:    "",
			shouldErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := svc.AssignUserToRole(ctx, tt.companyID, tt.userID, tt.roleID)
			if (err != nil) != tt.shouldErr {
				t.Errorf("AssignUserToRole() error = %v, shouldErr %v", err, tt.shouldErr)
			}
		})
	}
}

func TestAddPermissionToRole(t *testing.T) {
	repo := newMockRepository()
	svc := NewService(repo)
	ctx := context.Background()

	role := &Role{
		ID:        "role-1",
		CompanyID: "company-1",
		Name:      "Admin",
		IsBuiltin: false,
	}
	repo.CreateRole(ctx, role)

	tests := []struct {
		name      string
		roleID    string
		resource  string
		action    string
		scope     string
		shouldErr bool
	}{
		{
			name:      "valid permission",
			roleID:    "role-1",
			resource:  "users",
			action:    "create",
			scope:     "own_company",
			shouldErr: false,
		},
		{
			name:      "missing roleID",
			roleID:    "",
			resource:  "users",
			action:    "create",
			scope:     "own_company",
			shouldErr: true,
		},
		{
			name:      "missing resource",
			roleID:    "role-1",
			resource:  "",
			action:    "create",
			scope:     "own_company",
			shouldErr: true,
		},
		{
			name:      "invalid resource",
			roleID:    "role-1",
			resource:  "invalid",
			action:    "create",
			scope:     "own_company",
			shouldErr: true,
		},
		{
			name:      "invalid action",
			roleID:    "role-1",
			resource:  "users",
			action:    "invalid",
			scope:     "own_company",
			shouldErr: true,
		},
		{
			name:      "invalid scope",
			roleID:    "role-1",
			resource:  "users",
			action:    "create",
			scope:     "invalid",
			shouldErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := svc.AddPermissionToRole(ctx, tt.roleID, tt.resource, tt.action, tt.scope, nil)
			if (err != nil) != tt.shouldErr {
				t.Errorf("AddPermissionToRole() error = %v, shouldErr %v", err, tt.shouldErr)
			}
		})
	}
}

func TestCreateAuditPolicy(t *testing.T) {
	repo := newMockRepository()
	svc := NewService(repo)
	ctx := context.Background()

	tests := []struct {
		name      string
		companyID string
		domainID  string
		shouldErr bool
	}{
		{
			name:      "valid audit policy",
			companyID: "company-1",
			domainID:  "domain-1",
			shouldErr: false,
		},
		{
			name:      "missing companyID",
			companyID: "",
			domainID:  "domain-1",
			shouldErr: true,
		},
		{
			name:      "missing domainID",
			companyID: "company-1",
			domainID:  "",
			shouldErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := DefaultAuditPolicy()
			config.CompanyID = tt.companyID
			config.DomainID = tt.domainID
			err := svc.CreateAuditPolicy(ctx, &config)
			if (err != nil) != tt.shouldErr {
				t.Errorf("CreateAuditPolicy() error = %v, shouldErr %v", err, tt.shouldErr)
			}
		})
	}
}

func TestLogAuditEvent(t *testing.T) {
	repo := newMockRepository()
	svc := NewService(repo)
	ctx := context.Background()

	tests := []struct {
		name      string
		action    string
		shouldErr bool
	}{
		{
			name:      "valid audit event",
			action:    "user.create",
			shouldErr: false,
		},
		{
			name:      "missing action",
			action:    "",
			shouldErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := svc.LogAuditEvent(ctx, "company-1", "admin-1", tt.action, "user", "user-1", nil)
			if (err != nil) != tt.shouldErr {
				t.Errorf("LogAuditEvent() error = %v, shouldErr %v", err, tt.shouldErr)
			}
		})
	}
}
