package ldap

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/gogomail/gogomail/internal/maildb"
)

// AdminService provides admin operations for LDAP sync.
type AdminService struct {
	provider   *Provider
	repository *maildb.Repository
}

// NewAdminService creates a new LDAP admin service.
func NewAdminService(provider *Provider, repo *maildb.Repository) *AdminService {
	return &AdminService{
		provider:   provider,
		repository: repo,
	}
}

// TriggerLDAPSync initiates an on-demand LDAP sync for users, groups, or memberships.
// Returns the sync run ID and result summary.
func (s *AdminService) TriggerLDAPSync(ctx context.Context, domainID string, syncType string) (map[string]interface{}, error) {
	if syncType != "users" && syncType != "groups" && syncType != "memberships" {
		return nil, fmt.Errorf("invalid sync_type: must be 'users', 'groups', or 'memberships'")
	}

	domainUUID, err := uuid.Parse(domainID)
	if err != nil {
		return nil, fmt.Errorf("invalid domain_id: %w", err)
	}

	// Create sync run record
	runID, err := s.repository.CreateLDAPSyncRun(ctx, domainUUID, syncType, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create sync run: %w", err)
	}

	// Execute sync based on type
	syncResult, syncErr := s.executeLDAPSync(ctx, domainUUID, syncType)

	// Determine status
	status := "success"
	errorMsg := ""
	if syncErr != nil {
		status = "failed"
		errorMsg = syncErr.Error()
	} else if syncResult.ConflictCount > 0 {
		status = "partial"
	}

	// Update sync run with results
	errMsg := (*string)(nil)
	if errorMsg != "" {
		errMsg = &errorMsg
	}

	err = s.repository.UpdateLDAPSyncRun(ctx, runID, status,
		syncResult.UsersCreated, syncResult.UsersUpdated, syncResult.UsersDeleted,
		syncResult.ConflictCount, syncResult.ErrorCount, errMsg)
	if err != nil {
		return nil, fmt.Errorf("failed to update sync run: %w", err)
	}

	return map[string]interface{}{
		"sync_run_id":     runID.String(),
		"status":          status,
		"created_count":   syncResult.UsersCreated,
		"updated_count":   syncResult.UsersUpdated,
		"deleted_count":   syncResult.UsersDeleted,
		"conflict_count":  syncResult.ConflictCount,
		"error_count":     syncResult.ErrorCount,
		"last_sync_time":  syncResult.LastSyncTime,
		"error":           syncErr,
	}, nil
}

// GetLDAPSyncHistory retrieves past LDAP sync runs for a domain.
func (s *AdminService) GetLDAPSyncHistory(ctx context.Context, domainID string, limit, offset int) ([]maildb.LDAPSyncRunView, error) {
	domainUUID, err := uuid.Parse(domainID)
	if err != nil {
		return nil, fmt.Errorf("invalid domain_id: %w", err)
	}

	req := maildb.LDAPSyncRunListRequest{
		DomainID: domainUUID.String(),
		Limit:    limit,
		Offset:   offset,
	}

	runs, err := s.repository.GetLDAPSyncRuns(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to get sync history: %w", err)
	}
	return runs, nil
}

// GetLDAPSyncConflicts retrieves unresolved conflicts for a domain.
func (s *AdminService) GetLDAPSyncConflicts(ctx context.Context, domainID string, unresolvedOnly bool, limit, offset int) ([]maildb.LDAPSyncConflictView, error) {
	domainUUID, err := uuid.Parse(domainID)
	if err != nil {
		return nil, fmt.Errorf("invalid domain_id: %w", err)
	}

	req := maildb.LDAPSyncConflictListRequest{
		DomainID:       domainUUID.String(),
		UnresolvedOnly: unresolvedOnly,
		Limit:          limit,
		Offset:         offset,
	}

	conflicts, err := s.repository.GetLDAPSyncConflicts(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to get conflicts: %w", err)
	}
	return conflicts, nil
}

// ResolveLDAPSyncConflict manually resolves a sync conflict.
func (s *AdminService) ResolveLDAPSyncConflict(ctx context.Context, conflictID string, resolution string) error {
	if resolution != "prefer_local" && resolution != "prefer_ldap" && resolution != "manual" {
		return fmt.Errorf("invalid resolution: must be 'prefer_local', 'prefer_ldap', or 'manual'")
	}

	conflictUUID, err := uuid.Parse(conflictID)
	if err != nil {
		return fmt.Errorf("invalid conflict_id: %w", err)
	}

	err = s.repository.ResolveLDAPSyncConflict(ctx, conflictUUID, resolution)
	if err != nil {
		return fmt.Errorf("failed to resolve conflict: %w", err)
	}
	return nil
}

// executeLDAPSync runs the actual sync operation based on type.
func (s *AdminService) executeLDAPSync(ctx context.Context, domainID uuid.UUID, syncType string) (SyncResult, error) {
	req := SyncRequest{
		DomainID:  domainID.String(),
		Timestamp: time.Now(),
	}

	switch syncType {
	case "users":
		return s.provider.SyncUsers(ctx, req)
	case "groups":
		return s.provider.SyncGroups(ctx, req)
	case "memberships":
		return s.provider.SyncMemberships(ctx, req)
	default:
		return SyncResult{}, fmt.Errorf("unknown sync type: %s", syncType)
	}
}
