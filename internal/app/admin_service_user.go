package app

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/gogomail/gogomail/internal/admin"
	"github.com/gogomail/gogomail/internal/configstore"
	"github.com/gogomail/gogomail/internal/idprovider"
	ldapidp "github.com/gogomail/gogomail/internal/idprovider/ldap"
	rdbmsidp "github.com/gogomail/gogomail/internal/idprovider/rdbms"
	"github.com/gogomail/gogomail/internal/maildb"
	"github.com/google/uuid"
)

func (s adminService) CreateDomain(ctx context.Context, req maildb.CreateDomainRequest) (maildb.DomainView, error) {
	domain, err := s.Repository.CreateDomain(ctx, req)
	if err != nil {
		return domain, err
	}
	if s.configStore != nil {
		if err := s.configStore.PropagateFromParent(ctx, configstore.ScopeDomain, domain.ID, configstore.ScopeCompany, domain.CompanyID); err != nil {
			slog.WarnContext(ctx, "failed to propagate config after domain creation", "err", err, "domainID", domain.ID)
		}
	}
	return domain, nil
}

func (s adminService) CreateUser(ctx context.Context, req maildb.CreateUserRequest) (maildb.UserView, error) {
	user, err := s.Repository.CreateUser(ctx, req)
	if err != nil {
		return user, err
	}
	if s.configStore != nil {
		if err := s.configStore.PropagateFromParent(ctx, configstore.ScopeUser, user.ID, configstore.ScopeDomain, user.DomainID); err != nil {
			slog.WarnContext(ctx, "failed to propagate config after user creation", "err", err, "userID", user.ID)
		}
	}
	return user, nil
}

func (s adminService) GetUserMFAStatus(ctx context.Context, userID string) (maildb.UserMFAStatus, error) {
	return s.Repository.GetUserMFAStatus(ctx, userID)
}

func (s adminService) ResetUserMFA(ctx context.Context, userID string) error {
	return s.Repository.ResetUserMFA(ctx, userID)
}

func (s adminService) GetMFAStats(ctx context.Context, companyID string) (maildb.MFAStats, error) {
	return s.Repository.GetMFAStats(ctx, companyID)
}

func (s adminService) ListLoginAttempts(ctx context.Context, filter admin.LoginAuditFilter) ([]admin.LoginAuditLog, error) {
	if s.adminSvc == nil {
		return nil, fmt.Errorf("admin service is not configured")
	}
	if filter.CompanyID == "" {
		return nil, fmt.Errorf("company id required for login audit lookup")
	}
	return s.adminSvc.ListLoginAttempts(ctx, filter)
}

func (s adminService) TriggerLDAPSync(ctx context.Context, domainID, syncType string) (map[string]interface{}, error) {
	if syncType != "users" && syncType != "groups" && syncType != "memberships" {
		return nil, fmt.Errorf("invalid sync_type: must be 'users', 'groups', or 'memberships'")
	}

	domainUUID, err := uuid.Parse(domainID)
	if err != nil {
		return nil, fmt.Errorf("invalid domain_id: %w", err)
	}

	// Create sync run record
	runID, err := s.Repository.CreateLDAPSyncRun(ctx, domainUUID, syncType, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create sync run: %w", err)
	}

	status := "failed"
	errMsg := ldapidp.ErrSyncNotConfigured.Error()

	err = s.Repository.UpdateLDAPSyncRun(ctx, runID, status,
		0, 0, 0,
		0, 0, &errMsg)
	if err != nil {
		return nil, fmt.Errorf("failed to update sync run: %w", err)
	}

	return nil, ldapidp.ErrSyncNotConfigured
}

func (s adminService) GetLDAPSyncRuns(ctx context.Context, req maildb.LDAPSyncRunListRequest) ([]maildb.LDAPSyncRunView, error) {
	return s.Repository.GetLDAPSyncRuns(ctx, req)
}

func (s adminService) GetLDAPSyncRun(ctx context.Context, runID string) (*maildb.LDAPSyncRunView, error) {
	id, err := uuid.Parse(runID)
	if err != nil {
		return nil, fmt.Errorf("invalid run id: %w", err)
	}
	return s.Repository.GetLDAPSyncRun(ctx, id)
}

func (s adminService) GetLDAPSyncConflicts(ctx context.Context, req maildb.LDAPSyncConflictListRequest) ([]maildb.LDAPSyncConflictView, error) {
	return s.Repository.GetLDAPSyncConflicts(ctx, req)
}

func (s adminService) GetLDAPSyncConflict(ctx context.Context, conflictID string) (*maildb.LDAPSyncConflictView, error) {
	id, err := uuid.Parse(conflictID)
	if err != nil {
		return nil, fmt.Errorf("invalid conflict id: %w", err)
	}
	return s.Repository.GetLDAPSyncConflict(ctx, id)
}

func (s adminService) ResolveLDAPSyncConflict(ctx context.Context, conflictID, resolution string) error {
	id, err := uuid.Parse(conflictID)
	if err != nil {
		return fmt.Errorf("invalid conflict id: %w", err)
	}
	return s.Repository.ResolveLDAPSyncConflict(ctx, id, resolution)
}

func (s adminService) TriggerRDBMSSync(ctx context.Context, domainID, syncType string) (map[string]interface{}, error) {
	if syncType != "users" && syncType != "groups" && syncType != "memberships" {
		return nil, fmt.Errorf("invalid sync_type: must be 'users', 'groups', or 'memberships'")
	}

	domainUUID, err := uuid.Parse(domainID)
	if err != nil {
		return nil, fmt.Errorf("invalid domain_id: %w", err)
	}

	// Create sync run record
	runID, err := s.Repository.CreateRDBMSSyncRun(ctx, domainUUID, syncType, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create sync run: %w", err)
	}

	status := "failed"
	errMsg := rdbmsidp.ErrSyncNotConfigured.Error()

	err = s.Repository.UpdateRDBMSSyncRun(ctx, runID, status,
		0, 0, 0, 0, 0, 0,
		0, 0, &errMsg)
	if err != nil {
		return nil, fmt.Errorf("failed to update sync run: %w", err)
	}

	return nil, rdbmsidp.ErrSyncNotConfigured
}

func (s adminService) GetRDBMSSyncRuns(ctx context.Context, req maildb.RDBMSSyncRunListRequest) ([]maildb.RDBMSSyncRunView, error) {
	return s.Repository.GetRDBMSSyncRuns(ctx, req)
}

func (s adminService) GetRDBMSSyncRun(ctx context.Context, runID string) (*maildb.RDBMSSyncRunView, error) {
	id, err := uuid.Parse(runID)
	if err != nil {
		return nil, fmt.Errorf("invalid run id: %w", err)
	}
	return s.Repository.GetRDBMSSyncRun(ctx, id)
}

func (s adminService) GetRDBMSSyncConflicts(ctx context.Context, req maildb.RDBMSSyncConflictListRequest) ([]maildb.RDBMSSyncConflictView, error) {
	return s.Repository.GetRDBMSSyncConflicts(ctx, req)
}

func (s adminService) GetRDBMSSyncConflict(ctx context.Context, conflictID string) (*maildb.RDBMSSyncConflictView, error) {
	id, err := uuid.Parse(conflictID)
	if err != nil {
		return nil, fmt.Errorf("invalid conflict id: %w", err)
	}
	return s.Repository.GetRDBMSSyncConflict(ctx, id)
}

func (s adminService) ResolveRDBMSSyncConflict(ctx context.Context, conflictID, resolution string) error {
	id, err := uuid.Parse(conflictID)
	if err != nil {
		return fmt.Errorf("invalid conflict id: %w", err)
	}
	return s.Repository.ResolveRDBMSSyncConflict(ctx, id, resolution)
}

func (s adminService) GetDomainIdPConfig(ctx context.Context, domainID string) (*idprovider.Config, error) {
	if s.idpConfigRepo == nil {
		return &idprovider.Config{DomainID: domainID, ProviderType: "database", Settings: map[string]interface{}{}}, nil
	}
	return s.idpConfigRepo.GetConfigByDomain(ctx, domainID)
}

func (s adminService) SetDomainIdPConfig(ctx context.Context, cfg *idprovider.Config) error {
	if s.idpConfigRepo == nil {
		return fmt.Errorf("idp config repository is not configured")
	}
	existing, err := s.idpConfigRepo.GetConfigByDomain(ctx, cfg.DomainID)
	if err != nil {
		return err
	}
	if existing.ProviderType == "database" {
		return s.idpConfigRepo.CreateConfig(ctx, cfg)
	}
	return s.idpConfigRepo.UpdateConfig(ctx, cfg)
}

func (s adminService) DeleteDomainIdPConfig(ctx context.Context, domainID string) error {
	if s.idpConfigRepo == nil {
		return fmt.Errorf("idp config repository is not configured")
	}
	return s.idpConfigRepo.DeleteConfig(ctx, domainID)
}
