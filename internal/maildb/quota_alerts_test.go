package maildb

import (
	"strings"
	"testing"
	"time"
)

func TestQuotaAlertThresholdListSQLUsesStableSargableOrdering(t *testing.T) {
	t.Parallel()

	query, args := buildQuotaAlertThresholdListSQL(QuotaAlertThresholdListRequest{
		CompanyID: "11111111-1111-1111-1111-111111111111",
		Scope:     string(QuotaAlertScopeUser),
		Limit:     50,
	})
	for _, want := range []string{
		"company_id = $1::uuid",
		"scope = $2",
		"ORDER BY created_at DESC, id DESC",
		"LIMIT $3",
	} {
		if !strings.Contains(query, want) {
			t.Fatalf("threshold list query missing %q:\n%s", want, query)
		}
	}
	for _, forbidden := range []string{"company_id::text =", "scope_id::text ="} {
		if strings.Contains(query, forbidden) {
			t.Fatalf("threshold list query should not cast indexed columns to text:\n%s", query)
		}
	}
	if len(args) != 3 {
		t.Fatalf("args length = %d, want 3", len(args))
	}
}

func TestQuotaAlertListSQLUsesStableSargableOrdering(t *testing.T) {
	t.Parallel()

	query, args := buildQuotaAlertListSQL(QuotaAlertListRequest{
		CompanyID: "11111111-1111-1111-1111-111111111111",
		DomainID:  "22222222-2222-2222-2222-222222222222",
		UserID:    "33333333-3333-3333-3333-333333333333",
		Scope:     string(QuotaAlertScopeUser),
		AlertType: string(QuotaAlertTypeCritical),
		Since:     time.Date(2026, 5, 21, 0, 0, 0, 0, time.UTC),
		Until:     time.Date(2026, 5, 22, 0, 0, 0, 0, time.UTC),
		Limit:     100,
	})
	for _, want := range []string{
		"company_id = $1::uuid",
		"domain_id = $2::uuid",
		"user_id = $3::uuid",
		"scope = $4",
		"alert_type = $5",
		"created_at >= $6",
		"created_at <= $7",
		"ORDER BY created_at DESC, id DESC",
		"LIMIT $8",
	} {
		if !strings.Contains(query, want) {
			t.Fatalf("alert list query missing %q:\n%s", want, query)
		}
	}
	for _, forbidden := range []string{"company_id::text =", "domain_id::text =", "user_id::text ="} {
		if strings.Contains(query, forbidden) {
			t.Fatalf("alert list query should not cast indexed columns to text:\n%s", query)
		}
	}
	if len(args) != 8 {
		t.Fatalf("args length = %d, want 8", len(args))
	}
}

func TestQuotaAlertThresholdScopeSQLUsesStableSargableOrdering(t *testing.T) {
	t.Parallel()

	query, args := buildQuotaAlertThresholdsForScopeSQL(
		"11111111-1111-1111-1111-111111111111",
		QuotaAlertScopeDomain,
		"22222222-2222-2222-2222-222222222222",
	)
	for _, want := range []string{
		"company_id = $1::uuid",
		"scope = $2",
		"scope_id = $3::uuid",
		"ORDER BY created_at DESC, id DESC",
	} {
		if !strings.Contains(query, want) {
			t.Fatalf("scope threshold query missing %q:\n%s", want, query)
		}
	}
	if strings.Contains(query, "scope_id::text =") {
		t.Fatalf("scope threshold query should not cast indexed columns to text:\n%s", query)
	}
	if len(args) != 3 {
		t.Fatalf("args length = %d, want 3", len(args))
	}
}

func TestQuotaAlertSentSQLUsesScopeSpecificSargablePredicate(t *testing.T) {
	t.Parallel()

	query, args := buildQuotaAlertSentSQL(
		"11111111-1111-1111-1111-111111111111",
		QuotaAlertScopeDomain,
		"22222222-2222-2222-2222-222222222222",
		QuotaAlertTypeWarning,
		24*time.Hour,
	)
	for _, want := range []string{
		"company_id = $1::uuid",
		"scope = $2",
		"alert_type = $3",
		"created_at >= $4",
		"AND domain_id = $5::uuid",
	} {
		if !strings.Contains(query, want) {
			t.Fatalf("sent-check query missing %q:\n%s", want, query)
		}
	}
	for _, forbidden := range []string{"company_id::text =", "domain_id::text =", "user_id::text =", "AND user_id = $4"} {
		if strings.Contains(query, forbidden) {
			t.Fatalf("sent-check query should use the requested scope's indexed column:\n%s", query)
		}
	}
	if len(args) != 5 {
		t.Fatalf("args length = %d, want 5", len(args))
	}
}
