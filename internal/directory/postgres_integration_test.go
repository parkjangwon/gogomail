package directory

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gogomail/gogomail/internal/database"
	_ "github.com/jackc/pgx/v5/stdlib"
)

func TestPostgresCheckEffectiveDelegationExpandsGroupDelegates(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	db := openDirectoryPostgresTestDB(t)
	seed := seedDirectoryDelegationGraph(t, db)
	repo := NewRepository(db)

	got, err := repo.CheckEffectiveDelegation(ctx, CheckDelegationRequest{
		CompanyID:    seed.companyID,
		OwnerKind:    PrincipalKindResource,
		OwnerID:      seed.roomID,
		DelegateKind: PrincipalKindUser,
		DelegateID:   seed.aliceID,
		Scope:        DelegationScopeCalendar,
		RequiredRole: DelegationRoleRead,
		ActiveOnly:   true,
	})
	if err != nil {
		t.Fatalf("CheckEffectiveDelegation direct returned error: %v", err)
	}
	if !got {
		t.Fatal("direct user delegation was not satisfied")
	}

	got, err = repo.CheckEffectiveDelegation(ctx, CheckDelegationRequest{
		CompanyID:    seed.companyID,
		OwnerKind:    PrincipalKindResource,
		OwnerID:      seed.roomID,
		DelegateKind: PrincipalKindUser,
		DelegateID:   seed.bobID,
		Scope:        DelegationScopeCalendar,
		RequiredRole: DelegationRoleRead,
		ActiveOnly:   true,
		MaxDepth:     2,
	})
	if err != nil {
		t.Fatalf("CheckEffectiveDelegation nested returned error: %v", err)
	}
	if !got {
		t.Fatal("nested group delegation did not satisfy user delegate")
	}

	got, err = repo.CheckEffectiveDelegation(ctx, CheckDelegationRequest{
		CompanyID:    seed.companyID,
		OwnerKind:    PrincipalKindResource,
		OwnerID:      seed.roomID,
		DelegateKind: PrincipalKindUser,
		DelegateID:   seed.bobID,
		Scope:        DelegationScopeCalendar,
		RequiredRole: DelegationRoleWrite,
		ActiveOnly:   true,
		MaxDepth:     2,
	})
	if err != nil {
		t.Fatalf("CheckEffectiveDelegation write returned error: %v", err)
	}
	if !got {
		t.Fatal("write group delegation did not satisfy write requirement")
	}

	got, err = repo.CheckEffectiveDelegation(ctx, CheckDelegationRequest{
		CompanyID:    seed.companyID,
		OwnerKind:    PrincipalKindResource,
		OwnerID:      seed.roomID,
		DelegateKind: PrincipalKindUser,
		DelegateID:   seed.bobID,
		Scope:        DelegationScopeCalendar,
		RequiredRole: DelegationRoleManage,
		ActiveOnly:   true,
		MaxDepth:     2,
	})
	if err != nil {
		t.Fatalf("CheckEffectiveDelegation manage returned error: %v", err)
	}
	if got {
		t.Fatal("write group delegation satisfied manage requirement")
	}

	for _, tc := range []struct {
		name string
		kind string
		id   string
	}{
		{name: "organization", kind: PrincipalKindOrganization, id: seed.orgID},
		{name: "group", kind: PrincipalKindGroup, id: seed.deeperID},
		{name: "resource", kind: PrincipalKindResource, id: seed.equipmentID},
	} {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			got, err := repo.CheckEffectiveDelegation(ctx, CheckDelegationRequest{
				CompanyID:    seed.companyID,
				OwnerKind:    PrincipalKindResource,
				OwnerID:      seed.roomID,
				DelegateKind: tc.kind,
				DelegateID:   tc.id,
				Scope:        DelegationScopeCalendar,
				RequiredRole: DelegationRoleRead,
				ActiveOnly:   true,
				MaxDepth:     2,
			})
			if err != nil {
				t.Fatalf("CheckEffectiveDelegation returned error: %v", err)
			}
			if !got {
				t.Fatalf("%s delegate did not satisfy effective group delegation", tc.kind)
			}
		})
	}

	got, err = repo.CheckEffectiveDelegation(ctx, CheckDelegationRequest{
		CompanyID:    seed.companyID,
		OwnerKind:    PrincipalKindResource,
		OwnerID:      seed.roomID,
		DelegateKind: PrincipalKindUser,
		DelegateID:   seed.bobID,
		Scope:        DelegationScopeCalendar,
		RequiredRole: DelegationRoleRead,
		ActiveOnly:   true,
		MaxDepth:     1,
	})
	if err != nil {
		t.Fatalf("CheckEffectiveDelegation shallow returned error: %v", err)
	}
	if got {
		t.Fatal("nested group delegation ignored requested depth cap")
	}

	if _, err := db.ExecContext(ctx, `UPDATE users SET status = 'suspended' WHERE id = $1::uuid`, seed.bobID); err != nil {
		t.Fatalf("suspend nested delegate user: %v", err)
	}
	got, err = repo.CheckEffectiveDelegation(ctx, CheckDelegationRequest{
		CompanyID:    seed.companyID,
		OwnerKind:    PrincipalKindResource,
		OwnerID:      seed.roomID,
		DelegateKind: PrincipalKindUser,
		DelegateID:   seed.bobID,
		Scope:        DelegationScopeCalendar,
		RequiredRole: DelegationRoleRead,
		ActiveOnly:   true,
		MaxDepth:     2,
	})
	if err != nil {
		t.Fatalf("CheckEffectiveDelegation inactive delegate returned error: %v", err)
	}
	if got {
		t.Fatal("inactive delegate principal satisfied effective delegation")
	}
}

func TestPostgresSearchPrincipalsFindsUsersResourcesAndGroups(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	db := openDirectoryPostgresTestDB(t)
	seed := seedDirectoryDelegationGraph(t, db)
	repo := NewRepository(db)

	principals, err := repo.SearchPrincipals(ctx, SearchPrincipalsRequest{
		CompanyID:  seed.companyID,
		Query:      "one",
		ActiveOnly: true,
		Kinds:      []string{PrincipalKindResource},
		Limit:      10,
	})
	if err != nil {
		t.Fatalf("SearchPrincipals resource returned error: %v", err)
	}
	if got, want := principalIDsByKind(principals, PrincipalKindResource), []string{seed.roomID, seed.equipmentID}; !sameStringSet(got, want) {
		t.Fatalf("resource search ids = %#v, want %#v", got, want)
	}

	principals, err = repo.SearchPrincipals(ctx, SearchPrincipalsRequest{
		CompanyID:      seed.companyID,
		OrganizationID: seed.orgID,
		ActiveOnly:     true,
		Kinds:          []string{PrincipalKindOrganization},
	})
	if err != nil {
		t.Fatalf("SearchPrincipals organization returned error: %v", err)
	}
	if len(principals) != 1 || principals[0].ID != seed.orgID || principals[0].Kind != PrincipalKindOrganization {
		t.Fatalf("organization-scoped search = %+v", principals)
	}

	if _, err := db.ExecContext(ctx, `UPDATE directory_groups SET status = 'suspended' WHERE id = $1::uuid`, seed.teamID); err != nil {
		t.Fatalf("suspend directory group: %v", err)
	}
	principals, err = repo.SearchPrincipals(ctx, SearchPrincipalsRequest{
		CompanyID:  seed.companyID,
		Query:      "team",
		ActiveOnly: true,
		Kinds:      []string{PrincipalKindGroup},
	})
	if err != nil {
		t.Fatalf("SearchPrincipals active group returned error: %v", err)
	}
	if len(principals) != 0 {
		t.Fatalf("active group search returned suspended principals: %+v", principals)
	}
}

func TestPostgresListAliasesFiltersTargetDomainAndQuery(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	db := openDirectoryPostgresTestDB(t)
	seed := seedDirectoryDelegationGraph(t, db)
	repo := NewRepository(db)

	if _, err := db.ExecContext(ctx, `
INSERT INTO directory_aliases (company_id, domain_id, alias_address, alias_address_ace, target_kind, target_id)
VALUES
  ($1::uuid, $2::uuid, 'ops@example.com', 'ops@example.com', 'group', $3::uuid),
  ($1::uuid, $2::uuid, 'projector@example.com', 'projector@example.com', 'resource', $4::uuid),
  ($1::uuid, $2::uuid, 'old-ops@example.com', 'old-ops@example.com', 'group', $3::uuid)`,
		seed.companyID, seed.domainID, seed.teamID, seed.equipmentID); err != nil {
		t.Fatalf("seed directory aliases: %v", err)
	}
	if _, err := db.ExecContext(ctx, `UPDATE directory_aliases SET status = 'deleted' WHERE alias_address_ace = 'old-ops@example.com'`); err != nil {
		t.Fatalf("delete old alias: %v", err)
	}

	aliases, err := repo.ListAliases(ctx, ListAliasesRequest{
		CompanyID:  seed.companyID,
		DomainID:   seed.domainID,
		TargetKind: PrincipalKindGroup,
		TargetID:   seed.teamID,
		Query:      "ops",
		ActiveOnly: true,
		Limit:      10,
	})
	if err != nil {
		t.Fatalf("ListAliases group returned error: %v", err)
	}
	if len(aliases) != 1 ||
		aliases[0].Address != "ops@example.com" ||
		aliases[0].TargetKind != PrincipalKindGroup ||
		aliases[0].TargetPrincipal.ID != seed.teamID ||
		aliases[0].TargetPrincipal.Kind != PrincipalKindGroup {
		t.Fatalf("group aliases = %+v", aliases)
	}

	aliases, err = repo.ListAliases(ctx, ListAliasesRequest{
		CompanyID:  seed.companyID,
		Query:      "old",
		ActiveOnly: true,
		Limit:      10,
	})
	if err != nil {
		t.Fatalf("ListAliases active returned error: %v", err)
	}
	if len(aliases) != 0 {
		t.Fatalf("active alias list returned deleted rows: %+v", aliases)
	}
}

func TestPostgresCreateAliasValidatesDomainTargetAndUniqueness(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	db := openDirectoryPostgresTestDB(t)
	seed := seedDirectoryDelegationGraph(t, db)
	repo := NewRepository(db)

	alias, err := repo.CreateAliasWithAudit(ctx, CreateAliasRequest{
		CompanyID:  seed.companyID,
		DomainID:   seed.domainID,
		Address:    " Ops@Example.COM ",
		TargetKind: PrincipalKindGroup,
		TargetID:   seed.teamID,
	})
	if err != nil {
		t.Fatalf("CreateAlias returned error: %v", err)
	}
	if alias.Address != "ops@example.com" ||
		alias.AddressACE != "ops@example.com" ||
		alias.CompanyID != seed.companyID ||
		alias.DomainID != seed.domainID ||
		alias.TargetKind != PrincipalKindGroup ||
		alias.TargetID != seed.teamID ||
		alias.TargetPrincipal.ID != seed.teamID ||
		alias.TargetPrincipal.Kind != PrincipalKindGroup ||
		alias.Status != "active" {
		t.Fatalf("alias = %+v", alias)
	}

	resolved, err := repo.ResolveAlias(ctx, ResolveAliasRequest{Address: "ops@example.com", ActiveOnly: true})
	if err != nil {
		t.Fatalf("ResolveAlias returned error: %v", err)
	}
	if resolved.ID != alias.ID || resolved.TargetPrincipal.ID != seed.teamID {
		t.Fatalf("resolved alias = %+v, want id %q target %q", resolved, alias.ID, seed.teamID)
	}
	var auditCount int
	if err := db.QueryRowContext(ctx, `
SELECT count(*)
FROM audit_logs
WHERE company_id = $1::uuid
  AND domain_id = $2::uuid
  AND action = 'directory_alias.create'
  AND target_type = 'directory_alias'
  AND target_id = $3
  AND result = 'created'`, seed.companyID, seed.domainID, alias.ID).Scan(&auditCount); err != nil {
		t.Fatalf("query directory alias audit log: %v", err)
	}
	if auditCount != 1 {
		t.Fatalf("directory alias audit rows = %d, want 1", auditCount)
	}
	_, err = repo.CreateAlias(ctx, CreateAliasRequest{
		CompanyID:  seed.companyID,
		DomainID:   seed.domainID,
		Address:    "ops@example.com",
		TargetKind: PrincipalKindResource,
		TargetID:   seed.roomID,
	})
	if !errors.Is(err, ErrAliasAlreadyExists) {
		t.Fatalf("duplicate CreateAlias error = %v, want ErrAliasAlreadyExists", err)
	}
	deleted, err := repo.DeleteAliasWithAudit(ctx, alias.ID)
	if err != nil {
		t.Fatalf("DeleteAliasWithAudit returned error: %v", err)
	}
	if deleted.ID != alias.ID ||
		deleted.Status != "deleted" ||
		deleted.TargetPrincipal.ID != seed.teamID {
		t.Fatalf("deleted alias = %+v", deleted)
	}
	if err := db.QueryRowContext(ctx, `
SELECT count(*)
FROM audit_logs
WHERE company_id = $1::uuid
  AND domain_id = $2::uuid
  AND action = 'directory_alias.delete'
  AND target_type = 'directory_alias'
  AND target_id = $3
  AND result = 'deleted'`, seed.companyID, seed.domainID, alias.ID).Scan(&auditCount); err != nil {
		t.Fatalf("query directory alias delete audit log: %v", err)
	}
	if auditCount != 1 {
		t.Fatalf("directory alias delete audit rows = %d, want 1", auditCount)
	}

	_, err = repo.CreateAlias(ctx, CreateAliasRequest{
		CompanyID:  seed.companyID,
		DomainID:   seed.domainID,
		Address:    "ops@example.net",
		TargetKind: PrincipalKindGroup,
		TargetID:   seed.teamID,
	})
	if err == nil || !strings.Contains(err.Error(), "domain does not match") {
		t.Fatalf("mismatched domain error = %v, want domain rejection", err)
	}

	if _, err := db.ExecContext(ctx, `UPDATE directory_groups SET status = 'suspended' WHERE id = $1::uuid`, seed.teamID); err != nil {
		t.Fatalf("suspend alias target group: %v", err)
	}
	_, err = repo.CreateAlias(ctx, CreateAliasRequest{
		CompanyID:  seed.companyID,
		DomainID:   seed.domainID,
		Address:    "inactive-team@example.com",
		TargetKind: PrincipalKindGroup,
		TargetID:   seed.teamID,
	})
	if err == nil || !strings.Contains(err.Error(), "target") {
		t.Fatalf("inactive target error = %v, want target rejection", err)
	}
}

func TestPostgresListDelegationsFiltersOwnerDelegateAndScope(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	db := openDirectoryPostgresTestDB(t)
	seed := seedDirectoryDelegationGraph(t, db)
	repo := NewRepository(db)

	delegations, err := repo.ListDelegations(ctx, ListDelegationsRequest{
		CompanyID:  seed.companyID,
		OwnerKind:  PrincipalKindResource,
		OwnerID:    seed.roomID,
		Scope:      DelegationScopeCalendar,
		ActiveOnly: true,
		Limit:      10,
	})
	if err != nil {
		t.Fatalf("ListDelegations owner returned error: %v", err)
	}
	if len(delegations) != 2 {
		t.Fatalf("owner delegations = %+v, want 2 rows", delegations)
	}

	delegations, err = repo.ListDelegations(ctx, ListDelegationsRequest{
		CompanyID:    seed.companyID,
		DelegateKind: PrincipalKindGroup,
		DelegateID:   seed.teamID,
		Scope:        DelegationScopeCalendar,
		Role:         DelegationRoleWrite,
		ActiveOnly:   true,
		Limit:        10,
	})
	if err != nil {
		t.Fatalf("ListDelegations delegate returned error: %v", err)
	}
	if len(delegations) != 1 ||
		delegations[0].OwnerKind != PrincipalKindResource ||
		delegations[0].OwnerID != seed.roomID ||
		delegations[0].DelegateKind != PrincipalKindGroup ||
		delegations[0].DelegateID != seed.teamID ||
		delegations[0].Scope != DelegationScopeCalendar ||
		delegations[0].Role != DelegationRoleWrite ||
		delegations[0].Status != "active" {
		t.Fatalf("filtered delegation = %+v", delegations)
	}

	if _, err := db.ExecContext(ctx, `
UPDATE directory_delegations
SET status = 'deleted'
WHERE company_id = $1::uuid
  AND delegate_kind = 'user'
  AND delegate_id = $2::uuid`, seed.companyID, seed.aliceID); err != nil {
		t.Fatalf("delete direct user delegation: %v", err)
	}
	delegations, err = repo.ListDelegations(ctx, ListDelegationsRequest{
		CompanyID:    seed.companyID,
		DelegateKind: PrincipalKindUser,
		DelegateID:   seed.aliceID,
		ActiveOnly:   true,
		Limit:        10,
	})
	if err != nil {
		t.Fatalf("ListDelegations active returned error: %v", err)
	}
	if len(delegations) != 0 {
		t.Fatalf("active delegation list returned deleted rows: %+v", delegations)
	}
}

func TestPostgresCreateDelegationWithAuditValidatesPrincipalsAndUniqueness(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	db := openDirectoryPostgresTestDB(t)
	seed := seedDirectoryDelegationGraph(t, db)
	repo := NewRepository(db)

	delegation, err := repo.CreateDelegationWithAudit(ctx, CreateDelegationRequest{
		CompanyID:    seed.companyID,
		OwnerKind:    PrincipalKindResource,
		OwnerID:      seed.equipmentID,
		DelegateKind: PrincipalKindGroup,
		DelegateID:   seed.deeperID,
		Scope:        DelegationScopeContacts,
		Role:         DelegationRoleManage,
	})
	if err != nil {
		t.Fatalf("CreateDelegationWithAudit returned error: %v", err)
	}
	if delegation.CompanyID != seed.companyID ||
		delegation.OwnerKind != PrincipalKindResource ||
		delegation.OwnerID != seed.equipmentID ||
		delegation.DelegateKind != PrincipalKindGroup ||
		delegation.DelegateID != seed.deeperID ||
		delegation.Scope != DelegationScopeContacts ||
		delegation.Role != DelegationRoleManage ||
		delegation.Status != "active" {
		t.Fatalf("delegation = %+v", delegation)
	}
	ok, err := repo.CheckDelegation(ctx, CheckDelegationRequest{
		CompanyID:    seed.companyID,
		OwnerKind:    PrincipalKindResource,
		OwnerID:      seed.equipmentID,
		DelegateKind: PrincipalKindGroup,
		DelegateID:   seed.deeperID,
		Scope:        DelegationScopeContacts,
		RequiredRole: DelegationRoleWrite,
		ActiveOnly:   true,
	})
	if err != nil {
		t.Fatalf("CheckDelegation returned error: %v", err)
	}
	if !ok {
		t.Fatal("created delegation did not satisfy write role through manage grant")
	}
	var auditCount int
	if err := db.QueryRowContext(ctx, `
SELECT count(*)
FROM audit_logs
WHERE company_id = $1::uuid
  AND action = 'directory_delegation.create'
  AND target_type = 'directory_delegation'
  AND target_id = $2
  AND result = 'created'`, seed.companyID, delegation.ID).Scan(&auditCount); err != nil {
		t.Fatalf("query directory delegation audit log: %v", err)
	}
	if auditCount != 1 {
		t.Fatalf("directory delegation audit rows = %d, want 1", auditCount)
	}
	_, err = repo.CreateDelegationWithAudit(ctx, CreateDelegationRequest{
		CompanyID:    seed.companyID,
		OwnerKind:    PrincipalKindResource,
		OwnerID:      seed.equipmentID,
		DelegateKind: PrincipalKindGroup,
		DelegateID:   seed.deeperID,
		Scope:        DelegationScopeContacts,
		Role:         DelegationRoleManage,
	})
	if !errors.Is(err, ErrDelegationAlreadyExists) {
		t.Fatalf("duplicate CreateDelegationWithAudit error = %v, want ErrDelegationAlreadyExists", err)
	}
	deleted, err := repo.DeleteDelegationWithAudit(ctx, delegation.ID)
	if err != nil {
		t.Fatalf("DeleteDelegationWithAudit returned error: %v", err)
	}
	if deleted.ID != delegation.ID ||
		deleted.Status != "deleted" ||
		deleted.OwnerID != seed.equipmentID ||
		deleted.DelegateID != seed.deeperID {
		t.Fatalf("deleted delegation = %+v", deleted)
	}
	if err := db.QueryRowContext(ctx, `
SELECT count(*)
FROM audit_logs
WHERE company_id = $1::uuid
  AND action = 'directory_delegation.delete'
  AND target_type = 'directory_delegation'
  AND target_id = $2
  AND result = 'deleted'`, seed.companyID, delegation.ID).Scan(&auditCount); err != nil {
		t.Fatalf("query directory delegation delete audit log: %v", err)
	}
	if auditCount != 1 {
		t.Fatalf("directory delegation delete audit rows = %d, want 1", auditCount)
	}

	if _, err := db.ExecContext(ctx, `UPDATE directory_resources SET status = 'suspended' WHERE id = $1::uuid`, seed.equipmentID); err != nil {
		t.Fatalf("suspend delegation owner resource: %v", err)
	}
	_, err = repo.CreateDelegationWithAudit(ctx, CreateDelegationRequest{
		CompanyID:    seed.companyID,
		OwnerKind:    PrincipalKindResource,
		OwnerID:      seed.equipmentID,
		DelegateKind: PrincipalKindUser,
		DelegateID:   seed.aliceID,
		Scope:        DelegationScopeDrive,
		Role:         DelegationRoleRead,
	})
	if err == nil || !strings.Contains(err.Error(), "principals") {
		t.Fatalf("inactive owner error = %v, want principal rejection", err)
	}
}

func TestPostgresCreateGroupMembershipWithAuditValidatesPrincipalsAndUniqueness(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	db := openDirectoryPostgresTestDB(t)
	seed := seedDirectoryDelegationGraph(t, db)
	repo := NewRepository(db)

	membership, err := repo.CreateGroupMembershipWithAudit(ctx, CreateGroupMembershipRequest{
		GroupID:    seed.teamID,
		MemberKind: PrincipalKindUser,
		MemberID:   seed.aliceID,
		Role:       GroupMembershipRoleManager,
	})
	if err != nil {
		t.Fatalf("CreateGroupMembershipWithAudit returned error: %v", err)
	}
	if membership.GroupID != seed.teamID ||
		membership.CompanyID != seed.companyID ||
		membership.MemberKind != PrincipalKindUser ||
		membership.MemberID != seed.aliceID ||
		membership.Role != GroupMembershipRoleManager ||
		membership.Status != "active" {
		t.Fatalf("membership = %+v", membership)
	}
	ok, err := repo.CheckDirectGroupMembership(ctx, CheckGroupMembershipRequest{
		GroupID:    seed.teamID,
		MemberKind: PrincipalKindUser,
		MemberID:   seed.aliceID,
		ActiveOnly: true,
	})
	if err != nil {
		t.Fatalf("CheckDirectGroupMembership returned error: %v", err)
	}
	if !ok {
		t.Fatal("created membership was not visible to direct membership check")
	}
	var auditCount int
	if err := db.QueryRowContext(ctx, `
SELECT count(*)
FROM audit_logs
WHERE company_id = $1::uuid
  AND action = 'directory_group_membership.create'
  AND target_type = 'directory_group_membership'
  AND target_id = $2
  AND result = 'created'`, seed.companyID, membership.ID).Scan(&auditCount); err != nil {
		t.Fatalf("query directory group membership audit log: %v", err)
	}
	if auditCount != 1 {
		t.Fatalf("directory group membership audit rows = %d, want 1", auditCount)
	}

	memberships, err := repo.ListGroupMemberships(ctx, ListGroupMembershipsRequest{
		CompanyID:  seed.companyID,
		GroupID:    seed.teamID,
		MemberKind: PrincipalKindUser,
		MemberID:   seed.aliceID,
		Role:       GroupMembershipRoleManager,
		ActiveOnly: true,
		Limit:      10,
	})
	if err != nil {
		t.Fatalf("ListGroupMemberships returned error: %v", err)
	}
	if len(memberships) != 1 ||
		memberships[0].ID != membership.ID ||
		memberships[0].CompanyID != seed.companyID ||
		memberships[0].GroupID != seed.teamID ||
		memberships[0].MemberKind != PrincipalKindUser ||
		memberships[0].MemberID != seed.aliceID ||
		memberships[0].Role != GroupMembershipRoleManager ||
		memberships[0].Status != "active" {
		t.Fatalf("filtered memberships = %+v", memberships)
	}

	_, err = repo.CreateGroupMembershipWithAudit(ctx, CreateGroupMembershipRequest{
		GroupID:    seed.teamID,
		MemberKind: PrincipalKindUser,
		MemberID:   seed.aliceID,
		Role:       GroupMembershipRoleOwner,
	})
	if !errors.Is(err, ErrGroupMembershipAlreadyExists) {
		t.Fatalf("duplicate CreateGroupMembershipWithAudit error = %v, want ErrGroupMembershipAlreadyExists", err)
	}

	updated, err := repo.UpdateGroupMembershipRoleWithAudit(ctx, UpdateGroupMembershipRoleRequest{
		ID:   membership.ID,
		Role: GroupMembershipRoleOwner,
	})
	if err != nil {
		t.Fatalf("UpdateGroupMembershipRoleWithAudit returned error: %v", err)
	}
	if updated.ID != membership.ID ||
		updated.GroupID != seed.teamID ||
		updated.CompanyID != seed.companyID ||
		updated.MemberKind != PrincipalKindUser ||
		updated.MemberID != seed.aliceID ||
		updated.Role != GroupMembershipRoleOwner ||
		updated.Status != "active" {
		t.Fatalf("updated membership = %+v", updated)
	}
	if err := db.QueryRowContext(ctx, `
SELECT count(*)
FROM audit_logs
WHERE company_id = $1::uuid
  AND action = 'directory_group_membership.role_update'
  AND target_type = 'directory_group_membership'
  AND target_id = $2
  AND result = 'updated'`, seed.companyID, membership.ID).Scan(&auditCount); err != nil {
		t.Fatalf("query directory group membership role update audit log: %v", err)
	}
	if auditCount != 1 {
		t.Fatalf("directory group membership role update audit rows = %d, want 1", auditCount)
	}

	reassigned, err := repo.ReassignGroupMembershipWithAudit(ctx, ReassignGroupMembershipRequest{
		ID:         membership.ID,
		GroupID:    seed.nestedID,
		MemberKind: PrincipalKindUser,
		MemberID:   seed.aliceID,
	})
	if err != nil {
		t.Fatalf("ReassignGroupMembershipWithAudit returned error: %v", err)
	}
	if reassigned.ID != membership.ID ||
		reassigned.GroupID != seed.nestedID ||
		reassigned.CompanyID != seed.companyID ||
		reassigned.MemberKind != PrincipalKindUser ||
		reassigned.MemberID != seed.aliceID ||
		reassigned.Role != GroupMembershipRoleOwner ||
		reassigned.Status != "active" {
		t.Fatalf("reassigned membership = %+v", reassigned)
	}
	if err := db.QueryRowContext(ctx, `
SELECT count(*)
FROM audit_logs
WHERE company_id = $1::uuid
  AND action = 'directory_group_membership.reassign'
  AND target_type = 'directory_group_membership'
  AND target_id = $2
  AND result = 'updated'`, seed.companyID, membership.ID).Scan(&auditCount); err != nil {
		t.Fatalf("query directory group membership reassign audit log: %v", err)
	}
	if auditCount != 1 {
		t.Fatalf("directory group membership reassign audit rows = %d, want 1", auditCount)
	}

	deleted, err := repo.DeleteGroupMembershipWithAudit(ctx, membership.ID)
	if err != nil {
		t.Fatalf("DeleteGroupMembershipWithAudit returned error: %v", err)
	}
	if deleted.ID != membership.ID ||
		deleted.GroupID != seed.nestedID ||
		deleted.CompanyID != seed.companyID ||
		deleted.MemberKind != PrincipalKindUser ||
		deleted.MemberID != seed.aliceID ||
		deleted.Role != GroupMembershipRoleOwner ||
		deleted.Status != "deleted" {
		t.Fatalf("deleted membership = %+v", deleted)
	}
	if err := db.QueryRowContext(ctx, `
SELECT count(*)
FROM audit_logs
WHERE company_id = $1::uuid
  AND action = 'directory_group_membership.delete'
  AND target_type = 'directory_group_membership'
  AND target_id = $2
  AND result = 'deleted'`, seed.companyID, membership.ID).Scan(&auditCount); err != nil {
		t.Fatalf("query directory group membership delete audit log: %v", err)
	}
	if auditCount != 1 {
		t.Fatalf("directory group membership delete audit rows = %d, want 1", auditCount)
	}
	memberships, err = repo.ListGroupMemberships(ctx, ListGroupMembershipsRequest{
		CompanyID:  seed.companyID,
		GroupID:    seed.nestedID,
		MemberKind: PrincipalKindUser,
		MemberID:   seed.aliceID,
		ActiveOnly: true,
		Limit:      10,
	})
	if err != nil {
		t.Fatalf("ListGroupMemberships active returned error: %v", err)
	}
	if len(memberships) != 0 {
		t.Fatalf("active group membership list returned deleted rows: %+v", memberships)
	}

	_, err = repo.CreateGroupMembershipWithAudit(ctx, CreateGroupMembershipRequest{
		GroupID:    seed.deeperID,
		MemberKind: PrincipalKindGroup,
		MemberID:   seed.teamID,
	})
	if err == nil || !strings.Contains(err.Error(), "cycle") {
		t.Fatalf("cycle membership error = %v, want cycle rejection", err)
	}
}

type directoryDelegationSeed struct {
	companyID   string
	domainID    string
	aliceID     string
	bobID       string
	orgID       string
	teamID      string
	nestedID    string
	deeperID    string
	roomID      string
	equipmentID string
}

func principalIDsByKind(principals []Principal, kind string) []string {
	ids := make([]string, 0, len(principals))
	for _, principal := range principals {
		if principal.Kind == kind {
			ids = append(ids, principal.ID)
		}
	}
	return ids
}

func sameStringSet(got []string, want []string) bool {
	if len(got) != len(want) {
		return false
	}
	seen := make(map[string]int, len(got))
	for _, value := range got {
		seen[value]++
	}
	for _, value := range want {
		if seen[value] == 0 {
			return false
		}
		seen[value]--
	}
	return true
}

func seedDirectoryDelegationGraph(t *testing.T, db *sql.DB) directoryDelegationSeed {
	t.Helper()

	var seed directoryDelegationSeed
	if err := db.QueryRowContext(context.Background(), `
WITH company AS (
  INSERT INTO companies (name) VALUES ('Directory Delegation Co') RETURNING id
), domain AS (
  INSERT INTO domains (company_id, name, name_ace)
  SELECT id, 'example.com', 'example.com' FROM company RETURNING id, company_id
), alice AS (
  INSERT INTO users (domain_id, username, display_name)
  SELECT id, 'alice', 'Alice' FROM domain RETURNING id
), bob AS (
  INSERT INTO users (domain_id, username, display_name)
  SELECT id, 'bob', 'Bob' FROM domain RETURNING id
), org AS (
  INSERT INTO organizations (domain_id, name, code)
  SELECT id, 'Research', 'research' FROM domain RETURNING id
), team AS (
  INSERT INTO directory_groups (company_id, domain_id, name, slug)
  SELECT company_id, id, 'Team Calendar', 'team-calendar' FROM domain RETURNING id
), nested AS (
  INSERT INTO directory_groups (company_id, domain_id, name, slug)
  SELECT company_id, id, 'Nested Calendar', 'nested-calendar' FROM domain RETURNING id
), deeper AS (
  INSERT INTO directory_groups (company_id, domain_id, name, slug)
  SELECT company_id, id, 'Deeper Calendar', 'deeper-calendar' FROM domain RETURNING id
), room AS (
  INSERT INTO directory_resources (company_id, domain_id, resource_type, name, slug)
  SELECT company_id, id, 'room', 'Room One', 'room-one' FROM domain RETURNING id
), equipment AS (
  INSERT INTO directory_resources (company_id, domain_id, resource_type, name, slug)
  SELECT company_id, id, 'equipment', 'Projector One', 'projector-one' FROM domain RETURNING id
), direct_delegation AS (
  INSERT INTO directory_delegations (company_id, owner_kind, owner_id, delegate_kind, delegate_id, scope, role)
  SELECT domain.company_id, 'resource', room.id, 'user', alice.id, 'calendar', 'read'
  FROM domain, room, alice
), group_delegation AS (
  INSERT INTO directory_delegations (company_id, owner_kind, owner_id, delegate_kind, delegate_id, scope, role)
  SELECT domain.company_id, 'resource', room.id, 'group', team.id, 'calendar', 'write'
  FROM domain, room, team
), team_membership AS (
  INSERT INTO directory_group_memberships (group_id, member_kind, member_id)
  SELECT team.id, 'group', nested.id FROM team, nested
), nested_membership AS (
  INSERT INTO directory_group_memberships (group_id, member_kind, member_id)
  SELECT nested.id, 'group', deeper.id FROM nested, deeper
), deeper_membership AS (
  INSERT INTO directory_group_memberships (group_id, member_kind, member_id)
  SELECT deeper.id, 'user', bob.id FROM deeper, bob
), org_membership AS (
  INSERT INTO directory_group_memberships (group_id, member_kind, member_id)
  SELECT deeper.id, 'organization', org.id FROM deeper, org
), resource_membership AS (
  INSERT INTO directory_group_memberships (group_id, member_kind, member_id)
  SELECT deeper.id, 'resource', equipment.id FROM deeper, equipment
)
SELECT
  domain.company_id::text,
  domain.id::text,
  alice.id::text,
  bob.id::text,
  org.id::text,
  team.id::text,
  nested.id::text,
  deeper.id::text,
  room.id::text,
  equipment.id::text
FROM domain, alice, bob, org, team, nested, deeper, room, equipment`).Scan(
		&seed.companyID,
		&seed.domainID,
		&seed.aliceID,
		&seed.bobID,
		&seed.orgID,
		&seed.teamID,
		&seed.nestedID,
		&seed.deeperID,
		&seed.roomID,
		&seed.equipmentID,
	); err != nil {
		t.Fatalf("seed directory delegation graph: %v", err)
	}
	return seed
}

func openDirectoryPostgresTestDB(t *testing.T) *sql.DB {
	t.Helper()

	baseURL := strings.TrimSpace(os.Getenv("GOGOMAIL_TEST_DATABASE_URL"))
	if baseURL == "" {
		t.Skip("set GOGOMAIL_TEST_DATABASE_URL to run PostgreSQL directory integration tests")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	t.Cleanup(cancel)

	adminDB, err := sql.Open("pgx", baseURL)
	if err != nil {
		t.Fatalf("open postgres admin connection: %v", err)
	}
	t.Cleanup(func() { _ = adminDB.Close() })

	schema := fmt.Sprintf("gogomail_directory_test_%d", time.Now().UnixNano())
	if _, err := adminDB.ExecContext(ctx, `CREATE SCHEMA `+schema); err != nil {
		t.Fatalf("create test schema: %v", err)
	}
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cleanupCancel()
		_, _ = adminDB.ExecContext(cleanupCtx, `DROP SCHEMA IF EXISTS `+schema+` CASCADE`)
	})

	dbURL := directoryPostgresURLWithSearchPath(t, baseURL, schema)
	db, err := database.Open(ctx, dbURL)
	if err != nil {
		t.Fatalf("open postgres test database: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	migrationDir, err := filepath.Abs(filepath.Join("..", "..", "migrations"))
	if err != nil {
		t.Fatalf("resolve migration directory: %v", err)
	}
	if err := database.MigrateUp(ctx, db, migrationDir); err != nil {
		t.Fatalf("migrate postgres test database: %v", err)
	}
	return db
}

func directoryPostgresURLWithSearchPath(t *testing.T, rawURL string, schema string) string {
	t.Helper()

	parsed, err := url.Parse(rawURL)
	if err != nil {
		t.Fatalf("parse GOGOMAIL_TEST_DATABASE_URL: %v", err)
	}
	query := parsed.Query()
	options := strings.TrimSpace(query.Get("options"))
	searchPathOption := "-c search_path=" + schema + ",public"
	if options != "" {
		options += " "
	}
	options += searchPathOption
	query.Set("options", options)
	parsed.RawQuery = query.Encode()
	return parsed.String()
}
