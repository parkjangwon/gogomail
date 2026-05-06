package directory

import (
	"fmt"
	"strings"
	"unicode"

	"github.com/gogomail/gogomail/internal/mail"
)

const (
	PrincipalKindUser         = "user"
	PrincipalKindOrganization = "organization"
	PrincipalKindGroup        = "group"
	PrincipalKindResource     = "resource"

	ResourceTypeRoom      = "room"
	ResourceTypeEquipment = "equipment"
	ResourceTypeVehicle   = "vehicle"
	ResourceTypeOther     = "other"

	DelegationScopeCalendar = "calendar"
	DelegationScopeContacts = "contacts"
	DelegationScopeDrive    = "drive"
	DelegationScopeMailbox  = "mailbox"

	DelegationRoleRead   = "read"
	DelegationRoleWrite  = "write"
	DelegationRoleManage = "manage"

	GroupMembershipRoleMember  = "member"
	GroupMembershipRoleManager = "manager"
	GroupMembershipRoleOwner   = "owner"

	MaxPrincipalIDBytes             = 200
	MaxGroupMembershipDepth         = 16
	DefaultMembershipMaxDepth       = 8
	MaxPrincipalSearchBytes         = 200
	DefaultPrincipalSearchLimit     = 20
	MaxPrincipalSearchLimit         = 100
	MaxAliasAddressBytes            = 320
	MaxAliasSearchBytes             = 200
	DefaultAliasListLimit           = 50
	MaxAliasListLimit               = 200
	DefaultDelegationListLimit      = 50
	MaxDelegationListLimit          = 200
	DefaultGroupMembershipListLimit = 50
	MaxGroupMembershipListLimit     = 200
)

type Principal struct {
	ID             string
	Kind           string
	CompanyID      string
	DomainID       string
	OrganizationID string
	DisplayName    string
	PrimaryEmail   string
	Status         string
	ResourceType   string
}

type ResolvePrincipalRequest struct {
	ID         string
	Kind       string
	ActiveOnly bool
}

type SearchPrincipalsRequest struct {
	CompanyID      string
	DomainID       string
	OrganizationID string
	Kinds          []string
	Query          string
	ActiveOnly     bool
	Limit          int
}

type Alias struct {
	ID              string
	CompanyID       string
	DomainID        string
	Address         string
	AddressACE      string
	TargetKind      string
	TargetID        string
	Status          string
	TargetPrincipal Principal
}

type CreateAliasRequest struct {
	CompanyID  string `json:"company_id"`
	DomainID   string `json:"domain_id"`
	Address    string `json:"address"`
	TargetKind string `json:"target_kind"`
	TargetID   string `json:"target_id"`
}

type ResolveAliasRequest struct {
	Address    string
	ActiveOnly bool
}

type ListAliasesRequest struct {
	CompanyID  string
	DomainID   string
	TargetKind string
	TargetID   string
	Query      string
	ActiveOnly bool
	Limit      int
}

type CheckGroupMembershipRequest struct {
	GroupID    string
	MemberKind string
	MemberID   string
	ActiveOnly bool
	MaxDepth   int
}

type GroupMembership struct {
	ID         string
	GroupID    string
	CompanyID  string
	MemberKind string
	MemberID   string
	Role       string
	Status     string
}

type CreateGroupMembershipRequest struct {
	GroupID    string `json:"group_id"`
	MemberKind string `json:"member_kind"`
	MemberID   string `json:"member_id"`
	Role       string `json:"role"`
}

type UpdateGroupMembershipRoleRequest struct {
	ID   string `json:"-"`
	Role string `json:"role"`
}

type ReassignGroupMembershipRequest struct {
	ID         string `json:"-"`
	GroupID    string `json:"group_id"`
	MemberKind string `json:"member_kind"`
	MemberID   string `json:"member_id"`
}

type ListGroupMembershipsRequest struct {
	CompanyID  string
	GroupID    string
	MemberKind string
	MemberID   string
	Role       string
	ActiveOnly bool
	Limit      int
}

type Delegation struct {
	ID           string
	CompanyID    string
	OwnerKind    string
	OwnerID      string
	DelegateKind string
	DelegateID   string
	Scope        string
	Role         string
	Status       string
}

type CheckDelegationRequest struct {
	CompanyID    string
	OwnerKind    string
	OwnerID      string
	DelegateKind string
	DelegateID   string
	Scope        string
	RequiredRole string
	ActiveOnly   bool
	MaxDepth     int
}

type CreateDelegationRequest struct {
	CompanyID    string `json:"company_id"`
	OwnerKind    string `json:"owner_kind"`
	OwnerID      string `json:"owner_id"`
	DelegateKind string `json:"delegate_kind"`
	DelegateID   string `json:"delegate_id"`
	Scope        string `json:"scope"`
	Role         string `json:"role"`
}

type UpdateDelegationRoleRequest struct {
	ID   string `json:"-"`
	Role string `json:"role"`
}

type ReassignDelegationRequest struct {
	ID           string `json:"-"`
	OwnerKind    string `json:"owner_kind"`
	OwnerID      string `json:"owner_id"`
	DelegateKind string `json:"delegate_kind"`
	DelegateID   string `json:"delegate_id"`
	Scope        string `json:"scope"`
}

type ListDelegationsRequest struct {
	CompanyID    string
	OwnerKind    string
	OwnerID      string
	DelegateKind string
	DelegateID   string
	Scope        string
	Role         string
	ActiveOnly   bool
	Limit        int
}

func NormalizePrincipalKind(kind string) (string, error) {
	kind = strings.TrimSpace(strings.ToLower(kind))
	if kind == "" {
		return PrincipalKindUser, nil
	}
	switch kind {
	case PrincipalKindUser, PrincipalKindOrganization, PrincipalKindGroup, PrincipalKindResource:
		return kind, nil
	default:
		return "", fmt.Errorf("unsupported principal kind %q", kind)
	}
}

func NormalizeDelegationScope(scope string) (string, error) {
	scope = strings.TrimSpace(strings.ToLower(scope))
	switch scope {
	case DelegationScopeCalendar, DelegationScopeContacts, DelegationScopeDrive, DelegationScopeMailbox:
		return scope, nil
	default:
		return "", fmt.Errorf("unsupported delegation scope %q", scope)
	}
}

func NormalizeDelegationRole(role string) (string, error) {
	role = strings.TrimSpace(strings.ToLower(role))
	switch role {
	case DelegationRoleRead, DelegationRoleWrite, DelegationRoleManage:
		return role, nil
	default:
		return "", fmt.Errorf("unsupported delegation role %q", role)
	}
}

func NormalizeGroupMembershipRole(role string) (string, error) {
	role = strings.TrimSpace(strings.ToLower(role))
	if role == "" {
		return GroupMembershipRoleMember, nil
	}
	switch role {
	case GroupMembershipRoleMember, GroupMembershipRoleManager, GroupMembershipRoleOwner:
		return role, nil
	default:
		return "", fmt.Errorf("unsupported group membership role %q", role)
	}
}

func NormalizePrincipalID(id string) (string, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return "", fmt.Errorf("principal id is required")
	}
	if len(id) > MaxPrincipalIDBytes {
		return "", fmt.Errorf("principal id is too long")
	}
	if strings.ContainsAny(id, "\r\n") {
		return "", fmt.Errorf("principal id must not contain line breaks")
	}
	for _, r := range id {
		if unicode.IsControl(r) {
			return "", fmt.Errorf("principal id must not contain control characters")
		}
	}
	return id, nil
}

func NormalizeResolvePrincipalRequest(req ResolvePrincipalRequest) (ResolvePrincipalRequest, error) {
	id, err := NormalizePrincipalID(req.ID)
	if err != nil {
		return ResolvePrincipalRequest{}, err
	}
	kind, err := NormalizePrincipalKind(req.Kind)
	if err != nil {
		return ResolvePrincipalRequest{}, err
	}
	req.ID = id
	req.Kind = kind
	return req, nil
}

func NormalizeSearchPrincipalsRequest(req SearchPrincipalsRequest) (SearchPrincipalsRequest, error) {
	companyID, err := NormalizePrincipalID(req.CompanyID)
	if err != nil {
		return SearchPrincipalsRequest{}, fmt.Errorf("company id: %w", err)
	}
	req.CompanyID = companyID
	if strings.TrimSpace(req.DomainID) != "" {
		domainID, err := NormalizePrincipalID(req.DomainID)
		if err != nil {
			return SearchPrincipalsRequest{}, fmt.Errorf("domain id: %w", err)
		}
		req.DomainID = domainID
	}
	if strings.TrimSpace(req.OrganizationID) != "" {
		orgID, err := NormalizePrincipalID(req.OrganizationID)
		if err != nil {
			return SearchPrincipalsRequest{}, fmt.Errorf("organization id: %w", err)
		}
		req.OrganizationID = orgID
	}
	if strings.ContainsAny(req.Query, "\r\n") {
		return SearchPrincipalsRequest{}, fmt.Errorf("principal search query must not contain line breaks")
	}
	query := strings.Join(strings.Fields(req.Query), " ")
	if len(query) > MaxPrincipalSearchBytes {
		return SearchPrincipalsRequest{}, fmt.Errorf("principal search query is too long")
	}
	for _, r := range query {
		if unicode.IsControl(r) {
			return SearchPrincipalsRequest{}, fmt.Errorf("principal search query must not contain control characters")
		}
	}
	req.Query = query
	if req.Limit < 0 {
		return SearchPrincipalsRequest{}, fmt.Errorf("principal search limit must not be negative")
	}
	if req.Limit == 0 {
		req.Limit = DefaultPrincipalSearchLimit
	}
	if req.Limit > MaxPrincipalSearchLimit {
		return SearchPrincipalsRequest{}, fmt.Errorf("principal search limit is too large")
	}
	if len(req.Kinds) == 0 {
		req.Kinds = []string{PrincipalKindUser, PrincipalKindOrganization, PrincipalKindGroup, PrincipalKindResource}
		return req, nil
	}
	kinds := make([]string, 0, len(req.Kinds))
	seen := make(map[string]struct{}, len(req.Kinds))
	for _, raw := range req.Kinds {
		kind, err := NormalizePrincipalKind(raw)
		if err != nil {
			return SearchPrincipalsRequest{}, err
		}
		if _, ok := seen[kind]; ok {
			continue
		}
		seen[kind] = struct{}{}
		kinds = append(kinds, kind)
	}
	req.Kinds = kinds
	return req, nil
}

func NormalizeCheckDelegationRequest(req CheckDelegationRequest) (CheckDelegationRequest, error) {
	companyID, err := NormalizePrincipalID(req.CompanyID)
	if err != nil {
		return CheckDelegationRequest{}, fmt.Errorf("company id: %w", err)
	}
	ownerID, err := NormalizePrincipalID(req.OwnerID)
	if err != nil {
		return CheckDelegationRequest{}, fmt.Errorf("owner id: %w", err)
	}
	delegateID, err := NormalizePrincipalID(req.DelegateID)
	if err != nil {
		return CheckDelegationRequest{}, fmt.Errorf("delegate id: %w", err)
	}
	ownerKind, err := NormalizePrincipalKind(req.OwnerKind)
	if err != nil {
		return CheckDelegationRequest{}, fmt.Errorf("owner kind: %w", err)
	}
	delegateKind, err := NormalizePrincipalKind(req.DelegateKind)
	if err != nil {
		return CheckDelegationRequest{}, fmt.Errorf("delegate kind: %w", err)
	}
	scope, err := NormalizeDelegationScope(req.Scope)
	if err != nil {
		return CheckDelegationRequest{}, err
	}
	role, err := NormalizeDelegationRole(req.RequiredRole)
	if err != nil {
		return CheckDelegationRequest{}, err
	}
	req.CompanyID = companyID
	req.OwnerID = ownerID
	req.DelegateID = delegateID
	req.OwnerKind = ownerKind
	req.DelegateKind = delegateKind
	if req.OwnerKind == req.DelegateKind && req.OwnerID == req.DelegateID {
		return CheckDelegationRequest{}, fmt.Errorf("delegation owner and delegate must differ")
	}
	if req.MaxDepth < 0 {
		return CheckDelegationRequest{}, fmt.Errorf("delegation max depth must not be negative")
	}
	if req.MaxDepth == 0 {
		req.MaxDepth = DefaultMembershipMaxDepth
	}
	if req.MaxDepth > MaxGroupMembershipDepth {
		return CheckDelegationRequest{}, fmt.Errorf("delegation max depth is too large")
	}
	req.Scope = scope
	req.RequiredRole = role
	return req, nil
}

func NormalizeCreateDelegationRequest(req CreateDelegationRequest) (CreateDelegationRequest, error) {
	check, err := NormalizeCheckDelegationRequest(CheckDelegationRequest{
		CompanyID:    req.CompanyID,
		OwnerKind:    req.OwnerKind,
		OwnerID:      req.OwnerID,
		DelegateKind: req.DelegateKind,
		DelegateID:   req.DelegateID,
		Scope:        req.Scope,
		RequiredRole: req.Role,
	})
	if err != nil {
		return CreateDelegationRequest{}, err
	}
	return CreateDelegationRequest{
		CompanyID:    check.CompanyID,
		OwnerKind:    check.OwnerKind,
		OwnerID:      check.OwnerID,
		DelegateKind: check.DelegateKind,
		DelegateID:   check.DelegateID,
		Scope:        check.Scope,
		Role:         check.RequiredRole,
	}, nil
}

func NormalizeUpdateDelegationRoleRequest(req UpdateDelegationRoleRequest) (UpdateDelegationRoleRequest, error) {
	id, err := NormalizePrincipalID(req.ID)
	if err != nil {
		return UpdateDelegationRoleRequest{}, fmt.Errorf("delegation id: %w", err)
	}
	role, err := NormalizeDelegationRole(req.Role)
	if err != nil {
		return UpdateDelegationRoleRequest{}, err
	}
	req.ID = id
	req.Role = role
	return req, nil
}

func NormalizeReassignDelegationRequest(req ReassignDelegationRequest) (ReassignDelegationRequest, error) {
	id, err := NormalizePrincipalID(req.ID)
	if err != nil {
		return ReassignDelegationRequest{}, fmt.Errorf("delegation id: %w", err)
	}
	ownerID, err := NormalizePrincipalID(req.OwnerID)
	if err != nil {
		return ReassignDelegationRequest{}, fmt.Errorf("owner id: %w", err)
	}
	delegateID, err := NormalizePrincipalID(req.DelegateID)
	if err != nil {
		return ReassignDelegationRequest{}, fmt.Errorf("delegate id: %w", err)
	}
	ownerKind, err := NormalizePrincipalKind(req.OwnerKind)
	if err != nil {
		return ReassignDelegationRequest{}, fmt.Errorf("owner kind: %w", err)
	}
	delegateKind, err := NormalizePrincipalKind(req.DelegateKind)
	if err != nil {
		return ReassignDelegationRequest{}, fmt.Errorf("delegate kind: %w", err)
	}
	scope, err := NormalizeDelegationScope(req.Scope)
	if err != nil {
		return ReassignDelegationRequest{}, err
	}
	if ownerKind == delegateKind && ownerID == delegateID {
		return ReassignDelegationRequest{}, fmt.Errorf("delegation owner and delegate must differ")
	}
	req.ID = id
	req.OwnerKind = ownerKind
	req.OwnerID = ownerID
	req.DelegateKind = delegateKind
	req.DelegateID = delegateID
	req.Scope = scope
	return req, nil
}

func NormalizeListDelegationsRequest(req ListDelegationsRequest) (ListDelegationsRequest, error) {
	companyID, err := NormalizePrincipalID(req.CompanyID)
	if err != nil {
		return ListDelegationsRequest{}, fmt.Errorf("company id: %w", err)
	}
	req.CompanyID = companyID
	if strings.TrimSpace(req.OwnerKind) != "" {
		ownerKind, err := NormalizePrincipalKind(req.OwnerKind)
		if err != nil {
			return ListDelegationsRequest{}, fmt.Errorf("owner kind: %w", err)
		}
		req.OwnerKind = ownerKind
	}
	if strings.TrimSpace(req.OwnerID) != "" {
		if req.OwnerKind == "" {
			return ListDelegationsRequest{}, fmt.Errorf("owner kind is required when owner id is set")
		}
		ownerID, err := NormalizePrincipalID(req.OwnerID)
		if err != nil {
			return ListDelegationsRequest{}, fmt.Errorf("owner id: %w", err)
		}
		req.OwnerID = ownerID
	}
	if strings.TrimSpace(req.DelegateKind) != "" {
		delegateKind, err := NormalizePrincipalKind(req.DelegateKind)
		if err != nil {
			return ListDelegationsRequest{}, fmt.Errorf("delegate kind: %w", err)
		}
		req.DelegateKind = delegateKind
	}
	if strings.TrimSpace(req.DelegateID) != "" {
		if req.DelegateKind == "" {
			return ListDelegationsRequest{}, fmt.Errorf("delegate kind is required when delegate id is set")
		}
		delegateID, err := NormalizePrincipalID(req.DelegateID)
		if err != nil {
			return ListDelegationsRequest{}, fmt.Errorf("delegate id: %w", err)
		}
		req.DelegateID = delegateID
	}
	if strings.TrimSpace(req.Scope) != "" {
		scope, err := NormalizeDelegationScope(req.Scope)
		if err != nil {
			return ListDelegationsRequest{}, err
		}
		req.Scope = scope
	}
	if strings.TrimSpace(req.Role) != "" {
		role, err := NormalizeDelegationRole(req.Role)
		if err != nil {
			return ListDelegationsRequest{}, err
		}
		req.Role = role
	}
	if req.OwnerKind != "" && req.OwnerKind == req.DelegateKind && req.OwnerID != "" && req.OwnerID == req.DelegateID {
		return ListDelegationsRequest{}, fmt.Errorf("delegation owner and delegate must differ")
	}
	if req.Limit < 0 {
		return ListDelegationsRequest{}, fmt.Errorf("delegation list limit must not be negative")
	}
	if req.Limit == 0 {
		req.Limit = DefaultDelegationListLimit
	}
	if req.Limit > MaxDelegationListLimit {
		return ListDelegationsRequest{}, fmt.Errorf("delegation list limit is too large")
	}
	return req, nil
}

func DelegationRoleSatisfies(granted string, required string) bool {
	granted, grantedErr := NormalizeDelegationRole(granted)
	required, requiredErr := NormalizeDelegationRole(required)
	if grantedErr != nil || requiredErr != nil {
		return false
	}
	return delegationRoleRank(granted) >= delegationRoleRank(required)
}

func delegationRoleRank(role string) int {
	switch role {
	case DelegationRoleRead:
		return 1
	case DelegationRoleWrite:
		return 2
	case DelegationRoleManage:
		return 3
	default:
		return 0
	}
}

func NormalizeResolveAliasRequest(req ResolveAliasRequest) (ResolveAliasRequest, error) {
	address, err := mail.NormalizeAddress(req.Address)
	if err != nil {
		return ResolveAliasRequest{}, err
	}
	if len(address) > MaxAliasAddressBytes {
		return ResolveAliasRequest{}, fmt.Errorf("alias address is too long")
	}
	if strings.ContainsAny(address, "\r\n") {
		return ResolveAliasRequest{}, fmt.Errorf("alias address must not contain line breaks")
	}
	req.Address = address
	return req, nil
}

func NormalizeCreateAliasRequest(req CreateAliasRequest) (CreateAliasRequest, error) {
	companyID, err := NormalizePrincipalID(req.CompanyID)
	if err != nil {
		return CreateAliasRequest{}, fmt.Errorf("company id: %w", err)
	}
	domainID, err := NormalizePrincipalID(req.DomainID)
	if err != nil {
		return CreateAliasRequest{}, fmt.Errorf("domain id: %w", err)
	}
	addressReq, err := NormalizeResolveAliasRequest(ResolveAliasRequest{Address: req.Address})
	if err != nil {
		return CreateAliasRequest{}, err
	}
	targetKind, err := NormalizePrincipalKind(req.TargetKind)
	if err != nil {
		return CreateAliasRequest{}, fmt.Errorf("target kind: %w", err)
	}
	targetID, err := NormalizePrincipalID(req.TargetID)
	if err != nil {
		return CreateAliasRequest{}, fmt.Errorf("target id: %w", err)
	}
	req.CompanyID = companyID
	req.DomainID = domainID
	req.Address = addressReq.Address
	req.TargetKind = targetKind
	req.TargetID = targetID
	return req, nil
}

func NormalizeListAliasesRequest(req ListAliasesRequest) (ListAliasesRequest, error) {
	companyID, err := NormalizePrincipalID(req.CompanyID)
	if err != nil {
		return ListAliasesRequest{}, fmt.Errorf("company id: %w", err)
	}
	req.CompanyID = companyID
	if strings.TrimSpace(req.DomainID) != "" {
		domainID, err := NormalizePrincipalID(req.DomainID)
		if err != nil {
			return ListAliasesRequest{}, fmt.Errorf("domain id: %w", err)
		}
		req.DomainID = domainID
	}
	if strings.TrimSpace(req.TargetKind) != "" {
		targetKind, err := NormalizePrincipalKind(req.TargetKind)
		if err != nil {
			return ListAliasesRequest{}, fmt.Errorf("target kind: %w", err)
		}
		req.TargetKind = targetKind
	}
	if strings.TrimSpace(req.TargetID) != "" {
		if req.TargetKind == "" {
			return ListAliasesRequest{}, fmt.Errorf("target kind is required when target id is set")
		}
		targetID, err := NormalizePrincipalID(req.TargetID)
		if err != nil {
			return ListAliasesRequest{}, fmt.Errorf("target id: %w", err)
		}
		req.TargetID = targetID
	}
	if strings.ContainsAny(req.Query, "\r\n") {
		return ListAliasesRequest{}, fmt.Errorf("alias search query must not contain line breaks")
	}
	query := strings.Join(strings.Fields(req.Query), " ")
	if len(query) > MaxAliasSearchBytes {
		return ListAliasesRequest{}, fmt.Errorf("alias search query is too long")
	}
	for _, r := range query {
		if unicode.IsControl(r) {
			return ListAliasesRequest{}, fmt.Errorf("alias search query must not contain control characters")
		}
	}
	req.Query = query
	if req.Limit < 0 {
		return ListAliasesRequest{}, fmt.Errorf("alias list limit must not be negative")
	}
	if req.Limit == 0 {
		req.Limit = DefaultAliasListLimit
	}
	if req.Limit > MaxAliasListLimit {
		return ListAliasesRequest{}, fmt.Errorf("alias list limit is too large")
	}
	return req, nil
}

func NormalizeCheckGroupMembershipRequest(req CheckGroupMembershipRequest) (CheckGroupMembershipRequest, error) {
	groupID, err := NormalizePrincipalID(req.GroupID)
	if err != nil {
		return CheckGroupMembershipRequest{}, fmt.Errorf("group id: %w", err)
	}
	memberID, err := NormalizePrincipalID(req.MemberID)
	if err != nil {
		return CheckGroupMembershipRequest{}, fmt.Errorf("member id: %w", err)
	}
	memberKind, err := NormalizePrincipalKind(req.MemberKind)
	if err != nil {
		return CheckGroupMembershipRequest{}, err
	}
	req.GroupID = groupID
	req.MemberID = memberID
	req.MemberKind = memberKind
	if req.MaxDepth < 0 {
		return CheckGroupMembershipRequest{}, fmt.Errorf("membership max depth must not be negative")
	}
	if req.MaxDepth == 0 {
		req.MaxDepth = DefaultMembershipMaxDepth
	}
	if req.MaxDepth > MaxGroupMembershipDepth {
		return CheckGroupMembershipRequest{}, fmt.Errorf("membership max depth is too large")
	}
	return req, nil
}

func NormalizeListGroupMembershipsRequest(req ListGroupMembershipsRequest) (ListGroupMembershipsRequest, error) {
	companyID, err := NormalizePrincipalID(req.CompanyID)
	if err != nil {
		return ListGroupMembershipsRequest{}, fmt.Errorf("company id: %w", err)
	}
	req.CompanyID = companyID
	if strings.TrimSpace(req.GroupID) != "" {
		groupID, err := NormalizePrincipalID(req.GroupID)
		if err != nil {
			return ListGroupMembershipsRequest{}, fmt.Errorf("group id: %w", err)
		}
		req.GroupID = groupID
	}
	if strings.TrimSpace(req.MemberKind) != "" {
		memberKind, err := NormalizePrincipalKind(req.MemberKind)
		if err != nil {
			return ListGroupMembershipsRequest{}, fmt.Errorf("member kind: %w", err)
		}
		req.MemberKind = memberKind
	}
	if strings.TrimSpace(req.MemberID) != "" {
		if req.MemberKind == "" {
			return ListGroupMembershipsRequest{}, fmt.Errorf("member kind is required when member id is set")
		}
		memberID, err := NormalizePrincipalID(req.MemberID)
		if err != nil {
			return ListGroupMembershipsRequest{}, fmt.Errorf("member id: %w", err)
		}
		req.MemberID = memberID
	}
	if strings.TrimSpace(req.Role) != "" {
		role, err := NormalizeGroupMembershipRole(req.Role)
		if err != nil {
			return ListGroupMembershipsRequest{}, err
		}
		req.Role = role
	}
	if req.GroupID != "" && req.MemberKind == PrincipalKindGroup && req.GroupID == req.MemberID {
		return ListGroupMembershipsRequest{}, fmt.Errorf("group membership cannot include itself")
	}
	if req.Limit < 0 {
		return ListGroupMembershipsRequest{}, fmt.Errorf("group membership list limit must not be negative")
	}
	if req.Limit == 0 {
		req.Limit = DefaultGroupMembershipListLimit
	}
	if req.Limit > MaxGroupMembershipListLimit {
		return ListGroupMembershipsRequest{}, fmt.Errorf("group membership list limit is too large")
	}
	return req, nil
}

func NormalizeUpdateGroupMembershipRoleRequest(req UpdateGroupMembershipRoleRequest) (UpdateGroupMembershipRoleRequest, error) {
	id, err := NormalizePrincipalID(req.ID)
	if err != nil {
		return UpdateGroupMembershipRoleRequest{}, fmt.Errorf("membership id: %w", err)
	}
	role, err := NormalizeGroupMembershipRole(req.Role)
	if err != nil {
		return UpdateGroupMembershipRoleRequest{}, err
	}
	req.ID = id
	req.Role = role
	return req, nil
}

func NormalizeReassignGroupMembershipRequest(req ReassignGroupMembershipRequest) (ReassignGroupMembershipRequest, error) {
	id, err := NormalizePrincipalID(req.ID)
	if err != nil {
		return ReassignGroupMembershipRequest{}, fmt.Errorf("membership id: %w", err)
	}
	check, err := NormalizeCheckGroupMembershipRequest(CheckGroupMembershipRequest{
		GroupID:    req.GroupID,
		MemberKind: req.MemberKind,
		MemberID:   req.MemberID,
	})
	if err != nil {
		return ReassignGroupMembershipRequest{}, err
	}
	req.ID = id
	req.GroupID = check.GroupID
	req.MemberKind = check.MemberKind
	req.MemberID = check.MemberID
	if req.MemberKind == PrincipalKindGroup && req.GroupID == req.MemberID {
		return ReassignGroupMembershipRequest{}, fmt.Errorf("group membership cannot include itself")
	}
	return req, nil
}

func NormalizeCreateGroupMembershipRequest(req CreateGroupMembershipRequest) (CreateGroupMembershipRequest, error) {
	check, err := NormalizeCheckGroupMembershipRequest(CheckGroupMembershipRequest{
		GroupID:    req.GroupID,
		MemberKind: req.MemberKind,
		MemberID:   req.MemberID,
	})
	if err != nil {
		return CreateGroupMembershipRequest{}, err
	}
	role, err := NormalizeGroupMembershipRole(req.Role)
	if err != nil {
		return CreateGroupMembershipRequest{}, err
	}
	req.GroupID = check.GroupID
	req.MemberKind = check.MemberKind
	req.MemberID = check.MemberID
	req.Role = role
	if req.MemberKind == PrincipalKindGroup && req.GroupID == req.MemberID {
		return CreateGroupMembershipRequest{}, fmt.Errorf("group membership cannot include itself")
	}
	return req, nil
}
