package directory

import (
	"context"
	"database/sql"
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
