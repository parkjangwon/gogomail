package directory

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"
)

func TestNormalizeResolvePrincipalRequest(t *testing.T) {
	t.Parallel()

	got, err := NormalizeResolvePrincipalRequest(ResolvePrincipalRequest{
		ID:         " user-1 ",
		Kind:       " USER ",
		ActiveOnly: true,
	})
	if err != nil {
		t.Fatalf("NormalizeResolvePrincipalRequest returned error: %v", err)
	}
	if got.ID != "user-1" || got.Kind != PrincipalKindUser || !got.ActiveOnly {
		t.Fatalf("request = %+v", got)
	}
}

func TestNormalizePrincipalKindRejectsUnsupportedKinds(t *testing.T) {
	t.Parallel()

	if _, err := NormalizePrincipalKind("calendar"); err == nil {
		t.Fatal("NormalizePrincipalKind accepted unsupported kind")
	}
}

func TestNormalizePrincipalKindAcceptsOrganizationPrincipals(t *testing.T) {
	t.Parallel()

	tests := map[string]string{
		" Organization ": PrincipalKindOrganization,
		" GROUP ":        PrincipalKindGroup,
		" Resource ":     PrincipalKindResource,
	}
	for value, want := range tests {
		value, want := value, want
		t.Run(want, func(t *testing.T) {
			t.Parallel()

			got, err := NormalizePrincipalKind(value)
			if err != nil {
				t.Fatalf("NormalizePrincipalKind returned error: %v", err)
			}
			if got != want {
				t.Fatalf("kind = %q, want %q", got, want)
			}
		})
	}
}

func TestNormalizePrincipalIDRejectsUnsafeInput(t *testing.T) {
	t.Parallel()

	tests := []string{
		"",
		"user\n1",
		strings.Repeat("x", MaxPrincipalIDBytes+1),
	}
	for _, value := range tests {
		value := value
		t.Run(value, func(t *testing.T) {
			t.Parallel()

			if _, err := NormalizePrincipalID(value); err == nil {
				t.Fatalf("NormalizePrincipalID(%q) error = nil, want rejection", value)
			}
		})
	}
}

func TestNormalizeResolveAliasRequest(t *testing.T) {
	t.Parallel()

	got, err := NormalizeResolveAliasRequest(ResolveAliasRequest{
		Address:    " Ops@Example.COM ",
		ActiveOnly: true,
	})
	if err != nil {
		t.Fatalf("NormalizeResolveAliasRequest returned error: %v", err)
	}
	if got.Address != "ops@example.com" || !got.ActiveOnly {
		t.Fatalf("request = %+v", got)
	}
}

func TestNormalizeResolveAliasRequestRejectsInvalidAddresses(t *testing.T) {
	t.Parallel()

	tests := []string{
		"",
		"not an address",
		"ops@example.com\nbcc@example.net",
		strings.Repeat("local", 90) + "@example.com",
	}
	for _, address := range tests {
		address := address
		t.Run(address, func(t *testing.T) {
			t.Parallel()

			if _, err := NormalizeResolveAliasRequest(ResolveAliasRequest{Address: address}); err == nil {
				t.Fatalf("NormalizeResolveAliasRequest(%q) error = nil, want rejection", address)
			}
		})
	}
}

func TestNormalizeCreateAliasRequest(t *testing.T) {
	t.Parallel()

	got, err := NormalizeCreateAliasRequest(CreateAliasRequest{
		CompanyID:  " company-1 ",
		DomainID:   " domain-1 ",
		Address:    " Gogo Ops <Ops@Example.COM> ",
		TargetKind: " GROUP ",
		TargetID:   " group-1 ",
	})
	if err != nil {
		t.Fatalf("NormalizeCreateAliasRequest returned error: %v", err)
	}
	if got.CompanyID != "company-1" ||
		got.DomainID != "domain-1" ||
		got.Address != "ops@example.com" ||
		got.TargetKind != PrincipalKindGroup ||
		got.TargetID != "group-1" {
		t.Fatalf("request = %+v", got)
	}
}

func TestNormalizeCreateAliasRequestRejectsUnsafeInput(t *testing.T) {
	t.Parallel()

	tests := []CreateAliasRequest{
		{DomainID: "domain-1", Address: "ops@example.com", TargetKind: PrincipalKindGroup, TargetID: "group-1"},
		{CompanyID: "company-1", Address: "ops@example.com", TargetKind: PrincipalKindGroup, TargetID: "group-1"},
		{CompanyID: "company-1", DomainID: "domain-1", Address: "not an address", TargetKind: PrincipalKindGroup, TargetID: "group-1"},
		{CompanyID: "company-1", DomainID: "domain-1", Address: "ops@example.com", TargetKind: "calendar", TargetID: "group-1"},
		{CompanyID: "company-1", DomainID: "domain-1", Address: "ops@example.com", TargetKind: PrincipalKindGroup},
		{CompanyID: "company-1", DomainID: "domain-1", Address: "ops@example.com\nbcc@example.net", TargetKind: PrincipalKindGroup, TargetID: "group-1"},
		{CompanyID: "company-1", DomainID: "domain-1", Address: strings.Repeat("local", 90) + "@example.com", TargetKind: PrincipalKindGroup, TargetID: "group-1"},
	}
	for _, req := range tests {
		req := req
		t.Run(req.CompanyID+"/"+req.DomainID+"/"+req.Address, func(t *testing.T) {
			t.Parallel()

			if _, err := NormalizeCreateAliasRequest(req); err == nil {
				t.Fatalf("NormalizeCreateAliasRequest(%+v) error = nil, want rejection", req)
			}
		})
	}
}

func TestAliasAddressMatchesDomain(t *testing.T) {
	t.Parallel()

	if !aliasAddressMatchesDomain("ops@example.com", "EXAMPLE.COM") {
		t.Fatal("aliasAddressMatchesDomain rejected matching normalized domain")
	}
	if aliasAddressMatchesDomain("ops@example.net", "example.com") {
		t.Fatal("aliasAddressMatchesDomain accepted mismatched domain")
	}
}

func TestMapDirectoryAliasInsertErrorMapsActiveAddressUniqueIndex(t *testing.T) {
	t.Parallel()

	err := mapDirectoryAliasInsertError(&pgconn.PgError{
		Code:           "23505",
		ConstraintName: "idx_directory_aliases_active_address",
	})
	if !errors.Is(err, ErrAliasAlreadyExists) {
		t.Fatalf("mapped error = %v, want ErrAliasAlreadyExists", err)
	}
}

func TestMapDirectoryDelegationInsertErrorMapsActiveGrantUniqueIndex(t *testing.T) {
	t.Parallel()

	err := mapDirectoryDelegationInsertError(&pgconn.PgError{
		Code:           "23505",
		ConstraintName: "idx_directory_delegations_active_grant",
	})
	if !errors.Is(err, ErrDelegationAlreadyExists) {
		t.Fatalf("mapped error = %v, want ErrDelegationAlreadyExists", err)
	}
}

func TestMapDirectoryDelegationUpdateErrorMapsActiveGrantUniqueIndex(t *testing.T) {
	t.Parallel()

	err := mapDirectoryDelegationUpdateError(&pgconn.PgError{
		Code:           "23505",
		ConstraintName: "idx_directory_delegations_active_grant",
	})
	if !errors.Is(err, ErrDelegationAlreadyExists) {
		t.Fatalf("mapped error = %v, want ErrDelegationAlreadyExists", err)
	}
}

func TestMapDirectoryGroupMembershipInsertErrorMapsActiveMemberUniqueIndex(t *testing.T) {
	t.Parallel()

	err := mapDirectoryGroupMembershipInsertError(&pgconn.PgError{
		Code:           "23505",
		ConstraintName: "idx_directory_group_memberships_active_member",
	})
	if !errors.Is(err, ErrGroupMembershipAlreadyExists) {
		t.Fatalf("mapped error = %v, want ErrGroupMembershipAlreadyExists", err)
	}
}

func TestDirectoryAliasCreateAuditDetail(t *testing.T) {
	t.Parallel()

	detail, err := directoryAliasCreateAuditDetail(Alias{
		ID:         "alias-1",
		CompanyID:  "company-1",
		DomainID:   "domain-1",
		Address:    "ops@example.com",
		AddressACE: "ops@example.com",
		TargetKind: PrincipalKindGroup,
		TargetID:   "group-1",
		Status:     "active",
	})
	if err != nil {
		t.Fatalf("directoryAliasCreateAuditDetail returned error: %v", err)
	}
	if !strings.Contains(string(detail), `"alias_id":"alias-1"`) ||
		!strings.Contains(string(detail), `"target_kind":"group"`) ||
		strings.Contains(string(detail), "TargetPrincipal") {
		t.Fatalf("audit detail = %s", detail)
	}
}

func TestDirectoryAliasDeleteAuditDetail(t *testing.T) {
	t.Parallel()

	detail, err := directoryAliasDeleteAuditDetail(Alias{
		ID:         "alias-1",
		CompanyID:  "company-1",
		DomainID:   "domain-1",
		Address:    "ops@example.com",
		AddressACE: "ops@example.com",
		TargetKind: PrincipalKindGroup,
		TargetID:   "group-1",
		Status:     "deleted",
	})
	if err != nil {
		t.Fatalf("directoryAliasDeleteAuditDetail returned error: %v", err)
	}
	if !strings.Contains(string(detail), `"previous_status":"active"`) ||
		!strings.Contains(string(detail), `"status":"deleted"`) {
		t.Fatalf("audit detail = %s", detail)
	}
}

func TestNormalizeListAliasesRequest(t *testing.T) {
	t.Parallel()

	got, err := NormalizeListAliasesRequest(ListAliasesRequest{
		CompanyID:  " company-1 ",
		DomainID:   " domain-1 ",
		TargetKind: " GROUP ",
		TargetID:   " group-1 ",
		Query:      "  Ops   Alias  ",
		ActiveOnly: true,
		Limit:      5,
	})
	if err != nil {
		t.Fatalf("NormalizeListAliasesRequest returned error: %v", err)
	}
	if got.CompanyID != "company-1" ||
		got.DomainID != "domain-1" ||
		got.TargetKind != PrincipalKindGroup ||
		got.TargetID != "group-1" ||
		got.Query != "Ops Alias" ||
		!got.ActiveOnly ||
		got.Limit != 5 {
		t.Fatalf("request = %+v", got)
	}
}

func TestNormalizeListAliasesRequestDefaultsLimit(t *testing.T) {
	t.Parallel()

	got, err := NormalizeListAliasesRequest(ListAliasesRequest{CompanyID: "company-1"})
	if err != nil {
		t.Fatalf("NormalizeListAliasesRequest returned error: %v", err)
	}
	if got.Limit != DefaultAliasListLimit {
		t.Fatalf("limit = %d, want %d", got.Limit, DefaultAliasListLimit)
	}
}

func TestNormalizeListAliasesRequestRejectsUnsafeInput(t *testing.T) {
	t.Parallel()

	tests := []ListAliasesRequest{
		{Query: "ops"},
		{CompanyID: "company\n1"},
		{CompanyID: "company-1", DomainID: "domain\n1"},
		{CompanyID: "company-1", TargetKind: "calendar"},
		{CompanyID: "company-1", TargetID: "group-1"},
		{CompanyID: "company-1", TargetKind: PrincipalKindGroup, TargetID: "group\n1"},
		{CompanyID: "company-1", Query: "ops\nalias"},
		{CompanyID: "company-1", Query: strings.Repeat("x", MaxAliasSearchBytes+1)},
		{CompanyID: "company-1", Limit: -1},
		{CompanyID: "company-1", Limit: MaxAliasListLimit + 1},
	}
	for _, req := range tests {
		req := req
		t.Run(req.CompanyID+"/"+req.Query, func(t *testing.T) {
			t.Parallel()

			if _, err := NormalizeListAliasesRequest(req); err == nil {
				t.Fatalf("NormalizeListAliasesRequest(%+v) error = nil, want rejection", req)
			}
		})
	}
}

func TestNormalizeSearchPrincipalsRequest(t *testing.T) {
	t.Parallel()

	got, err := NormalizeSearchPrincipalsRequest(SearchPrincipalsRequest{
		CompanyID:      " company-1 ",
		DomainID:       " domain-1 ",
		OrganizationID: " org-1 ",
		Kinds:          []string{" USER ", "resource", "USER"},
		Query:          "  Alice   Room  ",
		ActiveOnly:     true,
		Limit:          5,
	})
	if err != nil {
		t.Fatalf("NormalizeSearchPrincipalsRequest returned error: %v", err)
	}
	if got.CompanyID != "company-1" ||
		got.DomainID != "domain-1" ||
		got.OrganizationID != "org-1" ||
		got.Query != "Alice Room" ||
		!got.ActiveOnly ||
		got.Limit != 5 {
		t.Fatalf("request = %+v", got)
	}
	if strings.Join(got.Kinds, ",") != PrincipalKindUser+","+PrincipalKindResource {
		t.Fatalf("kinds = %#v", got.Kinds)
	}
}

func TestNormalizeSearchPrincipalsRequestDefaultsKindsAndLimit(t *testing.T) {
	t.Parallel()

	got, err := NormalizeSearchPrincipalsRequest(SearchPrincipalsRequest{CompanyID: "company-1"})
	if err != nil {
		t.Fatalf("NormalizeSearchPrincipalsRequest returned error: %v", err)
	}
	if got.Limit != DefaultPrincipalSearchLimit {
		t.Fatalf("limit = %d, want %d", got.Limit, DefaultPrincipalSearchLimit)
	}
	if strings.Join(got.Kinds, ",") != PrincipalKindUser+","+PrincipalKindOrganization+","+PrincipalKindGroup+","+PrincipalKindResource {
		t.Fatalf("kinds = %#v", got.Kinds)
	}
}

func TestNormalizeSearchPrincipalsRequestRejectsUnsafeInput(t *testing.T) {
	t.Parallel()

	tests := []SearchPrincipalsRequest{
		{Query: "alice"},
		{CompanyID: "company\n1"},
		{CompanyID: "company-1", DomainID: "domain\n1"},
		{CompanyID: "company-1", OrganizationID: "org\n1"},
		{CompanyID: "company-1", Kinds: []string{"calendar"}},
		{CompanyID: "company-1", Query: "alice\nbob"},
		{CompanyID: "company-1", Query: strings.Repeat("x", MaxPrincipalSearchBytes+1)},
		{CompanyID: "company-1", Limit: -1},
		{CompanyID: "company-1", Limit: MaxPrincipalSearchLimit + 1},
	}
	for _, req := range tests {
		req := req
		t.Run(req.CompanyID+"/"+req.Query, func(t *testing.T) {
			t.Parallel()

			if _, err := NormalizeSearchPrincipalsRequest(req); err == nil {
				t.Fatalf("NormalizeSearchPrincipalsRequest(%+v) error = nil, want rejection", req)
			}
		})
	}
}

func TestPrincipalSearchPatternEscapesLikeWildcards(t *testing.T) {
	t.Parallel()

	got := principalSearchPattern(` A_%\ `)
	if got != `%a\_\%\\%` {
		t.Fatalf("principalSearchPattern = %q", got)
	}
	if got := principalSearchPattern("  "); got != "" {
		t.Fatalf("empty principalSearchPattern = %q", got)
	}
}

func TestNormalizeCheckGroupMembershipRequest(t *testing.T) {
	t.Parallel()

	got, err := NormalizeCheckGroupMembershipRequest(CheckGroupMembershipRequest{
		GroupID:    " group-1 ",
		MemberKind: " USER ",
		MemberID:   " user-1 ",
		ActiveOnly: true,
	})
	if err != nil {
		t.Fatalf("NormalizeCheckGroupMembershipRequest returned error: %v", err)
	}
	if got.GroupID != "group-1" || got.MemberKind != PrincipalKindUser || got.MemberID != "user-1" || !got.ActiveOnly || got.MaxDepth != DefaultMembershipMaxDepth {
		t.Fatalf("request = %+v", got)
	}
}

func TestNormalizeCreateGroupMembershipRequest(t *testing.T) {
	t.Parallel()

	got, err := NormalizeCreateGroupMembershipRequest(CreateGroupMembershipRequest{
		GroupID:    " group-1 ",
		MemberKind: " USER ",
		MemberID:   " user-1 ",
		Role:       " OWNER ",
	})
	if err != nil {
		t.Fatalf("NormalizeCreateGroupMembershipRequest returned error: %v", err)
	}
	if got.GroupID != "group-1" ||
		got.MemberKind != PrincipalKindUser ||
		got.MemberID != "user-1" ||
		got.Role != GroupMembershipRoleOwner {
		t.Fatalf("request = %+v", got)
	}
}

func TestNormalizeCreateGroupMembershipRequestRejectsUnsafeInput(t *testing.T) {
	t.Parallel()

	tests := []CreateGroupMembershipRequest{
		{MemberKind: PrincipalKindUser, MemberID: "user-1"},
		{GroupID: "group\n1", MemberKind: PrincipalKindUser, MemberID: "user-1"},
		{GroupID: "group-1", MemberKind: "calendar", MemberID: "user-1"},
		{GroupID: "group-1", MemberKind: PrincipalKindUser, MemberID: "user\n1"},
		{GroupID: "group-1", MemberKind: PrincipalKindGroup, MemberID: "group-1"},
		{GroupID: "group-1", MemberKind: PrincipalKindUser, MemberID: "user-1", Role: "admin"},
	}
	for _, req := range tests {
		req := req
		t.Run(req.GroupID+"/"+req.MemberID, func(t *testing.T) {
			t.Parallel()

			if _, err := NormalizeCreateGroupMembershipRequest(req); err == nil {
				t.Fatalf("NormalizeCreateGroupMembershipRequest(%+v) error = nil, want rejection", req)
			}
		})
	}
}

func TestNormalizeCheckGroupMembershipRequestHonorsExplicitDepth(t *testing.T) {
	t.Parallel()

	got, err := NormalizeCheckGroupMembershipRequest(CheckGroupMembershipRequest{
		GroupID:    "group-1",
		MemberKind: PrincipalKindGroup,
		MemberID:   "group-2",
		MaxDepth:   3,
	})
	if err != nil {
		t.Fatalf("NormalizeCheckGroupMembershipRequest returned error: %v", err)
	}
	if got.MaxDepth != 3 {
		t.Fatalf("max depth = %d, want 3", got.MaxDepth)
	}
}

func TestNormalizeCheckGroupMembershipRequestRejectsUnsafeInput(t *testing.T) {
	t.Parallel()

	tests := []CheckGroupMembershipRequest{
		{MemberKind: PrincipalKindUser, MemberID: "user-1"},
		{GroupID: "group-1", MemberKind: PrincipalKindUser},
		{GroupID: "group\n1", MemberKind: PrincipalKindUser, MemberID: "user-1"},
		{GroupID: "group-1", MemberKind: "calendar", MemberID: "user-1"},
		{GroupID: "group-1", MemberKind: PrincipalKindUser, MemberID: "user\n1"},
		{GroupID: "group-1", MemberKind: PrincipalKindUser, MemberID: "user-1", MaxDepth: -1},
		{GroupID: "group-1", MemberKind: PrincipalKindUser, MemberID: "user-1", MaxDepth: MaxGroupMembershipDepth + 1},
	}
	for _, req := range tests {
		req := req
		t.Run(req.GroupID+"/"+req.MemberID, func(t *testing.T) {
			t.Parallel()

			if _, err := NormalizeCheckGroupMembershipRequest(req); err == nil {
				t.Fatalf("NormalizeCheckGroupMembershipRequest(%+v) error = nil, want rejection", req)
			}
		})
	}
}

func TestDirectoryDelegationCreateAuditDetail(t *testing.T) {
	t.Parallel()

	detail, err := directoryDelegationCreateAuditDetail(Delegation{
		ID:           "delegation-1",
		CompanyID:    "company-1",
		OwnerKind:    PrincipalKindResource,
		OwnerID:      "room-1",
		DelegateKind: PrincipalKindGroup,
		DelegateID:   "team-1",
		Scope:        DelegationScopeCalendar,
		Role:         DelegationRoleWrite,
		Status:       "active",
	})
	if err != nil {
		t.Fatalf("directoryDelegationCreateAuditDetail returned error: %v", err)
	}
	if !strings.Contains(string(detail), `"delegation_id":"delegation-1"`) ||
		!strings.Contains(string(detail), `"scope":"calendar"`) ||
		!strings.Contains(string(detail), `"role":"write"`) {
		t.Fatalf("audit detail = %s", detail)
	}
}

func TestDirectoryDelegationRoleUpdateAuditDetail(t *testing.T) {
	t.Parallel()

	detail, err := directoryDelegationRoleUpdateAuditDetail(Delegation{
		ID:           "delegation-1",
		CompanyID:    "company-1",
		OwnerKind:    PrincipalKindResource,
		OwnerID:      "room-1",
		DelegateKind: PrincipalKindGroup,
		DelegateID:   "team-1",
		Scope:        DelegationScopeCalendar,
		Role:         DelegationRoleManage,
		Status:       "active",
	}, DelegationRoleRead)
	if err != nil {
		t.Fatalf("directoryDelegationRoleUpdateAuditDetail returned error: %v", err)
	}
	if !strings.Contains(string(detail), `"previous_role":"read"`) ||
		!strings.Contains(string(detail), `"role":"manage"`) ||
		!strings.Contains(string(detail), `"delegation_id":"delegation-1"`) {
		t.Fatalf("audit detail = %s", detail)
	}
}

func TestDirectoryDelegationReassignAuditDetail(t *testing.T) {
	t.Parallel()

	detail, err := directoryDelegationReassignAuditDetail(Delegation{
		ID:           "delegation-1",
		CompanyID:    "company-1",
		OwnerKind:    PrincipalKindResource,
		OwnerID:      "room-2",
		DelegateKind: PrincipalKindUser,
		DelegateID:   "user-1",
		Scope:        DelegationScopeCalendar,
		Role:         DelegationRoleWrite,
		Status:       "active",
	}, Delegation{
		ID:           "delegation-1",
		CompanyID:    "company-1",
		OwnerKind:    PrincipalKindResource,
		OwnerID:      "room-1",
		DelegateKind: PrincipalKindGroup,
		DelegateID:   "team-1",
		Scope:        DelegationScopeContacts,
		Role:         DelegationRoleWrite,
		Status:       "active",
	})
	if err != nil {
		t.Fatalf("directoryDelegationReassignAuditDetail returned error: %v", err)
	}
	if !strings.Contains(string(detail), `"previous_owner_id":"room-1"`) ||
		!strings.Contains(string(detail), `"owner_id":"room-2"`) ||
		!strings.Contains(string(detail), `"previous_scope":"contacts"`) ||
		!strings.Contains(string(detail), `"scope":"calendar"`) {
		t.Fatalf("audit detail = %s", detail)
	}
}

func TestDirectoryDelegationDeleteAuditDetail(t *testing.T) {
	t.Parallel()

	detail, err := directoryDelegationDeleteAuditDetail(Delegation{
		ID:           "delegation-1",
		CompanyID:    "company-1",
		OwnerKind:    PrincipalKindResource,
		OwnerID:      "room-1",
		DelegateKind: PrincipalKindGroup,
		DelegateID:   "team-1",
		Scope:        DelegationScopeCalendar,
		Role:         DelegationRoleWrite,
		Status:       "deleted",
	})
	if err != nil {
		t.Fatalf("directoryDelegationDeleteAuditDetail returned error: %v", err)
	}
	if !strings.Contains(string(detail), `"previous_status":"active"`) ||
		!strings.Contains(string(detail), `"status":"deleted"`) ||
		!strings.Contains(string(detail), `"delegation_id":"delegation-1"`) {
		t.Fatalf("audit detail = %s", detail)
	}
}

func TestNormalizeListGroupMembershipsRequest(t *testing.T) {
	t.Parallel()

	got, err := NormalizeListGroupMembershipsRequest(ListGroupMembershipsRequest{
		CompanyID:  " company-1 ",
		GroupID:    " group-1 ",
		MemberKind: " USER ",
		MemberID:   " user-1 ",
		Role:       " OWNER ",
		ActiveOnly: true,
		Limit:      25,
	})
	if err != nil {
		t.Fatalf("NormalizeListGroupMembershipsRequest returned error: %v", err)
	}
	if got.CompanyID != "company-1" ||
		got.GroupID != "group-1" ||
		got.MemberKind != PrincipalKindUser ||
		got.MemberID != "user-1" ||
		got.Role != GroupMembershipRoleOwner ||
		!got.ActiveOnly ||
		got.Limit != 25 {
		t.Fatalf("request = %+v", got)
	}
}

func TestNormalizeListGroupMembershipsRequestDefaultsLimit(t *testing.T) {
	t.Parallel()

	got, err := NormalizeListGroupMembershipsRequest(ListGroupMembershipsRequest{CompanyID: "company-1"})
	if err != nil {
		t.Fatalf("NormalizeListGroupMembershipsRequest returned error: %v", err)
	}
	if got.Limit != DefaultGroupMembershipListLimit {
		t.Fatalf("limit = %d, want %d", got.Limit, DefaultGroupMembershipListLimit)
	}
}

func TestNormalizeListGroupMembershipsRequestRejectsUnsafeInput(t *testing.T) {
	t.Parallel()

	tests := []ListGroupMembershipsRequest{
		{},
		{CompanyID: "company\n1"},
		{CompanyID: "company-1", GroupID: "group\n1"},
		{CompanyID: "company-1", MemberKind: "calendar"},
		{CompanyID: "company-1", MemberID: "user-1"},
		{CompanyID: "company-1", MemberKind: PrincipalKindUser, MemberID: "user\n1"},
		{CompanyID: "company-1", GroupID: "group-1", MemberKind: PrincipalKindGroup, MemberID: "group-1"},
		{CompanyID: "company-1", Role: "admin"},
		{CompanyID: "company-1", Limit: -1},
		{CompanyID: "company-1", Limit: MaxGroupMembershipListLimit + 1},
	}
	for _, req := range tests {
		req := req
		t.Run(req.CompanyID+"/"+req.GroupID+"/"+req.MemberID, func(t *testing.T) {
			t.Parallel()

			if _, err := NormalizeListGroupMembershipsRequest(req); err == nil {
				t.Fatalf("NormalizeListGroupMembershipsRequest(%+v) error = nil, want rejection", req)
			}
		})
	}
}

func TestNormalizeUpdateGroupMembershipRoleRequest(t *testing.T) {
	t.Parallel()

	got, err := NormalizeUpdateGroupMembershipRoleRequest(UpdateGroupMembershipRoleRequest{
		ID:   " membership-1 ",
		Role: " OWNER ",
	})
	if err != nil {
		t.Fatalf("NormalizeUpdateGroupMembershipRoleRequest returned error: %v", err)
	}
	if got.ID != "membership-1" || got.Role != GroupMembershipRoleOwner {
		t.Fatalf("request = %+v", got)
	}
}

func TestNormalizeUpdateGroupMembershipRoleRequestRejectsUnsafeInput(t *testing.T) {
	t.Parallel()

	tests := []UpdateGroupMembershipRoleRequest{
		{},
		{ID: "membership\n1", Role: GroupMembershipRoleOwner},
		{ID: "membership-1", Role: "admin"},
	}
	for _, req := range tests {
		req := req
		t.Run(req.ID+"/"+req.Role, func(t *testing.T) {
			t.Parallel()

			if _, err := NormalizeUpdateGroupMembershipRoleRequest(req); err == nil {
				t.Fatalf("NormalizeUpdateGroupMembershipRoleRequest(%+v) error = nil, want rejection", req)
			}
		})
	}
}

func TestNormalizeReassignGroupMembershipRequest(t *testing.T) {
	t.Parallel()

	got, err := NormalizeReassignGroupMembershipRequest(ReassignGroupMembershipRequest{
		ID:         " membership-1 ",
		GroupID:    " group-1 ",
		MemberKind: " USER ",
		MemberID:   " user-1 ",
	})
	if err != nil {
		t.Fatalf("NormalizeReassignGroupMembershipRequest returned error: %v", err)
	}
	if got.ID != "membership-1" ||
		got.GroupID != "group-1" ||
		got.MemberKind != PrincipalKindUser ||
		got.MemberID != "user-1" {
		t.Fatalf("request = %+v", got)
	}
}

func TestNormalizeReassignGroupMembershipRequestRejectsUnsafeInput(t *testing.T) {
	t.Parallel()

	tests := []ReassignGroupMembershipRequest{
		{},
		{ID: "membership\n1", GroupID: "group-1", MemberKind: PrincipalKindUser, MemberID: "user-1"},
		{ID: "membership-1", GroupID: "group\n1", MemberKind: PrincipalKindUser, MemberID: "user-1"},
		{ID: "membership-1", GroupID: "group-1", MemberKind: "calendar", MemberID: "user-1"},
		{ID: "membership-1", GroupID: "group-1", MemberKind: PrincipalKindGroup, MemberID: "group-1"},
	}
	for _, req := range tests {
		req := req
		t.Run(req.ID+"/"+req.GroupID+"/"+req.MemberID, func(t *testing.T) {
			t.Parallel()

			if _, err := NormalizeReassignGroupMembershipRequest(req); err == nil {
				t.Fatalf("NormalizeReassignGroupMembershipRequest(%+v) error = nil, want rejection", req)
			}
		})
	}
}

func TestDirectoryGroupMembershipCreateAuditDetail(t *testing.T) {
	t.Parallel()

	detail, err := directoryGroupMembershipCreateAuditDetail(GroupMembership{
		ID:         "membership-1",
		GroupID:    "group-1",
		CompanyID:  "company-1",
		MemberKind: PrincipalKindUser,
		MemberID:   "user-1",
		Role:       GroupMembershipRoleManager,
		Status:     "active",
	})
	if err != nil {
		t.Fatalf("directoryGroupMembershipCreateAuditDetail returned error: %v", err)
	}
	if !strings.Contains(string(detail), `"membership_id":"membership-1"`) ||
		!strings.Contains(string(detail), `"role":"manager"`) {
		t.Fatalf("audit detail = %s", detail)
	}
}

func TestDirectoryGroupMembershipRoleUpdateAuditDetail(t *testing.T) {
	t.Parallel()

	detail, err := directoryGroupMembershipRoleUpdateAuditDetail(GroupMembership{
		ID:         "membership-1",
		GroupID:    "group-1",
		CompanyID:  "company-1",
		MemberKind: PrincipalKindUser,
		MemberID:   "user-1",
		Role:       GroupMembershipRoleOwner,
		Status:     "active",
	}, GroupMembershipRoleMember)
	if err != nil {
		t.Fatalf("directoryGroupMembershipRoleUpdateAuditDetail returned error: %v", err)
	}
	if !strings.Contains(string(detail), `"membership_id":"membership-1"`) ||
		!strings.Contains(string(detail), `"previous_role":"member"`) ||
		!strings.Contains(string(detail), `"role":"owner"`) {
		t.Fatalf("audit detail = %s", detail)
	}
}

func TestDirectoryGroupMembershipReassignAuditDetail(t *testing.T) {
	t.Parallel()

	detail, err := directoryGroupMembershipReassignAuditDetail(GroupMembership{
		ID:         "membership-1",
		GroupID:    "group-2",
		CompanyID:  "company-1",
		MemberKind: PrincipalKindUser,
		MemberID:   "user-2",
		Role:       GroupMembershipRoleOwner,
		Status:     "active",
	}, GroupMembership{
		ID:         "membership-1",
		GroupID:    "group-1",
		CompanyID:  "company-1",
		MemberKind: PrincipalKindUser,
		MemberID:   "user-1",
		Role:       GroupMembershipRoleOwner,
		Status:     "active",
	})
	if err != nil {
		t.Fatalf("directoryGroupMembershipReassignAuditDetail returned error: %v", err)
	}
	if !strings.Contains(string(detail), `"membership_id":"membership-1"`) ||
		!strings.Contains(string(detail), `"previous_group_id":"group-1"`) ||
		!strings.Contains(string(detail), `"group_id":"group-2"`) ||
		!strings.Contains(string(detail), `"previous_member_id":"user-1"`) ||
		!strings.Contains(string(detail), `"member_id":"user-2"`) {
		t.Fatalf("audit detail = %s", detail)
	}
}

func TestDirectoryGroupMembershipDeleteAuditDetail(t *testing.T) {
	t.Parallel()

	detail, err := directoryGroupMembershipDeleteAuditDetail(GroupMembership{
		ID:         "membership-1",
		GroupID:    "group-1",
		CompanyID:  "company-1",
		MemberKind: PrincipalKindGroup,
		MemberID:   "team-1",
		Role:       GroupMembershipRoleOwner,
		Status:     "deleted",
	})
	if err != nil {
		t.Fatalf("directoryGroupMembershipDeleteAuditDetail returned error: %v", err)
	}
	if !strings.Contains(string(detail), `"membership_id":"membership-1"`) ||
		!strings.Contains(string(detail), `"previous_status":"active"`) ||
		!strings.Contains(string(detail), `"status":"deleted"`) {
		t.Fatalf("audit detail = %s", detail)
	}
}

func TestNormalizeGroupMembershipRole(t *testing.T) {
	t.Parallel()

	got, err := NormalizeGroupMembershipRole(" MANAGER ")
	if err != nil {
		t.Fatalf("NormalizeGroupMembershipRole returned error: %v", err)
	}
	if got != GroupMembershipRoleManager {
		t.Fatalf("role = %q", got)
	}
	if got, err := NormalizeGroupMembershipRole(""); err != nil || got != GroupMembershipRoleMember {
		t.Fatalf("default role = %q err=%v", got, err)
	}
	if _, err := NormalizeGroupMembershipRole("admin"); err == nil {
		t.Fatal("NormalizeGroupMembershipRole accepted unsupported role")
	}
}

func TestNormalizeCheckDelegationRequest(t *testing.T) {
	t.Parallel()

	got, err := NormalizeCheckDelegationRequest(CheckDelegationRequest{
		CompanyID:    " company-1 ",
		OwnerKind:    " Resource ",
		OwnerID:      " room-1 ",
		DelegateKind: " GROUP ",
		DelegateID:   " team-1 ",
		Scope:        " Calendar ",
		RequiredRole: " WRITE ",
		ActiveOnly:   true,
	})
	if err != nil {
		t.Fatalf("NormalizeCheckDelegationRequest returned error: %v", err)
	}
	if got.CompanyID != "company-1" ||
		got.OwnerKind != PrincipalKindResource ||
		got.OwnerID != "room-1" ||
		got.DelegateKind != PrincipalKindGroup ||
		got.DelegateID != "team-1" ||
		got.Scope != DelegationScopeCalendar ||
		got.RequiredRole != DelegationRoleWrite ||
		!got.ActiveOnly ||
		got.MaxDepth != DefaultMembershipMaxDepth {
		t.Fatalf("request = %+v", got)
	}
}

func TestNormalizeCreateDelegationRequest(t *testing.T) {
	t.Parallel()

	got, err := NormalizeCreateDelegationRequest(CreateDelegationRequest{
		CompanyID:    " company-1 ",
		OwnerKind:    " Resource ",
		OwnerID:      " room-1 ",
		DelegateKind: " GROUP ",
		DelegateID:   " team-1 ",
		Scope:        " Calendar ",
		Role:         " WRITE ",
	})
	if err != nil {
		t.Fatalf("NormalizeCreateDelegationRequest returned error: %v", err)
	}
	if got.CompanyID != "company-1" ||
		got.OwnerKind != PrincipalKindResource ||
		got.OwnerID != "room-1" ||
		got.DelegateKind != PrincipalKindGroup ||
		got.DelegateID != "team-1" ||
		got.Scope != DelegationScopeCalendar ||
		got.Role != DelegationRoleWrite {
		t.Fatalf("request = %+v", got)
	}
}

func TestNormalizeUpdateDelegationRoleRequest(t *testing.T) {
	t.Parallel()

	got, err := NormalizeUpdateDelegationRoleRequest(UpdateDelegationRoleRequest{
		ID:   " delegation-1 ",
		Role: " MANAGE ",
	})
	if err != nil {
		t.Fatalf("NormalizeUpdateDelegationRoleRequest returned error: %v", err)
	}
	if got.ID != "delegation-1" || got.Role != DelegationRoleManage {
		t.Fatalf("request = %+v", got)
	}
}

func TestNormalizeUpdateDelegationRoleRequestRejectsUnsafeInput(t *testing.T) {
	t.Parallel()

	tests := []UpdateDelegationRoleRequest{
		{},
		{ID: "delegation\n1", Role: DelegationRoleManage},
		{ID: "delegation-1", Role: "owner"},
	}
	for _, req := range tests {
		req := req
		t.Run(req.ID+"/"+req.Role, func(t *testing.T) {
			t.Parallel()

			if _, err := NormalizeUpdateDelegationRoleRequest(req); err == nil {
				t.Fatalf("NormalizeUpdateDelegationRoleRequest(%+v) error = nil, want rejection", req)
			}
		})
	}
}

func TestNormalizeReassignDelegationRequest(t *testing.T) {
	t.Parallel()

	got, err := NormalizeReassignDelegationRequest(ReassignDelegationRequest{
		ID:           " delegation-1 ",
		OwnerKind:    " Resource ",
		OwnerID:      " room-2 ",
		DelegateKind: " USER ",
		DelegateID:   " user-1 ",
		Scope:        " Drive ",
	})
	if err != nil {
		t.Fatalf("NormalizeReassignDelegationRequest returned error: %v", err)
	}
	if got.ID != "delegation-1" ||
		got.OwnerKind != PrincipalKindResource ||
		got.OwnerID != "room-2" ||
		got.DelegateKind != PrincipalKindUser ||
		got.DelegateID != "user-1" ||
		got.Scope != DelegationScopeDrive {
		t.Fatalf("request = %+v", got)
	}
}

func TestNormalizeReassignDelegationRequestRejectsUnsafeInput(t *testing.T) {
	t.Parallel()

	tests := []ReassignDelegationRequest{
		{},
		{ID: "delegation\n1", OwnerKind: PrincipalKindResource, OwnerID: "room-1", DelegateKind: PrincipalKindUser, DelegateID: "user-1", Scope: DelegationScopeCalendar},
		{ID: "delegation-1", OwnerKind: "calendar", OwnerID: "room-1", DelegateKind: PrincipalKindUser, DelegateID: "user-1", Scope: DelegationScopeCalendar},
		{ID: "delegation-1", OwnerKind: PrincipalKindResource, OwnerID: "room\n1", DelegateKind: PrincipalKindUser, DelegateID: "user-1", Scope: DelegationScopeCalendar},
		{ID: "delegation-1", OwnerKind: PrincipalKindResource, OwnerID: "room-1", DelegateKind: "calendar", DelegateID: "user-1", Scope: DelegationScopeCalendar},
		{ID: "delegation-1", OwnerKind: PrincipalKindResource, OwnerID: "room-1", DelegateKind: PrincipalKindUser, DelegateID: "user\n1", Scope: DelegationScopeCalendar},
		{ID: "delegation-1", OwnerKind: PrincipalKindUser, OwnerID: "user-1", DelegateKind: PrincipalKindUser, DelegateID: "user-1", Scope: DelegationScopeCalendar},
		{ID: "delegation-1", OwnerKind: PrincipalKindResource, OwnerID: "room-1", DelegateKind: PrincipalKindUser, DelegateID: "user-1", Scope: "files"},
	}
	for _, req := range tests {
		req := req
		t.Run(req.ID+"/"+req.OwnerID+"/"+req.DelegateID, func(t *testing.T) {
			t.Parallel()

			if _, err := NormalizeReassignDelegationRequest(req); err == nil {
				t.Fatalf("NormalizeReassignDelegationRequest(%+v) error = nil, want rejection", req)
			}
		})
	}
}

func TestNormalizeCheckDelegationRequestHonorsExplicitDepth(t *testing.T) {
	t.Parallel()

	got, err := NormalizeCheckDelegationRequest(CheckDelegationRequest{
		CompanyID:    "company-1",
		OwnerKind:    PrincipalKindResource,
		OwnerID:      "room-1",
		DelegateKind: PrincipalKindGroup,
		DelegateID:   "team-1",
		Scope:        DelegationScopeCalendar,
		RequiredRole: DelegationRoleRead,
		MaxDepth:     3,
	})
	if err != nil {
		t.Fatalf("NormalizeCheckDelegationRequest returned error: %v", err)
	}
	if got.MaxDepth != 3 {
		t.Fatalf("max depth = %d, want 3", got.MaxDepth)
	}
}

func TestNormalizeCheckDelegationRequestRejectsUnsafeInput(t *testing.T) {
	t.Parallel()

	tests := []CheckDelegationRequest{
		{OwnerKind: PrincipalKindUser, OwnerID: "owner-1", DelegateKind: PrincipalKindUser, DelegateID: "delegate-1", Scope: DelegationScopeCalendar, RequiredRole: DelegationRoleRead},
		{CompanyID: "company\n1", OwnerKind: PrincipalKindUser, OwnerID: "owner-1", DelegateKind: PrincipalKindUser, DelegateID: "delegate-1", Scope: DelegationScopeCalendar, RequiredRole: DelegationRoleRead},
		{CompanyID: "company-1", OwnerKind: "calendar", OwnerID: "owner-1", DelegateKind: PrincipalKindUser, DelegateID: "delegate-1", Scope: DelegationScopeCalendar, RequiredRole: DelegationRoleRead},
		{CompanyID: "company-1", OwnerKind: PrincipalKindUser, OwnerID: "owner\n1", DelegateKind: PrincipalKindUser, DelegateID: "delegate-1", Scope: DelegationScopeCalendar, RequiredRole: DelegationRoleRead},
		{CompanyID: "company-1", OwnerKind: PrincipalKindUser, OwnerID: "owner-1", DelegateKind: "calendar", DelegateID: "delegate-1", Scope: DelegationScopeCalendar, RequiredRole: DelegationRoleRead},
		{CompanyID: "company-1", OwnerKind: PrincipalKindUser, OwnerID: "owner-1", DelegateKind: PrincipalKindUser, DelegateID: "delegate\n1", Scope: DelegationScopeCalendar, RequiredRole: DelegationRoleRead},
		{CompanyID: "company-1", OwnerKind: PrincipalKindUser, OwnerID: "owner-1", DelegateKind: PrincipalKindUser, DelegateID: "owner-1", Scope: DelegationScopeCalendar, RequiredRole: DelegationRoleRead},
		{CompanyID: "company-1", OwnerKind: PrincipalKindUser, OwnerID: "owner-1", DelegateKind: PrincipalKindUser, DelegateID: "delegate-1", Scope: "files", RequiredRole: DelegationRoleRead},
		{CompanyID: "company-1", OwnerKind: PrincipalKindUser, OwnerID: "owner-1", DelegateKind: PrincipalKindUser, DelegateID: "delegate-1", Scope: DelegationScopeCalendar, RequiredRole: "owner"},
		{CompanyID: "company-1", OwnerKind: PrincipalKindUser, OwnerID: "owner-1", DelegateKind: PrincipalKindUser, DelegateID: "delegate-1", Scope: DelegationScopeCalendar, RequiredRole: DelegationRoleRead, MaxDepth: -1},
		{CompanyID: "company-1", OwnerKind: PrincipalKindUser, OwnerID: "owner-1", DelegateKind: PrincipalKindUser, DelegateID: "delegate-1", Scope: DelegationScopeCalendar, RequiredRole: DelegationRoleRead, MaxDepth: MaxGroupMembershipDepth + 1},
	}
	for _, req := range tests {
		req := req
		t.Run(req.CompanyID+"/"+req.OwnerID+"/"+req.DelegateID, func(t *testing.T) {
			t.Parallel()

			if _, err := NormalizeCheckDelegationRequest(req); err == nil {
				t.Fatalf("NormalizeCheckDelegationRequest(%+v) error = nil, want rejection", req)
			}
		})
	}
}

func TestNormalizeListDelegationsRequest(t *testing.T) {
	t.Parallel()

	got, err := NormalizeListDelegationsRequest(ListDelegationsRequest{
		CompanyID:    " company-1 ",
		OwnerKind:    " Resource ",
		OwnerID:      " room-1 ",
		DelegateKind: " GROUP ",
		DelegateID:   " team-1 ",
		Scope:        " Calendar ",
		Role:         " WRITE ",
		ActiveOnly:   true,
		Limit:        25,
	})
	if err != nil {
		t.Fatalf("NormalizeListDelegationsRequest returned error: %v", err)
	}
	if got.CompanyID != "company-1" ||
		got.OwnerKind != PrincipalKindResource ||
		got.OwnerID != "room-1" ||
		got.DelegateKind != PrincipalKindGroup ||
		got.DelegateID != "team-1" ||
		got.Scope != DelegationScopeCalendar ||
		got.Role != DelegationRoleWrite ||
		!got.ActiveOnly ||
		got.Limit != 25 {
		t.Fatalf("request = %+v", got)
	}
}

func TestNormalizeListDelegationsRequestDefaultsLimit(t *testing.T) {
	t.Parallel()

	got, err := NormalizeListDelegationsRequest(ListDelegationsRequest{CompanyID: "company-1"})
	if err != nil {
		t.Fatalf("NormalizeListDelegationsRequest returned error: %v", err)
	}
	if got.Limit != DefaultDelegationListLimit {
		t.Fatalf("limit = %d, want %d", got.Limit, DefaultDelegationListLimit)
	}
}

func TestNormalizeListDelegationsRequestRejectsUnsafeInput(t *testing.T) {
	t.Parallel()

	tests := []ListDelegationsRequest{
		{OwnerKind: PrincipalKindUser},
		{CompanyID: "company\n1"},
		{CompanyID: "company-1", OwnerKind: "calendar"},
		{CompanyID: "company-1", OwnerID: "owner-1"},
		{CompanyID: "company-1", OwnerKind: PrincipalKindUser, OwnerID: "owner\n1"},
		{CompanyID: "company-1", DelegateKind: "calendar"},
		{CompanyID: "company-1", DelegateID: "delegate-1"},
		{CompanyID: "company-1", DelegateKind: PrincipalKindUser, DelegateID: "delegate\n1"},
		{CompanyID: "company-1", OwnerKind: PrincipalKindUser, OwnerID: "owner-1", DelegateKind: PrincipalKindUser, DelegateID: "owner-1"},
		{CompanyID: "company-1", Scope: "files"},
		{CompanyID: "company-1", Role: "owner"},
		{CompanyID: "company-1", Limit: -1},
		{CompanyID: "company-1", Limit: MaxDelegationListLimit + 1},
	}
	for _, req := range tests {
		req := req
		t.Run(req.CompanyID+"/"+req.OwnerID+"/"+req.DelegateID, func(t *testing.T) {
			t.Parallel()

			if _, err := NormalizeListDelegationsRequest(req); err == nil {
				t.Fatalf("NormalizeListDelegationsRequest(%+v) error = nil, want rejection", req)
			}
		})
	}
}

func TestDelegationRoleSatisfiesHierarchy(t *testing.T) {
	t.Parallel()

	tests := []struct {
		granted  string
		required string
		want     bool
	}{
		{granted: DelegationRoleRead, required: DelegationRoleRead, want: true},
		{granted: DelegationRoleWrite, required: DelegationRoleRead, want: true},
		{granted: DelegationRoleManage, required: DelegationRoleWrite, want: true},
		{granted: DelegationRoleRead, required: DelegationRoleWrite, want: false},
		{granted: DelegationRoleWrite, required: DelegationRoleManage, want: false},
		{granted: "owner", required: DelegationRoleRead, want: false},
	}
	for _, tc := range tests {
		tc := tc
		t.Run(tc.granted+"/"+tc.required, func(t *testing.T) {
			t.Parallel()

			if got := DelegationRoleSatisfies(tc.granted, tc.required); got != tc.want {
				t.Fatalf("DelegationRoleSatisfies(%q, %q) = %v, want %v", tc.granted, tc.required, got, tc.want)
			}
		})
	}
}

func TestRepositoryResolvePrincipalRequiresDatabase(t *testing.T) {
	t.Parallel()

	_, err := NewRepository(nil).ResolvePrincipal(context.Background(), ResolvePrincipalRequest{ID: "user-1"})
	if err == nil || !strings.Contains(err.Error(), "database handle is required") {
		t.Fatalf("error = %v, want database handle requirement", err)
	}
}

func TestRepositoryResolveAliasRequiresDatabase(t *testing.T) {
	t.Parallel()

	_, err := NewRepository(nil).ResolveAlias(context.Background(), ResolveAliasRequest{Address: "ops@example.com"})
	if err == nil || !strings.Contains(err.Error(), "database handle is required") {
		t.Fatalf("error = %v, want database handle requirement", err)
	}
}

func TestRepositoryListAliasesRequiresDatabase(t *testing.T) {
	t.Parallel()

	_, err := NewRepository(nil).ListAliases(context.Background(), ListAliasesRequest{CompanyID: "company-1"})
	if err == nil || !strings.Contains(err.Error(), "database handle is required") {
		t.Fatalf("error = %v, want database handle requirement", err)
	}
}

func TestRepositoryCheckDirectGroupMembershipRequiresDatabase(t *testing.T) {
	t.Parallel()

	_, err := NewRepository(nil).CheckDirectGroupMembership(context.Background(), CheckGroupMembershipRequest{
		GroupID:    "group-1",
		MemberKind: PrincipalKindUser,
		MemberID:   "user-1",
	})
	if err == nil || !strings.Contains(err.Error(), "database handle is required") {
		t.Fatalf("error = %v, want database handle requirement", err)
	}
}

func TestRepositoryListGroupMembershipsRequiresDatabase(t *testing.T) {
	t.Parallel()

	_, err := NewRepository(nil).ListGroupMemberships(context.Background(), ListGroupMembershipsRequest{
		CompanyID: "company-1",
	})
	if err == nil {
		t.Fatal("ListGroupMemberships error = nil, want database handle error")
	}
}

func TestRepositoryUpdateGroupMembershipRoleRequiresDatabase(t *testing.T) {
	t.Parallel()

	_, err := NewRepository(nil).UpdateGroupMembershipRoleWithAudit(context.Background(), UpdateGroupMembershipRoleRequest{
		ID:   "membership-1",
		Role: GroupMembershipRoleOwner,
	})
	if err == nil {
		t.Fatal("UpdateGroupMembershipRoleWithAudit error = nil, want database handle error")
	}
}

func TestRepositoryReassignGroupMembershipRequiresDatabase(t *testing.T) {
	t.Parallel()

	_, err := NewRepository(nil).ReassignGroupMembershipWithAudit(context.Background(), ReassignGroupMembershipRequest{
		ID:         "membership-1",
		GroupID:    "group-1",
		MemberKind: PrincipalKindUser,
		MemberID:   "user-1",
	})
	if err == nil {
		t.Fatal("ReassignGroupMembershipWithAudit error = nil, want database handle error")
	}
}

func TestRepositoryCheckEffectiveGroupMembershipRequiresDatabase(t *testing.T) {
	t.Parallel()

	_, err := NewRepository(nil).CheckEffectiveGroupMembership(context.Background(), CheckGroupMembershipRequest{
		GroupID:    "group-1",
		MemberKind: PrincipalKindUser,
		MemberID:   "user-1",
	})
	if err == nil || !strings.Contains(err.Error(), "database handle is required") {
		t.Fatalf("error = %v, want database handle requirement", err)
	}
}

func TestRepositoryCheckDelegationRequiresDatabase(t *testing.T) {
	t.Parallel()

	_, err := NewRepository(nil).CheckDelegation(context.Background(), CheckDelegationRequest{
		CompanyID:    "company-1",
		OwnerKind:    PrincipalKindResource,
		OwnerID:      "room-1",
		DelegateKind: PrincipalKindGroup,
		DelegateID:   "team-1",
		Scope:        DelegationScopeCalendar,
		RequiredRole: DelegationRoleRead,
	})
	if err == nil || !strings.Contains(err.Error(), "database handle is required") {
		t.Fatalf("error = %v, want database handle requirement", err)
	}
}

func TestRepositoryCheckEffectiveDelegationRequiresDatabase(t *testing.T) {
	t.Parallel()

	_, err := NewRepository(nil).CheckEffectiveDelegation(context.Background(), CheckDelegationRequest{
		CompanyID:    "company-1",
		OwnerKind:    PrincipalKindResource,
		OwnerID:      "room-1",
		DelegateKind: PrincipalKindUser,
		DelegateID:   "user-1",
		Scope:        DelegationScopeCalendar,
		RequiredRole: DelegationRoleRead,
	})
	if err == nil || !strings.Contains(err.Error(), "database handle is required") {
		t.Fatalf("error = %v, want database handle requirement", err)
	}
}

func TestRepositoryUpdateDelegationRoleRequiresDatabase(t *testing.T) {
	t.Parallel()

	_, err := NewRepository(nil).UpdateDelegationRoleWithAudit(context.Background(), UpdateDelegationRoleRequest{
		ID:   "delegation-1",
		Role: DelegationRoleManage,
	})
	if err == nil || !strings.Contains(err.Error(), "database handle is required") {
		t.Fatalf("error = %v, want database handle requirement", err)
	}
}

func TestRepositoryReassignDelegationRequiresDatabase(t *testing.T) {
	t.Parallel()

	_, err := NewRepository(nil).ReassignDelegationWithAudit(context.Background(), ReassignDelegationRequest{
		ID:           "delegation-1",
		OwnerKind:    PrincipalKindResource,
		OwnerID:      "room-2",
		DelegateKind: PrincipalKindUser,
		DelegateID:   "user-1",
		Scope:        DelegationScopeDrive,
	})
	if err == nil || !strings.Contains(err.Error(), "database handle is required") {
		t.Fatalf("error = %v, want database handle requirement", err)
	}
}

func TestRepositoryListDelegationsRequiresDatabase(t *testing.T) {
	t.Parallel()

	_, err := NewRepository(nil).ListDelegations(context.Background(), ListDelegationsRequest{
		CompanyID: "company-1",
	})
	if err == nil || !strings.Contains(err.Error(), "database handle is required") {
		t.Fatalf("error = %v, want database handle requirement", err)
	}
}
