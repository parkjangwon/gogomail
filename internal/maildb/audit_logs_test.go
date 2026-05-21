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

func TestAuditLogListQueryUsesTypedUUIDFilters(t *testing.T) {
	t.Parallel()

	query, args := buildAuditLogListQuery(normalizeAuditLogListRequest(AuditLogListRequest{
		Category:   "admin",
		Action:     "user.updated",
		Result:     "success",
		TargetType: "user",
		CompanyID:  "11111111-1111-1111-1111-111111111111",
		DomainID:   "22222222-2222-2222-2222-222222222222",
		UserID:     "33333333-3333-3333-3333-333333333333",
		ActorID:    "44444444-4444-4444-4444-444444444444",
		TargetID:   "55555555-5555-5555-5555-555555555555",
		Limit:      25,
	}))
	for _, want := range []string{
		"category = $1",
		"action = $2",
		"result = $3",
		"target_type = $4",
		"company_id = $5::uuid",
		"domain_id = $6::uuid",
		"user_id = $7::uuid",
		"actor_id = $8::uuid",
		"target_id = $9::uuid",
		"ORDER BY created_at DESC, id DESC",
		"LIMIT $10",
	} {
		if !strings.Contains(query, want) {
			t.Fatalf("audit list query missing %q:\n%s", want, query)
		}
	}
	for _, forbidden := range []string{
		"company_id::text =",
		"domain_id::text =",
		"user_id::text =",
		"actor_id::text =",
		"target_id::text =",
	} {
		if strings.Contains(query, forbidden) {
			t.Fatalf("audit list query casts indexed UUID column in predicate: %s\n%s", forbidden, query)
		}
	}
	if len(args) != 10 {
		t.Fatalf("args length = %d, want 10", len(args))
	}
}
