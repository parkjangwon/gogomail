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

func TestResolveDirectoryLookupQueriesUseSargableActiveFilters(t *testing.T) {
	t.Parallel()

	aliasReq, err := NormalizeResolveAliasRequest(ResolveAliasRequest{
		Address:    " Ops@Example.com ",
		ActiveOnly: true,
	})
	if err != nil {
		t.Fatalf("NormalizeResolveAliasRequest returned error: %v", err)
	}
	aliasQuery, aliasArgs := buildResolveAliasQuery(aliasReq)
	for _, want := range []string{
		"FROM directory_aliases a",
		"WHERE lower(a.alias_address_ace) = $1",
		"AND (a.status = 'active' AND d.status = 'active' AND c.status = 'active')",
	} {
		if !strings.Contains(aliasQuery, want) {
			t.Fatalf("resolve alias query missing %q:\n%s", want, aliasQuery)
		}
	}
	for _, forbidden := range []string{
		"$2::boolean = false OR",
		"NULLIF($",
	} {
		if strings.Contains(aliasQuery, forbidden) {
			t.Fatalf("resolve alias query contains non-sargable active filter %q:\n%s", forbidden, aliasQuery)
		}
	}
	if len(aliasArgs) != 1 || aliasArgs[0] != "ops@example.com" {
		t.Fatalf("alias args = %#v, want normalized address only", aliasArgs)
	}

	inactiveAliasQuery, inactiveAliasArgs := buildResolveAliasQuery(ResolveAliasRequest{Address: "ops@example.com"})
	if strings.Contains(inactiveAliasQuery, "status = 'active'") {
		t.Fatalf("inactive alias query unexpectedly includes active predicate:\n%s", inactiveAliasQuery)
	}
	if len(inactiveAliasArgs) != 1 {
		t.Fatalf("inactive alias args len = %d, want 1", len(inactiveAliasArgs))
	}

	userQuery, userArgs := buildResolveUserByEmailQuery(ResolveUserByEmailRequest{
		Email:      "user@example.com",
		ActiveOnly: true,
	})
	for _, want := range []string{
		"JOIN user_addresses a ON a.user_id = u.id AND lower(a.address) = lower($1)",
		"WHERE (u.status = 'active' AND d.status = 'active' AND c.status = 'active')",
	} {
		if !strings.Contains(userQuery, want) {
			t.Fatalf("resolve user query missing %q:\n%s", want, userQuery)
		}
	}
	if strings.Contains(userQuery, "$2::boolean = false OR") {
		t.Fatalf("resolve user query contains non-sargable active filter:\n%s", userQuery)
	}
	if len(userArgs) != 1 || userArgs[0] != "user@example.com" {
		t.Fatalf("user args = %#v, want email only", userArgs)
	}

	inactiveUserQuery, inactiveUserArgs := buildResolveUserByEmailQuery(ResolveUserByEmailRequest{Email: "user@example.com"})
	if strings.Contains(inactiveUserQuery, "WHERE") {
		t.Fatalf("inactive user query unexpectedly includes WHERE:\n%s", inactiveUserQuery)
	}
	if len(inactiveUserArgs) != 1 {
		t.Fatalf("inactive user args len = %d, want 1", len(inactiveUserArgs))
	}

	batchUserQuery := buildResolveUsersByEmailsQuery(true)
	for _, want := range []string{
		"FROM unnest($1::text[]) WITH ORDINALITY AS req(email, email_order)",
		"JOIN user_addresses lookup ON lower(lookup.address) = lower(req.email)",
		"LEFT JOIN user_addresses primary_addr ON primary_addr.user_id = u.id AND primary_addr.is_primary = true",
		"WHERE (u.status = 'active' AND d.status = 'active' AND c.status = 'active')",
		"ORDER BY req.email_order",
	} {
		if !strings.Contains(batchUserQuery, want) {
			t.Fatalf("batch resolve user query missing %q:\n%s", want, batchUserQuery)
		}
	}
	for _, forbidden := range []string{
		"array_position",
		"SELECT *",
		"$1 = '' OR",
	} {
		if strings.Contains(batchUserQuery, forbidden) {
			t.Fatalf("batch resolve user query contains forbidden pattern %q:\n%s", forbidden, batchUserQuery)
		}
	}
}

func TestResolvePrincipalQueriesUseSargableActiveFilters(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		kind       string
		activeWant string
		build      func(ResolvePrincipalRequest) (string, []any)
	}{
		{
			name:       "user",
			kind:       PrincipalKindUser,
			activeWant: "AND (u.status = 'active' AND d.status = 'active' AND c.status = 'active')",
			build:      buildResolveUserPrincipalQuery,
		},
		{
			name:       "organization",
			kind:       PrincipalKindOrganization,
			activeWant: "AND (o.status = 'active' AND d.status = 'active' AND c.status = 'active')",
			build:      buildResolveOrganizationPrincipalQuery,
		},
		{
			name:       "group",
			kind:       PrincipalKindGroup,
			activeWant: "AND (g.status = 'active' AND d.status = 'active' AND c.status = 'active')",
			build:      buildResolveGroupPrincipalQuery,
		},
		{
			name:       "resource",
			kind:       PrincipalKindResource,
			activeWant: "AND (rsrc.status = 'active' AND d.status = 'active' AND c.status = 'active')",
			build:      buildResolveResourcePrincipalQuery,
		},
	}
	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			req, err := NormalizeResolvePrincipalRequest(ResolvePrincipalRequest{
				ID:         " principal-1 ",
				Kind:       tc.kind,
				ActiveOnly: true,
			})
			if err != nil {
				t.Fatalf("NormalizeResolvePrincipalRequest returned error: %v", err)
			}
			query, args := tc.build(req)
			for _, want := range []string{
				"WHERE ",
				"= $1::uuid",
				tc.activeWant,
			} {
				if !strings.Contains(query, want) {
					t.Fatalf("resolve %s query missing %q:\n%s", tc.name, want, query)
				}
			}
			if strings.Contains(query, "$2::boolean = false OR") {
				t.Fatalf("resolve %s query contains non-sargable active filter:\n%s", tc.name, query)
			}
			if len(args) != 1 || args[0] != "principal-1" {
				t.Fatalf("resolve %s args = %#v, want normalized id only", tc.name, args)
			}

			inactiveQuery, inactiveArgs := tc.build(ResolvePrincipalRequest{ID: "principal-1", Kind: tc.kind})
			if strings.Contains(inactiveQuery, "status = 'active'") {
				t.Fatalf("inactive resolve %s query unexpectedly includes active predicate:\n%s", tc.name, inactiveQuery)
			}
			if len(inactiveArgs) != 1 {
				t.Fatalf("inactive resolve %s args len = %d, want 1", tc.name, len(inactiveArgs))
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

func TestListAliasesQueryUsesSargableOptionalFilters(t *testing.T) {
	t.Parallel()

	req, err := NormalizeListAliasesRequest(ListAliasesRequest{
		CompanyID:  " company-1 ",
		DomainID:   " domain-1 ",
		TargetKind: " GROUP ",
		TargetID:   " group-1 ",
		Query:      " Ops_Alias ",
		ActiveOnly: true,
		Limit:      25,
	})
	if err != nil {
		t.Fatalf("NormalizeListAliasesRequest returned error: %v", err)
	}
	query, args := buildListAliasesQuery(req)
	for _, want := range []string{
		"FROM directory_aliases a",
		"WHERE a.company_id = $1::uuid",
		"AND a.domain_id = $2::uuid",
		"AND a.target_kind = $3",
		"AND a.target_id = $4::uuid",
		"lower(a.alias_address) LIKE $5 ESCAPE",
		"AND (a.status = 'active' AND d.status = 'active' AND c.status = 'active')",
		"ORDER BY lower(a.alias_address_ace), a.id",
		"LIMIT $6",
	} {
		if !strings.Contains(query, want) {
			t.Fatalf("list aliases query missing %q:\n%s", want, query)
		}
	}
	for _, forbidden := range []string{
		"$2 = '' OR",
		"NULLIF($",
		"$6::boolean = false OR",
	} {
		if strings.Contains(query, forbidden) {
			t.Fatalf("list aliases query contains non-sargable optional filter %q:\n%s", forbidden, query)
		}
	}
	if len(args) != 6 {
		t.Fatalf("args len = %d, want 6", len(args))
	}
	if args[4] != `%ops\_alias%` {
		t.Fatalf("query arg = %#v, want escaped alias pattern", args[4])
	}
	if args[5] != 25 {
		t.Fatalf("limit arg = %#v, want 25", args[5])
	}
}

func TestListOrgTreeQueryUsesSargableDomainFilter(t *testing.T) {
	t.Parallel()

	query, args := buildListOrgTreeQuery(" company-1 ", " domain-1 ")
	for _, want := range []string{
		"FROM organizations o",
		"WHERE c.id = $1::uuid",
		"AND o.status = 'active'",
		"AND d.id = $2::uuid",
		"ORDER BY o.depth, o.order_index, lower(o.name)",
	} {
		if !strings.Contains(query, want) {
			t.Fatalf("list org tree query missing %q:\n%s", want, query)
		}
	}
	for _, forbidden := range []string{
		"$2 = '' OR",
		"NULLIF($2, '')",
	} {
		if strings.Contains(query, forbidden) {
			t.Fatalf("list org tree query contains non-sargable optional filter %q:\n%s", forbidden, query)
		}
	}
	if len(args) != 2 {
		t.Fatalf("args len = %d, want 2", len(args))
	}
	if args[0] != "company-1" || args[1] != "domain-1" {
		t.Fatalf("args = %#v, want trimmed company/domain", args)
	}

	query, args = buildListOrgTreeQuery("company-1", "")
	if strings.Contains(query, "d.id = $2::uuid") {
		t.Fatalf("empty-domain query unexpectedly includes domain predicate:\n%s", query)
	}
	if len(args) != 1 {
		t.Fatalf("empty-domain args len = %d, want 1", len(args))
	}
}

func TestListOrgMembersByOrgIDsQueryUsesSingleOrdinalityBatchLookup(t *testing.T) {
	t.Parallel()

	query := buildListOrgMembersByOrgIDsQuery("domain-1")
	for _, want := range []string{
		"FROM unnest($3::text[]) WITH ORDINALITY AS req(org_id, org_order)",
		"JOIN organizations o ON o.id = req.org_id",
		"JOIN users u ON u.org_id = o.id AND u.domain_id = d.id",
		"LEFT JOIN user_addresses primary_addr ON primary_addr.user_id = u.id AND primary_addr.is_primary = true",
		"WHERE c.id = $1::uuid",
		"AND d.id = $2::uuid",
		"row_number() OVER (PARTITION BY req.org_id ORDER BY lower(u.display_name), u.id)",
		"WHERE member_rank <= $4",
		"ORDER BY org_order, member_rank",
	} {
		if !strings.Contains(query, want) {
			t.Fatalf("org members query missing %q:\n%s", want, query)
		}
	}
	for _, forbidden := range []string{
		"array_position",
		"SELECT *",
		"$1 = '' OR",
	} {
		if strings.Contains(query, forbidden) {
			t.Fatalf("org members query contains forbidden pattern %q:\n%s", forbidden, query)
		}
	}

	query = buildListOrgMembersByOrgIDsQuery("")
	if strings.Contains(query, "AND d.id = $2::uuid") {
		t.Fatalf("empty-domain org members query unexpectedly includes domain predicate:\n%s", query)
	}
	if !strings.Contains(query, "FROM unnest($2::text[]) WITH ORDINALITY AS req(org_id, org_order)") ||
		!strings.Contains(query, "WHERE member_rank <= $3") {
		t.Fatalf("empty-domain org members query uses wrong argument positions:\n%s", query)
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

func TestSearchPrincipalsQueryUsesSargableOptionalFilters(t *testing.T) {
	t.Parallel()

	req, err := NormalizeSearchPrincipalsRequest(SearchPrincipalsRequest{
		CompanyID:      " company-1 ",
		DomainID:       " domain-1 ",
		OrganizationID: " org-1 ",
		Kinds:          []string{" USER ", "resource"},
		Query:          ` A_%\ `,
		ActiveOnly:     true,
		Limit:          25,
		Offset:         50,
	})
	if err != nil {
		t.Fatalf("NormalizeSearchPrincipalsRequest returned error: %v", err)
	}
	query, args := buildSearchPrincipalsQuery(req)
	for _, want := range []string{
		"FROM users u",
		"FROM directory_resources rsrc",
		"WHERE c.id = $1::uuid",
		"AND d.id = $2::uuid",
		"AND u.org_id = $3::uuid",
		"AND rsrc.org_id = $3::uuid",
		"LIKE $4 ESCAPE",
		"AND (u.status = 'active' AND d.status = 'active' AND c.status = 'active')",
		"AND (rsrc.status = 'active' AND d.status = 'active' AND c.status = 'active')",
		"ORDER BY kind_rank, lower(display_name), id",
		"LIMIT $5 OFFSET $6",
	} {
		if !strings.Contains(query, want) {
			t.Fatalf("search principals query missing %q:\n%s", want, query)
		}
	}
	for _, unexpected := range []string{
		"FROM organizations o",
		"FROM directory_groups g",
		"$7::boolean",
		"NULLIF($",
		" = '' OR",
		"COALESCE(u.org_id::text, '') =",
		"COALESCE(rsrc.org_id::text, '') =",
	} {
		if strings.Contains(query, unexpected) {
			t.Fatalf("search principals query unexpectedly includes %q:\n%s", unexpected, query)
		}
	}
	if len(args) != 6 {
		t.Fatalf("args len = %d, want 6", len(args))
	}
	if args[0] != "company-1" || args[1] != "domain-1" || args[2] != "org-1" || args[3] != `%a\_\%\\%` || args[4] != 25 || args[5] != 50 {
		t.Fatalf("args = %#v", args)
	}

	query, args = buildSearchPrincipalsQuery(SearchPrincipalsRequest{
		CompanyID:  "company-1",
		Kinds:      []string{PrincipalKindGroup},
		Limit:      10,
		Offset:     0,
		ActiveOnly: false,
	})
	if !strings.Contains(query, "FROM directory_groups g") {
		t.Fatalf("group-only query missing group branch:\n%s", query)
	}
	for _, unexpected := range []string{
		"FROM users u",
		"FROM organizations o",
		"FROM directory_resources rsrc",
		"status = 'active'",
		"domain_id = $",
		"org_id = $",
		"LIKE $",
	} {
		if strings.Contains(query, unexpected) {
			t.Fatalf("group-only query unexpectedly includes %q:\n%s", unexpected, query)
		}
	}
	if len(args) != 3 {
		t.Fatalf("group-only args len = %d, want 3", len(args))
	}
	if args[0] != "company-1" || args[1] != 10 || args[2] != 0 {
		t.Fatalf("group-only args = %#v", args)
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

func TestGroupMembershipCheckQueriesUseSargableActiveFilters(t *testing.T) {
	t.Parallel()

	req, err := NormalizeCheckGroupMembershipRequest(CheckGroupMembershipRequest{
		GroupID:    " group-1 ",
		MemberKind: " USER ",
		MemberID:   " user-1 ",
		ActiveOnly: true,
		MaxDepth:   5,
	})
	if err != nil {
		t.Fatalf("NormalizeCheckGroupMembershipRequest returned error: %v", err)
	}
	directQuery, directArgs := buildCheckDirectGroupMembershipQuery(req)
	for _, want := range []string{
		"m.group_id = $1::uuid",
		"AND m.member_kind = $2",
		"AND m.member_id = $3::uuid",
		"AND (m.status = 'active' AND g.status = 'active' AND d.status = 'active' AND c.status = 'active')",
	} {
		if !strings.Contains(directQuery, want) {
			t.Fatalf("direct membership query missing %q:\n%s", want, directQuery)
		}
	}
	if strings.Contains(directQuery, "$4::boolean = false OR") {
		t.Fatalf("direct membership query contains non-sargable active filter:\n%s", directQuery)
	}
	if len(directArgs) != 3 {
		t.Fatalf("direct args len = %d, want 3", len(directArgs))
	}

	effectiveQuery, effectiveArgs := buildCheckEffectiveGroupMembershipQuery(req)
	for _, want := range []string{
		"group_tree.depth < $4",
		"AND (m.status = 'active' AND nested_group.status = 'active' AND d.status = 'active' AND c.status = 'active')",
		"AND (m.status = 'active' AND owning_group.status = 'active' AND d.status = 'active' AND c.status = 'active')",
	} {
		if !strings.Contains(effectiveQuery, want) {
			t.Fatalf("effective membership query missing %q:\n%s", want, effectiveQuery)
		}
	}
	if strings.Contains(effectiveQuery, "$4::boolean = false OR") ||
		strings.Contains(effectiveQuery, "group_tree.depth < $5") {
		t.Fatalf("effective membership query contains stale optional-filter placeholders:\n%s", effectiveQuery)
	}
	if len(effectiveArgs) != 4 || effectiveArgs[3] != 5 {
		t.Fatalf("effective args = %#v, want explicit depth as fourth arg", effectiveArgs)
	}

	inactiveDirectQuery, _ := buildCheckDirectGroupMembershipQuery(CheckGroupMembershipRequest{
		GroupID: "group-1", MemberKind: PrincipalKindUser, MemberID: "user-1",
	})
	if strings.Contains(inactiveDirectQuery, "status = 'active'") {
		t.Fatalf("inactive direct membership query unexpectedly includes active predicate:\n%s", inactiveDirectQuery)
	}
	inactiveEffectiveQuery, inactiveEffectiveArgs := buildCheckEffectiveGroupMembershipQuery(CheckGroupMembershipRequest{
		GroupID: "group-1", MemberKind: PrincipalKindUser, MemberID: "user-1", MaxDepth: 3,
	})
	if strings.Contains(inactiveEffectiveQuery, "status = 'active'") {
		t.Fatalf("inactive effective membership query unexpectedly includes active predicate:\n%s", inactiveEffectiveQuery)
	}
	if len(inactiveEffectiveArgs) != 4 || inactiveEffectiveArgs[3] != 3 {
		t.Fatalf("inactive effective args = %#v, want explicit depth", inactiveEffectiveArgs)
	}
}

func TestGroupMembershipExcludingQueryUsesSargableActiveFilters(t *testing.T) {
	t.Parallel()

	req, err := NormalizeCheckGroupMembershipRequest(CheckGroupMembershipRequest{
		GroupID:    " group-1 ",
		MemberKind: " Group ",
		MemberID:   " child-1 ",
		ActiveOnly: true,
		MaxDepth:   7,
	})
	if err != nil {
		t.Fatalf("NormalizeCheckGroupMembershipRequest returned error: %v", err)
	}
	query, args := buildCheckEffectiveGroupMembershipExcludingQuery(req, "membership-1")
	for _, want := range []string{
		"m.id <> $5::uuid",
		"group_tree.depth < $4",
		"AND (m.status = 'active' AND nested_group.status = 'active' AND d.status = 'active' AND c.status = 'active')",
		"AND (m.status = 'active' AND owning_group.status = 'active' AND d.status = 'active' AND c.status = 'active')",
	} {
		if !strings.Contains(query, want) {
			t.Fatalf("excluding effective membership query missing %q:\n%s", want, query)
		}
	}
	if strings.Contains(query, "$4::boolean = false OR") ||
		strings.Contains(query, "$7::boolean = false OR") ||
		strings.Contains(query, "group_tree.depth < $5") {
		t.Fatalf("excluding effective membership query contains stale optional-filter placeholders:\n%s", query)
	}
	if len(args) != 5 || args[3] != 7 || args[4] != "membership-1" {
		t.Fatalf("args = %#v, want explicit depth and excluded membership id", args)
	}

	inactiveQuery, inactiveArgs := buildCheckEffectiveGroupMembershipExcludingQuery(CheckGroupMembershipRequest{
		GroupID: "group-1", MemberKind: PrincipalKindGroup, MemberID: "child-1", MaxDepth: 4,
	}, "membership-1")
	if strings.Contains(inactiveQuery, "status = 'active'") {
		t.Fatalf("inactive excluding effective membership query unexpectedly includes active predicate:\n%s", inactiveQuery)
	}
	if len(inactiveArgs) != 5 || inactiveArgs[3] != 4 {
		t.Fatalf("inactive args = %#v, want explicit depth", inactiveArgs)
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

func TestListGroupMembershipsQueryUsesSargableOptionalFilters(t *testing.T) {
	t.Parallel()

	req, err := NormalizeListGroupMembershipsRequest(ListGroupMembershipsRequest{
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
	query, args := buildListGroupMembershipsQuery(req)
	for _, want := range []string{
		"FROM directory_group_memberships m",
		"WHERE g.company_id = $1::uuid",
		"AND m.group_id = $2::uuid",
		"AND m.member_kind = $3",
		"AND m.member_id = $4::uuid",
		"AND m.role = $5",
		"AND (m.status = 'active' AND g.status = 'active' AND d.status = 'active' AND c.status = 'active')",
		"ORDER BY m.updated_at DESC, m.id",
		"LIMIT $6",
	} {
		if !strings.Contains(query, want) {
			t.Fatalf("list group memberships query missing %q:\n%s", want, query)
		}
	}
	for _, forbidden := range []string{
		"$2 = '' OR",
		"NULLIF($",
		"$6::boolean = false OR",
	} {
		if strings.Contains(query, forbidden) {
			t.Fatalf("list group memberships query contains non-sargable optional filter %q:\n%s", forbidden, query)
		}
	}
	if len(args) != 6 {
		t.Fatalf("args len = %d, want 6", len(args))
	}
	if args[0] != "company-1" || args[5] != 25 {
		t.Fatalf("args = %#v, want company and limit preserved", args)
	}

	query, args = buildListGroupMembershipsQuery(ListGroupMembershipsRequest{CompanyID: "company-1", Limit: 50})
	for _, unexpected := range []string{
		"m.group_id = $2::uuid",
		"m.member_kind = $",
		"m.member_id = $",
		"m.role = $",
		"status = 'active'",
	} {
		if strings.Contains(query, unexpected) {
			t.Fatalf("minimal list group memberships query unexpectedly contains %q:\n%s", unexpected, query)
		}
	}
	if len(args) != 2 {
		t.Fatalf("minimal args len = %d, want 2", len(args))
	}
}

func TestBatchGroupMembershipQueriesUseOrdinalityAndPerPrincipalLimits(t *testing.T) {
	t.Parallel()

	groupQuery := buildListGroupMembershipsForGroupsQuery(true)
	for _, want := range []string{
		"JOIN unnest($2::uuid[]) WITH ORDINALITY AS req(group_id, group_order)",
		"ON req.group_id = m.group_id",
		"WHERE g.company_id = $1::uuid",
		"row_number() OVER (PARTITION BY group_id ORDER BY updated_at DESC, id) AS rn",
		"WHERE rn <= $3",
		"ORDER BY group_order, rn",
		"AND (m.status = 'active' AND g.status = 'active' AND d.status = 'active' AND c.status = 'active')",
	} {
		if !strings.Contains(groupQuery, want) {
			t.Fatalf("batch group membership query missing %q:\n%s", want, groupQuery)
		}
	}
	for _, forbidden := range []string{
		"$2 = '' OR",
		"array_position",
		"SELECT *",
	} {
		if strings.Contains(groupQuery, forbidden) {
			t.Fatalf("batch group membership query contains forbidden pattern %q:\n%s", forbidden, groupQuery)
		}
	}

	memberQuery := buildListGroupMembershipsForMembersQuery(true)
	for _, want := range []string{
		"JOIN unnest($2::text[], $3::uuid[]) WITH ORDINALITY AS req(member_kind, member_id, member_order)",
		"ON req.member_kind = m.member_kind AND req.member_id = m.member_id",
		"WHERE g.company_id = $1::uuid",
		"row_number() OVER (PARTITION BY member_kind, member_id ORDER BY updated_at DESC, id) AS rn",
		"WHERE rn <= $4",
		"ORDER BY member_order, rn",
		"AND (m.status = 'active' AND g.status = 'active' AND d.status = 'active' AND c.status = 'active')",
	} {
		if !strings.Contains(memberQuery, want) {
			t.Fatalf("batch member membership query missing %q:\n%s", want, memberQuery)
		}
	}
	for _, forbidden := range []string{
		"$3 = '' OR",
		"array_position",
		"SELECT *",
	} {
		if strings.Contains(memberQuery, forbidden) {
			t.Fatalf("batch member membership query contains forbidden pattern %q:\n%s", forbidden, memberQuery)
		}
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

func TestCheckDelegationQueryUsesSargableActiveFilter(t *testing.T) {
	t.Parallel()

	req, err := NormalizeCheckDelegationRequest(CheckDelegationRequest{
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
	query, args := buildCheckDelegationQuery(req)
	for _, want := range []string{
		"FROM directory_delegations d",
		"WHERE d.company_id = $1::uuid",
		"AND d.owner_kind = $2",
		"AND d.owner_id = $3::uuid",
		"AND d.delegate_kind = $4",
		"AND d.delegate_id = $5::uuid",
		"AND d.scope = $6",
		"AND (d.status = 'active' AND c.status = 'active')",
	} {
		if !strings.Contains(query, want) {
			t.Fatalf("check delegation query missing %q:\n%s", want, query)
		}
	}
	if strings.Contains(query, "$7::boolean = false OR") {
		t.Fatalf("check delegation query contains non-sargable active filter:\n%s", query)
	}
	if len(args) != 6 {
		t.Fatalf("args len = %d, want 6", len(args))
	}

	inactiveQuery, inactiveArgs := buildCheckDelegationQuery(CheckDelegationRequest{
		CompanyID:    "company-1",
		OwnerKind:    PrincipalKindResource,
		OwnerID:      "room-1",
		DelegateKind: PrincipalKindGroup,
		DelegateID:   "team-1",
		Scope:        DelegationScopeCalendar,
	})
	if strings.Contains(inactiveQuery, "status = 'active'") {
		t.Fatalf("inactive check delegation query unexpectedly includes active predicate:\n%s", inactiveQuery)
	}
	if len(inactiveArgs) != 6 {
		t.Fatalf("inactive args len = %d, want 6", len(inactiveArgs))
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

func TestListDelegationsQueryUsesSargableOptionalFilters(t *testing.T) {
	t.Parallel()

	req, err := NormalizeListDelegationsRequest(ListDelegationsRequest{
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
	query, args := buildListDelegationsQuery(req)
	for _, want := range []string{
		"FROM directory_delegations",
		"WHERE company_id = $1::uuid",
		"AND owner_kind = $2",
		"AND owner_id = $3::uuid",
		"AND delegate_kind = $4",
		"AND delegate_id = $5::uuid",
		"AND scope = $6",
		"AND role = $7",
		"AND status = 'active'",
		"ORDER BY updated_at DESC, id",
		"LIMIT $8",
	} {
		if !strings.Contains(query, want) {
			t.Fatalf("list delegation query missing %q:\n%s", want, query)
		}
	}
	for _, forbidden := range []string{
		"$2 = '' OR",
		"NULLIF($",
		"$8::boolean = false OR",
	} {
		if strings.Contains(query, forbidden) {
			t.Fatalf("list delegation query contains non-sargable optional filter %q:\n%s", forbidden, query)
		}
	}
	if len(args) != 8 {
		t.Fatalf("args len = %d, want 8", len(args))
	}
	if args[0] != "company-1" || args[7] != 25 {
		t.Fatalf("args = %#v", args)
	}

	query, args = buildListDelegationsQuery(ListDelegationsRequest{
		CompanyID: "company-1",
		Limit:     50,
	})
	for _, unexpected := range []string{
		"owner_kind = $",
		"owner_id = $",
		"delegate_kind = $",
		"delegate_id = $",
		"scope = $",
		"role = $",
		"status = 'active'",
	} {
		if strings.Contains(query, unexpected) {
			t.Fatalf("list delegation query unexpectedly includes %q:\n%s", unexpected, query)
		}
	}
	if len(args) != 2 {
		t.Fatalf("args len = %d, want 2", len(args))
	}
	if args[0] != "company-1" || args[1] != 50 {
		t.Fatalf("args = %#v", args)
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

func TestRepositoryResolveUserByEmailRequiresDatabase(t *testing.T) {
	t.Parallel()

	_, err := NewRepository(nil).ResolveUserByEmail(context.Background(), ResolveUserByEmailRequest{Email: "user@example.com"})
	if err == nil || !strings.Contains(err.Error(), "database handle is required") {
		t.Fatalf("error = %v, want database handle requirement", err)
	}
}

func TestRepositoryResolveUserByEmailRejectsEmptyEmail(t *testing.T) {
	t.Parallel()

	repo := NewRepository(nil)
	ctx := context.Background()
	_, err := repo.ResolveUserByEmail(ctx, ResolveUserByEmailRequest{Email: ""})
	if err == nil {
		t.Fatal("error = nil, want email is required")
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
