package admin

import (
	"strings"
	"testing"
)

func TestAdminUserRoleActiveQueriesAvoidExpiryOptionalOR(t *testing.T) {
	queries := map[string]string{
		"list role summaries": listRoleSummariesQuery,
		"get user roles":      getUserRolesQuery,
		"list roles for user": listRolesForUserQuery,
	}

	for name, query := range queries {
		t.Run(name, func(t *testing.T) {
			if strings.Contains(query, "expires_at IS NULL OR") {
				t.Fatalf("query still uses nullable expiry optional OR:\n%s", query)
			}
			if !strings.Contains(query, "UNION ALL") {
				t.Fatalf("query should split permanent and expiring active role assignments:\n%s", query)
			}
			if !strings.Contains(query, "expires_at IS NULL") || !strings.Contains(query, "expires_at > NOW()") {
				t.Fatalf("query should preserve permanent and future-expiring active role semantics:\n%s", query)
			}
		})
	}
}
