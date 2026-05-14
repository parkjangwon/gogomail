package rdbms

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/gogomail/gogomail/internal/maildb"
)

// AdminService provides admin operations for RDBMS sync.
type AdminService struct {
	provider   *Provider
	repository *maildb.Repository
}

// NewAdminService creates a new RDBMS admin service.
func NewAdminService(provider *Provider, repo *maildb.Repository) *AdminService {
	return &AdminService{
		provider:   provider,
		repository: repo,
	}
}

// TriggerRDBMSSync initiates an on-demand RDBMS sync for users, groups, or memberships.
func (s *AdminService) TriggerRDBMSSync(ctx context.Context, domainID string, syncType string) (map[string]interface{}, error) {
	if syncType != "users" && syncType != "groups" && syncType != "memberships" {
		return nil, fmt.Errorf("invalid sync_type: must be 'users', 'groups', or 'memberships'")
	}

	domainUUID, err := uuid.Parse(domainID)
	if err != nil {
		return nil, fmt.Errorf("invalid domain_id: %w", err)
	}

	// Create sync run record
	runID, err := s.repository.CreateRDBMSSyncRun(ctx, domainUUID, syncType, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create sync run: %w", err)
	}

	// Execute sync based on type
	syncResult, syncErr := s.executeRDBMSSync(ctx, domainUUID, syncType)

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

	err = s.repository.UpdateRDBMSSyncRun(ctx, runID, status,
		syncResult.UsersCreated, syncResult.UsersUpdated, syncResult.UsersDeleted,
		syncResult.GroupsCreated, syncResult.GroupsUpdated, syncResult.GroupsDeleted,
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

// GetRDBMSSyncHistory retrieves past RDBMS sync runs for a domain.
func (s *AdminService) GetRDBMSSyncHistory(ctx context.Context, domainID string, limit, offset int) ([]maildb.RDBMSSyncRunView, error) {
	domainUUID, err := uuid.Parse(domainID)
	if err != nil {
		return nil, fmt.Errorf("invalid domain_id: %w", err)
	}

	req := maildb.RDBMSSyncRunListRequest{
		DomainID: domainUUID.String(),
		Limit:    limit,
		Offset:   offset,
	}

	runs, err := s.repository.GetRDBMSSyncRuns(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to get sync history: %w", err)
	}
	return runs, nil
}

// GetRDBMSSyncConflicts retrieves unresolved conflicts for a domain.
func (s *AdminService) GetRDBMSSyncConflicts(ctx context.Context, domainID string, unresolvedOnly bool, limit, offset int) ([]maildb.RDBMSSyncConflictView, error) {
	domainUUID, err := uuid.Parse(domainID)
	if err != nil {
		return nil, fmt.Errorf("invalid domain_id: %w", err)
	}

	req := maildb.RDBMSSyncConflictListRequest{
		DomainID:       domainUUID.String(),
		UnresolvedOnly: unresolvedOnly,
		Limit:          limit,
		Offset:         offset,
	}

	conflicts, err := s.repository.GetRDBMSSyncConflicts(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to get conflicts: %w", err)
	}
	return conflicts, nil
}

// ResolveRDBMSSyncConflict manually resolves a sync conflict.
func (s *AdminService) ResolveRDBMSSyncConflict(ctx context.Context, conflictID string, resolution string) error {
	if resolution != "prefer_local" && resolution != "prefer_rdbms" && resolution != "manual" {
		return fmt.Errorf("invalid resolution: must be 'prefer_local', 'prefer_rdbms', or 'manual'")
	}

	conflictUUID, err := uuid.Parse(conflictID)
	if err != nil {
		return fmt.Errorf("invalid conflict_id: %w", err)
	}

	err = s.repository.ResolveRDBMSSyncConflict(ctx, conflictUUID, resolution)
	if err != nil {
		return fmt.Errorf("failed to resolve conflict: %w", err)
	}
	return nil
}

// TestRDBMSConnection tests connectivity to the external RDBMS.
func (s *AdminService) TestRDBMSConnection(ctx context.Context, connString string) error {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	return TestConnection(connString)
}

// ValidateRDBMSQuery validates a user or group SQL query.
func (s *AdminService) ValidateRDBMSQuery(ctx context.Context, queryType string) error {
	if s.provider == nil {
		return fmt.Errorf("provider not initialized")
	}

	switch queryType {
	case "user":
		return s.provider.ValidateUserQuery(ctx)
	case "group":
		return s.provider.ValidateGroupQuery(ctx)
	default:
		return fmt.Errorf("invalid query_type: must be 'user' or 'group'")
	}
}

// executeRDBMSSync runs the actual sync operation based on type.
func (s *AdminService) executeRDBMSSync(ctx context.Context, domainID uuid.UUID, syncType string) (SyncResult, error) {
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
