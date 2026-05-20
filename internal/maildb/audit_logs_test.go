package maildb

import (
	"strings"
	"testing"
)

func TestAuditLogIntegrityQueryProjectsRecentColumns(t *testing.T) {
	t.Parallel()

	query, _ := auditLogIntegrityQuery(normalizeAuditLogIntegrityRequest(AuditLogIntegrityRequest{Limit: 10}))
	for _, want := range []string{
		"FROM (\n  SELECT\n    id,\n    company_id,\n    domain_id,\n    user_id,\n    actor_id",
		"ORDER BY created_at DESC, id DESC",
		"ORDER BY created_at ASC, id ASC",
	} {
		if !strings.Contains(query, want) {
			t.Fatalf("audit integrity query missing %q:\n%s", want, query)
		}
	}
	if strings.Contains(query, "SELECT *") {
		t.Fatalf("audit integrity query still projects every audit log column:\n%s", query)
	}
}
