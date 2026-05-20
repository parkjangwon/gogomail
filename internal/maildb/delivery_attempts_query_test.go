package maildb

import (
	"strings"
	"testing"
	"time"
)

func TestListDeliveryAttemptsQueryUsesSargableFilters(t *testing.T) {
	t.Parallel()

	query, args := buildListDeliveryAttemptsQuery(deliveryAttemptFilters{
		Status:          "failed",
		RecipientDomain: "example.com",
		MessageID:       "123e4567-e89b-12d3-a456-426614174000",
		Farm:            "mx-1",
		Sender:          "sender@example.com",
	}, time.Date(2026, 5, 21, 1, 2, 3, 0, time.FixedZone("KST", 9*60*60)), 201)

	for _, want := range []string{
		"WHERE status = $1",
		"AND attempted_at >= $2",
		"AND recipient_domain = $3",
		"AND message_id::text = $4",
		"AND farm = $5",
		"AND lower(sender) = $6",
		"ORDER BY attempted_at DESC, id DESC",
		"LIMIT $7",
	} {
		if !strings.Contains(query, want) {
			t.Fatalf("list delivery attempts query missing %q:\n%s", want, query)
		}
	}
	for _, forbidden := range []string{
		"NULLIF",
		" OR ",
		"::timestamptz IS NULL",
	} {
		if strings.Contains(query, forbidden) {
			t.Fatalf("list delivery attempts query contains non-sargable filter %q:\n%s", forbidden, query)
		}
	}
	if len(args) != 7 {
		t.Fatalf("args len = %d, want 7", len(args))
	}
	if got := args[1].(time.Time).Location(); got != time.UTC {
		t.Fatalf("since arg location = %v, want UTC", got)
	}
}
