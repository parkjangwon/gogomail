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
		"AND message_id = $4::uuid",
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
		"message_id::text =",
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

func TestDeliveryAttemptStatsQueryUsesSargableFilters(t *testing.T) {
	t.Parallel()

	query, args := buildDeliveryAttemptStatsQuery(deliveryAttemptFilters{
		Status:          "bounced",
		RecipientDomain: "example.net",
		MessageID:       "123e4567-e89b-12d3-a456-426614174001",
		Farm:            "mx-2",
		Sender:          "sender@example.net",
	}, time.Date(2026, 5, 21, 4, 5, 6, 0, time.FixedZone("KST", 9*60*60)))

	for _, want := range []string{
		"FROM delivery_attempts",
		"WHERE status = $1",
		"AND attempted_at >= $2",
		"AND recipient_domain = $3",
		"AND message_id = $4::uuid",
		"AND farm = $5",
		"AND lower(sender) = $6",
	} {
		if !strings.Contains(query, want) {
			t.Fatalf("delivery attempt stats query missing %q:\n%s", want, query)
		}
	}
	for _, forbidden := range []string{
		"NULLIF",
		" OR ",
		"::timestamptz IS NULL",
		"message_id::text =",
	} {
		if strings.Contains(query, forbidden) {
			t.Fatalf("delivery attempt stats query contains non-sargable filter %q:\n%s", forbidden, query)
		}
	}
	if len(args) != 6 {
		t.Fatalf("args len = %d, want 6", len(args))
	}
	if got := args[1].(time.Time).Location(); got != time.UTC {
		t.Fatalf("since arg location = %v, want UTC", got)
	}
}

func TestExhaustedDeliveryAttemptQueryUsesStatusPredicate(t *testing.T) {
	t.Parallel()

	query, args := buildListDeliveryAttemptsQuery(deliveryAttemptFilters{
		Status:          "exhausted",
		RecipientDomain: "example.org",
	}, time.Time{}, 50)

	for _, want := range []string{
		"WHERE status = $1",
		"AND recipient_domain = $2",
		"LIMIT $3",
	} {
		if !strings.Contains(query, want) {
			t.Fatalf("exhausted delivery attempt query missing %q:\n%s", want, query)
		}
	}
	for _, forbidden := range []string{
		"NULLIF",
		" OR ",
		"::timestamptz IS NULL",
	} {
		if strings.Contains(query, forbidden) {
			t.Fatalf("exhausted delivery attempt query contains non-sargable filter %q:\n%s", forbidden, query)
		}
	}
	if len(args) != 3 {
		t.Fatalf("args len = %d, want 3", len(args))
	}
}
