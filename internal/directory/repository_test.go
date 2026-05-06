package directory

import (
	"context"
	"strings"
	"testing"
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
		!got.ActiveOnly {
		t.Fatalf("request = %+v", got)
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
