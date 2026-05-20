package maildb

import (
	"strings"
	"testing"
)

func TestAdminOperationalListQueriesUseSargableFilters(t *testing.T) {
	t.Parallel()

	suppressionQuery, suppressionArgs := buildListSuppressionEntriesQuery(SuppressionEntryListRequest{
		DomainID: "11111111-1111-1111-1111-111111111111",
		Email:    "user@example.net",
		Reason:   "hard_bounce",
	}, 51)
	trustedRelayQuery, trustedRelayArgs := buildListTrustedRelaysQuery("192.0.2.0/24", "edge", 52)
	deliveryRouteQuery, deliveryRouteArgs := buildListDeliveryRoutesQuery(DeliveryRouteListRequest{
		Status:        "active",
		Farm:          "transactional",
		DomainPattern: "*.example.net",
	}, 53)
	adminUsersQuery, adminUsersArgs := buildListAdminUsersQuery("22222222-2222-2222-2222-222222222222", 54)

	tests := []struct {
		name      string
		query     string
		args      []any
		want      []string
		forbidden []string
	}{
		{
			name:  "suppression entries",
			query: suppressionQuery,
			args:  suppressionArgs,
			want: []string{
				"WHERE domain_id = $1::uuid",
				"AND email = $2",
				"AND reason = $3",
				"LIMIT $4",
			},
			forbidden: []string{"$1 = '' OR", "domain_id::text =", " OR "},
		},
		{
			name:  "trusted relays",
			query: trustedRelayQuery,
			args:  trustedRelayArgs,
			want: []string{
				"WHERE cidr = $1::cidr",
				"AND description ILIKE '%' || $2 || '%'",
				"LIMIT $3",
			},
			forbidden: []string{"$1 = '' OR", "$2 = '' OR", " OR "},
		},
		{
			name:  "delivery routes",
			query: deliveryRouteQuery,
			args:  deliveryRouteArgs,
			want: []string{
				"WHERE status = $1",
				"AND farm = $2",
				"AND domain_pattern = $3",
				"LIMIT $4",
			},
			forbidden: []string{"$1 = '' OR", "$2 = '' OR", "$3 = '' OR", " OR "},
		},
		{
			name:  "admin users",
			query: adminUsersQuery,
			args:  adminUsersArgs,
			want: []string{
				"WHERE u.role IN ('system_admin', 'company_admin')",
				"AND d.company_id = $1::uuid",
				"LIMIT $2",
			},
			forbidden: []string{"$1 = '' OR", "d.company_id::text =", " OR "},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			for _, want := range tt.want {
				if !strings.Contains(tt.query, want) {
					t.Fatalf("query missing %q:\n%s", want, tt.query)
				}
			}
			for _, forbidden := range tt.forbidden {
				if strings.Contains(tt.query, forbidden) {
					t.Fatalf("query contains non-sargable filter %q:\n%s", forbidden, tt.query)
				}
			}
			if len(tt.args) == 0 {
				t.Fatalf("args must include limit")
			}
		})
	}
}
