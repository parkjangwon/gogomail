package app

import (
	"context"
	"fmt"

	"github.com/gogomail/gogomail/internal/directory"
)

func (s adminService) ListDirectoryDelegations(ctx context.Context, req directory.ListDelegationsRequest) ([]directory.Delegation, error) {
	if s.directory == nil {
		return nil, fmt.Errorf("directory backend is not configured")
	}
	return s.directory.ListDelegations(ctx, req)
}

func (s adminService) CreateDirectoryDelegation(ctx context.Context, req directory.CreateDelegationRequest) (directory.Delegation, error) {
	if s.directory == nil {
		return directory.Delegation{}, fmt.Errorf("directory backend is not configured")
	}
	return s.directory.CreateDelegationWithAudit(ctx, req)
}

func (s adminService) CreateDirectoryGroupMembership(ctx context.Context, req directory.CreateGroupMembershipRequest) (directory.GroupMembership, error) {
	if s.directory == nil {
		return directory.GroupMembership{}, fmt.Errorf("directory backend is not configured")
	}
	return s.directory.CreateGroupMembershipWithAudit(ctx, req)
}

func (s adminService) ListDirectoryGroupMemberships(ctx context.Context, req directory.ListGroupMembershipsRequest) ([]directory.GroupMembership, error) {
	if s.directory == nil {
		return nil, fmt.Errorf("directory backend is not configured")
	}
	return s.directory.ListGroupMemberships(ctx, req)
}

func (s adminService) DeleteDirectoryDelegation(ctx context.Context, id string) (directory.Delegation, error) {
	if s.directory == nil {
		return directory.Delegation{}, fmt.Errorf("directory backend is not configured")
	}
	return s.directory.DeleteDelegationWithAudit(ctx, id)
}

func (s adminService) DeleteDirectoryGroupMembership(ctx context.Context, id string) (directory.GroupMembership, error) {
	if s.directory == nil {
		return directory.GroupMembership{}, fmt.Errorf("directory backend is not configured")
	}
	return s.directory.DeleteGroupMembershipWithAudit(ctx, id)
}

func (s adminService) UpdateDirectoryDelegationRole(ctx context.Context, req directory.UpdateDelegationRoleRequest) (directory.Delegation, error) {
	if s.directory == nil {
		return directory.Delegation{}, fmt.Errorf("directory backend is not configured")
	}
	return s.directory.UpdateDelegationRoleWithAudit(ctx, req)
}

func (s adminService) ReassignDirectoryDelegation(ctx context.Context, req directory.ReassignDelegationRequest) (directory.Delegation, error) {
	if s.directory == nil {
		return directory.Delegation{}, fmt.Errorf("directory backend is not configured")
	}
	return s.directory.ReassignDelegationWithAudit(ctx, req)
}

func (s adminService) UpdateDirectoryGroupMembershipRole(ctx context.Context, req directory.UpdateGroupMembershipRoleRequest) (directory.GroupMembership, error) {
	if s.directory == nil {
		return directory.GroupMembership{}, fmt.Errorf("directory backend is not configured")
	}
	return s.directory.UpdateGroupMembershipRoleWithAudit(ctx, req)
}

func (s adminService) ReassignDirectoryGroupMembership(ctx context.Context, req directory.ReassignGroupMembershipRequest) (directory.GroupMembership, error) {
	if s.directory == nil {
		return directory.GroupMembership{}, fmt.Errorf("directory backend is not configured")
	}
	return s.directory.ReassignGroupMembershipWithAudit(ctx, req)
}

func (s adminService) SearchDirectoryPrincipals(ctx context.Context, req directory.SearchPrincipalsRequest) ([]directory.Principal, error) {
	if s.directory == nil {
		return nil, fmt.Errorf("directory backend is not configured")
	}
	return s.directory.SearchPrincipals(ctx, req)
}

func (s adminService) ResolveDirectoryAlias(ctx context.Context, req directory.ResolveAliasRequest) (directory.Alias, error) {
	if s.directory == nil {
		return directory.Alias{}, fmt.Errorf("directory backend is not configured")
	}
	return s.directory.ResolveAlias(ctx, req)
}

func (s adminService) CreateDirectoryAlias(ctx context.Context, req directory.CreateAliasRequest) (directory.Alias, error) {
	if s.directory == nil {
		return directory.Alias{}, fmt.Errorf("directory backend is not configured")
	}
	return s.directory.CreateAliasWithAudit(ctx, req)
}

func (s adminService) DeleteDirectoryAlias(ctx context.Context, id string) (directory.Alias, error) {
	if s.directory == nil {
		return directory.Alias{}, fmt.Errorf("directory backend is not configured")
	}
	return s.directory.DeleteAliasWithAudit(ctx, id)
}

func (s adminService) ListDirectoryAliases(ctx context.Context, req directory.ListAliasesRequest) ([]directory.Alias, error) {
	if s.directory == nil {
		return nil, fmt.Errorf("directory backend is not configured")
	}
	return s.directory.ListAliases(ctx, req)
}
