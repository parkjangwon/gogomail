package carddavgw

import (
	"context"
	"errors"
	"fmt"

	"github.com/gogomail/gogomail/internal/accesspolicy"
	"github.com/gogomail/gogomail/internal/directory"
)

type DirectoryResolver interface {
	ResolvePrincipal(ctx context.Context, req directory.ResolvePrincipalRequest) (directory.Principal, error)
}

type DelegatedAccessPolicy struct {
	Directory  DirectoryResolver
	Authorizer accesspolicy.DelegatedAccessAuthorizer
}

func (p DelegatedAccessPolicy) AuthorizeAddressBookAccess(ctx context.Context, req AccessRequest) (AccessDecision, error) {
	if p.Directory == nil {
		return AccessDecision{}, fmt.Errorf("directory resolver is required")
	}
	owner, err := p.Directory.ResolvePrincipal(ctx, directory.ResolvePrincipalRequest{
		ID:         req.OwnerUserID,
		Kind:       directory.PrincipalKindUser,
		ActiveOnly: true,
	})
	if err != nil {
		if errors.Is(err, directory.ErrPrincipalNotFound) {
			return AccessDecision{Allowed: false}, nil
		}
		return AccessDecision{}, fmt.Errorf("resolve CardDAV owner principal: %w", err)
	}
	actor, err := p.Directory.ResolvePrincipal(ctx, directory.ResolvePrincipalRequest{
		ID:         req.ActorUserID,
		Kind:       directory.PrincipalKindUser,
		ActiveOnly: true,
	})
	if err != nil {
		if errors.Is(err, directory.ErrPrincipalNotFound) {
			return AccessDecision{Allowed: false}, nil
		}
		return AccessDecision{}, fmt.Errorf("resolve CardDAV actor principal: %w", err)
	}
	if owner.CompanyID == "" || actor.CompanyID != owner.CompanyID {
		return AccessDecision{Allowed: false}, nil
	}
	decision, err := p.Authorizer.CheckAndRecordDelegatedAccess(ctx, accesspolicy.DelegatedAccessRequest{
		CompanyID:    owner.CompanyID,
		Owner:        accesspolicy.Principal(directory.PrincipalKindUser, owner.ID),
		Actor:        accesspolicy.Principal(directory.PrincipalKindUser, actor.ID),
		Scope:        directory.DelegationScopeContacts,
		RequiredRole: req.RequiredRole,
	})
	if err != nil {
		return AccessDecision{}, err
	}
	if !decision.Allowed {
		return AccessDecision{Allowed: false}, nil
	}
	privileges, err := p.addressBookPrivileges(ctx, owner.CompanyID, owner.ID, actor.ID)
	if err != nil {
		return AccessDecision{}, err
	}
	return AccessDecision{Allowed: true, Privileges: privileges}, nil
}

func (p DelegatedAccessPolicy) addressBookPrivileges(ctx context.Context, companyID string, ownerID string, actorID string) ([]XMLName, error) {
	if p.Authorizer.Checker == nil {
		return nil, fmt.Errorf("effective delegation checker is required")
	}
	for _, role := range []string{directory.DelegationRoleManage, directory.DelegationRoleWrite, directory.DelegationRoleRead} {
		allowed, err := p.Authorizer.Checker.CheckEffectiveDelegation(ctx, directory.CheckDelegationRequest{
			CompanyID:    companyID,
			OwnerKind:    directory.PrincipalKindUser,
			OwnerID:      ownerID,
			DelegateKind: directory.PrincipalKindUser,
			DelegateID:   actorID,
			Scope:        directory.DelegationScopeContacts,
			RequiredRole: role,
			ActiveOnly:   true,
		})
		if err != nil {
			return nil, err
		}
		if allowed {
			return webDAVPrivilegeNames(accesspolicy.WebDAVPrivilegesForDecision(accesspolicy.Decision{
				Allowed:      true,
				RequiredRole: role,
			})), nil
		}
	}
	return nil, nil
}

func webDAVPrivilegeNames(names []string) []XMLName {
	privileges := make([]XMLName, 0, len(names))
	for _, name := range names {
		switch name {
		case accesspolicy.WebDAVPrivilegeRead:
			privileges = append(privileges, PrivilegeRead)
		case accesspolicy.WebDAVPrivilegeBind:
			privileges = append(privileges, PrivilegeBind)
		case accesspolicy.WebDAVPrivilegeUnbind:
			privileges = append(privileges, PrivilegeUnbind)
		case accesspolicy.WebDAVPrivilegeWriteContent:
			privileges = append(privileges, PrivilegeWriteContent)
		case accesspolicy.WebDAVPrivilegeWriteProperties:
			privileges = append(privileges, PrivilegeWriteProperties)
		}
	}
	return privileges
}
