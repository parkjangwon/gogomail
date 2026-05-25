package app

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/gogomail/gogomail/internal/admin"
	"github.com/gogomail/gogomail/internal/configstore"
)

func (s adminService) GetCompanyConfig(ctx context.Context, companyID, key string) (configstore.ConfigEntry, error) {
	if s.configStore == nil {
		return configstore.ConfigEntry{}, fmt.Errorf("config store is not configured")
	}
	entry, err := s.configStore.Get(ctx, configstore.ScopeCompany, companyID, key)
	if err != nil {
		return configstore.ConfigEntry{}, err
	}
	return *entry, nil
}

func (s adminService) SetCompanyConfig(ctx context.Context, companyID, key string, value json.RawMessage, locked bool, expectedVersion int64) (configstore.ConfigEntry, error) {
	if s.configStore == nil {
		return configstore.ConfigEntry{}, fmt.Errorf("config store is not configured")
	}
	entry, err := s.configStore.Set(ctx, configstore.ScopeCompany, companyID, key, value, locked, expectedVersion)
	if err != nil {
		return configstore.ConfigEntry{}, err
	}
	return *entry, nil
}

func (s adminService) DeleteCompanyConfig(ctx context.Context, companyID, key string, expectedVersion int64) error {
	if s.configStore == nil {
		return fmt.Errorf("config store is not configured")
	}
	return s.configStore.Delete(ctx, configstore.ScopeCompany, companyID, key, expectedVersion)
}

func (s adminService) ListCompanyConfig(ctx context.Context, companyID string) ([]configstore.ConfigEntry, error) {
	if s.configStore == nil {
		return nil, fmt.Errorf("config store is not configured")
	}
	return s.configStore.List(ctx, configstore.ScopeCompany, companyID)
}

func (s adminService) GetDomainConfig(ctx context.Context, domainID, key string) (configstore.ConfigEntry, error) {
	if s.configStore == nil {
		return configstore.ConfigEntry{}, fmt.Errorf("config store is not configured")
	}
	entry, err := s.configStore.Get(ctx, configstore.ScopeDomain, domainID, key)
	if err != nil {
		return configstore.ConfigEntry{}, err
	}
	return *entry, nil
}

func (s adminService) SetDomainConfig(ctx context.Context, domainID, key string, value json.RawMessage, locked bool, expectedVersion int64) (configstore.ConfigEntry, error) {
	if s.configStore == nil {
		return configstore.ConfigEntry{}, fmt.Errorf("config store is not configured")
	}
	entry, err := s.configStore.Set(ctx, configstore.ScopeDomain, domainID, key, value, locked, expectedVersion)
	if err != nil {
		return configstore.ConfigEntry{}, err
	}
	return *entry, nil
}

func (s adminService) DeleteDomainConfig(ctx context.Context, domainID, key string, expectedVersion int64) error {
	if s.configStore == nil {
		return fmt.Errorf("config store is not configured")
	}
	return s.configStore.Delete(ctx, configstore.ScopeDomain, domainID, key, expectedVersion)
}

func (s adminService) ListDomainConfig(ctx context.Context, domainID string) ([]configstore.ConfigEntry, error) {
	if s.configStore == nil {
		return nil, fmt.Errorf("config store is not configured")
	}
	return s.configStore.List(ctx, configstore.ScopeDomain, domainID)
}

func (s adminService) GetUserConfig(ctx context.Context, userID, key string) (configstore.ConfigEntry, error) {
	if s.configStore == nil {
		return configstore.ConfigEntry{}, fmt.Errorf("config store is not configured")
	}
	entry, err := s.configStore.Get(ctx, configstore.ScopeUser, userID, key)
	if err != nil {
		return configstore.ConfigEntry{}, err
	}
	return *entry, nil
}

func (s adminService) SetUserConfig(ctx context.Context, userID, key string, value json.RawMessage, locked bool, expectedVersion int64) (configstore.ConfigEntry, error) {
	if s.configStore == nil {
		return configstore.ConfigEntry{}, fmt.Errorf("config store is not configured")
	}
	entry, err := s.configStore.Set(ctx, configstore.ScopeUser, userID, key, value, locked, expectedVersion)
	if err != nil {
		return configstore.ConfigEntry{}, err
	}
	return *entry, nil
}

func (s adminService) DeleteUserConfig(ctx context.Context, userID, key string, expectedVersion int64) error {
	if s.configStore == nil {
		return fmt.Errorf("config store is not configured")
	}
	return s.configStore.Delete(ctx, configstore.ScopeUser, userID, key, expectedVersion)
}

func (s adminService) ListUserConfig(ctx context.Context, userID string) ([]configstore.ConfigEntry, error) {
	if s.configStore == nil {
		return nil, fmt.Errorf("config store is not configured")
	}
	return s.configStore.List(ctx, configstore.ScopeUser, userID)
}

func (s adminService) PropagateCompanyConfig(ctx context.Context, companyID string, scope configstore.PropagateScope, key string, value json.RawMessage, locked bool) error {
	if s.configStore == nil {
		return fmt.Errorf("config store is not configured")
	}
	return s.configStore.Propagate(ctx, companyID, scope, key, value, locked)
}

func (s adminService) GetDomainSettings(ctx context.Context, domainID string) (*admin.DomainSettings, error) {
	if s.adminSvc == nil {
		return nil, fmt.Errorf("admin service is not configured")
	}
	return s.adminSvc.GetDomainSettings(ctx, domainID)
}

func (s adminService) ListAdminRoles(ctx context.Context, companyID string) ([]admin.RoleSummary, error) {
	if s.adminSvc == nil {
		return nil, fmt.Errorf("admin service is not configured")
	}
	return s.adminSvc.ListRoleSummaries(ctx, companyID)
}

func (s adminService) CreateAdminRole(ctx context.Context, req admin.CreateRoleRequest) (admin.RoleSummary, error) {
	if s.adminSvc == nil {
		return admin.RoleSummary{}, fmt.Errorf("admin service is not configured")
	}
	return s.adminSvc.CreateRoleSummary(ctx, req)
}

func (s adminService) UpdateDomainSettings(ctx context.Context, settings *admin.DomainSettings) error {
	if s.adminSvc == nil {
		return fmt.Errorf("admin service is not configured")
	}
	return s.adminSvc.UpdateDomainSettings(ctx, settings)
}

func (s adminService) GetAPISettings(ctx context.Context, domainID string) (*admin.APISettings, error) {
	if s.adminSvc == nil {
		return nil, fmt.Errorf("admin service is not configured")
	}
	return s.adminSvc.GetAPISettings(ctx, domainID)
}

func (s adminService) UpdateAPISettings(ctx context.Context, settings *admin.APISettings) error {
	if s.adminSvc == nil {
		return fmt.Errorf("admin service is not configured")
	}
	return s.adminSvc.UpdateAPISettings(ctx, settings)
}

func (s adminService) CreateAPIKey(ctx context.Context, key *admin.APIKey) (secret string, err error) {
	if s.adminSvc == nil {
		return "", fmt.Errorf("admin service is not configured")
	}
	return s.adminSvc.CreateAPIKey(ctx, key)
}

func (s adminService) ListAPIKeys(ctx context.Context, domainID string) ([]admin.APIKey, error) {
	if s.adminSvc == nil {
		return nil, fmt.Errorf("admin service is not configured")
	}
	return s.adminSvc.ListAPIKeys(ctx, domainID)
}

func (s adminService) DeleteAPIKey(ctx context.Context, keyID string) error {
	if s.adminSvc == nil {
		return fmt.Errorf("admin service is not configured")
	}
	return s.adminSvc.DeleteAPIKey(ctx, keyID)
}

func (s adminService) RotateAPIKey(ctx context.Context, keyID string) (newSecret string, err error) {
	if s.adminSvc == nil {
		return "", fmt.Errorf("admin service is not configured")
	}
	return s.adminSvc.RotateAPIKey(ctx, keyID)
}

func (s adminService) CreateAlertRule(ctx context.Context, rule *admin.AlertRule) error {
	if s.adminSvc == nil {
		return fmt.Errorf("admin service is not configured")
	}
	return s.adminSvc.CreateAlertRule(ctx, rule)
}

func (s adminService) GetAlertRule(ctx context.Context, ruleID string) (*admin.AlertRule, error) {
	if s.adminSvc == nil {
		return nil, fmt.Errorf("admin service is not configured")
	}
	return s.adminSvc.GetAlertRule(ctx, ruleID)
}

func (s adminService) ListAlertRules(ctx context.Context, companyID string) ([]admin.AlertRule, error) {
	if s.adminSvc == nil {
		return nil, fmt.Errorf("admin service is not configured")
	}
	return s.adminSvc.ListAlertRules(ctx, companyID)
}

func (s adminService) UpdateAlertRule(ctx context.Context, rule *admin.AlertRule) error {
	if s.adminSvc == nil {
		return fmt.Errorf("admin service is not configured")
	}
	return s.adminSvc.UpdateAlertRule(ctx, rule)
}

func (s adminService) DeleteAlertRule(ctx context.Context, ruleID string) error {
	if s.adminSvc == nil {
		return fmt.Errorf("admin service is not configured")
	}
	return s.adminSvc.DeleteAlertRule(ctx, ruleID)
}

func (s adminService) CreateAlertChannel(ctx context.Context, channel *admin.AlertChannel) error {
	if s.adminSvc == nil {
		return fmt.Errorf("admin service is not configured")
	}
	return s.adminSvc.CreateAlertChannel(ctx, channel)
}

func (s adminService) GetAlertChannel(ctx context.Context, channelID string) (*admin.AlertChannel, error) {
	if s.adminSvc == nil {
		return nil, fmt.Errorf("admin service is not configured")
	}
	return s.adminSvc.GetAlertChannel(ctx, channelID)
}

func (s adminService) ListAlertChannels(ctx context.Context, companyID string) ([]admin.AlertChannel, error) {
	if s.adminSvc == nil {
		return nil, fmt.Errorf("admin service is not configured")
	}
	return s.adminSvc.ListAlertChannels(ctx, companyID)
}

func (s adminService) UpdateAlertChannel(ctx context.Context, channel *admin.AlertChannel) error {
	if s.adminSvc == nil {
		return fmt.Errorf("admin service is not configured")
	}
	return s.adminSvc.UpdateAlertChannel(ctx, channel)
}

func (s adminService) DeleteAlertChannel(ctx context.Context, channelID string) error {
	if s.adminSvc == nil {
		return fmt.Errorf("admin service is not configured")
	}
	return s.adminSvc.DeleteAlertChannel(ctx, channelID)
}

func (s adminService) ListAlertEvents(ctx context.Context, filter admin.AlertEventFilter) ([]admin.AlertEvent, bool, error) {
	if s.adminSvc == nil {
		return nil, false, fmt.Errorf("admin service is not configured")
	}
	return s.adminSvc.ListAlertEvents(ctx, filter)
}

func (s adminService) LogAlertEvent(ctx context.Context, event *admin.AlertEvent) error {
	if s.adminSvc == nil {
		return fmt.Errorf("admin service is not configured")
	}
	return s.adminSvc.LogAlertEvent(ctx, event)
}
