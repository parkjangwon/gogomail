package accesspolicy

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/gogomail/gogomail/internal/directory"
)

const (
	DecisionReasonDelegationAllowed = "delegation_allowed"
	DecisionReasonDelegationDenied  = "delegation_denied"
)

const (
	WebDAVPrivilegeRead            = "read"
	WebDAVPrivilegeBind            = "bind"
	WebDAVPrivilegeUnbind          = "unbind"
	WebDAVPrivilegeWriteContent    = "write-content"
	WebDAVPrivilegeWriteProperties = "write-properties"
)

type PrincipalRef struct {
	Kind string
	ID   string
}

type DelegatedAccessRequest struct {
	CompanyID    string
	Owner        PrincipalRef
	Actor        PrincipalRef
	Scope        string
	RequiredRole string
	MaxDepth     int
}

type Decision struct {
	Allowed      bool
	Reason       string
	Scope        string
	RequiredRole string
}

type EffectiveDelegationChecker interface {
	CheckEffectiveDelegation(ctx context.Context, req directory.CheckDelegationRequest) (bool, error)
}

type DelegationEvaluator struct {
	Checker EffectiveDelegationChecker
}

func (e DelegationEvaluator) CheckDelegatedAccess(ctx context.Context, req DelegatedAccessRequest) (Decision, error) {
	if e.Checker == nil {
		return Decision{}, fmt.Errorf("effective delegation checker is required")
	}
	check, err := directory.NormalizeCheckDelegationRequest(directory.CheckDelegationRequest{
		CompanyID:    req.CompanyID,
		OwnerKind:    req.Owner.Kind,
		OwnerID:      req.Owner.ID,
		DelegateKind: req.Actor.Kind,
		DelegateID:   req.Actor.ID,
		Scope:        req.Scope,
		RequiredRole: req.RequiredRole,
		ActiveOnly:   true,
		MaxDepth:     req.MaxDepth,
	})
	if err != nil {
		return Decision{}, err
	}
	allowed, err := e.Checker.CheckEffectiveDelegation(ctx, check)
	if err != nil {
		return Decision{}, err
	}
	reason := DecisionReasonDelegationDenied
	if allowed {
		reason = DecisionReasonDelegationAllowed
	}
	return Decision{
		Allowed:      allowed,
		Reason:       reason,
		Scope:        check.Scope,
		RequiredRole: check.RequiredRole,
	}, nil
}

func Principal(kind string, id string) PrincipalRef {
	return PrincipalRef{Kind: strings.TrimSpace(kind), ID: strings.TrimSpace(id)}
}

func WebDAVPrivilegesForDecision(decision Decision) []string {
	if !decision.Allowed {
		return nil
	}
	switch decision.RequiredRole {
	case directory.DelegationRoleRead:
		return []string{WebDAVPrivilegeRead}
	case directory.DelegationRoleWrite:
		return []string{
			WebDAVPrivilegeRead,
			WebDAVPrivilegeWriteContent,
			WebDAVPrivilegeWriteProperties,
		}
	case directory.DelegationRoleManage:
		return []string{
			WebDAVPrivilegeRead,
			WebDAVPrivilegeBind,
			WebDAVPrivilegeUnbind,
			WebDAVPrivilegeWriteContent,
			WebDAVPrivilegeWriteProperties,
		}
	default:
		return nil
	}
}

func DelegatedAccessAuditDetail(req DelegatedAccessRequest, decision Decision) (json.RawMessage, error) {
	check, err := directory.NormalizeCheckDelegationRequest(directory.CheckDelegationRequest{
		CompanyID:    req.CompanyID,
		OwnerKind:    req.Owner.Kind,
		OwnerID:      req.Owner.ID,
		DelegateKind: req.Actor.Kind,
		DelegateID:   req.Actor.ID,
		Scope:        req.Scope,
		RequiredRole: req.RequiredRole,
		ActiveOnly:   true,
		MaxDepth:     req.MaxDepth,
	})
	if err != nil {
		return nil, err
	}
	reason := normalizedDecisionReason(decision)
	detail, err := json.Marshal(struct {
		CompanyID        string   `json:"company_id"`
		OwnerKind        string   `json:"owner_kind"`
		OwnerID          string   `json:"owner_id"`
		ActorKind        string   `json:"actor_kind"`
		ActorID          string   `json:"actor_id"`
		Scope            string   `json:"scope"`
		RequiredRole     string   `json:"required_role"`
		Allowed          bool     `json:"allowed"`
		Reason           string   `json:"reason"`
		WebDAVPrivileges []string `json:"webdav_privileges,omitempty"`
	}{
		CompanyID:        check.CompanyID,
		OwnerKind:        check.OwnerKind,
		OwnerID:          check.OwnerID,
		ActorKind:        check.DelegateKind,
		ActorID:          check.DelegateID,
		Scope:            check.Scope,
		RequiredRole:     check.RequiredRole,
		Allowed:          decision.Allowed,
		Reason:           reason,
		WebDAVPrivileges: WebDAVPrivilegesForDecision(Decision{Allowed: decision.Allowed, RequiredRole: check.RequiredRole}),
	})
	if err != nil {
		return nil, fmt.Errorf("marshal delegated access audit detail: %w", err)
	}
	return detail, nil
}

func normalizedDecisionReason(decision Decision) string {
	reason := strings.TrimSpace(decision.Reason)
	if decision.Allowed && reason == DecisionReasonDelegationAllowed {
		return reason
	}
	if !decision.Allowed && reason == DecisionReasonDelegationDenied {
		return reason
	}
	if decision.Allowed {
		return DecisionReasonDelegationAllowed
	}
	return DecisionReasonDelegationDenied
}
