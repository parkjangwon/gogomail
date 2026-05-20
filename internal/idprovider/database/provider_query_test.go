package database

import (
	"strings"
	"testing"

	"github.com/gogomail/gogomail/internal/idprovider"
)

func TestBuildListGroupsQueryUsesParameterizedSearchAndOffset(t *testing.T) {
	orgID := "org-1"
	search := " Ops "
	query, args := buildListGroupsQuery(&idprovider.GroupFilter{
		OrgID:       &orgID,
		SearchQuery: &search,
		Limit:       25,
		Offset:      50,
	})

	for _, want := range []string{
		"FROM directory_groups WHERE status = 'active'",
		"AND org_id = $1",
		"AND (lower(name) LIKE $2 OR lower(slug) LIKE $2 OR lower(description) LIKE $2)",
		"ORDER BY lower(name), id LIMIT $3 OFFSET $4",
	} {
		if !strings.Contains(query, want) {
			t.Fatalf("query missing %q:\n%s", want, query)
		}
	}
	wantArgs := []any{orgID, "%ops%", 25, 50}
	if len(args) != len(wantArgs) {
		t.Fatalf("args len = %d, want %d (%#v)", len(args), len(wantArgs), args)
	}
	for i := range wantArgs {
		if args[i] != wantArgs[i] {
			t.Fatalf("args[%d] = %#v, want %#v (all args %#v)", i, args[i], wantArgs[i], args)
		}
	}
}

func TestBuildListGroupsQueryDefaultsLimitAndOmitsOffset(t *testing.T) {
	query, args := buildListGroupsQuery(nil)
	if !strings.Contains(query, "ORDER BY lower(name), id LIMIT $1") {
		t.Fatalf("query missing deterministic default limit:\n%s", query)
	}
	if strings.Contains(query, "OFFSET") {
		t.Fatalf("query unexpectedly includes OFFSET:\n%s", query)
	}
	if len(args) != 1 || args[0] != 100 {
		t.Fatalf("args = %#v, want default limit only", args)
	}
}
