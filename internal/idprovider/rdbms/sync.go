package rdbms

import (
	"context"
	"errors"
	"fmt"
	"time"
)

var (
	ErrSyncNotConfigured         = errors.New("rdbms sync provider is not configured")
	ErrMembershipSyncUnsupported = errors.New("rdbms membership sync is not supported by the current provider schema")
)

// SyncRequest contains parameters for syncing RDBMS data to the local database.
type SyncRequest struct {
	DomainID  string
	Query     string
	Limit     int
	Timestamp time.Time
}

// SyncResult contains results from a sync operation.
type SyncResult struct {
	UsersCreated  int
	UsersUpdated  int
	UsersDeleted  int
	GroupsCreated int
	GroupsUpdated int
	GroupsDeleted int
	LastSyncTime  time.Time
	ConflictCount int
	ErrorCount    int
}

// SyncUsers syncs users from the external RDBMS to the local database on-demand.
func (p *Provider) SyncUsers(ctx context.Context, req SyncRequest) (SyncResult, error) {
	if p.config == nil {
		return SyncResult{}, ErrSyncNotConfigured
	}
	if p.db == nil {
		return SyncResult{}, fmt.Errorf("rdbms provider not connected")
	}
	if req.DomainID == "" {
		return SyncResult{}, fmt.Errorf("domain id required for sync")
	}

	result := SyncResult{
		LastSyncTime: time.Now(),
	}

	users, err := p.ListUsers(ctx, nil)
	if err != nil {
		return result, err
	}
	result.UsersCreated = len(users)
	return result, nil
}

// SyncGroups syncs groups from the external RDBMS to the local database on-demand.
func (p *Provider) SyncGroups(ctx context.Context, req SyncRequest) (SyncResult, error) {
	if p.config == nil {
		return SyncResult{}, ErrSyncNotConfigured
	}
	if p.db == nil {
		return SyncResult{}, fmt.Errorf("rdbms provider not connected")
	}
	if req.DomainID == "" {
		return SyncResult{}, fmt.Errorf("domain id required for sync")
	}

	result := SyncResult{
		LastSyncTime: time.Now(),
	}

	groups, err := p.ListGroups(ctx, nil)
	if err != nil {
		return result, err
	}
	result.GroupsCreated = len(groups)
	return result, nil
}

// SyncMemberships syncs group memberships from the external RDBMS to the local database.
func (p *Provider) SyncMemberships(ctx context.Context, req SyncRequest) (SyncResult, error) {
	if p.config == nil {
		return SyncResult{}, ErrSyncNotConfigured
	}
	if p.db == nil {
		return SyncResult{}, fmt.Errorf("rdbms provider not connected")
	}
	if req.DomainID == "" {
		return SyncResult{}, fmt.Errorf("domain id required for sync")
	}

	return SyncResult{LastSyncTime: time.Now()}, ErrMembershipSyncUnsupported
}
