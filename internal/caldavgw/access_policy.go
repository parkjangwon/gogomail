package caldavgw

import (
	"context"
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

func (p DelegatedAccessPolicy) AuthorizeCalendarAccess(ctx context.Context, req AccessRequest) (AccessDecision, error) {
	if p.Directory == nil {
		return AccessDecision{}, fmt.Errorf("directory resolver is required")
	}
	owner, err := p.Directory.ResolvePrincipal(ctx, directory.ResolvePrincipalRequest{
		ID:         req.OwnerUserID,
		Kind:       directory.PrincipalKindUser,
		ActiveOnly: true,
	})
	if err != nil {
		return AccessDecision{}, fmt.Errorf("resolve CalDAV owner principal: %w", err)
	}
	actor, err := p.Directory.ResolvePrincipal(ctx, directory.ResolvePrincipalRequest{
		ID:         req.ActorUserID,
		Kind:       directory.PrincipalKindUser,
		ActiveOnly: true,
	})
	if err != nil {
		return AccessDecision{}, fmt.Errorf("resolve CalDAV actor principal: %w", err)
	}
	if owner.CompanyID == "" || actor.CompanyID != owner.CompanyID {
		return AccessDecision{Allowed: false}, nil
	}
	decision, err := p.Authorizer.CheckAndRecordDelegatedAccess(ctx, accesspolicy.DelegatedAccessRequest{
		CompanyID:    owner.CompanyID,
		Owner:        accesspolicy.Principal(directory.PrincipalKindUser, owner.ID),
		Actor:        accesspolicy.Principal(directory.PrincipalKindUser, actor.ID),
		Scope:        directory.DelegationScopeCalendar,
		RequiredRole: req.RequiredRole,
	})
	if err != nil {
		return AccessDecision{}, err
	}
	return AccessDecision{Allowed: decision.Allowed}, nil
}
