package maildb

import (
	"strings"
	"testing"
)

func TestQuotaReconciliationScopedQueriesUseDirectPredicates(t *testing.T) {
	tests := map[string]struct {
		query string
		want  []string
	}{
		"list company": {
			query: quotaReconciliationFilteredQuery("company"),
			want:  []string{"WHERE company_id = $1"},
		},
		"list domain": {
			query: quotaReconciliationFilteredQuery("domain"),
			want:  []string{"WHERE domain_id = $1"},
		},
		"list user": {
			query: quotaReconciliationFilteredQuery("user"),
			want:  []string{"WHERE scope = 'user' AND id = $1"},
		},
		"update users domain": {
			query: quotaCorrectionUpdateUsersSQL("domain"),
			want:  []string{"AND user_actual.domain_id = $1::uuid"},
		},
		"update domains user": {
			query: quotaCorrectionUpdateDomainsSQL("user"),
			want:  []string{"EXISTS (SELECT 1 FROM users u WHERE u.id = $1::uuid AND u.domain_id = d.id)"},
		},
		"update companies user": {
			query: quotaCorrectionUpdateCompaniesSQL("user"),
			want:  []string{"WHERE u.id = $1::uuid AND d.company_id = c.id"},
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			for _, forbidden := range []string{
				"$1 = '' OR",
				"AND ($1 = '' OR",
				"` = '",
				"::text = $1",
			} {
				if strings.Contains(tt.query, forbidden) {
					t.Fatalf("query contains optional scope OR %q:\n%s", forbidden, tt.query)
				}
			}
			for _, want := range tt.want {
				if !strings.Contains(tt.query, want) {
					t.Fatalf("query missing %q:\n%s", want, tt.query)
				}
			}
		})
	}
}

func TestQuotaReconciliationAllQueriesOmitScopeFilters(t *testing.T) {
	queries := map[string]string{
		"list":             quotaReconciliationFilteredQuery("all"),
		"update users":     quotaCorrectionUpdateUsersSQL("all"),
		"update domains":   quotaCorrectionUpdateDomainsSQL("all"),
		"update companies": quotaCorrectionUpdateCompaniesSQL("all"),
	}

	for name, query := range queries {
		t.Run(name, func(t *testing.T) {
			if strings.Contains(query, "$1 = '' OR") || strings.Contains(query, " = $1") || strings.Contains(query, "::text = $1") {
				t.Fatalf("all-scope query unexpectedly includes scoped predicate:\n%s", query)
			}
		})
	}
}
