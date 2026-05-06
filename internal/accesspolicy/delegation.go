package accesspolicy

import (
	"context"
	"fmt"
	"strings"

	"github.com/gogomail/gogomail/internal/directory"
)

const (
	DecisionReasonDelegationAllowed = "delegation_allowed"
	DecisionReasonDelegationDenied  = "delegation_denied"
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
