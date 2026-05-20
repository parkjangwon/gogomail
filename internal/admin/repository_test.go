package admin

import (
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestBuildAuditLogListQueriesKeepsCountAndRowsFiltersAligned(t *testing.T) {
	t.Parallel()

	start := time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2026, 5, 2, 0, 0, 0, 0, time.UTC)
	filter := AuditLogFilter{
		CompanyID:    "company-1",
		AdminUserID:  "admin-1",
		Action:       "update",
		ResourceType: "domain",
		StartTime:    &start,
		EndTime:      &end,
		Limit:        50,
		Offset:       100,
	}

	countQuery, countArgs, listQuery, listArgs := buildAuditLogListQueries(filter)

	wantPredicates := []string{
		"company_id = $1",
		"admin_user_id = $2",
		"action = $3",
		"resource_type = $4",
		"timestamp >= $5",
		"timestamp <= $6",
	}
	for _, predicate := range wantPredicates {
		if !strings.Contains(countQuery, predicate) {
			t.Fatalf("count query missing %q:\n%s", predicate, countQuery)
		}
		if !strings.Contains(listQuery, predicate) {
			t.Fatalf("list query missing %q:\n%s", predicate, listQuery)
		}
	}
	if !strings.Contains(listQuery, "ORDER BY timestamp DESC, id DESC LIMIT $7 OFFSET $8") {
		t.Fatalf("list query missing stable order and pagination placeholders:\n%s", listQuery)
	}

	wantCountArgs := []interface{}{"company-1", "admin-1", "update", "domain", start, end}
	if !reflect.DeepEqual(countArgs, wantCountArgs) {
		t.Fatalf("countArgs = %#v, want %#v", countArgs, wantCountArgs)
	}
	wantListArgs := append(append([]interface{}(nil), wantCountArgs...), 50, 100)
	if !reflect.DeepEqual(listArgs, wantListArgs) {
		t.Fatalf("listArgs = %#v, want %#v", listArgs, wantListArgs)
	}
}

func TestBuildAuditLogListQueriesOmitsEmptyOptionalPredicates(t *testing.T) {
	t.Parallel()

	countQuery, countArgs, listQuery, listArgs := buildAuditLogListQueries(AuditLogFilter{
		CompanyID: "company-1",
		Limit:     25,
		Offset:    0,
	})

	if strings.Contains(countQuery, " AND admin_user_id") || strings.Contains(countQuery, " AND resource_type") || strings.Contains(countQuery, "timestamp >=") {
		t.Fatalf("count query contains empty optional predicate:\n%s", countQuery)
	}
	if strings.Contains(listQuery, " AND admin_user_id") || strings.Contains(listQuery, " AND resource_type") || strings.Contains(listQuery, "timestamp >=") {
		t.Fatalf("list query contains empty optional predicate:\n%s", listQuery)
	}
	if !strings.Contains(listQuery, "LIMIT $2 OFFSET $3") {
		t.Fatalf("list query pagination placeholders drifted:\n%s", listQuery)
	}
	if !reflect.DeepEqual(countArgs, []interface{}{"company-1"}) {
		t.Fatalf("countArgs = %#v", countArgs)
	}
	if !reflect.DeepEqual(listArgs, []interface{}{"company-1", 25, 0}) {
		t.Fatalf("listArgs = %#v", listArgs)
	}
}
